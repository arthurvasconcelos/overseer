package cmd

import (
	"fmt"
	"sort"
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

// rootGroups defines the display order and grouping for root-level built-in commands.
// Native plugin commands can appear in any group by setting the "overseer/group" annotation.
// Any command not listed here and without an annotation falls through to an "Other" section.
var rootGroups = []cmdGroup{
	{"Setup", []string{"setup", "brain", "brew"}},
	{"Daily", []string{"daily", "standup", "prs", "note", "status"}},
	{"Dev", []string{"run", "repos", "git", "env", "ssh"}},
	{"AI", []string{"context", "mcp"}},
	{"System", []string{"accounts", "config", "plugins", "notify", "update", "completion"}},
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

	// Build the full group list: start from rootGroups, then merge in any native
	// plugin commands that declare overseer/group. If the declared group name
	// matches an existing group it is appended there; otherwise a new group is
	// added at the end (sorted among other new groups).
	type resolvedGroup struct {
		title string
		cmds  []*cobra.Command
	}
	groupIndex := map[string]int{}
	var groups []resolvedGroup
	for _, rg := range rootGroups {
		groupIndex[rg.title] = len(groups)
		groups = append(groups, resolvedGroup{title: rg.title})
	}

	rendered := map[string]bool{}
	var newGroupNames []string
	for name, c := range byName {
		g := c.Annotations["overseer/group"]
		if g == "" {
			continue
		}
		rendered[name] = true
		if idx, ok := groupIndex[g]; ok {
			groups[idx].cmds = append(groups[idx].cmds, c)
		} else {
			groupIndex[g] = len(groups)
			groups = append(groups, resolvedGroup{title: g, cmds: []*cobra.Command{c}})
			newGroupNames = append(newGroupNames, g)
		}
	}

	// Fill static group commands and mark rendered.
	for i, rg := range rootGroups {
		for _, name := range rg.commands {
			if c, ok := byName[name]; ok {
				groups[i].cmds = append(groups[i].cmds, c)
				rendered[name] = true
			}
		}
	}

	// Sort any newly created groups (stable order for static ones is preserved).
	sort.Strings(newGroupNames)

	for _, g := range groups {
		if len(g.cmds) == 0 {
			continue
		}
		fmt.Println()
		fmt.Println(tui.StyleDim.Render(g.title + ":"))
		printCmdListAligned(g.cmds, maxLen, false)
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
