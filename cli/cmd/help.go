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

type cmdGroup struct {
	title    string
	commands []string
}

// rootGroups defines the display order and grouping for root-level commands.
// Any command not listed here falls through to an "Other" section.
var rootGroups = []cmdGroup{
	{"Setup", []string{"setup", "brain", "accounts", "config"}},
	{"Daily", []string{"daily", "prs", "note", "status"}},
	{"Dev", []string{"brew", "repos", "git", "run"}},
	{"System", []string{"update", "completion"}},
}

func styledHelp(cmd *cobra.Command, _ []string) {
	isRoot := cmd.Parent() == nil

	if isRoot {
		fmt.Println(tui.Logo(Version))
		fmt.Println()
		fmt.Println(tui.StyleDim.Render("Usage:") + "  " + tui.StyleNormal.Render("overseer [command]"))
	} else {
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
	}

	if cmd.HasAvailableSubCommands() {
		if isRoot {
			printRootCommands(cmd)
		} else {
			printFlatCommands(cmd)
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

// printRootCommands renders commands in defined groups with a shared column
// width so names stay aligned across all sections.
func printRootCommands(cmd *cobra.Command) {
	byName := map[string]*cobra.Command{}
	var plugins []*cobra.Command
	for _, sub := range cmd.Commands() {
		if !sub.IsAvailableCommand() {
			continue
		}
		if sub.Annotations["overseer/plugin"] == "true" {
			plugins = append(plugins, sub)
		} else {
			byName[sub.Name()] = sub
		}
	}

	// Single max-width across all groups keeps columns aligned.
	maxLen := 0
	for name := range byName {
		if len(name) > maxLen {
			maxLen = len(name)
		}
	}
	for _, p := range plugins {
		if len(p.Name()) > maxLen {
			maxLen = len(p.Name())
		}
	}

	rendered := map[string]bool{}
	for _, group := range rootGroups {
		var groupCmds []*cobra.Command
		for _, name := range group.commands {
			if c, ok := byName[name]; ok {
				groupCmds = append(groupCmds, c)
				rendered[name] = true
			}
		}
		if len(groupCmds) == 0 {
			continue
		}
		fmt.Println()
		fmt.Println(tui.StyleDim.Render(group.title + ":"))
		printCmdListAligned(groupCmds, maxLen, false)
	}

	// Safety net for any commands added later that aren't in a group.
	var ungrouped []*cobra.Command
	for name, c := range byName {
		if !rendered[name] {
			ungrouped = append(ungrouped, c)
		}
	}
	if len(ungrouped) > 0 {
		fmt.Println()
		fmt.Println(tui.StyleDim.Render("Other:"))
		printCmdListAligned(ungrouped, maxLen, false)
	}

	if len(plugins) > 0 {
		fmt.Println()
		fmt.Println(tui.StyleDim.Render("Plugins:"))
		printCmdListAligned(plugins, maxLen, true)
	}
}

// printFlatCommands renders subcommands as a simple flat list (for non-root commands).
func printFlatCommands(cmd *cobra.Command) {
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

	maxLen := 0
	for _, c := range append(builtins, plugins...) {
		if len(c.Name()) > maxLen {
			maxLen = len(c.Name())
		}
	}

	if len(builtins) > 0 {
		fmt.Println()
		fmt.Println(tui.StyleDim.Render("Commands:"))
		printCmdListAligned(builtins, maxLen, false)
	}
	if len(plugins) > 0 {
		fmt.Println()
		fmt.Println(tui.StyleDim.Render("Plugins:"))
		printCmdListAligned(plugins, maxLen, true)
	}
}

func printCmdListAligned(cmds []*cobra.Command, maxLen int, isPlugin bool) {
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
