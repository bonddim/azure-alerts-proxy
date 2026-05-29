package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bonddim/azure-alerts-proxy/internal/fixtures"
	"github.com/bonddim/azure-alerts-proxy/internal/handler"
	"github.com/bonddim/azure-alerts-proxy/internal/slack"
)

func TestServerPort(t *testing.T) {
	t.Setenv("FUNCTIONS_CUSTOMHANDLER_PORT", "")
	t.Setenv("PORT", "")
	if got := serverPort(); got != "8080" {
		t.Fatalf("serverPort() = %q, want 8080", got)
	}
	t.Setenv("PORT", "9090")
	if got := serverPort(); got != "9090" {
		t.Fatalf("serverPort() = %q, want PORT", got)
	}
	t.Setenv("FUNCTIONS_CUSTOMHANDLER_PORT", "7071")
	if got := serverPort(); got != "7071" {
		t.Fatalf("serverPort() = %q, want FUNCTIONS_CUSTOMHANDLER_PORT", got)
	}
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	healthHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("body = %q, want ok", rec.Body.String())
	}
}

func TestCleanupHandlerWithoutState(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/cleanup", nil)
	rec := httptest.NewRecorder()
	cleanupHandler(nil, 30).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "{}" {
		t.Fatalf("body = %q, want {}", rec.Body.String())
	}
}

func TestRegisterRoutesAcceptsAzureFunctionsAPIPath(t *testing.T) {
	notifier := &routeNotifier{}
	deps := handler.Deps{
		DefaultChannel: "#alerts",
		Notifier:       notifier,
		Store:          nil,
	}
	mux := http.NewServeMux()
	registerRoutes(mux, deps, nil, 30)

	req := httptest.NewRequest(http.MethodPost, "/api/alerts", bytes.NewBufferString(fixtures.MetricStaticFired))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	if notifier.posts != 1 {
		t.Fatalf("posts = %d, want 1", notifier.posts)
	}
}

type routeNotifier struct {
	posts int
}

func (n *routeNotifier) Post(ctx context.Context, channel string, msg slack.Message) (slack.Posted, error) {
	n.posts++
	return slack.Posted{Channel: channel, TS: "123.456"}, nil
}

func (n *routeNotifier) Update(ctx context.Context, channel, ts string, msg slack.Message) error {
	return nil
}
