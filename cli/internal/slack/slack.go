package slack

import (
	"fmt"
	"strings"

	"github.com/slack-go/slack"
)

// Client wraps the slack-go client.
type Client struct {
	api *slack.Client
}

// New creates a Slack client using a bot token.
func New(token string) *Client {
	return &Client{api: slack.New(token)}
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

// Mentions returns recent messages that mention the bot user across all
// channels it is a member of. Only one page of history per channel is
// fetched to stay well within Slack rate limits.
func (c *Client) Mentions() ([]Mention, error) {
	info, err := c.api.AuthTest()
	if err != nil {
		return nil, fmt.Errorf("slack: auth test: %w", err)
	}
	userID := info.UserID

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
		for _, msg := range history.Messages {
			if strings.Contains(msg.Text, "<@"+userID+">") {
				name := ch.Name
				if name == "" {
					name = ch.ID
				}
				mentions = append(mentions, Mention{
					Channel: name,
					Text:    truncate(msg.Text, 100),
				})
			}
		}
	}

	return mentions, nil
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
