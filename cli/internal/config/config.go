package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all overseer configuration values.
type Config struct {
	Secrets SecretsConfig `mapstructure:"secrets"`
}

// SecretsConfig holds 1Password-related settings.
type SecretsConfig struct {
	Vault        string            `mapstructure:"vault"`
	Environments map[string]string `mapstructure:"environments"`
}

// Path returns the path to the config file.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "overseer", "config.yaml"), nil
}

// Load reads the config file and returns a Config.
// If no config file exists, default values are returned without error.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	v := viper.New()
	v.SetConfigFile(path)

	v.SetDefault("secrets.vault", "Personal")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("reading config: %w", err)
			}
		}
		// File doesn't exist yet — write defaults so the path is always valid.
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, fmt.Errorf("creating config dir: %w", err)
		}
		if err := v.WriteConfigAs(path); err != nil {
			return nil, fmt.Errorf("writing default config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}
