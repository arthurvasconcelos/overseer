package gcal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googlecalendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// joinURLRe matches Zoom, Google Meet, and Microsoft Teams URLs in plain text.
var joinURLRe = regexp.MustCompile(
	`https?://(?:[a-zA-Z0-9-]+\.)?(?:zoom\.us/j|meet\.google\.com|teams\.microsoft\.com/l/meetup-join)/[^\s<>"]+`,
)

// Event is a minimal representation of a calendar event.
type Event struct {
	Title   string
	Start   time.Time
	End     time.Time
	AllDay  bool
	JoinURL string // video conference join link, if available
}

// tokenPath returns the local path where the OAuth token for a named account
// is cached between runs.
func tokenPath(accountName string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "overseer", fmt.Sprintf("google-%s-token.json", accountName)), nil
}

// Client holds an authenticated Google Calendar service.
type Client struct {
	svc *googlecalendar.Service
}

// New creates a Client using OAuth2. credentialsJSON is the raw content of the
// OAuth client credentials JSON from Google Cloud Console. accountName is used
// to cache the token file so each account authenticates independently.
//
// On the first call a browser will open for consent; subsequent calls load the
// cached token automatically.
func New(ctx context.Context, credentialsJSON []byte, accountName string) (*Client, error) {
	cfg, err := google.ConfigFromJSON(credentialsJSON, googlecalendar.CalendarReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("gcal: parsing credentials: %w", err)
	}

	httpClient, err := httpClientFromConfig(ctx, cfg, accountName)
	if err != nil {
		return nil, err
	}

	svc, err := googlecalendar.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("gcal: creating service: %w", err)
	}

	return &Client{svc: svc}, nil
}

// TokenInfo describes the cached OAuth token state for an account.
type TokenInfo struct {
	Present bool
	Valid   bool
	Expiry  time.Time
}

// TokenStatus reads the cached token file and returns its state without
// triggering any auth flow.
func TokenStatus(accountName string) (TokenInfo, error) {
	path, err := tokenPath(accountName)
	if err != nil {
		return TokenInfo{}, err
	}
	token, err := loadToken(path)
	if err != nil {
		if os.IsNotExist(err) {
			return TokenInfo{Present: false}, nil
		}
		return TokenInfo{}, fmt.Errorf("gcal: reading token: %w", err)
	}
	return TokenInfo{
		Present: true,
		Valid:   token.Valid(),
		Expiry:  token.Expiry,
	}, nil
}

// WeekEvents returns all events for the current week (today through 7 days from now).
func (c *Client) WeekEvents(ctx context.Context) ([]Event, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfWeek := startOfDay.Add(7 * 24 * time.Hour)

	result, err := c.svc.Events.List("primary").
		Context(ctx).
		TimeMin(startOfDay.Format(time.RFC3339)).
		TimeMax(endOfWeek.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		Do()
	if err != nil {
		return nil, fmt.Errorf("gcal: listing events: %w", err)
	}

	var events []Event
	for _, item := range result.Items {
		e := Event{Title: item.Summary}
		if item.Start.DateTime != "" {
			e.Start, _ = time.Parse(time.RFC3339, item.Start.DateTime)
			e.End, _ = time.Parse(time.RFC3339, item.End.DateTime)
		} else {
			e.AllDay = true
			e.Start, _ = time.Parse("2006-01-02", item.Start.Date)
		}
		e.JoinURL = extractJoinURL(item)
		events = append(events, e)
	}
	return events, nil
}

// NextEvent returns the next upcoming event from now, or nil if none.
func (c *Client) NextEvent(ctx context.Context) (*Event, error) {
	now := time.Now()
	endWindow := now.Add(24 * time.Hour)

	result, err := c.svc.Events.List("primary").
		Context(ctx).
		TimeMin(now.Format(time.RFC3339)).
		TimeMax(endWindow.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		MaxResults(1).
		Do()
	if err != nil {
		return nil, fmt.Errorf("gcal: listing events: %w", err)
	}
	if len(result.Items) == 0 {
		return nil, nil
	}
	item := result.Items[0]
	e := Event{Title: item.Summary}
	if item.Start.DateTime != "" {
		e.Start, _ = time.Parse(time.RFC3339, item.Start.DateTime)
		e.End, _ = time.Parse(time.RFC3339, item.End.DateTime)
	} else {
		e.AllDay = true
		e.Start, _ = time.Parse("2006-01-02", item.Start.Date)
	}
	e.JoinURL = extractJoinURL(item)
	return &e, nil
}

// TodaysEvents returns all events for today across all calendars.
func (c *Client) TodaysEvents(ctx context.Context) ([]Event, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	result, err := c.svc.Events.List("primary").
		Context(ctx).
		TimeMin(startOfDay.Format(time.RFC3339)).
		TimeMax(endOfDay.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		Do()
	if err != nil {
		return nil, fmt.Errorf("gcal: listing events: %w", err)
	}

	var events []Event
	for _, item := range result.Items {
		e := Event{Title: item.Summary}
		if item.Start.DateTime != "" {
			e.Start, _ = time.Parse(time.RFC3339, item.Start.DateTime)
			e.End, _ = time.Parse(time.RFC3339, item.End.DateTime)
		} else {
			e.AllDay = true
			e.Start, _ = time.Parse("2006-01-02", item.Start.Date)
		}
		e.JoinURL = extractJoinURL(item)
		events = append(events, e)
	}
	return events, nil
}

// extractJoinURL returns a video conference join URL for a calendar event.
// Prefers conferenceData entry points; falls back to a regex over the description.
func extractJoinURL(item *googlecalendar.Event) string {
	if item.ConferenceData != nil {
		for _, ep := range item.ConferenceData.EntryPoints {
			if ep.EntryPointType == "video" && ep.Uri != "" {
				return ep.Uri
			}
		}
	}
	if item.Description != "" {
		if m := joinURLRe.FindString(item.Description); m != "" {
			return m
		}
	}
	return ""
}

func httpClientFromConfig(ctx context.Context, cfg *oauth2.Config, accountName string) (*http.Client, error) {
	tokenFile, err := tokenPath(accountName)
	if err != nil {
		return nil, err
	}

	token, err := loadToken(tokenFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("gcal: loading cached token: %w", err)
		}
		// No cached token yet — run the browser consent flow.
		token, err = runAuthFlow(ctx, cfg)
		if err != nil {
			return nil, err
		}
		if err := saveToken(tokenFile, token); err != nil {
			return nil, err
		}
	}

	return cfg.Client(ctx, token), nil
}

func runAuthFlow(ctx context.Context, cfg *oauth2.Config) (*oauth2.Token, error) {
	cfg.RedirectURL = "urn:ietf:wg:oauth:2.0:oob"
	url := cfg.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("\nOpen this URL in your browser to authorize Google Calendar access:\n\n  %s\n\nPaste the authorization code: ", url)

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		return nil, fmt.Errorf("gcal: reading auth code: %w", err)
	}

	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("gcal: exchanging auth code: %w", err)
	}
	return token, nil
}

func loadToken(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var token oauth2.Token
	if err := json.NewDecoder(f).Decode(&token); err != nil {
		return nil, err
	}
	return &token, nil
}

func saveToken(path string, token *oauth2.Token) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("gcal: saving token: %w", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}
