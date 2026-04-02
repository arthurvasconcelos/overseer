package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var accountsCmd = &cobra.Command{
	Use:   "accounts",
	Short: "List 1Password accounts and their IDs for use in config",
	Long: `Lists all 1Password accounts signed into the op CLI.

The USER ID shown here is what you set as op_account in your config
(~/.config/overseer/config.yaml) when a secret lives in a specific account:

    integrations:
      jira:
        - name: work
          email: "op://Vault/Item/field"
          op_account: <USER ID>`,
	RunE: runAccounts,
}

func init() {
	rootCmd.AddCommand(accountsCmd)
}

type opAccount struct {
	URL    string `json:"url"`
	Email  string `json:"email"`
	UserID string `json:"user_uuid"`
}

func runAccounts(_ *cobra.Command, _ []string) error {
	out, err := exec.Command("op", "account", "list", "--format=json").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("op account list: %s", exitErr.Stderr)
		}
		return fmt.Errorf("op account list: %w", err)
	}

	var accounts []opAccount
	if err := json.Unmarshal(out, &accounts); err != nil {
		return fmt.Errorf("parsing accounts: %w", err)
	}

	header := fmt.Sprintf("  %-30s  %-40s  %s", "ACCOUNT", "EMAIL", "USER ID (use as op_account)")
	fmt.Println(tui.StyleMuted.Render(header))
	for _, a := range accounts {
		fmt.Printf("  %s  %s  %s\n",
			tui.StyleAccent.Render(fmt.Sprintf("%-30s", a.URL)),
			tui.StyleNormal.Render(fmt.Sprintf("%-40s", a.Email)),
			tui.StyleDim.Render(a.UserID),
		)
	}

	return nil
}
