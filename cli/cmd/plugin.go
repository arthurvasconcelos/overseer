package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	"github.com/spf13/cobra"
)

// PluginManifest is the optional sidecar JSON file (overseer-<name>.json)
// that a plugin ships alongside its binary to declare metadata and required secrets.
type PluginManifest struct {
	Description string   `json:"description"`
	Secrets     []string `json:"secrets"` // e.g. ["github.personal", "gitlab.work"]
}

// PluginContext is serialized to JSON and injected as OVERSEER_CONTEXT before
// the plugin binary executes. Secrets are fully resolved — plugins never need
// to know which secrets manager Overseer is using.
type PluginContext struct {
	Version    string                       `json:"version"`
	ConfigPath string                       `json:"config_path"`
	Secrets    map[string]map[string]string `json:"secrets"`
}

// registerPlugins scans PATH for executables named "overseer-*" and registers
// each as a top-level subcommand that delegates execution to that binary.
// Built-in commands always take precedence over plugins with the same name.
func registerPlugins() {
	seen := map[string]bool{}
	for _, cmd := range rootCmd.Commands() {
		seen[cmd.Name()] = true
	}

	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasPrefix(name, "overseer-") {
				continue
			}
			pluginName := strings.TrimPrefix(name, "overseer-")
			if pluginName == "" || seen[pluginName] {
				continue
			}
			binPath := filepath.Join(dir, name)
			if err := isExecutable(binPath); err != nil {
				continue
			}
			seen[pluginName] = true

			manifest := loadManifest(binPath)
			short := "Plugin: " + binPath
			if manifest != nil && manifest.Description != "" {
				short = manifest.Description
			}

			rootCmd.AddCommand(&cobra.Command{
				Use:                pluginName,
				Short:              short,
				Annotations:        map[string]string{"overseer/plugin": "true"},
				DisableFlagParsing: true,
				RunE: func(_ *cobra.Command, args []string) error {
					return execPlugin(binPath, manifest, args)
				},
			})
		}
	}
}

// loadManifest reads the sidecar JSON manifest for a plugin binary, if present.
// The manifest is expected at <binPath>.json (e.g. overseer-learning.json).
func loadManifest(binPath string) *PluginManifest {
	data, err := os.ReadFile(binPath + ".json")
	if err != nil {
		return nil
	}
	var m PluginManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return &m
}

// execPlugin builds the plugin context, injects it as OVERSEER_CONTEXT, and
// execs the plugin binary with the given args.
func execPlugin(binPath string, manifest *PluginManifest, args []string) error {
	ctx, err := buildPluginContext(manifest)
	if err != nil {
		return fmt.Errorf("building plugin context: %w", err)
	}
	ctxJSON, err := json.Marshal(ctx)
	if err != nil {
		return fmt.Errorf("serializing plugin context: %w", err)
	}
	cmd := exec.Command(binPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "OVERSEER_CONTEXT="+string(ctxJSON))
	return cmd.Run()
}

// buildPluginContext constructs the PluginContext for a plugin, resolving any
// secrets declared in the manifest.
func buildPluginContext(manifest *PluginManifest) (*PluginContext, error) {
	configPath, err := config.Path()
	if err != nil {
		return nil, err
	}

	ctx := &PluginContext{
		Version:    Version,
		ConfigPath: configPath,
		Secrets:    map[string]map[string]string{},
	}

	if manifest == nil || len(manifest.Secrets) == 0 {
		return ctx, nil
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	for _, ref := range manifest.Secrets {
		parts := strings.SplitN(ref, ".", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid secret reference %q: expected <type>.<name>", ref)
		}
		resolved, err := resolveIntegrationSecrets(cfg, parts[0], parts[1])
		if err != nil {
			return nil, fmt.Errorf("resolving %q: %w", ref, err)
		}
		ctx.Secrets[ref] = resolved
	}

	return ctx, nil
}

// resolveIntegrationSecrets resolves the credentials for a named integration
// instance and returns them as a flat key/value map.
func resolveIntegrationSecrets(cfg *config.Config, kind, name string) (map[string]string, error) {
	switch kind {
	case "github":
		for _, inst := range cfg.Integrations.GitHub {
			if inst.Name == name {
				token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
				if err != nil {
					return nil, err
				}
				return map[string]string{"token": token}, nil
			}
		}
	case "gitlab":
		for _, inst := range cfg.Integrations.GitLab {
			if inst.Name == name {
				token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
				if err != nil {
					return nil, err
				}
				host := "gitlab.com"
				if inst.BaseURL != "" {
					if u, err := url.Parse(inst.BaseURL); err == nil && u.Host != "" {
						host = u.Host
					}
				}
				return map[string]string{"token": token, "host": host}, nil
			}
		}
	case "jira":
		for _, inst := range cfg.Integrations.Jira {
			if inst.Name == name {
				email, err := secrets.ReadAs(inst.Email, inst.OPAccount)
				if err != nil {
					return nil, err
				}
				token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
				if err != nil {
					return nil, err
				}
				return map[string]string{
					"email":    email,
					"token":    token,
					"base_url": inst.BaseURL,
				}, nil
			}
		}
	case "slack":
		for _, inst := range cfg.Integrations.Slack {
			if inst.Name == name {
				token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
				if err != nil {
					return nil, err
				}
				return map[string]string{"token": token}, nil
			}
		}
	default:
		return nil, fmt.Errorf("unknown integration type %q", kind)
	}
	return nil, fmt.Errorf("no %s instance named %q in config", kind, name)
}

func isExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode()&0o111 == 0 {
		return os.ErrPermission
	}
	return nil
}
