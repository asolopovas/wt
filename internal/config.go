package shared

import (
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

const CurrentConfigVersion = 1

type Config struct {
	Version         int    `yaml:"version"`
	Model           string `yaml:"model"`
	Language        string `yaml:"language"`
	Device          string `yaml:"device"`
	Engine          string `yaml:"engine,omitempty"` // "whisper" (default) or "zipformer"
	Threads         int    `yaml:"threads"`
	Speakers        int    `yaml:"speakers,omitempty"`
	NoDiarize       bool   `yaml:"no_diarize,omitempty"`
	TDRZ            bool   `yaml:"tdrz,omitempty"`
	CacheExpiryDays int    `yaml:"cache_expiry_days,omitempty"`
	// LogRetentionDays controls how long rotated wt.log archives are
	// kept. 0 = forever; 1 = 24h (default); 7 = week; 30 = month.
	LogRetentionDays int `yaml:"log_retention_days,omitempty"`
}

// Engine identifiers.
const (
	EngineWhisper    = "whisper"
	EngineZipformer  = "zipformer"
	EngineMoonshine  = "moonshine"
	EngineParakeet   = "parakeet"
	EngineSenseVoice = "sensevoice"
)

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
	return filepath.Join(Dir(), "config.yml")
}

func Defaults() Config {
	return Config{
		Version:         CurrentConfigVersion,
		Model:           defaultModel(),
		Device:          "auto",
		Engine:          EngineWhisper,
		Threads:         defaultThreads(),
		CacheExpiryDays:  30,
		LogRetentionDays: 1,
	}
}

func upgradeConfig(cfg *Config) (changed bool) {
	for cfg.Version < CurrentConfigVersion {
		switch cfg.Version {
		case 0:

			cfg.Version = 1
			changed = true
		default:

			cfg.Version = CurrentConfigVersion
			changed = true
		}
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
	if cfg.Model == "" {
		cfg.Model = defaultModel()
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
	if v, ok := envBool("WT_TDRZ"); ok {
		cfg.TDRZ = v
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
