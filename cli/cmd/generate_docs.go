package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var generateDocsCmd = &cobra.Command{
	Use:    "generate-docs [output-dir]",
	Short:  "Generate man pages for all commands",
	Hidden: true,
	Args:   cobra.MaximumNArgs(1),
	RunE:   runGenerateDocs,
}

func init() {
	rootCmd.AddCommand(generateDocsCmd)
}

func runGenerateDocs(_ *cobra.Command, args []string) error {
	dir := "./man"
	if len(args) > 0 {
		dir = args[0]
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Fix the date so man pages are reproducible in CI.
	epoch := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	header := &doc.GenManHeader{
		Title:   "OVERSEER",
		Section: "1",
		Date:    &epoch,
		Source:  "overseer " + Version,
		Manual:  "overseer Manual",
	}

	if err := doc.GenManTree(rootCmd, header, dir); err != nil {
		return fmt.Errorf("generating man pages: %w", err)
	}

	fmt.Printf("man pages written to %s/\n", dir)
	return nil
}
