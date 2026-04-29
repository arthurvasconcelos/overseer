package slack

import (
	"fmt"
	"strings"

	"github.com/slack-go/slack"
)

// Client wraps the slack-go client.
type Client struct {
	api      *slack.Client
	userAPI  *slack.Client // optional — user token for search
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

// Mention represents a message where the bot user was mentioned.
type Mention struct {
	Channel string
	Text    string
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
// usergroup handles. When a user token is configured it uses the Search API
// (sees all channels); otherwise falls back to scanning bot-joined channels.
func (c *Client) Mentions(groupHandles []string) ([]Mention, error) {
	if c.userAPI != nil {
		return c.searchMentions(groupHandles)
	}
	return c.scanMentions(groupHandles)
}

// searchMentions uses the Slack Search API (requires user token + search:read).
func (c *Client) searchMentions(groupHandles []string) ([]Mention, error) {
	info, err := c.userAPI.AuthTest()
	if err != nil {
		return nil, fmt.Errorf("slack: user auth test: %w", err)
	}
	queries := []string{"@" + info.User}
	for _, h := range groupHandles {
		queries = append(queries, "@"+h)
	}

	seen := map[string]bool{}
	var mentions []Mention
	for _, q := range queries {
		result, err := c.userAPI.SearchMessages(q, slack.SearchParameters{Count: 20})
		if err != nil {
			return nil, fmt.Errorf("slack: search %q: %w", q, err)
		}
		for _, msg := range result.Matches {
			key := msg.Channel.ID + msg.Timestamp
			if seen[key] {
				continue
			}
			seen[key] = true
			name := msg.Channel.Name
			if name == "" {
				name = msg.Channel.ID
			}
			mentions = append(mentions, Mention{
				Channel: name,
				Text:    truncate(msg.Text, 100),
			})
		}
	}
	return mentions, nil
}

// scanMentions scans history of bot-joined channels (fallback when no user token).
func (c *Client) scanMentions(groupHandles []string) ([]Mention, error) {
	info, err := c.api.AuthTest()
	if err != nil {
		return nil, fmt.Errorf("slack: auth test: %w", err)
	}
	userID := info.UserID

	groupIDs, err := c.resolveGroupIDs(groupHandles)
	if err != nil {
		groupIDs = nil
	}

	channels, _, err := c.api.GetConversations(&slack.GetConversationsParameters{
		Types:           []string{"public_channel", "private_channel", "im", "mpim"},
		ExcludeArchived: true,
		Limit:           100,
	})
	if err != nil {
		return nil, fmt.Errorf("slack: listing channels: %w", err)
	}

	var mentions []Mention
	for _, ch := range channels {
		if len(mentions) >= 50 {
			break
		}
		if !ch.IsMember {
			continue
		}
		history, err := c.api.GetConversationHistory(&slack.GetConversationHistoryParameters{
			ChannelID: ch.ID,
			Limit:     20,
		})
		if err != nil {
			continue
		}
		name := ch.Name
		if name == "" {
			name = ch.ID
		}
		for _, msg := range history.Messages {
			if isMentioned(msg.Text, userID, groupIDs) {
				mentions = append(mentions, Mention{
					Channel: name,
					Text:    truncate(msg.Text, 100),
				})
			}
		}
	}
	return mentions, nil
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
func (c *Client) resolveGroupIDs(handles []string) ([]string, error) {
	if len(handles) == 0 {
		return nil, nil
	}
	groups, err := c.api.GetUserGroups()
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
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
