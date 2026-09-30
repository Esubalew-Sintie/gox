package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/esubalew-Sintie/gox/internal/architecture"
	"github.com/esubalew-Sintie/gox/internal/constants/errors"
	"github.com/esubalew-Sintie/gox/internal/filesystem"
	"gopkg.in/yaml.v3"
)

const FileName = ".gox.yaml"

type Config struct {
	Version int    `yaml:"version"`
	Module  string `yaml:"module"`
	Layout  string `yaml:"layout"`
	HTTP    string `yaml:"http"`

	Storage  StorageConfig  `yaml:"storage"`
	Features FeaturesConfig `yaml:"features"`
}

type StorageConfig struct {
	Driver   string `yaml:"driver"`
	Postgres bool   `yaml:"postgres"`
	Mongo    bool   `yaml:"mongo"`
}

type FeaturesConfig struct {
	Redis bool `yaml:"redis"`
	Kafka bool `yaml:"kafka"`
}

type ConfigManager struct{ fs *filesystem.FileSystem }

func NewConfigManager(fs *filesystem.FileSystem) *ConfigManager { return &ConfigManager{fs: fs} }

func (m *ConfigManager) Load(projectPath string) (*Config, error) {
	data, err := m.fs.ReadFile(filepath.Join(projectPath, FileName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.ErrConfigNotFound
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := Normalize(&cfg); err != nil {
		return nil, err
	}
	if err := Validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (m *ConfigManager) Save(projectPath string, cfg *Config) error {
	if err := Normalize(cfg); err != nil {
		return err
	}
	if err := Validate(cfg); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := m.fs.WriteFile(filepath.Join(projectPath, FileName), data); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func Normalize(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("%w: config is nil", errors.ErrInvalidConfig)
	}
	cfg.Storage.Driver = strings.ToLower(strings.TrimSpace(cfg.Storage.Driver))
	if cfg.Storage.Driver == "" {
		switch {
		case cfg.Storage.Postgres && cfg.Storage.Mongo:
			return fmt.Errorf("%w: choose one storage driver", errors.ErrInvalidConfig)
		case cfg.Storage.Postgres:
			cfg.Storage.Driver = "postgres"
		case cfg.Storage.Mongo:
			cfg.Storage.Driver = "mongo"
		default:
			cfg.Storage.Driver = "memory"
		}
	}
	cfg.Storage.Postgres = cfg.Storage.Driver == "postgres"
	cfg.Storage.Mongo = cfg.Storage.Driver == "mongo"
	return nil
}

func Validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("%w: config is nil", errors.ErrInvalidConfig)
	}
	if cfg.Version != 1 {
		return fmt.Errorf("%w: unsupported version %d", errors.ErrInvalidConfig, cfg.Version)
	}
	if cfg.Module == "" {
		return fmt.Errorf("%w: module is required", errors.ErrInvalidConfig)
	}
	if !architecture.IsSupported(cfg.Layout) {
		return fmt.Errorf("%w: unsupported layout %q", errors.ErrInvalidConfig, cfg.Layout)
	}
	switch strings.ToLower(strings.TrimSpace(cfg.HTTP)) {
	case "net/http", "chi", "gin":
	default:
		return fmt.Errorf("%w: unsupported http router %q", errors.ErrInvalidConfig, cfg.HTTP)
	}
	switch cfg.Storage.Driver {
	case "memory", "postgres", "mongo":
	default:
		return fmt.Errorf("%w: unsupported storage driver %q", errors.ErrInvalidConfig, cfg.Storage.Driver)
	}
	return nil
}
