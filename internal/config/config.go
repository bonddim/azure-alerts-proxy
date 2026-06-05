// Package config loads and validates configuration from app settings.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// ChannelRoute maps matching alerts to a Slack channel.
type ChannelRoute struct {
	Service string            `json:"service"`
	Labels  map[string]string `json:"labels"`
	Channel string            `json:"channel"`
}

// Config is the validated application configuration.
type Config struct {
	SlackBotToken                string
	SlackDefaultChannel          string
	SlackChannelRoutes           []ChannelRoute
	StateTableName               string
	StateStorageEndpoint         string
	StateStorageConnectionString string
	StateRetentionDays           int
	PortalBase                   string
}

// Load reads configuration from the environment, applying defaults and
// validating required values.
func Load() (Config, error) {
	c := Config{
		SlackBotToken:                os.Getenv("SLACK_BOT_TOKEN"),
		SlackDefaultChannel:          os.Getenv("SLACK_DEFAULT_CHANNEL"),
		StateTableName:               envOr("STATE_TABLE_NAME", "alertmessages"),
		StateStorageEndpoint:         os.Getenv("STATE_STORAGE_ENDPOINT"),
		StateStorageConnectionString: os.Getenv("STATE_STORAGE_CONNECTION_STRING"),
		PortalBase:                   envOr("AZURE_PORTAL_BASE", "https://portal.azure.com"),
		StateRetentionDays:           30,
	}
	if c.SlackBotToken == "" {
		return c, fmt.Errorf("missing required app setting: SLACK_BOT_TOKEN")
	}
	if c.SlackDefaultChannel == "" {
		return c, fmt.Errorf("missing required app setting: SLACK_DEFAULT_CHANNEL")
	}
	if v := os.Getenv("SLACK_CHANNEL_ROUTES"); v != "" {
		if err := json.Unmarshal([]byte(v), &c.SlackChannelRoutes); err != nil {
			return c, fmt.Errorf("invalid SLACK_CHANNEL_ROUTES: %w", err)
		}
		for i, route := range c.SlackChannelRoutes {
			if route.Channel == "" {
				return c, fmt.Errorf("invalid SLACK_CHANNEL_ROUTES[%d]: missing channel", i)
			}
		}
	}
	// Local dev: fall back to the Functions storage account for the state table.
	if c.StateStorageEndpoint == "" && c.StateStorageConnectionString == "" {
		c.StateStorageConnectionString = os.Getenv("AzureWebJobsStorage")
	}
	if v := os.Getenv("STATE_RETENTION_DAYS"); v != "" {
		if days, err := strconv.Atoi(v); err == nil && days > 0 {
			c.StateRetentionDays = days
		}
	}
	return c, nil
}

// StateEnabled reports whether a state backend was configured.
func (c Config) StateEnabled() bool {
	return c.StateStorageEndpoint != "" || c.StateStorageConnectionString != ""
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
