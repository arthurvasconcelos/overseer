package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/learning"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for AI assistant integration",
	Long: `Starts a local MCP (Model Context Protocol) server over stdio.

This command is launched automatically by an MCP-compatible AI assistant
(such as Claude Code) — you do not run it directly in your terminal.

The assistant reads the server address from its config, spawns this process
as a subprocess, and communicates with it over stdin/stdout using JSON-RPC.
You interact with the AI as normal; it calls overseer tools behind the scenes.

To register with a specific AI assistant:

  overseer claude mcp install    — register overseer in Claude Code`,
	RunE: runMCP,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

const mcpServerInstructions = `overseer is a personal local assistant server for development context, repository status, notes, command execution, and structured learning.

For learning workflows, prefer the dedicated learning tools over shell commands. Use learning_status, learning_due, learning_search, and learning_get for read-heavy context before mutating data. Use learning_draft to preview an entry and check for duplicates, and learning_add only when the user wants to capture a durable learning entry. Use learning_review only after a review rating is known or explicitly provided.

If you discover that a saved entry is wrong, fix it with learning_edit rather than archiving and recapturing: editing in place keeps the entry id, review history, and schedule, and a revision_note is surfaced at the next review. Reserve learning_archive for entries that should stop being reviewed entirely.

Tools named <plugin>_<tool> are contributed by external plugins and run that plugin's own command. Their output is whatever the plugin prints, so read it as text unless the description says otherwise.`

func runMCP(_ *cobra.Command, _ []string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}

	if isTerminal(os.Stdin) {
		fmt.Fprintln(os.Stderr, "overseer MCP server")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "This command is launched automatically by an AI assistant — do not run it directly.")
		fmt.Fprintln(os.Stderr, "To register with Claude Code, run:")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  overseer claude mcp install")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Waiting for MCP client on stdin. Press Ctrl+C to exit.")
		fmt.Fprintln(os.Stderr, "")
	}

	return server.ServeStdio(newMCPServer(self))
}

func newMCPServer(self string) *server.MCPServer {
	s := server.NewMCPServer("overseer", Version, server.WithInstructions(mcpServerInstructions))

	s.AddTool(
		mcp.NewTool("list_commands",
			mcp.WithDescription("List all available overseer commands with descriptions"),
		),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcpListCommands(ctx)
		},
	)

	s.AddTool(
		mcp.NewTool("run_prs",
			mcp.WithDescription("Fetch open pull requests and merge requests from configured GitHub and GitLab instances"),
		),
		mcpSubcmd(self, "prs"),
	)

	s.AddTool(
		mcp.NewTool("run_repos_status",
			mcp.WithDescription("Show git status for all managed repositories"),
		),
		mcpSubcmd(self, "repos", "status"),
	)

	s.AddTool(
		mcp.NewTool("get_config",
			mcp.WithDescription("Return the active overseer configuration as JSON"),
		),
		mcpSubcmd(self, "config"),
	)

	s.AddTool(
		mcp.NewTool("run_command",
			mcp.WithDescription("Run a shell command with secrets injected from 1Password. Optionally specify a GitLab instance, GitHub instance, or 1Password environment to inject credentials as environment variables before the command runs."),
			mcp.WithString("command",
				mcp.Required(),
				mcp.Description("Shell command to run (executed via sh -c)"),
			),
			mcp.WithString("gitlab",
				mcp.Description("GitLab instance name from config — injects GITLAB_TOKEN and GITLAB_HOST"),
			),
			mcp.WithString("github",
				mcp.Description("GitHub instance name from config — injects GITHUB_TOKEN"),
			),
			mcp.WithString("env",
				mcp.Description("1Password environment name from config (e.g. p24) — injects its secrets as env vars"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			toolArgs := req.GetArguments()
			command, _ := toolArgs["command"].(string)
			if command == "" {
				return mcp.NewToolResultError("command is required"), nil
			}
			overseerArgs := []string{"run"}
			if v, _ := toolArgs["gitlab"].(string); v != "" {
				overseerArgs = append(overseerArgs, "--gitlab", v)
			}
			if v, _ := toolArgs["github"].(string); v != "" {
				overseerArgs = append(overseerArgs, "--github", v)
			}
			if v, _ := toolArgs["env"].(string); v != "" {
				overseerArgs = append(overseerArgs, "--env", v)
			}
			overseerArgs = append(overseerArgs, "--", "sh", "-c", command)
			out, err := exec.CommandContext(ctx, self, overseerArgs...).CombinedOutput()
			output := strings.TrimSpace(string(out))
			if err != nil {
				if output != "" {
					return mcp.NewToolResultError(output), nil
				}
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(output), nil
		},
	)

	s.AddTool(
		mcp.NewTool("run_note_search",
			mcp.WithDescription("Search the Obsidian vault for notes matching a query"),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query string"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query, _ := req.GetArguments()["query"].(string)
			if query == "" {
				return mcp.NewToolResultError("query is required"), nil
			}
			return mcpExec(ctx, self, "note", "search", "--format", "json", query)
		},
	)

	s.AddTool(
		mcp.NewTool("run_journal_context",
			mcp.WithDescription("Gather one day's calendar events, commits, merge requests, Jira issues, captured Claude Code sessions and learning entries, plus the vault path of that day's daily note. Read-only — writes nothing. Sources that fail are reported in the warnings array rather than failing the call."),
			mcp.WithString("date",
				mcp.Description("Day to gather in YYYY-MM-DD form (default today)"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := []string{"journal", "context", "--format", "json"}
			if date, _ := req.GetArguments()["date"].(string); date != "" {
				args = append(args, "--date", date)
			}
			return mcpExec(ctx, self, args...)
		},
	)

	s.AddTool(
		mcp.NewTool("learning_add",
			mcp.WithDescription("Add a structured learning entry"),
			mcp.WithString("topic", mcp.Required(), mcp.Description("Learning topic")),
			mcp.WithString("description", mcp.Required(), mcp.Description("Learning entry description")),
			mcp.WithArray("quiz", mcp.Required(), mcp.Description("Quiz questions"), mcp.WithStringItems()),
			mcp.WithString("source", mcp.Description("Optional source URL, note, or context")),
		),
		mcpLearningAdd,
	)

	s.AddTool(
		mcp.NewTool("learning_draft",
			mcp.WithDescription("Validate and preview a learning entry without saving it"),
			mcp.WithString("topic", mcp.Required(), mcp.Description("Learning topic")),
			mcp.WithString("description", mcp.Required(), mcp.Description("Learning entry description")),
			mcp.WithArray("quiz", mcp.Required(), mcp.Description("Quiz questions"), mcp.WithStringItems()),
			mcp.WithString("source", mcp.Description("Optional source URL, note, or context")),
		),
		mcpLearningDraft,
	)

	s.AddTool(
		mcp.NewTool("learning_edit",
			mcp.WithDescription("Correct an existing learning entry in place, preserving its id, review history, and schedule. Only the fields you pass are changed; passing quiz replaces every question."),
			mcp.WithNumber("entry_id", mcp.Required(), mcp.Description("Learning entry ID")),
			mcp.WithString("topic", mcp.Description("Replacement topic")),
			mcp.WithString("description", mcp.Description("Replacement description")),
			mcp.WithArray("quiz", mcp.Description("Replacement quiz questions (replaces all existing questions)"), mcp.WithStringItems()),
			mcp.WithString("source", mcp.Description("Replacement source URL, note, or context")),
			mcp.WithString("revision_note", mcp.Description("Why the entry changed; surfaced at the next review")),
		),
		mcpLearningEdit,
	)

	s.AddTool(
		mcp.NewTool("learning_due",
			mcp.WithDescription("Return learning entries due for review as JSON"),
		),
		mcpLearningDue,
	)

	s.AddTool(
		mcp.NewTool("learning_search",
			mcp.WithDescription("Search learning entries"),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
		),
		mcpLearningSearch,
	)

	s.AddTool(
		mcp.NewTool("learning_get",
			mcp.WithDescription("Return one learning entry with review history"),
			mcp.WithNumber("entry_id", mcp.Required(), mcp.Description("Learning entry ID")),
		),
		mcpLearningGet,
	)

	s.AddTool(
		mcp.NewTool("learning_status",
			mcp.WithDescription("Return learning counts and upcoming schedule"),
		),
		mcpLearningStatus,
	)

	s.AddTool(
		mcp.NewTool("learning_review",
			mcp.WithDescription("Record a learning review and return the updated schedule"),
			mcp.WithNumber("entry_id", mcp.Required(), mcp.Description("Learning entry ID")),
			mcp.WithString("rating", mcp.Required(), mcp.Description("Review rating: missed, hard, good, easy")),
			mcp.WithString("notes", mcp.Description("Optional review notes")),
		),
		mcpLearningReview,
	)

	s.AddTool(
		mcp.NewTool("learning_archive",
			mcp.WithDescription("Archive an active learning entry and return the archived entry"),
			mcp.WithNumber("entry_id", mcp.Required(), mcp.Description("Learning entry ID")),
		),
		mcpLearningArchive,
	)

	registerPluginTools(s)

	s.AddResource(
		mcp.NewResource("overseer://learning/status", "Learning Status",
			mcp.WithResourceDescription("Current learning counts and upcoming schedule as JSON"),
			mcp.WithMIMEType("application/json"),
		),
		mcpLearningStatusResource,
	)

	s.AddResource(
		mcp.NewResource("overseer://learning/due", "Due Learning Reviews",
			mcp.WithResourceDescription("Learning entries due for review as JSON"),
			mcp.WithMIMEType("application/json"),
		),
		mcpLearningDueResource,
	)

	s.AddResourceTemplate(
		mcp.NewResourceTemplate("overseer://learning/entries/{entry_id}", "Learning Entry Detail",
			mcp.WithTemplateDescription("One learning entry with review history as JSON"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		mcpLearningEntryResource,
	)

	s.AddPrompt(
		mcp.NewPrompt("learning_review_session",
			mcp.WithPromptDescription("Prepare an assistant-led review session using due learning entries"),
		),
		mcpLearningReviewSessionPrompt,
	)

	return s
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && (fi.Mode()&os.ModeCharDevice) != 0
}

// mcpSubcmd returns a tool handler that runs an overseer subcommand with --format json.
func mcpSubcmd(self string, args ...string) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cmdArgs := make([]string, 0, len(args)+2)
	cmdArgs = append(cmdArgs, args...)
	cmdArgs = append(cmdArgs, "--format", "json")
	return func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcpExec(ctx, self, cmdArgs...)
	}
}

// mcpExec runs the overseer binary with given args and returns stdout as a tool result.
func mcpExec(ctx context.Context, self string, args ...string) (*mcp.CallToolResult, error) {
	out, err := exec.CommandContext(ctx, self, args...).Output()
	if err != nil {
		var msg string
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			msg = strings.TrimSpace(string(exitErr.Stderr))
		} else {
			msg = err.Error()
		}
		return mcp.NewToolResultError(msg), nil
	}
	return mcp.NewToolResultText(strings.TrimSpace(string(out))), nil
}

type mcpCommandEntry struct {
	Name  string `json:"name"`
	Short string `json:"short"`
}

// mcpListCommands introspects cobra to list all available top-level commands.
func mcpListCommands(_ context.Context) (*mcp.CallToolResult, error) {
	var cmds []mcpCommandEntry
	for _, c := range rootCmd.Commands() {
		if !c.IsAvailableCommand() {
			continue
		}
		cmds = append(cmds, mcpCommandEntry{Name: c.Name(), Short: c.Short})
	}
	if cmds == nil {
		cmds = []mcpCommandEntry{}
	}
	b, err := json.MarshalIndent(cmds, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

// registerPluginTools exposes the tools external plugins declare in their
// manifests. Each is named <plugin>_<tool> and answered by running the plugin
// binary, which is what lets a plugin reach an AI assistant without overseer
// carrying any knowledge of it.
func registerPluginTools(s *server.MCPServer) {
	for _, pt := range ExternalPluginTools() {
		s.AddTool(
			mcp.NewTool(pt.plugin.name+"_"+pt.name, mcp.WithDescription(pt.description)),
			func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				// plainEnv asks for unstyled output; stripANSI is the fallback
				// for a plugin that colours regardless. What reaches the client
				// is read by a model, which pays tokens for every escape code.
				out, err := runPluginCapturePlain(ctx, pt.plugin, pt.args...)
				out = strings.TrimSpace(stripANSI(out))
				if err != nil {
					return mcp.NewToolResultError(stripANSI(pluginToolError(out, err))), nil
				}
				return mcp.NewToolResultText(out), nil
			},
		)
	}
}

// pluginToolError picks the most useful of the three things a failed plugin run
// can leave behind: what it wrote to stderr, what it wrote to stdout, or nothing
// but the exit status.
func pluginToolError(stdout string, err error) string {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if msg := strings.TrimSpace(string(exitErr.Stderr)); msg != "" {
			return msg
		}
	}
	if stdout != "" {
		return stdout
	}
	return err.Error()
}

func mcpLearningAdd(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	input, result := mcpLearningAddInput(req)
	if result != nil {
		return result, nil
	}
	svc, err := learningService(ctx)
	if err != nil {
		return mcpToolError("learning_service_unavailable", err.Error(), nil), nil
	}
	defer svc.Close()
	entry, err := svc.Add(ctx, input)
	if errors.Is(err, learning.ErrDuplicateTopic) {
		return mcpToolError("duplicate_topic", err.Error(), map[string]any{"topic": input.Topic}), nil
	}
	if err != nil {
		return mcpToolError("learning_add_failed", err.Error(), nil), nil
	}
	return mcpJSON(entry)
}

func mcpLearningAddInput(req mcp.CallToolRequest) (learning.AddInput, *mcp.CallToolResult) {
	topic, err := req.RequireString("topic")
	if err != nil {
		return learning.AddInput{}, mcpToolError("invalid_input", "topic is required", nil)
	}
	description, err := req.RequireString("description")
	if err != nil {
		return learning.AddInput{}, mcpToolError("invalid_input", "description is required", nil)
	}
	quiz, err := req.RequireStringSlice("quiz")
	if err != nil || len(cleanLearningQuestions(quiz)) == 0 {
		return learning.AddInput{}, mcpToolError("invalid_input", "quiz must include at least one question", nil)
	}
	source := req.GetString("source", "")
	return learning.AddInput{Topic: topic, Source: source, Description: description, Questions: quiz}, nil
}

func mcpLearningDraft(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	input, result := mcpLearningAddInput(req)
	if result != nil {
		return result, nil
	}
	svc, err := learningService(ctx)
	if err != nil {
		return mcpToolError("learning_service_unavailable", err.Error(), nil), nil
	}
	defer svc.Close()
	draft, err := svc.Draft(ctx, input)
	if err != nil {
		return mcpToolError("learning_draft_failed", err.Error(), nil), nil
	}
	return mcpJSON(draft)
}

func mcpLearningEdit(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	entryID, err := req.RequireInt("entry_id")
	if err != nil {
		return mcpToolError("invalid_input", "entry_id is required", nil), nil
	}
	args := req.GetArguments()
	input := learning.EditInput{RevisionNote: req.GetString("revision_note", "")}
	if _, ok := args["topic"]; ok {
		topic := req.GetString("topic", "")
		input.Topic = &topic
	}
	if _, ok := args["source"]; ok {
		source := req.GetString("source", "")
		input.Source = &source
	}
	if _, ok := args["description"]; ok {
		description := req.GetString("description", "")
		input.Description = &description
	}
	if _, ok := args["quiz"]; ok {
		quiz := cleanLearningQuestions(req.GetStringSlice("quiz", nil))
		if len(quiz) == 0 {
			return mcpToolError("invalid_input", "quiz must include at least one question", map[string]any{"entry_id": entryID}), nil
		}
		input.Questions = &quiz
	}
	if input.Topic == nil && input.Source == nil && input.Description == nil && input.Questions == nil {
		return mcpToolError("invalid_input", "provide at least one of topic, source, description, quiz", map[string]any{"entry_id": entryID}), nil
	}
	svc, err := learningService(ctx)
	if err != nil {
		return mcpToolError("learning_service_unavailable", err.Error(), nil), nil
	}
	defer svc.Close()
	entry, err := svc.Update(ctx, int64(entryID), input)
	if errors.Is(err, sql.ErrNoRows) {
		return mcpToolError("entry_not_found", fmt.Sprintf("learning entry not found: %d", entryID), map[string]any{"entry_id": entryID}), nil
	}
	if errors.Is(err, learning.ErrDuplicateTopic) {
		return mcpToolError("duplicate_topic", err.Error(), map[string]any{"entry_id": entryID}), nil
	}
	if err != nil {
		return mcpToolError("learning_edit_failed", err.Error(), map[string]any{"entry_id": entryID}), nil
	}
	return mcpJSON(entry)
}

func mcpLearningDue(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, err := learningService(ctx)
	if err != nil {
		return mcpToolError("learning_service_unavailable", err.Error(), nil), nil
	}
	defer svc.Close()
	entries, err := svc.Due(ctx)
	if err != nil {
		return mcpToolError("learning_due_failed", err.Error(), nil), nil
	}
	return mcpJSON(entries)
}

func mcpLearningSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return mcpToolError("invalid_input", "query is required", nil), nil
	}
	svc, err := learningService(ctx)
	if err != nil {
		return mcpToolError("learning_service_unavailable", err.Error(), nil), nil
	}
	defer svc.Close()
	entries, err := svc.Search(ctx, query)
	if err != nil {
		return mcpToolError("learning_search_failed", err.Error(), nil), nil
	}
	return mcpJSON(entries)
}

func mcpLearningGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	entryID, err := req.RequireInt("entry_id")
	if err != nil {
		return mcpToolError("invalid_input", "entry_id is required", nil), nil
	}
	svc, err := learningService(ctx)
	if err != nil {
		return mcpToolError("learning_service_unavailable", err.Error(), nil), nil
	}
	defer svc.Close()
	detail, err := svc.Detail(ctx, int64(entryID))
	if errors.Is(err, sql.ErrNoRows) {
		return mcpToolError("entry_not_found", fmt.Sprintf("learning entry not found: %d", entryID), map[string]any{"entry_id": entryID}), nil
	}
	if err != nil {
		return mcpToolError("learning_get_failed", err.Error(), nil), nil
	}
	return mcpJSON(detail)
}

func mcpLearningStatus(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, err := learningService(ctx)
	if err != nil {
		return mcpToolError("learning_service_unavailable", err.Error(), nil), nil
	}
	defer svc.Close()
	status, err := svc.Status(ctx)
	if err != nil {
		return mcpToolError("learning_status_failed", err.Error(), nil), nil
	}
	return mcpJSON(status)
}

func mcpLearningReview(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	entryID, err := req.RequireInt("entry_id")
	if err != nil {
		return mcpToolError("invalid_input", "entry_id is required", nil), nil
	}
	rating, err := req.RequireString("rating")
	if err != nil {
		return mcpToolError("invalid_input", "rating is required", nil), nil
	}
	notes := req.GetString("notes", "")
	svc, err := learningService(ctx)
	if err != nil {
		return mcpToolError("learning_service_unavailable", err.Error(), nil), nil
	}
	defer svc.Close()
	review, err := svc.Review(ctx, int64(entryID), rating, notes)
	if err != nil {
		return mcpToolError("learning_review_failed", err.Error(), map[string]any{"entry_id": entryID, "rating": rating}), nil
	}
	return mcpJSON(review)
}

func mcpLearningArchive(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	entryID, err := req.RequireInt("entry_id")
	if err != nil {
		return mcpToolError("invalid_input", "entry_id is required", nil), nil
	}
	svc, err := learningService(ctx)
	if err != nil {
		return mcpToolError("learning_service_unavailable", err.Error(), nil), nil
	}
	defer svc.Close()
	entry, err := svc.Archive(ctx, int64(entryID))
	if err != nil && strings.Contains(err.Error(), "active learning entry not found") {
		return mcpToolError("entry_not_found", err.Error(), map[string]any{"entry_id": entryID}), nil
	}
	if err != nil {
		return mcpToolError("learning_archive_failed", err.Error(), map[string]any{"entry_id": entryID}), nil
	}
	return mcpJSON(entry)
}

func mcpLearningStatusResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	svc, err := learningService(ctx)
	if err != nil {
		return nil, err
	}
	defer svc.Close()
	status, err := svc.Status(ctx)
	if err != nil {
		return nil, err
	}
	return mcpJSONResource(req.Params.URI, status)
}

func mcpLearningDueResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	svc, err := learningService(ctx)
	if err != nil {
		return nil, err
	}
	defer svc.Close()
	entries, err := svc.Due(ctx)
	if err != nil {
		return nil, err
	}
	return mcpJSONResource(req.Params.URI, entries)
}

func mcpLearningEntryResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	const prefix = "overseer://learning/entries/"
	rawID := strings.TrimPrefix(req.Params.URI, prefix)
	if rawID == req.Params.URI || rawID == "" {
		return nil, fmt.Errorf("learning entry resource URI must match %s{entry_id}", prefix)
	}
	entryID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("entry_id must be an integer")
	}
	svc, err := learningService(ctx)
	if err != nil {
		return nil, err
	}
	defer svc.Close()
	detail, err := svc.Detail(ctx, entryID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("learning entry not found: %d", entryID)
	}
	if err != nil {
		return nil, err
	}
	return mcpJSONResource(req.Params.URI, detail)
}

func mcpLearningReviewSessionPrompt(_ context.Context, _ mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return mcp.NewGetPromptResult(
		"Assistant-led learning review session",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				mcp.RoleUser,
				mcp.NewTextContent("Run a short learning review session. Read the due learning reviews resource first, ask one quiz question at a time, wait for my answer, then record each review only after I provide or confirm a rating."),
			),
			mcp.NewPromptMessage(
				mcp.RoleAssistant,
				mcp.NewResourceLink("overseer://learning/due", "Due Learning Reviews", "Current learning entries due for review", "application/json"),
			),
		},
	), nil
}

func mcpJSON(v any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

type mcpErrorPayload struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func mcpToolError(code, message string, details map[string]any) *mcp.CallToolResult {
	b, err := json.MarshalIndent(mcpErrorPayload{Code: code, Message: message, Details: details}, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(message)
	}
	return mcp.NewToolResultError(string(b))
}

func mcpJSONResource(uri string, v any) ([]mcp.ResourceContents, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(b),
		},
	}, nil
}
