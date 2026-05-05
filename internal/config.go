package shared

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const CurrentConfigVersion = 2

type Config struct {
	Version          int     `yaml:"version"`
	Model            string  `yaml:"model"`
	Diarizer         string  `yaml:"diarizer,omitempty"`
	LLM              string  `yaml:"llm,omitempty"`
	Language         string  `yaml:"language"`
	Device           string  `yaml:"device"`
	Engine           string  `yaml:"engine,omitempty"`
	Threads          int     `yaml:"threads"`
	Speakers         int     `yaml:"speakers,omitempty"`
	NoDiarize        bool    `yaml:"no_diarize,omitempty"`
	CacheExpiryDays  int     `yaml:"cache_expiry_days,omitempty"`
	LogRetentionDays int     `yaml:"log_retention_days,omitempty"`
	Models           []Model `yaml:"models,omitempty"`
}

type Model struct {
	ID             string      `yaml:"id"`
	Family         string      `yaml:"family"`
	Engine         string      `yaml:"engine,omitempty"`
	DisplayName    string      `yaml:"displayName"`
	Description    string      `yaml:"description,omitempty"`
	Languages      []string    `yaml:"languages,omitempty"`
	RAMHintMB      int         `yaml:"ramHintMB,omitempty"`
	SizeBytes      int64       `yaml:"sizeBytes,omitempty"`
	DefaultActive  bool        `yaml:"defaultActive,omitempty"`
	AndroidDefault bool        `yaml:"androidDefault,omitempty"`
	DiarSegRelPath string      `yaml:"diarSegRelPath,omitempty"`
	DiarEmbRelPath string      `yaml:"diarEmbRelPath,omitempty"`
	Files          []ModelFile `yaml:"files"`
}

type ModelFile struct {
	URL       string `yaml:"url"`
	RelPath   string `yaml:"relPath"`
	SizeBytes int64  `yaml:"sizeBytes"`
	SHA256    string `yaml:"sha256"`
}

const (
	EngineWhisperONNX = "whisper-onnx"
	EngineZipformer   = "zipformer"
	EngineParakeet    = "parakeet"
	EngineSenseVoice  = "sensevoice"
	EngineCanary      = "canary"
	EngineNemoCTC     = "nemo-ctc"

	EngineWhisper = EngineWhisperONNX
)

const (
	FamilyASR      = "asr"
	FamilyDiarizer = "diarizer"
	FamilyLLM      = "llm"
)

//go:embed default_config.yml
var defaultConfigYAML []byte

func PythonDir() string {
	return filepath.Join(Dir(), "python")
}

func PythonExe() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(PythonDir(), "Scripts", "python.exe")
	}
	return filepath.Join(PythonDir(), "bin", "python")
}

func Dir() string {
	return appDir()
}

func ModelsDir() string {
	if p := platformModelsDirOverride(); p != "" {
		return p
	}
	return filepath.Join(Dir(), "models")
}

func CacheDir() string {
	return filepath.Join(Dir(), "cache")
}

func FilePath() string {
	if p := platformConfigFileOverride(); p != "" {
		return p
	}
	return filepath.Join(Dir(), "config.yml")
}

func Defaults() Config {
	var cfg Config
	if err := yaml.Unmarshal(defaultConfigYAML, &cfg); err != nil {
		panic("shared: parse default_config.yml: " + err.Error())
	}
	if cfg.Version == 0 {
		cfg.Version = CurrentConfigVersion
	}
	if cfg.Threads == 0 {
		cfg.Threads = defaultThreads()
	}
	return cfg
}

func upgradeConfig(cfg *Config) (changed bool) {
	if cfg.Version < CurrentConfigVersion {
		cfg.Models = Defaults().Models
		cfg.Version = CurrentConfigVersion
		changed = true
	}
	if cfg.Engine == "whisper" || cfg.Engine == "moonshine" {
		cfg.Engine = EngineWhisperONNX
		changed = true
	}
	if cfg.Model == "moonshine-tiny-en-int8" {
		cfg.Model = ""
		changed = true
	}
	if len(cfg.Models) == 0 {
		cfg.Models = Defaults().Models
		changed = true
	}
	return changed
}

func Load() (Config, error) {
	cfg := Defaults()

	data, err := os.ReadFile(FilePath())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if mkErr := initDir(cfg); mkErr != nil {
				return cfg, fmt.Errorf("initializing config dir: %w", mkErr)
			}
			applyEnvOverrides(&cfg)
			return cfg, nil
		}
		return cfg, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Defaults(), fmt.Errorf("parsing %s: %w", FilePath(), err)
	}

	if cfg.Threads <= 0 {
		cfg.Threads = defaultThreads()
	}
	if cfg.Device == "" {
		cfg.Device = "auto"
	}
	if cfg.Engine == "" {
		cfg.Engine = EngineWhisper
	}
	if upgradeConfig(&cfg) {
		_ = Save(cfg)
	}
	applyEnvOverrides(&cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("WT_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("WT_LANGUAGE"); v != "" {
		cfg.Language = v
	}
	if v := os.Getenv("WT_DEVICE"); v != "" {
		cfg.Device = v
	}
	if v := os.Getenv("WT_ENGINE"); v != "" {
		cfg.Engine = v
	}
	if v := os.Getenv("WT_THREADS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Threads = n
		}
	}
	if v := os.Getenv("WT_SPEAKERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.Speakers = n
		}
	}
	if v, ok := envBool("WT_NO_DIARIZE"); ok {
		cfg.NoDiarize = v
	}
	if v := os.Getenv("WT_CACHE_EXPIRY_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.CacheExpiryDays = n
		}
	}
}

func envBool(key string) (bool, bool) {
	v := os.Getenv(key)
	if v == "" {
		return false, false
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	}
	return false, false
}

func initDir(cfg Config) error {
	if err := os.MkdirAll(ModelsDir(), 0o755); err != nil {
		return err
	}
	return Save(cfg)
}

func Save(cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	header := "# wt configuration\n# See: https://github.com/asolopovas/wt\n\n"
	return os.WriteFile(FilePath(), []byte(header+string(data)), 0o644)
}
