package shared

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

const CurrentConfigVersion = 1

type Config struct {
	Version         int    `yaml:"version"`
	Model           string `yaml:"model"`
	Language        string `yaml:"language"`
	Device          string `yaml:"device"`
	Threads         int    `yaml:"threads"`
	Speakers        int    `yaml:"speakers,omitempty"`
	NoDiarize       bool   `yaml:"no_diarize,omitempty"`
	TDRZ            bool   `yaml:"tdrz,omitempty"`
	CacheExpiryDays int    `yaml:"cache_expiry_days,omitempty"`
}

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
		Threads:         defaultThreads(),
		CacheExpiryDays: 30,
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

	if upgradeConfig(&cfg) {
		_ = Save(cfg)
	}
	return cfg, nil
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
