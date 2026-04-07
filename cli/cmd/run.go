package cmd

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	"github.com/spf13/cobra"
)

var (
	runGitLabName string
	runGitHubName string
	runEnvID      string
)

var runCmd = &cobra.Command{
	Use:   "run -- <cmd> [args...]",
	Short: "Run a command with secrets injected from config or 1Password",
	Long: `Resolve secrets and inject them as environment variables before running a command.

Examples:
  overseer run --gitlab work -- curl -H "PRIVATE-TOKEN: $GITLAB_TOKEN" https://$GITLAB_HOST/api/v4/...
  overseer run --github personal -- gh repo list
  overseer run --env my-env -- make deploy`,
	Args: cobra.MinimumNArgs(1),
	RunE: runRun,
}

func init() {
	runCmd.Flags().StringVar(&runGitLabName, "gitlab", "", "GitLab instance name from config (injects GITLAB_TOKEN, GITLAB_HOST)")
	runCmd.Flags().StringVar(&runGitHubName, "github", "", "GitHub instance name from config (injects GITHUB_TOKEN)")
	runCmd.Flags().StringVar(&runEnvID, "env", "", "1Password environment ID (delegates to `op run --environment`; mutually exclusive with --gitlab/--github)")
	rootCmd.AddCommand(runCmd)
}

func runRun(_ *cobra.Command, args []string) error {
	if runEnvID != "" && (runGitLabName != "" || runGitHubName != "") {
		return fmt.Errorf("--env cannot be combined with --gitlab or --github")
	}

	if runEnvID != "" {
		return secrets.RunWithEnv(runEnvID, args...)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	var extraEnv []string

	if runGitLabName != "" {
		env, err := resolveGitLabEnv(cfg, runGitLabName)
		if err != nil {
			return err
		}
		extraEnv = append(extraEnv, env...)
	}

	if runGitHubName != "" {
		env, err := resolveGitHubEnv(cfg, runGitHubName)
		if err != nil {
			return err
		}
		extraEnv = append(extraEnv, env...)
	}

	return execWithEnv(args, extraEnv)
}

func resolveGitLabEnv(cfg *config.Config, name string) ([]string, error) {
	for _, inst := range cfg.Integrations.GitLab {
		if inst.Name == name {
			token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
			if err != nil {
				return nil, fmt.Errorf("gitlab/%s: %w", name, err)
			}
			host := "gitlab.com"
			if inst.BaseURL != "" {
				if u, err := url.Parse(inst.BaseURL); err == nil && u.Host != "" {
					host = u.Host
				}
			}
			return []string{
				"GITLAB_TOKEN=" + token,
				"GITLAB_HOST=" + host,
			}, nil
		}
	}
	return nil, fmt.Errorf("no GitLab instance named %q in config", name)
}

func resolveGitHubEnv(cfg *config.Config, name string) ([]string, error) {
	for _, inst := range cfg.Integrations.GitHub {
		if inst.Name == name {
			token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
			if err != nil {
				return nil, fmt.Errorf("github/%s: %w", name, err)
			}
			return []string{
				"GITHUB_TOKEN=" + token,
			}, nil
		}
	}
	return nil, fmt.Errorf("no GitHub instance named %q in config", name)
}

func execWithEnv(args []string, extraEnv []string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), extraEnv...)
	return cmd.Run()
}
