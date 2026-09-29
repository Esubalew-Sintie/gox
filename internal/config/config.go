package config

import (
	stdError "errors"
	"fmt"
	"path/filepath"

	"github.com/esubalew-Sintie/gox/internal/constants/errors"
	"github.com/esubalew-Sintie/gox/internal/filesystem"
	"gopkg.in/yaml.v3"
)

const FileName = ".gox.yaml"

type Config struct {
	Version int    `yaml:"version"`
	Module  string `yaml:"module"`
	Layout  string `yaml:"layout"`

	Storage  StorageConfig  `yaml:"storage"`
	Features FeaturesConfig `yaml:"features"`
}

type StorageConfig struct {
	Postgres bool `yaml:"postgres"`
}

type FeaturesConfig struct {
	Redis bool `yaml:"redis"`
	Kafka bool `yaml:"kafka"`
}

type ConfigManager struct {
	fs *filesystem.FileSystem
}

func NewConfigManager(fs *filesystem.FileSystem) *ConfigManager {
	return &ConfigManager{
		fs: fs,
	}
}

func (m *ConfigManager) Load(projectPath string) (*Config, error) {
	path := filepath.Join(projectPath, FileName)

	data, err := m.fs.ReadFile(path)
	if err != nil {
		if stdError.Is(err, errors.ErrPathNotFound) {
			return nil, errors.ErrConfigNotFound
		}

		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := Validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (m *ConfigManager) Save(projectPath string, cfg *Config) error {
	if err := Validate(cfg); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	path := filepath.Join(projectPath, FileName)

	if err := m.fs.WriteFile(path, data); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

func Validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("%w: config is nil", errors.ErrInvalidConfig)
	}

	if cfg.Version != 1 {
		return fmt.Errorf(
			"%w: unsupported version %d",
			errors.ErrInvalidConfig,
			cfg.Version,
		)
	}

	if cfg.Module == "" {
		return fmt.Errorf(
			"%w: module is required",
			errors.ErrInvalidConfig,
		)
	}

	if cfg.Layout == "" {
		return fmt.Errorf(
			"%w: layout is required",
			errors.ErrInvalidConfig,
		)
	}

	return nil
}
