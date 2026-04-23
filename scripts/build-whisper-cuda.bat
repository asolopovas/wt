@echo off
setlocal

:: Find vcvarsall.bat
set "VCVARS="
for /f "delims=" %%i in ('dir /b /s "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvarsall.bat" 2^>nul') do set "VCVARS=%%i"
if "%VCVARS%"=="" for /f "delims=" %%i in ('dir /b /s "C:\Program Files\Microsoft Visual Studio\2022\*\VC\Auxiliary\Build\vcvarsall.bat" 2^>nul') do set "VCVARS=%%i"
if "%VCVARS%"=="" (
    echo ERROR: vcvarsall.bat not found
    exit /b 1
)

:: Set up MSVC x64 environment
call "%VCVARS%" x64 >nul 2>&1

:: Prefer system-installed CMake and Ninja over Strawberry Perl bundles
if exist "C:\Program Files\CMake\bin\cmake.exe" set "PATH=C:\Program Files\CMake\bin;%PATH%"
for /f "delims=" %%i in ('where ninja 2^>nul') do (
    if /i not "%%~dpi"=="C:\Strawberry\c\bin\" goto :ninja_ok
)
:: winget-installed ninja
set "NINJA_WINGET=%LOCALAPPDATA%\Microsoft\WinGet\Packages"
for /d %%d in ("%NINJA_WINGET%\Ninja-build.Ninja_*") do (
    if exist "%%d\ninja.exe" set "PATH=%%d;%PATH%"
)
:ninja_ok

:: Find CUDA toolkit and add to PATH
set "CUDA_PATH="
for /d %%d in ("C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v*") do set "CUDA_PATH=%%d"
if defined CUDA_PATH (
    set "PATH=%CUDA_PATH%\bin;%PATH%"
) else (
    echo ERROR: CUDA toolkit not found
    exit /b 1
)

set "WHISPER_ROOT=%~1"

:: CUDA_ARCH can be passed via environment variable or as %2.
:: Using env var avoids semicolon issues in cmd.exe argument parsing.
if not defined CUDA_ARCH (
    if not "%~2"=="" (
        set "CUDA_ARCH=%~2"
    ) else (
        set "CUDA_ARCH=61;75;80;86;89"
    )
)

echo Building whisper.cpp with CUDA (arch=%CUDA_ARCH%)...

cd /d "%WHISPER_ROOT%"

:: Clean previous build to avoid cached CMake config conflicts
if exist build rmdir /s /q build

cmake -B build -G Ninja ^
    -DCMAKE_POLICY_VERSION_MINIMUM=3.5 ^
    -DCMAKE_C_COMPILER=cl ^
    -DCMAKE_CXX_COMPILER=cl ^
    -DCMAKE_C_STANDARD=11 ^
    -DCMAKE_CXX_STANDARD=17 ^
    -DCMAKE_BUILD_TYPE=Release ^
    -DBUILD_SHARED_LIBS=ON ^
    -DGGML_CUDA=ON ^
    -DGGML_BACKEND_DL=ON ^
    -DGGML_NATIVE=OFF ^
    -DGGML_CPU_ALL_VARIANTS=ON ^
    "-DCMAKE_CUDA_ARCHITECTURES=%CUDA_ARCH%" ^
    -DWHISPER_BUILD_EXAMPLES=OFF ^
    -DWHISPER_BUILD_TESTS=OFF

if errorlevel 1 exit /b 1

cmake --build build --config Release -j %NUMBER_OF_PROCESSORS%

if errorlevel 1 exit /b 1

:: NOTE: CUDA runtime DLLs (cudart, cublas, cublasLt) are NOT copied here.
:: They are provided by the CUDA toolkit installation on the target machine.
:: Redistributing them would violate the NVIDIA CUDA EULA.

echo whisper.cpp CUDA build complete (shared libraries).
