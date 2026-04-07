package cmd

import (
	"fmt"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.SetHelpFunc(styledHelp)
}

func styledHelp(cmd *cobra.Command, _ []string) {
	fmt.Println(tui.SectionHeader(cmd.CommandPath(), cmd.Short))

	if cmd.Long != "" {
		fmt.Println()
		fmt.Println(tui.StyleMuted.Render(strings.TrimSpace(cmd.Long)))
	}

	fmt.Println()
	if cmd.Runnable() {
		fmt.Println(tui.StyleDim.Render("Usage:") + "  " + tui.StyleNormal.Render(cmd.UseLine()))
	}
	if cmd.HasAvailableSubCommands() {
		fmt.Println(tui.StyleDim.Render("Usage:") + "  " + tui.StyleNormal.Render(cmd.CommandPath()+" [command]"))
	}

	if cmd.HasAvailableSubCommands() {
		var builtins, plugins []*cobra.Command
		for _, sub := range cmd.Commands() {
			if !sub.IsAvailableCommand() {
				continue
			}
			if sub.Annotations["overseer/plugin"] == "true" {
				plugins = append(plugins, sub)
			} else {
				builtins = append(builtins, sub)
			}
		}
		if len(builtins) > 0 {
			fmt.Println()
			fmt.Println(tui.StyleDim.Render("Commands:"))
			printCmdList(builtins, false)
		}
		if len(plugins) > 0 {
			fmt.Println()
			fmt.Println(tui.StyleDim.Render("Plugins:"))
			printCmdList(plugins, true)
		}
	}

	if cmd.HasAvailableLocalFlags() {
		fmt.Println()
		fmt.Println(tui.StyleDim.Render("Flags:"))
		fmt.Print(cmd.LocalFlags().FlagUsages())
	}

	if cmd.HasAvailableInheritedFlags() {
		fmt.Println()
		fmt.Println(tui.StyleDim.Render("Global Flags:"))
		fmt.Print(cmd.InheritedFlags().FlagUsages())
	}

	if cmd.HasAvailableSubCommands() {
		fmt.Println()
		fmt.Println(tui.StyleMuted.Render(`Use "` + cmd.CommandPath() + ` [command] --help" for more information.`))
	}
}

func printCmdList(cmds []*cobra.Command, isPlugin bool) {
	maxLen := 0
	for _, c := range cmds {
		if len(c.Name()) > maxLen {
			maxLen = len(c.Name())
		}
	}
	for _, c := range cmds {
		pad := strings.Repeat(" ", maxLen-len(c.Name())+2)
		name := tui.StyleAccent.Render(c.Name())
		var desc string
		if isPlugin && strings.HasPrefix(c.Short, "Plugin: ") {
			desc = tui.StyleMuted.Render("(no description)")
		} else {
			desc = tui.StyleNormal.Render(c.Short)
		}
		fmt.Println("  " + name + pad + desc)
	}
}
