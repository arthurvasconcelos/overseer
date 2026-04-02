package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"

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

	fmt.Printf("%-30s  %-40s  %s\n", "ACCOUNT", "EMAIL", "USER ID (use as op_account)")
	fmt.Printf("%-30s  %-40s  %s\n", "-------", "-----", "--------------------------")
	for _, a := range accounts {
		fmt.Printf("%-30s  %-40s  %s\n", a.URL, a.Email, a.UserID)
	}

	return nil
}
