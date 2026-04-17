package devctx

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/nativeplugin"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

func init() {
	nativeplugin.Register(&nativeplugin.Plugin{
		Name:        "devctx",
		Description: "kubectl and Docker context switching",
		IsEnabled:   isEnabled,
		Commands:    commands,
	})
}

func isEnabled(cfg *config.Config) bool {
	if s, ok := cfg.Plugins.Settings["devctx"]; ok && !s.Enabled {
		return false
	}
	_, ke := exec.LookPath("kubectl")
	_, de := exec.LookPath("docker")
	return ke == nil || de == nil
}

func commands(_ *config.Config) []*cobra.Command {
	var cmds []*cobra.Command
	if _, err := exec.LookPath("kubectl"); err == nil {
		cmds = append(cmds, kubeCmd())
	}
	if _, err := exec.LookPath("docker"); err == nil {
		cmds = append(cmds, dockerCmd())
	}
	return cmds
}

// --- kubectl ---

func kubeCmd() *cobra.Command {
	root := &cobra.Command{
		Use:         "kube",
		Short:       "Switch kubectl context",
		Annotations: map[string]string{"overseer/group": "Dev"},
	}
	root.AddCommand(kubeListCmd())
	root.AddCommand(kubeUseCmd())
	return root
}

func kubeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available kubectl contexts",
		Args:  cobra.NoArgs,
		RunE:  runKubeList,
	}
}

func kubeUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use [context]",
		Short: "Switch to a kubectl context",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runKubeUse,
	}
}

func runKubeList(_ *cobra.Command, _ []string) error {
	contexts, err := kubectlContexts()
	if err != nil {
		return err
	}
	current, _ := kubectlCurrent()

	fmt.Println(tui.SectionHeader("kube contexts", ""))
	fmt.Println()
	for _, name := range contexts {
		if name == current {
			fmt.Printf("  %s %s\n", tui.StyleOK.Render("✓"), tui.StyleAccent.Render(name))
		} else {
			fmt.Printf("    %s\n", tui.StyleNormal.Render(name))
		}
	}
	return nil
}

func runKubeUse(_ *cobra.Command, args []string) error {
	ctx, err := resolveKubeContext(args)
	if err != nil {
		return err
	}
	out, runErr := exec.Command("kubectl", "config", "use-context", ctx).CombinedOutput()
	if runErr != nil {
		return fmt.Errorf("kubectl: %s", strings.TrimSpace(string(out)))
	}
	fmt.Printf("  %s kube: switched to %s\n", tui.StyleOK.Render("✓"), tui.StyleAccent.Render(ctx))
	return nil
}

func resolveKubeContext(args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	contexts, err := kubectlContexts()
	if err != nil {
		return "", err
	}
	current, _ := kubectlCurrent()
	items := make([]tui.SelectItem, len(contexts))
	for i, name := range contexts {
		subtitle := ""
		if name == current {
			subtitle = tui.StyleOK.Render("current")
		}
		items[i] = tui.SelectItem{Title: name, Subtitle: subtitle}
	}
	idx, err := tui.Select("Select kubectl context", items)
	if err != nil {
		return "", err
	}
	if idx < 0 {
		return "", fmt.Errorf("cancelled")
	}
	return contexts[idx], nil
}

func kubectlContexts() ([]string, error) {
	out, err := exec.Command("kubectl", "config", "get-contexts", "-o", "name").Output()
	if err != nil {
		return nil, fmt.Errorf("kubectl config get-contexts: %w", err)
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

func kubectlCurrent() (string, error) {
	out, err := exec.Command("kubectl", "config", "current-context").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// --- docker ---

func dockerCmd() *cobra.Command {
	root := &cobra.Command{
		Use:         "docker",
		Short:       "Switch Docker context",
		Annotations: map[string]string{"overseer/group": "Dev"},
	}
	root.AddCommand(dockerListCmd())
	root.AddCommand(dockerUseCmd())
	return root
}

func dockerListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available Docker contexts",
		Args:  cobra.NoArgs,
		RunE:  runDockerList,
	}
}

func dockerUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use [context]",
		Short: "Switch to a Docker context",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runDockerUse,
	}
}

func runDockerList(_ *cobra.Command, _ []string) error {
	contexts, err := dockerContexts()
	if err != nil {
		return err
	}
	current, _ := dockerCurrent()

	fmt.Println(tui.SectionHeader("docker contexts", ""))
	fmt.Println()
	for _, name := range contexts {
		if name == current {
			fmt.Printf("  %s %s\n", tui.StyleOK.Render("✓"), tui.StyleAccent.Render(name))
		} else {
			fmt.Printf("    %s\n", tui.StyleNormal.Render(name))
		}
	}
	return nil
}

func runDockerUse(_ *cobra.Command, args []string) error {
	ctx, err := resolveDockerContext(args)
	if err != nil {
		return err
	}
	out, runErr := exec.Command("docker", "context", "use", ctx).CombinedOutput()
	if runErr != nil {
		return fmt.Errorf("docker: %s", strings.TrimSpace(string(out)))
	}
	fmt.Printf("  %s docker: switched to %s\n", tui.StyleOK.Render("✓"), tui.StyleAccent.Render(ctx))
	return nil
}

func resolveDockerContext(args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	contexts, err := dockerContexts()
	if err != nil {
		return "", err
	}
	current, _ := dockerCurrent()
	items := make([]tui.SelectItem, len(contexts))
	for i, name := range contexts {
		subtitle := ""
		if name == current {
			subtitle = tui.StyleOK.Render("current")
		}
		items[i] = tui.SelectItem{Title: name, Subtitle: subtitle}
	}
	idx, err := tui.Select("Select Docker context", items)
	if err != nil {
		return "", err
	}
	if idx < 0 {
		return "", fmt.Errorf("cancelled")
	}
	return contexts[idx], nil
}

func dockerContexts() ([]string, error) {
	out, err := exec.Command("docker", "context", "ls", "--format", "{{.Name}}").Output()
	if err != nil {
		return nil, fmt.Errorf("docker context ls: %w", err)
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

func dockerCurrent() (string, error) {
	out, err := exec.Command("docker", "context", "show").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
