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
	} {
		t.Setenv(key, "")
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
