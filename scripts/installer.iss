; wt Inno Setup installer script
; Build with: iscc scripts\installer.iss
; Expects files staged in dist\ by `task build`
;
; Silent install: wt-setup.exe /VERYSILENT /SUPPRESSMSGBOXES /SP-
; All post-install logic runs natively in [Code] — no PowerShell dependency.

#define MyAppName "wt"
#define MyAppVersion "0.0.3"
#define MyAppPublisher "Andrius Solopovas"
#define MyAppURL "https://github.com/asolopovas/wt"
#define MyAppExeName "wt.exe"
#define MyAppCopyright "Copyright (c) 2026 Andrius Solopovas"

[Setup]
AppId={{A7F3E2C1-9B4D-4A8E-8C5F-2D1E3A4B6C7D}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}/issues
AppCopyright={#MyAppCopyright}
DefaultDirName={localappdata}\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
DisableWelcomePage=yes
DisableDirPage=auto
OutputDir=..\dist
OutputBaseFilename=wt-setup-{#MyAppVersion}
Compression=lzma2/ultra64
SolidCompression=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
ChangesEnvironment=yes
PrivilegesRequired=lowest
SetupIconFile=app.ico
UninstallDisplayIcon={app}\{#MyAppExeName}
WizardStyle=modern
MinVersion=10.0
SetupLogging=yes
LicenseFile=..\LICENSE

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Files]
Source: "..\dist\bin\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist\bin\wt-gui.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist\bin\*.dll"; DestDir: "{app}"; Flags: ignoreversion skipifsourcedoesntexist
Source: "..\dist\bin\diarize.py"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist\deps\uv.exe"; DestDir: "{app}\deps"; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\THIRD-PARTY-LICENSES.txt"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\WTranscribe"; Filename: "{app}\wt-gui.exe"
Name: "{group}\Uninstall {#MyAppName}"; Filename: "{uninstallexe}"

[Run]
Filename: "{app}\wt-gui.exe"; Description: "Launch WTranscribe"; \
    Flags: nowait postinstall skipifsilent

[Registry]
Root: HKCU; Subkey: "Environment"; \
    ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}"; \
    Check: NeedsAddPath('{app}')

[UninstallDelete]
Type: files; Name: "{app}\diarize.py"

[Code]
const
  ModelUrlBase = 'https://huggingface.co/ggerganov/whisper.cpp/resolve/main/';
  RequiredCudaVersion = '12.9';

var
  CfgModel, CfgLanguage, CfgDevice, CfgThreads: string;
  CfgSpeakers, CfgNoDiarize: string;
  SetupLogPath: string;
  OutputMemo: TNewMemo;
  OverallProgress: TNewProgressBar;
  TotalSteps, CurrentStep: Integer;

procedure InitLog();
var LogDir: string;
begin
  LogDir := ExpandConstant('{%APPDATA}\wt');
  ForceDirectories(LogDir);
  SetupLogPath := LogDir + '\setup.log';
end;

procedure Log(const Msg: string);
var Line: string;
begin
  if SetupLogPath = '' then InitLog();
  Line := '[' + GetDateTimeString('yyyy-mm-dd hh:nn:ss', '-', ':') + '] ' + Msg;
  SaveStringToFile(SetupLogPath, Line + #13#10, True);
end;

procedure LogOk(const Msg: string);    begin Log('  OK ' + Msg); end;
procedure LogStep(const Msg: string);  begin Log('  -> ' + Msg); end;
procedure LogWarn(const Msg: string);  begin Log('  WARN: ' + Msg); end;
procedure LogError(const Msg: string); begin Log('  ERROR: ' + Msg); end;

procedure MemoLog(const Msg: string);
begin
  if Assigned(OutputMemo) then
  begin
    OutputMemo.Lines.Add(Msg);
    SendMessage(OutputMemo.Handle, $00B6 {EM_LINESCROLL}, 0, OutputMemo.Lines.Count);
  end;
  WizardForm.Refresh;
end;

procedure SetStepStatus(const Msg: string);
begin
  WizardForm.StatusLabel.Caption := Msg;
  MemoLog('');
  MemoLog('--- ' + Msg + ' ---');
  WizardForm.Refresh;
end;

procedure AdvanceProgress();
begin
  CurrentStep := CurrentStep + 1;
  if Assigned(OverallProgress) then begin
    OverallProgress.Position := CurrentStep;
    WizardForm.Refresh;
  end;
end;

procedure StreamLogLine(const S: String; const Error, FirstLine: Boolean);
var Prefix: string;
begin
  if Error then Prefix := '  [!] ' else Prefix := '      ';
  Log('    ' + S);
  if Assigned(OutputMemo) then
  begin
    OutputMemo.Lines.Add(Prefix + S);
    SendMessage(OutputMemo.Handle, $00B6 {EM_LINESCROLL}, 0, OutputMemo.Lines.Count);
  end;
end;

function RunStreamed(const Description, Executable, Params: string): Integer;
var
  EC: Integer;
  Launched: Boolean;
begin
  LogStep(Description);
  LogStep('cmd: ' + Executable + ' ' + Params);
  MemoLog('> ' + Description);
  WizardForm.Refresh;

  EC := 0;
  Launched := ExecAndLogOutput(Executable, Params, '', SW_HIDE,
    ewWaitUntilTerminated, EC, @StreamLogLine);

  if not Launched then
  begin
    Result := -1;
    LogError('Failed to launch: ' + Executable + ' (error ' + IntToStr(EC) + ')');
    MemoLog('  [failed to launch: ' + Executable + ']');
    exit;
  end;

  Result := EC;
  Log('  exit code: ' + IntToStr(EC));
  if EC = 0 then
    MemoLog('  [OK]')
  else
    MemoLog('  [exit code ' + IntToStr(EC) + ']');
end;

function GetConfigPath(): string;
begin
  Result := ExpandConstant('{%APPDATA}\wt\config.yml');
end;

function ReadConfigValue(const Key: string): string;
var
  Lines: TArrayOfString;
  I, P: Integer;
  Line, Value: string;
begin
  Result := '';
  if not FileExists(GetConfigPath()) then exit;
  if not LoadStringsFromFile(GetConfigPath(), Lines) then exit;
  for I := 0 to GetArrayLength(Lines) - 1 do
  begin
    Line := Trim(Lines[I]);
    if (Length(Line) = 0) or (Line[1] = '#') then continue;
    P := Pos(Key + ':', Line);
    if P = 1 then
    begin
      Value := Trim(Copy(Line, Length(Key) + 2, Length(Line)));
      if (Length(Value) >= 2) and (Value[1] = '"') and (Value[Length(Value)] = '"') then
        Value := Copy(Value, 2, Length(Value) - 2);
      Result := Value;
      exit;
    end;
  end;
end;

procedure LoadExistingConfig();
begin
  CfgModel := ReadConfigValue('model');
  CfgLanguage := ReadConfigValue('language');
  CfgDevice := ReadConfigValue('device');
  CfgThreads := ReadConfigValue('threads');
  CfgSpeakers := ReadConfigValue('speakers');
  CfgNoDiarize := ReadConfigValue('no_diarize');
end;

function ModelFileName(const Model: string): string;
begin
       if Model = 'tiny'      then Result := 'ggml-tiny.bin'
  else if Model = 'tiny.en'   then Result := 'ggml-tiny.en.bin'
  else if Model = 'base'      then Result := 'ggml-base.bin'
  else if Model = 'base.en'   then Result := 'ggml-base.en.bin'
  else if Model = 'small'     then Result := 'ggml-small.bin'
  else if Model = 'small.en'  then Result := 'ggml-small.en.bin'
  else if Model = 'medium'    then Result := 'ggml-medium.bin'
  else if Model = 'medium.en' then Result := 'ggml-medium.en.bin'
  else if Model = 'large-v3'  then Result := 'ggml-large-v3.bin'
  else if Model = 'turbo'     then Result := 'ggml-large-v3-turbo.bin'
  else Result := 'ggml-large-v3-turbo.bin';
end;

function HasNvidiaGpu(): Boolean;
var EC: Integer;
begin
  Result := False;
  if Exec('cmd.exe', '/c nvidia-smi --query-gpu=name --format=csv,noheader >nul 2>&1',
          '', SW_HIDE, ewWaitUntilTerminated, EC) then
    Result := (EC = 0);
end;

function CudaInstalled(): Boolean;
begin
  Result := FileExists('C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v12.9\bin\nvcc.exe') or
            FileExists('C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v12.8\bin\nvcc.exe') or
            FileExists('C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v12.6\bin\nvcc.exe');
end;

procedure CreateLogControls();
var
  Page: TWinControl;
  L, W, MemoTop: Integer;
begin
  Page := WizardForm.InstallingPage;
  L := WizardForm.StatusLabel.Left;
  W := WizardForm.StatusLabel.Width;

  WizardForm.StatusLabel.Font.Style := [fsBold];

  OverallProgress := TNewProgressBar.Create(WizardForm);
  OverallProgress.Parent := Page;
  OverallProgress.Left := L;
  OverallProgress.Top := WizardForm.ProgressGauge.Top;
  OverallProgress.Width := W;
  OverallProgress.Height := WizardForm.ProgressGauge.Height;
  OverallProgress.Min := 0;
  OverallProgress.Max := 6;
  OverallProgress.Position := 0;

  MemoTop := OverallProgress.Top + OverallProgress.Height + ScaleY(10);

  OutputMemo := TNewMemo.Create(WizardForm);
  OutputMemo.Parent := Page;
  OutputMemo.Left := L;
  OutputMemo.Top := MemoTop;
  OutputMemo.Width := W;
  OutputMemo.Height := Page.ClientHeight - MemoTop - ScaleY(4);
  OutputMemo.Anchors := [akLeft, akTop, akRight, akBottom];
  OutputMemo.ReadOnly := True;
  OutputMemo.ScrollBars := ssVertical;
  OutputMemo.WantReturns := False;
  OutputMemo.Color := $001E1E1E;
  OutputMemo.Font.Color := $0000FF00;
  OutputMemo.Font.Name := 'Consolas';
  OutputMemo.Font.Size := 8;
end;

procedure ShowLogControls(Show: Boolean);
begin
  if Assigned(OverallProgress) then OverallProgress.Visible := Show;
  if Assigned(OutputMemo) then OutputMemo.Visible := Show;

  WizardForm.FilenameLabel.Visible := not Show;
  WizardForm.ProgressGauge.Visible := not Show;
end;

procedure InitializeWizard();
begin
  LoadExistingConfig();
  InitLog();

  if WizardSilent() then exit;

  CreateLogControls();
  ShowLogControls(False);
end;

function GetModel(Param: string): string;
begin
  if CfgModel <> '' then Result := CfgModel
  else Result := 'turbo';
end;

function GetLanguage(Param: string): string;
begin
  if CfgLanguage <> '' then Result := CfgLanguage
  else Result := 'auto';
end;

function GetDevice(Param: string): string;
begin
  if CfgDevice <> '' then Result := CfgDevice
  else Result := 'auto';
end;

function GetThreads(Param: string): string;
begin
  if CfgThreads <> '' then Result := CfgThreads
  else Result := ExpandConstant('{%NUMBER_OF_PROCESSORS|4}');
end;

function GetSpeakers(Param: string): string;
begin
  if CfgSpeakers <> '' then Result := CfgSpeakers
  else Result := '0';
end;

function GetNoDiarize(Param: string): string;
begin
  if CfgNoDiarize <> '' then Result := CfgNoDiarize
  else Result := 'false';
end;

procedure WriteConfig();
var
  ConfigDir, ConfigPath, Lang, Content: string;
begin
  ConfigDir := ExpandConstant('{%APPDATA}\wt');
  ForceDirectories(ConfigDir);
  ForceDirectories(ConfigDir + '\models');
  ConfigPath := ConfigDir + '\config.yml';

  Lang := GetLanguage('');
  if (Lang = 'auto') or (Lang = '') then Lang := '""';

  Content := '# wt configuration' + #13#10 +
             '# See: https://github.com/asolopovas/wt' + #13#10 + #13#10 +
             'model: ' + GetModel('') + #13#10 +
             'language: ' + Lang + #13#10 +
             'device: ' + GetDevice('') + #13#10 +
             'threads: ' + GetThreads('') + #13#10 +
             'speakers: ' + GetSpeakers('') + #13#10 +
             'no_diarize: ' + GetNoDiarize('') + #13#10;

  SaveStringToFile(ConfigPath, Content, False);
  Log('Settings: model=' + GetModel('') + ' language=' + GetLanguage('') +
      ' device=' + GetDevice('') + ' threads=' + GetThreads(''));
  LogOk('Config saved to ' + ConfigPath);
  MemoLog('  Config saved to ' + ConfigPath);
end;

procedure InstallFFmpeg();
var EC: Integer;
begin
  SetStepStatus('Checking ffmpeg...');
  EC := RunStreamed('Checking ffmpeg', 'where', 'ffmpeg');
  if EC = 0 then begin
    LogOk('ffmpeg found');
    MemoLog('  ffmpeg found in PATH');
    AdvanceProgress();
    exit;
  end;
  SetStepStatus('Installing ffmpeg via winget...');
  EC := RunStreamed('Installing ffmpeg', 'winget',
    'install --id Gyan.FFmpeg --accept-source-agreements --accept-package-agreements --silent');
  if EC = 0 then begin
    LogOk('ffmpeg installed');
    MemoLog('  ffmpeg installed successfully');
  end else begin
    LogWarn('ffmpeg install exited with code ' + IntToStr(EC));
    MemoLog('  WARNING: ffmpeg install exited with code ' + IntToStr(EC));
  end;
  AdvanceProgress();
end;

procedure InstallCuda();
var EC: Integer;
begin
  SetStepStatus('Checking NVIDIA GPU...');
  if not HasNvidiaGpu() then begin
    LogOk('No NVIDIA GPU detected, CPU mode');
    MemoLog('  No NVIDIA GPU detected, using CPU mode');
    AdvanceProgress();
    exit;
  end;

  RunStreamed('Detecting GPU', 'nvidia-smi', '--query-gpu=name --format=csv,noheader');

  if CudaInstalled() then begin
    LogOk('CUDA toolkit found');
    MemoLog('  CUDA toolkit already installed');
    AdvanceProgress();
    exit;
  end;

  EC := RunStreamed('Checking nvcc in PATH', 'where', 'nvcc');
  if EC = 0 then begin
    LogOk('CUDA toolkit found in PATH');
    MemoLog('  CUDA toolkit found in PATH');
    AdvanceProgress();
    exit;
  end;

  SetStepStatus('Installing CUDA ' + RequiredCudaVersion + '...');
  EC := RunStreamed('Installing CUDA ' + RequiredCudaVersion, 'winget',
    'install --id Nvidia.CUDA --version ' + RequiredCudaVersion +
    ' --accept-source-agreements --accept-package-agreements --silent');
  if EC = 0 then begin
    LogOk('CUDA ' + RequiredCudaVersion + ' installed');
    MemoLog('  CUDA ' + RequiredCudaVersion + ' installed');
  end else begin
    LogWarn('CUDA install exited with code ' + IntToStr(EC));
    MemoLog('  WARNING: CUDA install exited with code ' + IntToStr(EC));
  end;
  AdvanceProgress();
end;

function NemoInstalled(): Boolean;
var SiteDir: string;
begin
  SiteDir := ExpandConstant('{%APPDATA}\wt\python\Lib\site-packages\nemo');
  Result := DirExists(SiteDir);
end;

procedure InstallPythonEnv();
var
  UvPath, PythonDir, VenvPython: string;
  EC: Integer;
begin
  PythonDir := ExpandConstant('{%APPDATA}\wt\python');
  VenvPython := PythonDir + '\Scripts\python.exe';
  UvPath := ExpandConstant('{app}\deps\uv.exe');

  if not FileExists(UvPath) then begin
    LogError('Bundled uv.exe not found at ' + UvPath);
    MemoLog('  ERROR: uv.exe not found at ' + UvPath);
    AdvanceProgress();
    exit;
  end;
  MemoLog('  Using bundled uv: ' + UvPath);

  if not FileExists(VenvPython) then begin
    SetStepStatus('Downloading Python 3.12...');
    EC := RunStreamed('Creating Python 3.12 venv', UvPath,
      'venv "' + PythonDir + '" --python 3.12 --python-preference only-managed');
    if not FileExists(VenvPython) then begin
      LogError('Python venv creation failed (exit code ' + IntToStr(EC) + ')');
      MemoLog('  ERROR: Python venv creation failed');
      AdvanceProgress();
      exit;
    end;
    LogOk('Python venv created');
    MemoLog('  Python 3.12 venv created');
  end else begin
    LogOk('Python venv exists: ' + PythonDir);
    MemoLog('  Python venv already exists');
  end;

  if NemoInstalled() then begin
    LogOk('NeMo already installed');
    MemoLog('  NeMo already installed, skipping');
    AdvanceProgress();
    exit;
  end;

  SetStepStatus('Installing NeMo toolkit (this may take several minutes)...');
  MemoLog('  This is the longest step — pip will download ~2 GB of packages.');
  EC := RunStreamed('Installing NeMo toolkit', UvPath,
    'pip install "nemo_toolkit[asr]" --python "' + VenvPython + '"');
  if EC = 0 then begin
    LogOk('NeMo installed');
    MemoLog('  NeMo toolkit installed successfully');
  end else begin
    LogError('NeMo install exited with code ' + IntToStr(EC));
    MemoLog('  ERROR: NeMo install failed (exit code ' + IntToStr(EC) + ')');
  end;

  if HasNvidiaGpu() then begin
    SetStepStatus('Installing PyTorch with CUDA support...');
    EC := RunStreamed('Installing torch with CUDA', UvPath,
      'pip install torch torchvision torchaudio --index-url https://download.pytorch.org/whl/cu124 --python "' + VenvPython + '"');
    if EC = 0 then begin
      LogOk('CUDA torch installed');
      MemoLog('  PyTorch with CUDA installed');
    end else begin
      LogWarn('CUDA torch install exited with code ' + IntToStr(EC));
      MemoLog('  WARNING: CUDA torch install exited with code ' + IntToStr(EC));
    end;
  end;
  AdvanceProgress();
end;

procedure DownloadModel();
var
  ModelDir, ModelPath, ModelUrl, TmpPath: string;
  EC: Integer;
begin
  ModelDir := ExpandConstant('{%APPDATA}\wt\models');
  ForceDirectories(ModelDir);
  ModelPath := ModelDir + '\' + ModelFileName(GetModel(''));

  if FileExists(ModelPath) then begin
    LogOk('Model ''' + GetModel('') + ''' already downloaded: ' + ModelPath);
    MemoLog('  Model ''' + GetModel('') + ''' already downloaded');
    AdvanceProgress();
    exit;
  end;

  ModelUrl := ModelUrlBase + ModelFileName(GetModel(''));
  TmpPath := ModelPath + '.downloading';

  SetStepStatus('Downloading ' + GetModel('') + ' model...');
  EC := RunStreamed('Downloading model ' + GetModel(''), 'curl',
    '-L --silent --show-error --fail -o "' + TmpPath + '" "' + ModelUrl + '"');

  if (EC = 0) and FileExists(TmpPath) then begin
    RenameFile(TmpPath, ModelPath);
    LogOk('Model saved to ' + ModelPath);
    MemoLog('  Model saved to ' + ModelPath);
  end else begin
    LogError('Model download failed (curl exit code ' + IntToStr(EC) + ')');
    MemoLog('  ERROR: Model download failed');
    DeleteFile(TmpPath);
  end;
  AdvanceProgress();
end;

procedure DownloadVADModel();
var
  ModelDir, ModelPath, ModelUrl, TmpPath: string;
  EC: Integer;
begin
  ModelDir := ExpandConstant('{%APPDATA}\wt\models');
  ForceDirectories(ModelDir);
  ModelPath := ModelDir + '\ggml-silero-v6.2.0.bin';

  if FileExists(ModelPath) then begin
    LogOk('VAD model already downloaded: ' + ModelPath);
    MemoLog('  VAD model already downloaded');
    AdvanceProgress();
    exit;
  end;

  ModelUrl := 'https://huggingface.co/ggml-org/whisper-vad/resolve/main/ggml-silero-v6.2.0.bin';
  TmpPath := ModelPath + '.downloading';

  SetStepStatus('Downloading VAD model...');
  EC := RunStreamed('Downloading VAD model', 'curl',
    '-L --silent --show-error --fail -o "' + TmpPath + '" "' + ModelUrl + '"');

  if (EC = 0) and FileExists(TmpPath) then begin
    RenameFile(TmpPath, ModelPath);
    LogOk('VAD model saved to ' + ModelPath);
    MemoLog('  VAD model saved to ' + ModelPath);
  end else begin
    LogError('VAD model download failed (curl exit code ' + IntToStr(EC) + ')');
    MemoLog('  ERROR: VAD model download failed');
    DeleteFile(TmpPath);
  end;
  AdvanceProgress();
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep <> ssPostInstall then exit;

  InitLog();
  Log('');
  Log('=========================================');
  Log('wt Setup v{#MyAppVersion}');
  Log('=========================================');
  Log('App directory: ' + ExpandConstant('{app}'));
  Log('Log file: ' + SetupLogPath);

  TotalSteps := 6;
  CurrentStep := 0;
  if Assigned(OverallProgress) then begin
    OverallProgress.Max := TotalSteps;
    OverallProgress.Position := 0;
  end;

  ShowLogControls(True);
  WizardForm.CancelButton.Enabled := False;
  WizardForm.BackButton.Enabled := False;

  MemoLog('wt Setup v{#MyAppVersion}');
  MemoLog('App directory: ' + ExpandConstant('{app}'));
  MemoLog('Log file: ' + SetupLogPath);

  SetStepStatus('Writing configuration...');
  WriteConfig();
  AdvanceProgress();

  InstallFFmpeg();
  InstallCuda();
  InstallPythonEnv();
  DownloadModel();
  DownloadVADModel();

  Log('=========================================');
  Log('Setup complete.');
  Log('=========================================');

  SetStepStatus('Setup complete!');
  MemoLog('');
  MemoLog('=========================================');
  MemoLog('All steps completed.');
  MemoLog('=========================================');

  if Assigned(OverallProgress) then
    OverallProgress.Position := TotalSteps;

  ShowLogControls(False);
  WizardForm.CancelButton.Enabled := True;
end;

function NeedsAddPath(Param: string): Boolean;
var OrigPath: string;
begin
  if not RegQueryStringValue(HKCU, 'Environment', 'Path', OrigPath) then
  begin Result := True; exit; end;
  Result := Pos(';' + Param + ';', ';' + OrigPath + ';') = 0;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var OrigPath, NewPath, AppDir: string; P: Integer;
begin
  if CurUninstallStep <> usPostUninstall then exit;
  AppDir := ExpandConstant('{app}');
  if not RegQueryStringValue(HKCU, 'Environment', 'Path', OrigPath) then exit;
  NewPath := OrigPath;
  P := Pos(';' + AppDir, NewPath);
  if P > 0 then begin
    Delete(NewPath, P, Length(';' + AppDir));
    RegWriteExpandStringValue(HKCU, 'Environment', 'Path', NewPath);
  end;
  P := Pos(AppDir + ';', NewPath);
  if P = 1 then begin
    Delete(NewPath, 1, Length(AppDir + ';'));
    RegWriteExpandStringValue(HKCU, 'Environment', 'Path', NewPath);
  end;
end;
