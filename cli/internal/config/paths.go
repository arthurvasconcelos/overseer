package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/wsl"
)

// ExpandPath expands environment variables ($VAR, ${VAR}) and a leading ~ in path.
func ExpandPath(path string) string {
	if strings.Contains(path, "$") {
		path = os.ExpandEnv(path)
	}
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path[1:], "/"))
}

// ResolvePath expands path and anchors it at base when it is relative.
func ResolvePath(base, path string) string {
	path = ExpandPath(path)
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(base, path)
}

// ContractHome rewrites a path inside the home directory as ~/..., so a value
// written to the shared brain config stays valid on machines whose home differs.
func ContractHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if rel, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "~/" + filepath.ToSlash(rel)
	}
	return path
}

// Platform returns the name used for brain overlays: darwin, linux or wsl.
// OVERSEER_PLATFORM overrides detection, e.g. to preview another platform's config.
func Platform() string {
	if p := os.Getenv("OVERSEER_PLATFORM"); p != "" {
		return p
	}
	if runtime.GOOS == "linux" && wsl.Detected() {
		return "wsl"
	}
	return runtime.GOOS
}

// PlatformChain returns the overlay names that apply here, most general first.
// WSL layers on top of linux so shared Linux settings aren't duplicated.
func PlatformChain() []string {
	if p := Platform(); p != "wsl" {
		return []string{p}
	}
	return []string{"linux", "wsl"}
}

// OverlayConfigPaths returns brain/overseer/config.<platform>.yaml for every
// platform in the chain, in merge order, whether or not the files exist.
func OverlayConfigPaths(cfg *Config) []string {
	dir := BrainOverseerPath(cfg)
	paths := make([]string, 0, 2)
	for _, p := range PlatformChain() {
		paths = append(paths, filepath.Join(dir, "config."+p+".yaml"))
	}
	return paths
}

// DotfilesDirs returns the shared dotfiles directory followed by the
// dotfiles.<platform> overlays, in layering order, whether or not they exist.
func DotfilesDirs(cfg *Config) []string {
	dir := BrainOverseerPath(cfg)
	dirs := []string{filepath.Join(dir, "dotfiles")}
	for _, p := range PlatformChain() {
		dirs = append(dirs, filepath.Join(dir, "dotfiles."+p))
	}
	return dirs
}

// ConfigLayers returns the config files that exist, in merge order.
func ConfigLayers(cfg *Config) []string {
	var layers []string
	candidates := append([]string{filepath.Join(BrainOverseerPath(cfg), "config.yaml")}, OverlayConfigPaths(cfg)...)
	if local, err := LocalPath(); err == nil {
		candidates = append(candidates, local)
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			layers = append(layers, path)
		}
	}
	return layers
}

// ResolveReposPath returns the repos root from OVERSEER_REPOS_PATH, then
// system.repos_path. It is empty when neither is set.
func ResolveReposPath(cfg *Config) string {
	if h := os.Getenv("OVERSEER_REPOS_PATH"); h != "" {
		return ExpandPath(h)
	}
	return ExpandPath(cfg.System.ReposPath)
}

// ResolveVaultPath returns the Obsidian vault directory; relative paths are
// anchored at the repos root.
func ResolveVaultPath(cfg *Config) string {
	if cfg.Obsidian.VaultPath == "" {
		return ""
	}
	return ResolvePath(ResolveReposPath(cfg), cfg.Obsidian.VaultPath)
}
