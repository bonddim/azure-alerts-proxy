package config_test

import (
	"testing"

	"github.com/bonddim/azure-alerts-proxy/internal/config"
)

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"SLACK_BOT_TOKEN",
		"SLACK_DEFAULT_CHANNEL",
		"STATE_TABLE_NAME",
		"STATE_STORAGE_ENDPOINT",
		"STATE_STORAGE_CONNECTION_STRING",
		"AzureWebJobsStorage",
		"STATE_RETENTION_DAYS",
		"AZURE_PORTAL_BASE",
		"SLACK_CHANNEL_ROUTES",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadAllowsMissingSlackChannelRoutes(t *testing.T) {
	clearEnv(t)
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-test")
	t.Setenv("SLACK_DEFAULT_CHANNEL", "#alerts")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.SlackChannelRoutes) != 0 {
		t.Fatalf("SlackChannelRoutes length = %d, want 0", len(cfg.SlackChannelRoutes))
	}
}

func TestLoadParsesSlackChannelRoutes(t *testing.T) {
	clearEnv(t)
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-test")
	t.Setenv("SLACK_DEFAULT_CHANNEL", "#alerts")
	t.Setenv("SLACK_CHANNEL_ROUTES", `[
		{"service":"Prometheus","labels":{"namespace":"production"},"channel":"#prod-alerts"},
		{"service":"Prometheus","labels":{"namespace":"qa"},"channel":"C1234567890"}
	]`)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.SlackChannelRoutes) != 2 {
		t.Fatalf("SlackChannelRoutes length = %d, want 2", len(cfg.SlackChannelRoutes))
	}
	if got := cfg.SlackChannelRoutes[0].Labels["namespace"]; got != "production" {
		t.Errorf("first namespace = %q", got)
	}
	if got := cfg.SlackChannelRoutes[1].Channel; got != "C1234567890" {
		t.Errorf("second channel = %q", got)
	}
}

func TestLoadRejectsInvalidSlackChannelRoutes(t *testing.T) {
	clearEnv(t)
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-test")
	t.Setenv("SLACK_DEFAULT_CHANNEL", "#alerts")
	t.Setenv("SLACK_CHANNEL_ROUTES", `{`)
	if _, err := config.Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid SLACK_CHANNEL_ROUTES error")
	}
}

func TestLoadAllowsMissingStateStorage(t *testing.T) {
	clearEnv(t)
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-test")
	t.Setenv("SLACK_DEFAULT_CHANNEL", "#alerts")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.StateEnabled() {
		t.Fatal("StateEnabled() = true, want false")
	}
}

func TestLoadUsesExplicitStateStorage(t *testing.T) {
	clearEnv(t)
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-test")
	t.Setenv("SLACK_DEFAULT_CHANNEL", "#alerts")
	t.Setenv("STATE_STORAGE_ENDPOINT", "https://example.table.core.windows.net")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.StateEnabled() {
		t.Fatal("StateEnabled() = false, want true")
	}
}

func TestLoadFallsBackToAzureWebJobsStorage(t *testing.T) {
	clearEnv(t)
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-test")
	t.Setenv("SLACK_DEFAULT_CHANNEL", "#alerts")
	t.Setenv("AzureWebJobsStorage", "UseDevelopmentStorage=true")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.StateStorageConnectionString != "UseDevelopmentStorage=true" {
		t.Errorf("StateStorageConnectionString = %q", cfg.StateStorageConnectionString)
	}
	if !cfg.StateEnabled() {
		t.Fatal("StateEnabled() = false, want true")
	}
}
