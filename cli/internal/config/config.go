package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// BrewConfig holds Homebrew-related settings.
type BrewConfig struct {
	Brewfile string `mapstructure:"brewfile" json:"brewfile,omitempty"` // path relative to repos_path; defaults to "Brewfile"
}

// BrainConfig holds settings for the user's brain directory.
// url and git_profile live here (portable across machines).
// path is the canonical location; system.brain_path overrides it per machine.
type BrainConfig struct {
	Path       string `mapstructure:"path"        json:"path,omitempty"`        // canonical brain path (e.g. ~/brain)
	URL        string `mapstructure:"url"         json:"url,omitempty"`         // git remote URL for pull/clone
	GitProfile string `mapstructure:"git_profile" json:"git_profile,omitempty"` // git profile to use for brain commits
}

// Config holds all overseer configuration values.
type Config struct {
	Secrets      SecretsConfig      `mapstructure:"secrets"      json:"secrets,omitempty"`
	Integrations IntegrationsConfig `mapstructure:"integrations" json:"integrations,omitempty"`
	Git          GitConfig          `mapstructure:"git"          json:"git,omitempty"`
	System       SystemConfig       `mapstructure:"system"       json:"system,omitempty"`
	Brain        BrainConfig        `mapstructure:"brain"        json:"brain,omitempty"`
	Obsidian     ObsidianConfig     `mapstructure:"obsidian"     json:"obsidian,omitempty"`
	Brew         BrewConfig         `mapstructure:"brew"         json:"brew,omitempty"`
	Repos        []RepoConfig       `mapstructure:"repos"        json:"repos,omitempty"`
}

// RepoConfig defines a managed repository.
type RepoConfig struct {
	Name       string `mapstructure:"name"        json:"name"`
	URL        string `mapstructure:"url"         json:"url"`
	Path       string `mapstructure:"path"        json:"path"`                  // relative to OVERSEER_HOME
	Readonly   bool   `mapstructure:"readonly"    json:"readonly,omitempty"`    // skip push, warn on local changes
	GitProfile string `mapstructure:"git_profile" json:"git_profile,omitempty"` // profile name from git.profiles to apply after clone
}

// SecretsConfig holds 1Password-related settings.
type SecretsConfig struct {
	Environments map[string]string `mapstructure:"environments" json:"environments,omitempty"`
}

// IntegrationsConfig holds all third-party integration configs.
type IntegrationsConfig struct {
	Jira   []JiraInstance   `mapstructure:"jira"   json:"jira,omitempty"`
	Slack  []SlackWorkspace `mapstructure:"slack"  json:"slack,omitempty"`
	Google []GoogleAccount  `mapstructure:"google" json:"google,omitempty"`
	GitHub []GitHubInstance `mapstructure:"github" json:"github,omitempty"`
	GitLab []GitLabInstance `mapstructure:"gitlab" json:"gitlab,omitempty"`
}

// GitHubInstance configures a single GitHub account.
// Token is an op:// reference to a Personal Access Token.
type GitHubInstance struct {
	Name      string `mapstructure:"name"       json:"name"`
	Token     string `mapstructure:"token"      json:"token"`                // op:// reference
	OPAccount string `mapstructure:"op_account" json:"op_account,omitempty"` // optional 1Password account ID
}

// GitLabInstance configures a single GitLab instance.
// Token is an op:// reference to a Personal Access Token.
type GitLabInstance struct {
	Name      string `mapstructure:"name"       json:"name"`
	BaseURL   string `mapstructure:"base_url"   json:"base_url,omitempty"`   // default: https://gitlab.com
	Token     string `mapstructure:"token"      json:"token"`                // op:// reference
	OPAccount string `mapstructure:"op_account" json:"op_account,omitempty"` // optional 1Password account ID
}

// JiraInstance configures a single Jira instance.
// Email and Token are op:// references resolved at runtime via secrets.Get.
// OPAccount is the 1Password account ID to use (see: overseer accounts).
type JiraInstance struct {
	Name      string `mapstructure:"name"       json:"name"`
	BaseURL   string `mapstructure:"base_url"   json:"base_url"`
	Email     string `mapstructure:"email"      json:"email"`                // op:// reference
	Token     string `mapstructure:"token"      json:"token"`                // op:// reference
	OPAccount string `mapstructure:"op_account" json:"op_account,omitempty"` // optional 1Password account ID
}

// SlackWorkspace configures a single Slack workspace.
// Token is an op:// reference resolved at runtime via secrets.Get.
// OPAccount is the 1Password account ID to use (see: overseer accounts).
type SlackWorkspace struct {
	Name      string `mapstructure:"name"       json:"name"`
	Token     string `mapstructure:"token"      json:"token"`                // op:// reference
	OPAccount string `mapstructure:"op_account" json:"op_account,omitempty"` // optional 1Password account ID
}

// GoogleAccount configures a single Google account for Calendar access.
// CredentialsDoc is an op:// reference to a 1Password Document containing
// the OAuth2 credentials JSON downloaded from Google Cloud Console.
// OPAccount is the 1Password account ID to use (see: overseer accounts).
type GoogleAccount struct {
	Name           string `mapstructure:"name"            json:"name"`
	CredentialsDoc string `mapstructure:"credentials_doc" json:"credentials_doc"`
	OPAccount      string `mapstructure:"op_account"      json:"op_account,omitempty"` // optional 1Password account ID
}

// ObsidianConfig holds settings for the Obsidian vault integration.
type ObsidianConfig struct {
	VaultPath        string `mapstructure:"vault_path"         json:"vault_path,omitempty"`         // relative to repos_path or absolute
	VaultName        string `mapstructure:"vault_name"         json:"vault_name,omitempty"`         // basename as registered in Obsidian (for URI scheme)
	DailyNotesFolder string `mapstructure:"daily_notes_folder" json:"daily_notes_folder,omitempty"` // e.g. "06 - Daily"
	TemplatesFolder  string `mapstructure:"templates_folder"   json:"templates_folder,omitempty"`   // e.g. "99 - Meta/_templates"
	DefaultFolder    string `mapstructure:"default_folder"     json:"default_folder,omitempty"`     // default folder for new notes (empty = root)
}

// SystemConfig holds machine-specific overrides (lives in config.local.yaml).
type SystemConfig struct {
	GPGSSHProgram string `mapstructure:"gpg_ssh_program" json:"gpg_ssh_program,omitempty"`
	ReposPath     string `mapstructure:"repos_path"      json:"repos_path,omitempty"`
	BrainPath     string `mapstructure:"brain_path"      json:"brain_path,omitempty"`
}

// GitConfig holds git identity profiles and shared defaults.
type GitConfig struct {
	Defaults GitDefaults  `mapstructure:"defaults"  json:"defaults,omitempty"`
	Profiles []GitProfile `mapstructure:"profiles"  json:"profiles,omitempty"`
}

// GitDefaults holds git settings shared across all profiles.
// Any field can be overridden per profile.
type GitDefaults struct {
	UserName      string `mapstructure:"user_name"      json:"user_name,omitempty"`
	GPGFormat     string `mapstructure:"gpg_format"     json:"gpg_format,omitempty"`
	GPGSSHProgram string `mapstructure:"gpg_ssh_program" json:"gpg_ssh_program,omitempty"`
	CommitGPGSign bool   `mapstructure:"commit_gpgsign" json:"commit_gpgsign"`
}

// GitProfile holds a named git identity. Fields left empty inherit from GitDefaults.
// Values starting with op:// are resolved via 1Password at runtime.
type GitProfile struct {
	Name          string `mapstructure:"name"           json:"name"`
	Email         string `mapstructure:"email"          json:"email,omitempty"`
	SigningKey     string `mapstructure:"signing_key"    json:"signing_key,omitempty"`    // plain value or op:// reference
	UserName      string `mapstructure:"user_name"      json:"user_name,omitempty"`      // overrides defaults.user_name
	GPGFormat     string `mapstructure:"gpg_format"     json:"gpg_format,omitempty"`     // overrides defaults.gpg_format
	GPGSSHProgram string `mapstructure:"gpg_ssh_program" json:"gpg_ssh_program,omitempty"` // overrides defaults.gpg_ssh_program
	CommitGPGSign *bool  `mapstructure:"commit_gpgsign" json:"commit_gpgsign,omitempty"` // overrides defaults.commit_gpgsign
	OPAccount     string `mapstructure:"op_account"     json:"op_account,omitempty"`     // for op:// references in this profile
}

// ResolveBrainPath returns the brain directory using this precedence:
//  1. OVERSEER_BRAIN env var
//  2. system.brain_path in config.local.yaml  (machine-local override)
//  3. brain.path in brain's config.yaml        (portable canonical path)
//  4. ~/brain as default
func ResolveBrainPath(cfg *Config) string {
	if b := os.Getenv("OVERSEER_BRAIN"); b != "" {
		return b
	}
	if cfg != nil && cfg.System.BrainPath != "" {
		return cfg.System.BrainPath
	}
	if cfg != nil && cfg.Brain.Path != "" {
		home, _ := os.UserHomeDir()
		p := cfg.Brain.Path
		if len(p) > 1 && p[:2] == "~/" {
			p = filepath.Join(home, p[2:])
		}
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "brain"
	}
	return filepath.Join(home, "brain")
}

// BrainOverseerPath returns the overseer-specific subdirectory within the brain.
func BrainOverseerPath(cfg *Config) string {
	return filepath.Join(ResolveBrainPath(cfg), "overseer")
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

// Load reads config with this merge order (later overrides earlier):
//  1. brain/overseer/config.yaml  — shared portable config
//  2. ~/.config/overseer/config.local.yaml — machine-local overrides
//
// To resolve the brain path, config.local.yaml is read first in a lightweight
// pass, then the brain config is loaded as the base.
// config.local.yaml is optional — missing file is silently ignored.
func Load() (*Config, error) {
	localPath, err := LocalPath()
	if err != nil {
		return nil, err
	}

	// Pass 1: read local config only to resolve brain_path.
	var localOnly Config
	if _, err := os.Stat(localPath); err == nil {
		vl := viper.New()
		vl.SetConfigFile(localPath)
		if err := vl.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("reading config.local.yaml: %w", err)
		}
		if err := vl.Unmarshal(&localOnly); err != nil {
			return nil, fmt.Errorf("parsing config.local.yaml: %w", err)
		}
	}

	// Pass 2: load brain config as base, then merge local on top.
	v := viper.New()

	brainCfgPath := filepath.Join(BrainOverseerPath(&localOnly), "config.yaml")
	if _, err := os.Stat(brainCfgPath); err == nil {
		v.SetConfigFile(brainCfgPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("reading brain config: %w", err)
		}
	}

	// Merge machine-local overrides on top.
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
