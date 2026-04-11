package cmd

import (
	"fmt"

	"github.com/arthurvasconcelos/overseer/internal/notify"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var notifyCmd = &cobra.Command{
	Use:   "notify <title> <message>",
	Short: "Fire a native OS desktop notification",
	Args:  cobra.ExactArgs(2),
	RunE:  runNotify,
}

var notifySubtitle string

func init() {
	notifyCmd.Flags().StringVar(&notifySubtitle, "subtitle", "", "Optional subtitle (macOS only)")
	rootCmd.AddCommand(notifyCmd)
}

func runNotify(_ *cobra.Command, args []string) error {
	title, message := args[0], args[1]
	if err := notify.Send(title, message, notifySubtitle); err != nil {
		return err
	}
	fmt.Println(tui.StyleOK.Render("notification sent"))
	return nil
}
