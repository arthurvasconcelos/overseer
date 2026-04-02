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
	Git          GitConfig          `mapstructure:"git"`
	System       SystemConfig       `mapstructure:"system"`
	Repos        []RepoConfig       `mapstructure:"repos"`
}

// RepoConfig defines a managed repository.
type RepoConfig struct {
	Name       string `mapstructure:"name"`
	URL        string `mapstructure:"url"`
	Path       string `mapstructure:"path"`        // relative to OVERSEER_HOME
	Readonly   bool   `mapstructure:"readonly"`    // skip push, warn on local changes
	GitProfile string `mapstructure:"git_profile"` // profile name from git.profiles to apply after clone
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
// OPAccount is the 1Password account ID to use (see: overseer accounts).
type JiraInstance struct {
	Name      string `mapstructure:"name"`
	BaseURL   string `mapstructure:"base_url"`
	Email     string `mapstructure:"email"`      // op:// reference
	Token     string `mapstructure:"token"`      // op:// reference
	OPAccount string `mapstructure:"op_account"` // optional 1Password account ID
}

// SlackWorkspace configures a single Slack workspace.
// Token is an op:// reference resolved at runtime via secrets.Get.
// OPAccount is the 1Password account ID to use (see: overseer accounts).
type SlackWorkspace struct {
	Name      string `mapstructure:"name"`
	Token     string `mapstructure:"token"`      // op:// reference
	OPAccount string `mapstructure:"op_account"` // optional 1Password account ID
}

// GoogleAccount configures a single Google account for Calendar access.
// CredentialsDoc is an op:// reference to a 1Password Document containing
// the OAuth2 credentials JSON downloaded from Google Cloud Console.
// OPAccount is the 1Password account ID to use (see: overseer accounts).
type GoogleAccount struct {
	Name           string `mapstructure:"name"`
	CredentialsDoc string `mapstructure:"credentials_doc"`
	OPAccount      string `mapstructure:"op_account"` // optional 1Password account ID
}

// SystemConfig holds machine-specific overrides (lives in config.local.yaml).
type SystemConfig struct {
	GPGSSHProgram string `mapstructure:"gpg_ssh_program"`
	OverseerHome  string `mapstructure:"overseer_home"`
}

// GitConfig holds git identity profiles and shared defaults.
type GitConfig struct {
	Defaults GitDefaults  `mapstructure:"defaults"`
	Profiles []GitProfile `mapstructure:"profiles"`
}

// GitDefaults holds git settings shared across all profiles.
// Any field can be overridden per profile.
type GitDefaults struct {
	UserName       string `mapstructure:"user_name"`
	GPGFormat      string `mapstructure:"gpg_format"`
	GPGSSHProgram  string `mapstructure:"gpg_ssh_program"`
	CommitGPGSign  bool   `mapstructure:"commit_gpgsign"`
}

// GitProfile holds a named git identity. Fields left empty inherit from GitDefaults.
// Values starting with op:// are resolved via 1Password at runtime.
type GitProfile struct {
	Name          string `mapstructure:"name"`
	Email         string `mapstructure:"email"`
	SigningKey     string `mapstructure:"signing_key"`   // plain value or op:// reference
	UserName      string `mapstructure:"user_name"`     // overrides defaults.user_name
	GPGFormat     string `mapstructure:"gpg_format"`    // overrides defaults.gpg_format
	GPGSSHProgram string `mapstructure:"gpg_ssh_program"` // overrides defaults.gpg_ssh_program
	CommitGPGSign *bool  `mapstructure:"commit_gpgsign"`  // overrides defaults.commit_gpgsign
	OPAccount     string `mapstructure:"op_account"`    // for op:// references in this profile
}

// Path returns the path to the config file.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "overseer", "config.yaml"), nil
}

// LocalPath returns the path to the machine-specific config override file.
func LocalPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "overseer", "config.local.yaml"), nil
}

// Load reads config.yaml and merges config.local.yaml on top.
// If config.yaml does not exist it is created with empty defaults.
// config.local.yaml is optional — missing file is silently ignored.
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
		// File doesn't exist yet — write empty file so the path is always valid.
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, fmt.Errorf("creating config dir: %w", err)
		}
		if err := v.WriteConfigAs(path); err != nil {
			return nil, fmt.Errorf("writing default config: %w", err)
		}
	}

	// Merge machine-local overrides if present.
	localPath, err := LocalPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(localPath); err == nil {
		vl := viper.New()
		vl.SetConfigFile(localPath)
		if err := vl.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("reading config.local.yaml: %w", err)
		}
		if err := v.MergeConfigMap(vl.AllSettings()); err != nil {
			return nil, fmt.Errorf("merging config.local.yaml: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}
