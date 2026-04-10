package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/nativeplugin"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// PluginManifest is the optional sidecar JSON file (overseer-<name>.json)
// that a plugin ships alongside its binary to declare metadata and required secrets.
type PluginManifest struct {
	Description string   `json:"description"`
	Secrets     []string `json:"secrets"` // e.g. ["github.personal", "gitlab.work"]
	Hooks       []string `json:"hooks"`   // e.g. ["daily", "status"]
}

// PluginContext is serialized to JSON and injected as OVERSEER_CONTEXT before
// the plugin binary executes. Secrets are fully resolved — plugins never need
// to know which secrets manager Overseer is using.
type PluginContext struct {
	Version    string                       `json:"version"`
	ConfigPath string                       `json:"config_path"`
	Secrets    map[string]map[string]string `json:"secrets"`
}

// externalPlugin holds a registered external plugin binary with its manifest.
type externalPlugin struct {
	name     string
	binPath  string
	manifest *PluginManifest
}

// externalRegistry stores all discovered external plugins after registration.
var externalRegistry []externalPlugin

var pluginsCmd = &cobra.Command{
	Use:   "plugins",
	Short: "List available native and external plugins",
	RunE:  runPluginsList,
}

var pluginsToggleCmd = &cobra.Command{
	Use:   "toggle",
	Short: "Interactively enable or disable native plugins",
	RunE:  runPluginsToggle,
}

func init() {
	pluginsCmd.AddCommand(pluginsToggleCmd)
	rootCmd.AddCommand(pluginsCmd)
}

func runPluginsList(_ *cobra.Command, _ []string) error {
	cfg, _ := config.Load()

	fmt.Println(tui.SectionHeader("native plugins", ""))
	maxLen := 0
	for _, p := range nativeplugin.All() {
		if len(p.Name) > maxLen {
			maxLen = len(p.Name)
		}
	}
	for _, p := range nativeplugin.All() {
		enabled := cfg != nil && p.IsEnabled != nil && p.IsEnabled(cfg)
		icon := tui.StyleOK.Render("✓ enabled ")
		if !enabled {
			icon = tui.StyleError.Render("✗ disabled")
		}
		padding := strings.Repeat(" ", maxLen-len(p.Name)+2)
		fmt.Printf("  %s%s%s  %s\n",
			tui.StyleNormal.Render(p.Name),
			padding,
			icon,
			tui.StyleDim.Render(p.Description),
		)
	}

	if len(externalRegistry) > 0 {
		fmt.Println()
		fmt.Println(tui.SectionHeader("external plugins", ""))
		extMaxLen := 0
		for _, ep := range externalRegistry {
			if len(ep.name) > extMaxLen {
				extMaxLen = len(ep.name)
			}
		}
		for _, ep := range externalRegistry {
			desc := ep.binPath
			if ep.manifest != nil && ep.manifest.Description != "" {
				desc = ep.manifest.Description
			}
			padding := strings.Repeat(" ", extMaxLen-len(ep.name)+2)
			fmt.Printf("  %s%s%s\n",
				tui.StyleAccent.Render(ep.name),
				padding,
				tui.StyleDim.Render(desc),
			)
		}
	}

	return nil
}

func runPluginsToggle(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	all := nativeplugin.All()

	// Build MultiSelect options pre-selected to match current enabled state.
	opts := make([]huh.Option[string], len(all))
	for i, p := range all {
		label := p.Name
		if p.Description != "" {
			label += "  " + tui.StyleDim.Render(p.Description)
		}
		enabled := p.IsEnabled != nil && p.IsEnabled(cfg)
		opts[i] = huh.NewOption(label, p.Name).Selected(enabled)
	}

	var selected []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Enable / disable native plugins").
				Description("Space to toggle · Enter to confirm · Esc to cancel").
				Options(opts...).
				Value(&selected),
		),
	)
	err = form.Run()
	if errors.Is(err, huh.ErrUserAborted) {
		fmt.Println(tui.StyleMuted.Render("cancelled"))
		return nil
	}
	if err != nil {
		return err
	}

	// Compute what changed and write back only the deltas.
	selectedSet := make(map[string]bool, len(selected))
	for _, name := range selected {
		selectedSet[name] = true
	}

	updates := make(map[string]*config.PluginSettings)
	for _, p := range all {
		wasEnabled := p.IsEnabled != nil && p.IsEnabled(cfg)
		nowEnabled := selectedSet[p.Name]
		if wasEnabled == nowEnabled {
			continue
		}
		if nowEnabled {
			// Explicitly enabling: write enabled=true only for opt-in plugins
			// (those with no auto-detect condition). For auto-detect plugins,
			// clear any explicit override so the default logic kicks back in.
			if hasAutoDetect(p, cfg) {
				updates[p.Name] = nil // remove explicit override
			} else {
				updates[p.Name] = &config.PluginSettings{Enabled: true}
			}
		} else {
			// Disabling: always write an explicit enabled=false.
			updates[p.Name] = &config.PluginSettings{Enabled: false}
		}
	}

	if len(updates) == 0 {
		fmt.Println(tui.StyleMuted.Render("no changes"))
		return nil
	}

	if err := config.WriteBrainPluginSettings(cfg, updates); err != nil {
		return err
	}

	for name, ps := range updates {
		if ps == nil || ps.Enabled {
			fmt.Printf("  %s  %s\n", tui.StyleOK.Render("enabled "), tui.StyleNormal.Render(name))
		} else {
			fmt.Printf("  %s  %s\n", tui.StyleError.Render("disabled"), tui.StyleNormal.Render(name))
		}
	}
	return nil
}

// hasAutoDetect reports whether p has a natural enabled condition based on
// config entries (as opposed to requiring explicit opt-in).
// Plugins that auto-detect return true here so that re-enabling them removes
// the explicit override instead of writing enabled=true.
func hasAutoDetect(p *nativeplugin.Plugin, cfg *config.Config) bool {
	// Temporarily clear explicit overrides to test default behaviour.
	stripped := *cfg
	stripped.Plugins = config.PluginsConfig{}
	return p.IsEnabled != nil && p.IsEnabled(&stripped)
}

// registerPlugins scans PATH and the brain's plugins/ directory for executables
// named "overseer-*" and registers each as a top-level subcommand.
// Built-in commands always take precedence over plugins with the same name.
func registerPlugins() {
	seen := map[string]bool{}
	for _, cmd := range rootCmd.Commands() {
		seen[cmd.Name()] = true
	}

	// Collect scan directories: PATH entries + brain plugins dir.
	dirs := filepath.SplitList(os.Getenv("PATH"))
	cfg, err := config.Load()
	if err == nil {
		brainPlugins := filepath.Join(config.BrainOverseerPath(cfg), "plugins")
		if _, err := os.Stat(brainPlugins); err == nil {
			dirs = append(dirs, brainPlugins)
		}
	}

	for _, dir := range dirs {
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

			externalRegistry = append(externalRegistry, externalPlugin{
				name:     pluginName,
				binPath:  binPath,
				manifest: manifest,
			})

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

// ExternalPluginsWithHook returns all registered external plugins that declare
// the given hook (e.g. "daily", "status"). Called by daily.go and status.go.
func ExternalPluginsWithHook(hook string) []externalPlugin {
	var out []externalPlugin
	for _, ep := range externalRegistry {
		if ep.manifest == nil {
			continue
		}
		for _, h := range ep.manifest.Hooks {
			if h == hook {
				out = append(out, ep)
				break
			}
		}
	}
	return out
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

// runHook calls a plugin binary with a hook argument (e.g. "daily", "status")
// and returns its captured stdout. OVERSEER_CONTEXT is injected normally.
func runHook(ep externalPlugin, hook string) (string, error) {
	ctx, err := buildPluginContext(ep.manifest)
	if err != nil {
		return "", fmt.Errorf("building plugin context: %w", err)
	}
	ctxJSON, err := json.Marshal(ctx)
	if err != nil {
		return "", fmt.Errorf("serializing plugin context: %w", err)
	}
	cmd := exec.Command(ep.binPath, hook)
	cmd.Env = append(os.Environ(), "OVERSEER_CONTEXT="+string(ctxJSON))
	out, err := cmd.Output()
	return string(out), err
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
