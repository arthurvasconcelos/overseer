package cmd

// Blank imports trigger each native plugin's init() function,
// which calls nativeplugin.Register() to enroll the plugin in the registry.
import (
	_ "github.com/arthurvasconcelos/overseer/internal/plugins/claude"
	_ "github.com/arthurvasconcelos/overseer/internal/plugins/devctx"
	_ "github.com/arthurvasconcelos/overseer/internal/plugins/github"
	_ "github.com/arthurvasconcelos/overseer/internal/plugins/gitlab"
	_ "github.com/arthurvasconcelos/overseer/internal/plugins/google"
	_ "github.com/arthurvasconcelos/overseer/internal/plugins/jira"
	_ "github.com/arthurvasconcelos/overseer/internal/plugins/obsidian"
	_ "github.com/arthurvasconcelos/overseer/internal/plugins/slack"
)
