package learning

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	StatusActive   = "active"
	StatusArchived = "archived"

	RatingMissed = "missed"
	RatingHard   = "hard"
	RatingGood   = "good"
	RatingEasy   = "easy"
)

var ErrDuplicateTopic = errors.New("active learning topic already exists")

type Service struct {
	db  *sql.DB
	now func() time.Time
}

type Entry struct {
	ID           int64      `json:"id"`
	Topic        string     `json:"topic"`
	Source       string     `json:"source,omitempty"`
	Description  string     `json:"description"`
	Status       string     `json:"status"`
	Questions    []Question `json:"quiz"`
	NextDueAt    time.Time  `json:"next_due_at"`
	IntervalDays int        `json:"interval_days"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type Question struct {
	Position int    `json:"position"`
	Question string `json:"question"`
}

type AddInput struct {
	Topic          string
	Source         string
	Description    string
	Questions      []string
	AllowDuplicate bool
}

type Review struct {
	ID            int64     `json:"id"`
	EntryID       int64     `json:"entry_id"`
	ReviewedAt    time.Time `json:"reviewed_at"`
	Rating        string    `json:"rating"`
	Notes         string    `json:"notes,omitempty"`
	PreviousDueAt time.Time `json:"previous_due_at"`
	NextDueAt     time.Time `json:"next_due_at"`
	IntervalDays  int       `json:"interval_days"`
	Entry         *Entry    `json:"entry,omitempty"`
}

type Status struct {
	ActiveEntries       int     `json:"active_entries"`
	DueCount            int     `json:"due_count"`
	UpcomingReviews     []Entry `json:"upcoming_reviews"`
	RecentReviewCount   int     `json:"recent_review_count"`
	RecentReviewDays    int     `json:"recent_review_days"`
	DatabaseInitialized bool    `json:"database_initialized"`
}

func DBPath(brainOverseerPath string) string {
	return filepath.Join(brainOverseerPath, "learning.db")
}

func Open(ctx context.Context, path string) (*Service, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("creating learning database directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening learning database: %w", err)
	}
	s := &Service{db: db, now: time.Now}
	if err := s.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func NewForDB(db *sql.DB, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{db: db, now: now}
}

func (s *Service) Close() error {
	return s.db.Close()
}

func (s *Service) Migrate(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	var version int
	if err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("checking learning database version: %w", err)
	}
	if version < 1 {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		stmts := []string{
			`CREATE TABLE IF NOT EXISTS learning_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			topic TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			next_due_at TEXT NOT NULL,
			interval_days INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
			`CREATE TABLE IF NOT EXISTS learning_questions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entry_id INTEGER NOT NULL,
			question TEXT NOT NULL,
			position INTEGER NOT NULL,
			FOREIGN KEY(entry_id) REFERENCES learning_entries(id) ON DELETE CASCADE
		)`,
			`CREATE INDEX IF NOT EXISTS learning_questions_entry_idx ON learning_questions(entry_id, position)`,
			`CREATE TABLE IF NOT EXISTS learning_reviews (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entry_id INTEGER NOT NULL,
			reviewed_at TEXT NOT NULL,
			rating TEXT NOT NULL,
			notes TEXT NOT NULL DEFAULT '',
			previous_due_at TEXT NOT NULL,
			next_due_at TEXT NOT NULL,
			interval_days INTEGER NOT NULL,
			FOREIGN KEY(entry_id) REFERENCES learning_entries(id) ON DELETE CASCADE
		)`,
			`CREATE INDEX IF NOT EXISTS learning_reviews_entry_idx ON learning_reviews(entry_id, reviewed_at DESC)`,
			`PRAGMA user_version = 1`,
		}
		for _, stmt := range stmts {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migrating learning database: %w", err)
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		version = 1
	}
	if version < 2 {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DROP INDEX IF EXISTS learning_entries_active_topic_unique`); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrating learning database: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `PRAGMA user_version = 2`); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrating learning database: %w", err)
		}
		return tx.Commit()
	}
	return nil
}

func (s *Service) Add(ctx context.Context, input AddInput) (Entry, error) {
	input.Topic = strings.TrimSpace(input.Topic)
	input.Source = strings.TrimSpace(input.Source)
	input.Description = strings.TrimSpace(input.Description)
	input.Questions = cleanQuestions(input.Questions)
	if err := validateAdd(input); err != nil {
		return Entry{}, err
	}

	now := truncateSecond(s.now())
	due := now.AddDate(0, 0, 1)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Entry{}, err
	}
	defer tx.Rollback()

	if !input.AllowDuplicate {
		var existing int
		err := tx.QueryRowContext(ctx, `SELECT id FROM learning_entries WHERE status = ? AND topic = ? LIMIT 1`, StatusActive, input.Topic).Scan(&existing)
		if err == nil {
			return Entry{}, fmt.Errorf("%w: %s", ErrDuplicateTopic, input.Topic)
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return Entry{}, err
		}
	}

	res, err := tx.ExecContext(ctx, `INSERT INTO learning_entries
		(topic, source, description, status, next_due_at, interval_days, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		input.Topic, input.Source, input.Description, StatusActive, formatTime(due), 1, formatTime(now), formatTime(now))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return Entry{}, fmt.Errorf("%w: %s", ErrDuplicateTopic, input.Topic)
		}
		return Entry{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Entry{}, err
	}
	for i, q := range input.Questions {
		if _, err := tx.ExecContext(ctx, `INSERT INTO learning_questions (entry_id, question, position) VALUES (?, ?, ?)`, id, q, i+1); err != nil {
			return Entry{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Entry{}, err
	}
	return s.Get(ctx, id)
}

func (s *Service) Get(ctx context.Context, id int64) (Entry, error) {
	rows, err := s.db.QueryContext(ctx, entrySelectSQL()+` WHERE e.id = ?`, id)
	if err != nil {
		return Entry{}, err
	}
	defer rows.Close()
	entries, err := scanEntries(ctx, s.db, rows)
	if err != nil {
		return Entry{}, err
	}
	if len(entries) == 0 {
		return Entry{}, sql.ErrNoRows
	}
	return entries[0], nil
}

func (s *Service) Due(ctx context.Context) ([]Entry, error) {
	todayEnd := endOfDay(s.now())
	rows, err := s.db.QueryContext(ctx, entrySelectSQL()+` WHERE e.status = ? AND e.next_due_at <= ? ORDER BY e.next_due_at ASC, e.id ASC`, StatusActive, formatTime(todayEnd))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(ctx, s.db, rows)
}

func (s *Service) Search(ctx context.Context, query string) ([]Entry, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	like := "%" + escapeLike(query) + "%"
	rows, err := s.db.QueryContext(ctx, entrySelectSQL()+`
		WHERE e.status = ? AND (
			e.topic LIKE ? ESCAPE '\'
			OR e.source LIKE ? ESCAPE '\'
			OR e.description LIKE ? ESCAPE '\'
			OR EXISTS (
				SELECT 1 FROM learning_questions q
				WHERE q.entry_id = e.id AND q.question LIKE ? ESCAPE '\'
			)
		)
		ORDER BY e.updated_at DESC, e.id DESC`, StatusActive, like, like, like, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(ctx, s.db, rows)
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	st := Status{RecentReviewDays: 7, DatabaseInitialized: true}
	todayEnd := endOfDay(s.now())
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM learning_entries WHERE status = ?`, StatusActive).Scan(&st.ActiveEntries); err != nil {
		return st, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM learning_entries WHERE status = ? AND next_due_at <= ?`, StatusActive, formatTime(todayEnd)).Scan(&st.DueCount); err != nil {
		return st, err
	}
	since := truncateSecond(s.now()).AddDate(0, 0, -st.RecentReviewDays)
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM learning_reviews WHERE reviewed_at >= ?`, formatTime(since)).Scan(&st.RecentReviewCount); err != nil {
		return st, err
	}
	rows, err := s.db.QueryContext(ctx, entrySelectSQL()+` WHERE e.status = ? AND e.next_due_at > ? ORDER BY e.next_due_at ASC, e.id ASC LIMIT 5`, StatusActive, formatTime(todayEnd))
	if err != nil {
		return st, err
	}
	defer rows.Close()
	upcoming, err := scanEntries(ctx, s.db, rows)
	if err != nil {
		return st, err
	}
	st.UpcomingReviews = upcoming
	return st, nil
}

func (s *Service) Review(ctx context.Context, entryID int64, rating, notes string) (Review, error) {
	rating = strings.TrimSpace(strings.ToLower(rating))
	notes = strings.TrimSpace(notes)
	if !ValidRating(rating) {
		return Review{}, fmt.Errorf("rating must be one of: missed, hard, good, easy")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Review{}, err
	}
	defer tx.Rollback()

	var previousDueAt string
	var currentInterval int
	err = tx.QueryRowContext(ctx, `SELECT next_due_at, interval_days FROM learning_entries WHERE id = ? AND status = ?`, entryID, StatusActive).Scan(&previousDueAt, &currentInterval)
	if errors.Is(err, sql.ErrNoRows) {
		return Review{}, fmt.Errorf("active learning entry not found: %d", entryID)
	}
	if err != nil {
		return Review{}, err
	}

	interval := NextIntervalDays(currentInterval, rating)
	now := truncateSecond(s.now())
	nextDue := now.AddDate(0, 0, interval)
	res, err := tx.ExecContext(ctx, `INSERT INTO learning_reviews
		(entry_id, reviewed_at, rating, notes, previous_due_at, next_due_at, interval_days)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entryID, formatTime(now), rating, notes, previousDueAt, formatTime(nextDue), interval)
	if err != nil {
		return Review{}, err
	}
	reviewID, err := res.LastInsertId()
	if err != nil {
		return Review{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE learning_entries SET next_due_at = ?, interval_days = ?, updated_at = ? WHERE id = ?`, formatTime(nextDue), interval, formatTime(now), entryID); err != nil {
		return Review{}, err
	}
	if err := tx.Commit(); err != nil {
		return Review{}, err
	}
	entry, err := s.Get(ctx, entryID)
	if err != nil {
		return Review{}, err
	}
	prev, err := parseTime(previousDueAt)
	if err != nil {
		return Review{}, err
	}
	return Review{
		ID: reviewID, EntryID: entryID, ReviewedAt: now, Rating: rating, Notes: notes,
		PreviousDueAt: prev, NextDueAt: nextDue, IntervalDays: interval, Entry: &entry,
	}, nil
}

func ValidRating(rating string) bool {
	switch rating {
	case RatingMissed, RatingHard, RatingGood, RatingEasy:
		return true
	default:
		return false
	}
}

func NextIntervalDays(current int, rating string) int {
	if current < 1 {
		current = 1
	}
	switch rating {
	case RatingMissed:
		return 1
	case RatingHard:
		if current == 1 {
			return 2
		}
		return current + 1
	case RatingGood:
		return current * 2
	case RatingEasy:
		return current*3 + 1
	default:
		return current
	}
}

func validateAdd(input AddInput) error {
	if input.Topic == "" {
		return fmt.Errorf("topic is required")
	}
	if input.Description == "" {
		return fmt.Errorf("description is required")
	}
	if len(input.Questions) == 0 {
		return fmt.Errorf("at least one quiz question is required")
	}
	return nil
}

func cleanQuestions(in []string) []string {
	var out []string
	for _, q := range in {
		q = strings.TrimSpace(q)
		if q != "" {
			out = append(out, q)
		}
	}
	return out
}

func entrySelectSQL() string {
	return `SELECT e.id, e.topic, e.source, e.description, e.status, e.next_due_at, e.interval_days, e.created_at, e.updated_at FROM learning_entries e`
}

func scanEntries(ctx context.Context, db *sql.DB, rows *sql.Rows) ([]Entry, error) {
	var entries []Entry
	for rows.Next() {
		var e Entry
		var due, created, updated string
		if err := rows.Scan(&e.ID, &e.Topic, &e.Source, &e.Description, &e.Status, &due, &e.IntervalDays, &created, &updated); err != nil {
			return nil, err
		}
		var err error
		if e.NextDueAt, err = parseTime(due); err != nil {
			return nil, err
		}
		if e.CreatedAt, err = parseTime(created); err != nil {
			return nil, err
		}
		if e.UpdatedAt, err = parseTime(updated); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range entries {
		qs, err := questionsForEntry(ctx, db, entries[i].ID)
		if err != nil {
			return nil, err
		}
		entries[i].Questions = qs
	}
	if entries == nil {
		return []Entry{}, nil
	}
	return entries, nil
}

func questionsForEntry(ctx context.Context, db *sql.DB, id int64) ([]Question, error) {
	rows, err := db.QueryContext(ctx, `SELECT position, question FROM learning_questions WHERE entry_id = ? ORDER BY position ASC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var qs []Question
	for rows.Next() {
		var q Question
		if err := rows.Scan(&q.Position, &q.Question); err != nil {
			return nil, err
		}
		qs = append(qs, q)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if qs == nil {
		return []Question{}, nil
	}
	return qs, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

func truncateSecond(t time.Time) time.Time {
	return t.UTC().Truncate(time.Second)
}

func endOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 23, 59, 59, 0, t.Location()).UTC()
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
