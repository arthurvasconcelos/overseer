package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all overseer configuration values.
type Config struct {
	Secrets      SecretsConfig      `mapstructure:"secrets"`
	Integrations IntegrationsConfig `mapstructure:"integrations"`
}

// SecretsConfig holds 1Password-related settings.
type SecretsConfig struct {
	Environments map[string]string `mapstructure:"environments"`
}

// IntegrationsConfig holds all third-party integration configs.
type IntegrationsConfig struct {
	Jira   []JiraInstance    `mapstructure:"jira"`
	Slack  []SlackWorkspace  `mapstructure:"slack"`
	Google []GoogleAccount   `mapstructure:"google"`
}

// JiraInstance configures a single Jira instance.
// Email and Token are op:// references resolved at runtime via secrets.Get.
type JiraInstance struct {
	Name    string `mapstructure:"name"`
	BaseURL string `mapstructure:"base_url"`
	Email   string `mapstructure:"email"` // op:// reference
	Token   string `mapstructure:"token"` // op:// reference
}

// SlackWorkspace configures a single Slack workspace.
// Token is an op:// reference resolved at runtime via secrets.Get.
type SlackWorkspace struct {
	Name  string `mapstructure:"name"`
	Token string `mapstructure:"token"` // op:// reference
}

// GoogleAccount configures a single Google account for Calendar access.
// CredentialsFile is a path to an OAuth2 credentials JSON file.
type GoogleAccount struct {
	Name            string `mapstructure:"name"`
	CredentialsFile string `mapstructure:"credentials_file"`
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
