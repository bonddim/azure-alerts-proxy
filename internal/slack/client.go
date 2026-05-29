package slack

import (
	"context"
	"fmt"

	"github.com/slack-go/slack"
)

// Notifier posts and updates alert messages in Slack. Requires a bot token with
// chat:write (and chat:write.public to post to channels without an invite).
type Notifier interface {
	Post(ctx context.Context, channel string, msg Message) (Posted, error)
	Update(ctx context.Context, channel, ts string, msg Message) error
}

// Posted identifies a posted message for later updates.
type Posted struct {
	Channel string
	TS      string
}

type client struct {
	api *slack.Client
}

// New returns a Notifier backed by the Slack Web API.
func New(token string) Notifier {
	return &client{api: slack.New(token)}
}

// Post sends a new alert message (coloured attachment only, no top-level text).
func (c *client) Post(ctx context.Context, channel string, msg Message) (Posted, error) {
	respChannel, ts, err := c.api.PostMessageContext(ctx, channel,
		slack.MsgOptionAttachments(msg.Attachment))
	if err != nil {
		return Posted{}, fmt.Errorf("chat.postMessage: %w", err)
	}
	return Posted{Channel: respChannel, TS: ts}, nil
}

// Update edits an existing message in place (used when an alert resolves).
func (c *client) Update(ctx context.Context, channel, ts string, msg Message) error {
	if _, _, _, err := c.api.UpdateMessageContext(ctx, channel, ts,
		slack.MsgOptionAttachments(msg.Attachment)); err != nil {
		return fmt.Errorf("chat.update: %w", err)
	}
	return nil
}
