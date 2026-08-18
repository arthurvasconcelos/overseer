package slack

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack"
)

// slackID matches a raw Slack user, workspace-user or DM identifier.
var slackID = regexp.MustCompile(`^[UWD][A-Z0-9]{6,}$`)

// Mention text is kept short for at-a-glance listings and longer when a day is
// being reviewed after the fact, where the message has to stand on its own.
const (
	shortMentionText = 100
	fullMentionText  = 400
)

// Client wraps the slack-go client.
type Client struct {
	api     *slack.Client
	userAPI *slack.Client // optional — user token for search
}

// New creates a Slack client using a bot token.
func New(token string) *Client {
	return &Client{api: slack.New(token)}
}

// NewWithUserToken creates a Slack client with both a bot token and a user token.
func NewWithUserToken(token, userToken string) *Client {
	return &Client{
		api:     slack.New(token),
		userAPI: slack.New(userToken),
	}
}

// Mention represents a message where the user, or a watched usergroup, was
// mentioned.
type Mention struct {
	Channel   string    `json:"channel"`
	Direct    bool      `json:"direct,omitempty"`
	Author    string    `json:"author,omitempty"`
	Text      string    `json:"text"`
	Permalink string    `json:"permalink,omitempty"`
	Time      time.Time `json:"time"`
}

// Ping verifies the token by calling auth.test.
// Returns the bot's display name.
func (c *Client) Ping() (string, error) {
	info, err := c.api.AuthTest()
	if err != nil {
		return "", fmt.Errorf("slack: ping: %w", err)
	}
	return info.User, nil
}

// Mentions returns recent messages that mention the user or any of the given
// usergroup handles.
func (c *Client) Mentions(groupHandles []string) ([]Mention, error) {
	mentions, err := c.mentions(context.Background(), time.Time{}, groupHandles)
	if err != nil {
		return nil, err
	}
	return truncateMentions(mentions, shortMentionText), nil
}

// MentionsOn returns the mentions posted on a single day, oldest first. It is
// the day-scoped form used when reviewing a day after the fact.
func (c *Client) MentionsOn(ctx context.Context, day time.Time, groupHandles []string) ([]Mention, error) {
	mentions, err := c.mentions(ctx, day, groupHandles)
	if err != nil {
		return nil, err
	}
	sort.Slice(mentions, func(i, j int) bool { return mentions[i].Time.Before(mentions[j].Time) })
	return truncateMentions(mentions, fullMentionText), nil
}

// mentions uses the Search API when a user token is configured (it sees all
// channels); otherwise it falls back to scanning bot-joined channels. A zero
// day means "recent" rather than a specific date.
func (c *Client) mentions(ctx context.Context, day time.Time, groupHandles []string) ([]Mention, error) {
	if c.userAPI != nil {
		return c.searchMentions(ctx, day, groupHandles)
	}
	return c.scanMentions(ctx, day, groupHandles)
}

// searchMentions uses the Slack Search API (requires user token + search:read).
func (c *Client) searchMentions(ctx context.Context, day time.Time, groupHandles []string) ([]Mention, error) {
	info, err := c.userAPI.AuthTestContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("slack: user auth test: %w", err)
	}
	queries := []string{"@" + info.User}
	for _, h := range groupHandles {
		queries = append(queries, "@"+h)
	}

	params := slack.SearchParameters{Count: 20}
	if !day.IsZero() {
		// Slack resolves on: against the workspace timezone.
		suffix := " on:" + day.Format("2006-01-02")
		for i := range queries {
			queries[i] += suffix
		}
		params = slack.SearchParameters{Count: 100, Sort: "timestamp", SortDirection: "asc"}
	}

	seen := map[string]bool{}
	names := map[string]string{}
	var mentions []Mention
	for _, q := range queries {
		result, err := c.userAPI.SearchMessagesContext(ctx, q, params)
		if err != nil {
			return nil, fmt.Errorf("slack: search %q: %w", q, err)
		}
		for _, msg := range result.Matches {
			key := msg.Channel.ID + msg.Timestamp
			if seen[key] {
				continue
			}
			seen[key] = true
			author := msg.Username
			if author == "" {
				author = c.displayName(ctx, msg.User, names)
			}
			channel, direct := c.channelLabel(ctx, msg.Channel.ID, msg.Channel.Name, names)
			mentions = append(mentions, Mention{
				Channel:   channel,
				Direct:    direct,
				Author:    author,
				Text:      msg.Text,
				Permalink: msg.Permalink,
				Time:      messageTime(msg.Timestamp),
			})
		}
	}
	return mentions, nil
}

// channelLabel names a conversation the way a person would recognise it. Direct
// messages come back from search without a channel name, so they are resolved
// to the person instead of left as a raw ID.
func (c *Client) channelLabel(ctx context.Context, id, name string, cache map[string]string) (string, bool) {
	// Search returns the counterpart's user ID as the "name" of a DM, which
	// real channel names — always lowercase — can never look like.
	if name != "" && !slackID.MatchString(name) {
		return name, false
	}
	// Either field can carry the user ID, depending on which endpoint the
	// mention came from; the DM channel ID needs a second lookup.
	for _, candidate := range []string{name, id} {
		if strings.HasPrefix(candidate, "U") || strings.HasPrefix(candidate, "W") {
			return c.displayName(ctx, candidate, cache), true
		}
	}
	if strings.HasPrefix(id, "D") {
		info, err := c.lookupAPI().GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{ChannelID: id})
		if err == nil && info.User != "" {
			return c.displayName(ctx, info.User, cache), true
		}
	}
	if id != "" {
		return id, true
	}
	return name, true
}

// displayName resolves a Slack user ID, falling back to the ID when the token
// lacks users:read. Results are cached for the life of the call.
func (c *Client) displayName(ctx context.Context, id string, cache map[string]string) string {
	if id == "" {
		return ""
	}
	if cached, ok := cache[id]; ok {
		return cached
	}
	label := id
	if user, err := c.lookupAPI().GetUserInfoContext(ctx, id); err == nil {
		if user.Profile.DisplayName != "" {
			label = user.Profile.DisplayName
		} else if user.RealName != "" {
			label = user.RealName
		} else if user.Name != "" {
			label = user.Name
		}
	}
	cache[id] = label
	return label
}

// lookupAPI prefers the user token for directory lookups: it sees the same
// conversations the search did.
func (c *Client) lookupAPI() *slack.Client {
	if c.userAPI != nil {
		return c.userAPI
	}
	return c.api
}

// scanMentions scans history of bot-joined channels (fallback when no user token).
func (c *Client) scanMentions(ctx context.Context, day time.Time, groupHandles []string) ([]Mention, error) {
	info, err := c.api.AuthTestContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("slack: auth test: %w", err)
	}
	userID := info.UserID

	groupIDs, err := c.resolveGroupIDs(ctx, groupHandles)
	if err != nil {
		groupIDs = nil
	}

	channels, _, err := c.api.GetConversationsContext(ctx, &slack.GetConversationsParameters{
		Types:           []string{"public_channel", "private_channel", "im", "mpim"},
		ExcludeArchived: true,
		Limit:           100,
	})
	if err != nil {
		return nil, fmt.Errorf("slack: listing channels: %w", err)
	}

	history := slack.GetConversationHistoryParameters{Limit: 20}
	if !day.IsZero() {
		start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
		history.Limit = 200
		history.Oldest = slackTimestamp(start)
		history.Latest = slackTimestamp(start.AddDate(0, 0, 1))
	}

	var mentions []Mention
	for _, ch := range channels {
		if len(mentions) >= 50 {
			break
		}
		if !ch.IsMember {
			continue
		}
		params := history
		params.ChannelID = ch.ID
		page, err := c.api.GetConversationHistoryContext(ctx, &params)
		if err != nil {
			continue
		}
		name := ch.Name
		if name == "" {
			name = ch.ID
		}
		for _, msg := range page.Messages {
			if isMentioned(msg.Text, userID, groupIDs) {
				mentions = append(mentions, Mention{
					Channel: name,
					Author:  msg.Username,
					Text:    msg.Text,
					Time:    messageTime(msg.Timestamp),
				})
			}
		}
	}
	return mentions, nil
}

// messageTime converts a Slack "1755500000.123456" timestamp to local time.
// An unparseable timestamp yields the zero time rather than an error: a mention
// is still worth reporting without its clock.
func messageTime(ts string) time.Time {
	seconds, err := strconv.ParseInt(strings.SplitN(ts, ".", 2)[0], 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(seconds, 0).Local()
}

func slackTimestamp(t time.Time) string {
	return strconv.FormatInt(t.Unix(), 10) + ".000000"
}

func truncateMentions(mentions []Mention, max int) []Mention {
	for i := range mentions {
		mentions[i].Text = truncate(strings.TrimSpace(mentions[i].Text), max)
	}
	return mentions
}

// isMentioned reports whether a message text contains a direct user mention
// or a mention of any of the given usergroup IDs.
func isMentioned(text, userID string, groupIDs []string) bool {
	if strings.Contains(text, "<@"+userID+">") {
		return true
	}
	for _, gid := range groupIDs {
		if strings.Contains(text, "<!subteam^"+gid) {
			return true
		}
	}
	return false
}

// resolveGroupIDs maps a list of usergroup handle names to their Slack IDs.
// Handles that are not found are silently skipped.
func (c *Client) resolveGroupIDs(ctx context.Context, handles []string) ([]string, error) {
	if len(handles) == 0 {
		return nil, nil
	}
	groups, err := c.api.GetUserGroupsContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("slack: listing usergroups: %w", err)
	}
	want := make(map[string]bool, len(handles))
	for _, h := range handles {
		want[h] = true
	}
	var ids []string
	for _, g := range groups {
		if want[g.Handle] {
			ids = append(ids, g.ID)
		}
	}
	return ids, nil
}

// Channel is a minimal representation of a Slack channel the bot is a member of.
type Channel struct {
	ID      string
	Name    string
	Private bool
}

// Channels returns the channels (public, private, DM) the bot is a member of.
func (c *Client) Channels() ([]Channel, error) {
	convs, _, err := c.api.GetConversations(&slack.GetConversationsParameters{
		Types:           []string{"public_channel", "private_channel", "im", "mpim"},
		ExcludeArchived: true,
		Limit:           200,
	})
	if err != nil {
		return nil, fmt.Errorf("slack: listing channels: %w", err)
	}
	var channels []Channel
	for _, ch := range convs {
		if !ch.IsMember {
			continue
		}
		name := ch.Name
		if name == "" {
			name = ch.ID
		}
		channels = append(channels, Channel{
			ID:      ch.ID,
			Name:    name,
			Private: ch.IsPrivate,
		})
	}
	return channels, nil
}

// Send posts a message to a channel or DM in the workspace.
func (c *Client) Send(channel, text string) error {
	_, _, err := c.api.PostMessage(channel, slack.MsgOptionText(text, false))
	if err != nil {
		return fmt.Errorf("slack: send: %w", err)
	}
	return nil
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
