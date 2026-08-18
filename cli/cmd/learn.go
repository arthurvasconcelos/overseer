package cmd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/learning"
	"github.com/arthurvasconcelos/overseer/internal/output"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var learnAddOpts struct {
	source         string
	description    string
	questions      []string
	allowDuplicate bool
}

var learnEditOpts struct {
	topic          string
	source         string
	description    string
	questions      []string
	note           string
	allowDuplicate bool
}

var learnReviewOpts struct {
	entryID int64
	rating  string
	notes   string
}

var learnCmd = &cobra.Command{
	Use:   "learn",
	Short: "Capture and review structured learning entries",
}

var learnAddCmd = &cobra.Command{
	Use:   "add [topic]",
	Short: "Add a learning entry",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runLearnAdd,
}

var learnDueCmd = &cobra.Command{
	Use:   "due",
	Short: "Show learning entries due for review",
	RunE:  runLearnDue,
}

var learnReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Review due learning entries",
	RunE:  runLearnReview,
}

var learnStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show learning system status",
	RunE:  runLearnStatus,
}

var learnSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search learning entries",
	Args:  cobra.ExactArgs(1),
	RunE:  runLearnSearch,
}

var learnShowCmd = &cobra.Command{
	Use:   "show <entry-id>",
	Short: "Show one learning entry with review history",
	Args:  cobra.ExactArgs(1),
	RunE:  runLearnShow,
}

var learnEditCmd = &cobra.Command{
	Use:   "edit <entry-id>",
	Short: "Edit a learning entry in place",
	Long: `Edit a learning entry without losing its id, review history, or schedule.

Only the fields you pass are changed. Passing --quiz replaces the whole quiz.
With no field flags, the current values are offered for editing interactively.`,
	Args: cobra.ExactArgs(1),
	RunE: runLearnEdit,
}

var learnArchiveCmd = &cobra.Command{
	Use:   "archive <entry-id>",
	Short: "Archive a learning entry",
	Args:  cobra.ExactArgs(1),
	RunE:  runLearnArchive,
}

func init() {
	learnAddCmd.Flags().StringVar(&learnAddOpts.source, "source", "", "Source URL, note, or context")
	learnAddCmd.Flags().StringVar(&learnAddOpts.description, "description", "", "Learning entry description")
	learnAddCmd.Flags().StringArrayVar(&learnAddOpts.questions, "quiz", nil, "Quiz question (repeatable)")
	learnAddCmd.Flags().BoolVar(&learnAddOpts.allowDuplicate, "allow-duplicate", false, "Allow duplicate active topics")
	learnEditCmd.Flags().StringVar(&learnEditOpts.topic, "topic", "", "Replace the topic")
	learnEditCmd.Flags().StringVar(&learnEditOpts.source, "source", "", "Replace the source")
	learnEditCmd.Flags().StringVar(&learnEditOpts.description, "description", "", "Replace the description")
	learnEditCmd.Flags().StringArrayVar(&learnEditOpts.questions, "quiz", nil, "Replace the quiz with this question (repeatable)")
	learnEditCmd.Flags().StringVar(&learnEditOpts.note, "note", "", "Why the entry changed")
	learnEditCmd.Flags().BoolVar(&learnEditOpts.allowDuplicate, "allow-duplicate", false, "Allow the new topic to duplicate another active entry")
	learnReviewCmd.Flags().Int64Var(&learnReviewOpts.entryID, "entry-id", 0, "Entry ID to review")
	learnReviewCmd.Flags().StringVar(&learnReviewOpts.rating, "rating", "", "Review rating: missed, hard, good, easy")
	learnReviewCmd.Flags().StringVar(&learnReviewOpts.notes, "notes", "", "Optional review notes")
	learnCmd.AddCommand(learnAddCmd)
	learnCmd.AddCommand(learnDueCmd)
	learnCmd.AddCommand(learnReviewCmd)
	learnCmd.AddCommand(learnStatusCmd)
	learnCmd.AddCommand(learnSearchCmd)
	learnCmd.AddCommand(learnShowCmd)
	learnCmd.AddCommand(learnEditCmd)
	learnCmd.AddCommand(learnArchiveCmd)
	rootCmd.AddCommand(learnCmd)
}

func learningService(ctx context.Context) (*learning.Service, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return learning.Open(ctx, learning.DBPath(config.BrainOverseerPath(cfg)))
}

func commandContext(cmd *cobra.Command) context.Context {
	ctx := cmd.Context()
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func runLearnAdd(cmd *cobra.Command, args []string) error {
	ctx := commandContext(cmd)
	svc, err := learningService(ctx)
	if err != nil {
		return err
	}
	defer svc.Close()

	topic := ""
	if len(args) > 0 {
		topic = args[0]
	}
	source := learnAddOpts.source
	description := learnAddOpts.description
	questions := append([]string(nil), learnAddOpts.questions...)

	supplied := topic != "" && description != "" && len(cleanLearningQuestions(questions)) > 0
	if topic == "" {
		topic, err = tui.Prompt("topic", "", "")
		if err != nil {
			return err
		}
	}
	if source == "" && !supplied {
		source, err = tui.Prompt("source (optional)", "", "")
		if err != nil {
			return err
		}
	}
	if description == "" {
		description, err = tui.Prompt("description", "", "")
		if err != nil {
			return err
		}
	}
	for len(cleanLearningQuestions(questions)) == 0 {
		q, err := tui.Prompt("quiz question", "", "")
		if err != nil {
			return err
		}
		questions = append(questions, q)
		for {
			more, err := tui.Confirm("add another quiz question?")
			if err != nil {
				return err
			}
			if !more {
				break
			}
			q, err := tui.Prompt("quiz question", "", "")
			if err != nil {
				return err
			}
			questions = append(questions, q)
		}
	}

	entry, err := svc.Add(ctx, learning.AddInput{
		Topic: topic, Source: source, Description: description, Questions: questions, AllowDuplicate: learnAddOpts.allowDuplicate,
	})
	if err != nil {
		return err
	}
	if output.Format == "json" {
		return output.PrintJSON(entry)
	}
	fmt.Printf("%s  added learning entry #%d: %s\n", tui.StyleOK.Render("✓"), entry.ID, tui.StyleAccent.Render(entry.Topic))
	fmt.Printf("   next review: %s\n", entry.NextDueAt.Local().Format("2006-01-02"))
	return nil
}

func runLearnDue(cmd *cobra.Command, _ []string) error {
	ctx := commandContext(cmd)
	svc, err := learningService(ctx)
	if err != nil {
		return err
	}
	defer svc.Close()
	entries, err := svc.Due(ctx)
	if err != nil {
		return err
	}
	if output.Format == "json" {
		return output.PrintJSON(entries)
	}
	if len(entries) == 0 {
		fmt.Println(tui.StyleMuted.Render("no learning reviews due"))
		return nil
	}
	fmt.Println(tui.SectionHeader("learning due", fmt.Sprintf("%d item(s)", len(entries))))
	for _, e := range entries {
		fmt.Printf("  #%d  %s  %s\n", e.ID, tui.StyleAccent.Render(e.Topic), tui.StyleMuted.Render(e.NextDueAt.Local().Format("2006-01-02")))
	}
	return nil
}

func runLearnReview(cmd *cobra.Command, _ []string) error {
	ctx := commandContext(cmd)
	svc, err := learningService(ctx)
	if err != nil {
		return err
	}
	defer svc.Close()

	var entry learning.Entry
	if learnReviewOpts.entryID > 0 {
		entry, err = svc.Get(ctx, learnReviewOpts.entryID)
		if err != nil {
			return err
		}
	} else {
		due, err := svc.Due(ctx)
		if err != nil {
			return err
		}
		if len(due) == 0 {
			fmt.Println(tui.StyleMuted.Render("no learning reviews due"))
			return nil
		}
		items := make([]tui.SelectItem, 0, len(due))
		for _, e := range due {
			items = append(items, tui.SelectItem{Title: e.Topic, Subtitle: fmt.Sprintf("#%d due %s", e.ID, e.NextDueAt.Local().Format("2006-01-02"))})
		}
		idx, err := tui.Select("select learning entry", items)
		if err != nil {
			return err
		}
		if idx < 0 {
			return nil
		}
		entry = due[idx]
	}

	interactiveReview := learnReviewOpts.rating == ""
	if interactiveReview {
		fmt.Println(tui.SectionHeader("learning review", entry.Topic))
		fmt.Println(entry.Description)
		fmt.Println()
		for _, q := range entry.Questions {
			if _, err := tui.Prompt(q.Question, "", "answer privately, then press enter"); err != nil {
				return err
			}
		}
		items := []tui.SelectItem{
			{Title: learning.RatingGood, Subtitle: "remembered"},
			{Title: learning.RatingHard, Subtitle: "remembered with effort"},
			{Title: learning.RatingEasy, Subtitle: "too easy"},
			{Title: learning.RatingMissed, Subtitle: "missed"},
		}
		idx, err := tui.Select("rating", items)
		if err != nil {
			return err
		}
		if idx < 0 {
			return nil
		}
		learnReviewOpts.rating = items[idx].Title
	}
	if interactiveReview && learnReviewOpts.notes == "" {
		learnReviewOpts.notes, err = tui.Prompt("review notes (optional)", "", "")
		if err != nil {
			return err
		}
	}
	review, err := svc.Review(ctx, entry.ID, learnReviewOpts.rating, learnReviewOpts.notes)
	if err != nil {
		return err
	}
	if output.Format == "json" {
		return output.PrintJSON(review)
	}
	fmt.Printf("%s  reviewed #%d: %s\n", tui.StyleOK.Render("✓"), entry.ID, tui.StyleAccent.Render(entry.Topic))
	fmt.Printf("   next review: %s (%d day interval)\n", review.NextDueAt.Local().Format("2006-01-02"), review.IntervalDays)
	return nil
}

func runLearnStatus(cmd *cobra.Command, _ []string) error {
	ctx := commandContext(cmd)
	svc, err := learningService(ctx)
	if err != nil {
		return err
	}
	defer svc.Close()
	status, err := svc.Status(ctx)
	if err != nil {
		return err
	}
	if output.Format == "json" {
		return output.PrintJSON(status)
	}
	fmt.Println(tui.SectionHeader("learning status", ""))
	fmt.Printf("  active entries: %d\n", status.ActiveEntries)
	fmt.Printf("  due now:        %d\n", status.DueCount)
	fmt.Printf("  reviews / 7d:   %d\n", status.RecentReviewCount)
	if len(status.UpcomingReviews) > 0 {
		fmt.Println()
		fmt.Println("  upcoming:")
		for _, e := range status.UpcomingReviews {
			fmt.Printf("    #%d  %s  %s\n", e.ID, e.Topic, e.NextDueAt.Local().Format("2006-01-02"))
		}
	}
	return nil
}

func runLearnSearch(cmd *cobra.Command, args []string) error {
	ctx := commandContext(cmd)
	svc, err := learningService(ctx)
	if err != nil {
		return err
	}
	defer svc.Close()
	entries, err := svc.Search(ctx, args[0])
	if err != nil {
		return err
	}
	if output.Format == "json" {
		return output.PrintJSON(entries)
	}
	if len(entries) == 0 {
		fmt.Println(tui.StyleMuted.Render("no learning entries found"))
		return nil
	}
	fmt.Println(tui.SectionHeader("learning search", fmt.Sprintf("%d result(s)", len(entries))))
	for _, e := range entries {
		fmt.Printf("  #%d  %s\n", e.ID, tui.StyleAccent.Render(e.Topic))
		if e.Source != "" {
			fmt.Printf("      %s\n", tui.StyleMuted.Render(e.Source))
		}
	}
	return nil
}

func runLearnShow(cmd *cobra.Command, args []string) error {
	ctx := commandContext(cmd)
	entryID, err := parseLearningEntryID(args[0])
	if err != nil {
		return err
	}
	svc, err := learningService(ctx)
	if err != nil {
		return err
	}
	defer svc.Close()
	detail, err := svc.Detail(ctx, entryID)
	if err != nil {
		return err
	}
	if output.Format == "json" {
		return output.PrintJSON(detail)
	}
	printLearningDetail(detail)
	return nil
}

func runLearnEdit(cmd *cobra.Command, args []string) error {
	ctx := commandContext(cmd)
	entryID, err := parseLearningEntryID(args[0])
	if err != nil {
		return err
	}
	svc, err := learningService(ctx)
	if err != nil {
		return err
	}
	defer svc.Close()

	entry, err := svc.Get(ctx, entryID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("learning entry not found: %d", entryID)
	}
	if err != nil {
		return err
	}

	input := learning.EditInput{
		RevisionNote:   learnEditOpts.note,
		AllowDuplicate: learnEditOpts.allowDuplicate,
	}
	flags := cmd.Flags()
	if flags.Changed("topic") {
		input.Topic = &learnEditOpts.topic
	}
	if flags.Changed("source") {
		input.Source = &learnEditOpts.source
	}
	if flags.Changed("description") {
		input.Description = &learnEditOpts.description
	}
	if flags.Changed("quiz") {
		questions := cleanLearningQuestions(learnEditOpts.questions)
		input.Questions = &questions
	}

	if input.Topic == nil && input.Source == nil && input.Description == nil && input.Questions == nil {
		if err := promptLearningEdit(entry, &input); err != nil {
			return err
		}
	}

	updated, err := svc.Update(ctx, entryID, input)
	if err != nil {
		return err
	}
	if output.Format == "json" {
		return output.PrintJSON(updated)
	}
	fmt.Printf("%s  edited #%d: %s\n", tui.StyleOK.Render("✓"), updated.ID, tui.StyleAccent.Render(updated.Topic))
	if updated.RevisionNote != "" {
		fmt.Printf("   note: %s\n", tui.StyleMuted.Render(updated.RevisionNote))
	}
	fmt.Printf("   next review unchanged: %s (%d day interval)\n", updated.NextDueAt.Local().Format("2006-01-02"), updated.IntervalDays)
	return nil
}

func promptLearningEdit(entry learning.Entry, input *learning.EditInput) error {
	fmt.Println(tui.SectionHeader("edit learning entry", fmt.Sprintf("#%d", entry.ID)))

	topic, err := tui.Prompt("topic", entry.Topic, "")
	if err != nil {
		return err
	}
	input.Topic = &topic

	source, err := tui.Prompt("source (optional)", entry.Source, "")
	if err != nil {
		return err
	}
	input.Source = &source

	description, err := tui.PromptText("description", entry.Description, "")
	if err != nil {
		return err
	}
	input.Description = &description

	rewriteQuiz, err := tui.Confirm("rewrite quiz questions?")
	if err != nil {
		return err
	}
	if rewriteQuiz {
		questions := make([]string, 0, len(entry.Questions))
		for _, q := range entry.Questions {
			revised, err := tui.Prompt(fmt.Sprintf("quiz question %d (empty to drop)", q.Position), q.Question, "")
			if err != nil {
				return err
			}
			if strings.TrimSpace(revised) != "" {
				questions = append(questions, revised)
			}
		}
		for {
			another, err := tui.Confirm("add another quiz question?")
			if err != nil {
				return err
			}
			if !another {
				break
			}
			question, err := tui.Prompt("quiz question", "", "")
			if err != nil {
				return err
			}
			if strings.TrimSpace(question) != "" {
				questions = append(questions, question)
			}
		}
		input.Questions = &questions
	}

	if input.RevisionNote == "" {
		note, err := tui.Prompt("revision note (optional)", "", "")
		if err != nil {
			return err
		}
		input.RevisionNote = note
	}
	return nil
}

func runLearnArchive(cmd *cobra.Command, args []string) error {
	ctx := commandContext(cmd)
	entryID, err := parseLearningEntryID(args[0])
	if err != nil {
		return err
	}
	svc, err := learningService(ctx)
	if err != nil {
		return err
	}
	defer svc.Close()
	entry, err := svc.Archive(ctx, entryID)
	if err != nil {
		return err
	}
	if output.Format == "json" {
		return output.PrintJSON(entry)
	}
	fmt.Printf("%s  archived learning entry #%d: %s\n", tui.StyleOK.Render("✓"), entry.ID, tui.StyleAccent.Render(entry.Topic))
	return nil
}

func printLearningDetail(detail learning.EntryDetail) {
	entry := detail.Entry
	fmt.Println(tui.SectionHeader("learning entry", fmt.Sprintf("#%d", entry.ID)))
	fmt.Println(tui.StyleAccent.Render(entry.Topic))
	if entry.Source != "" {
		fmt.Printf("source: %s\n", tui.StyleMuted.Render(entry.Source))
	}
	fmt.Printf("status: %s\n", entry.Status)
	fmt.Printf("next review: %s (%d day interval)\n", entry.NextDueAt.Local().Format("2006-01-02"), entry.IntervalDays)
	fmt.Println()
	fmt.Println(entry.Description)
	if len(entry.Questions) > 0 {
		fmt.Println()
		fmt.Println("quiz:")
		for _, q := range entry.Questions {
			fmt.Printf("  %d. %s\n", q.Position, q.Question)
		}
	}
	printLearningReviewSummary(detail.ReviewSummary)
	if len(detail.Reviews) > 0 {
		fmt.Println()
		fmt.Println("reviews:")
		for _, r := range detail.Reviews {
			line := fmt.Sprintf("  #%d  %s  %s  next %s (%d day interval)", r.ID, r.ReviewedAt.Local().Format("2006-01-02"), r.Rating, r.NextDueAt.Local().Format("2006-01-02"), r.IntervalDays)
			fmt.Println(line)
			if r.Notes != "" {
				fmt.Printf("      %s\n", tui.StyleMuted.Render(r.Notes))
			}
		}
	}
}

func printLearningReviewSummary(summary learning.ReviewSummary) {
	if summary.TotalReviews == 0 {
		return
	}
	fmt.Println()
	fmt.Println("review summary:")
	fmt.Printf("  total: %d  missed: %d  hard: %d  good: %d  easy: %d\n", summary.TotalReviews, summary.MissedCount, summary.HardCount, summary.GoodCount, summary.EasyCount)
	if summary.LastReviewedAt != nil {
		fmt.Printf("  last reviewed: %s\n", summary.LastReviewedAt.Local().Format("2006-01-02"))
	}
	if summary.ConsecutiveMissed > 0 || summary.ConsecutiveStruggled > 0 {
		fmt.Printf("  current streak: %d missed, %d struggled\n", summary.ConsecutiveMissed, summary.ConsecutiveStruggled)
	}
	if summary.LeechCandidate {
		fmt.Printf("  %s\n", tui.StyleWarn.Render("leech candidate: consider rewriting this entry"))
	}
}

func parseLearningEntryID(raw string) (int64, error) {
	entryID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("entry-id must be an integer")
	}
	return entryID, nil
}

func cleanLearningQuestions(in []string) []string {
	var out []string
	for _, q := range in {
		q = strings.TrimSpace(q)
		if q != "" {
			out = append(out, q)
		}
	}
	return out
}
