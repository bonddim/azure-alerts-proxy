// Command handler is the Azure Functions custom-handler web server. The
// Functions host forwards HTTP requests for the "alerts" function and timer
// invocations for the "cleanup" function to this process.
package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bonddim/azure-alerts-proxy/internal/config"
	"github.com/bonddim/azure-alerts-proxy/internal/handler"
	"github.com/bonddim/azure-alerts-proxy/internal/slack"
	"github.com/bonddim/azure-alerts-proxy/internal/state"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	var store *state.Store
	if cfg.StateEnabled() {
		store, err = state.New(cfg.StateStorageEndpoint, cfg.StateTableName, cfg.StateStorageConnectionString)
		if err != nil {
			log.Fatalf("state store: %v", err)
		}
	} else {
		log.Printf("state store disabled; alerts will always post new Slack messages")
	}
	notifier := slack.New(cfg.SlackBotToken)

	deps := handler.Deps{
		DefaultChannel: cfg.SlackDefaultChannel,
		ChannelRoutes:  cfg.SlackChannelRoutes,
		Logf:           log.Printf,
		Notifier:       notifier,
		PortalBase:     cfg.PortalBase,
		Store:          store,
	}

	mux := http.NewServeMux()
	registerRoutes(mux, deps, store, cfg.StateRetentionDays)

	port := serverPort()
	log.Printf("custom handler listening on :%s", port)
	if err := serve(context.Background(), ":"+port, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func serve(ctx context.Context, addr string, handler http.Handler) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := &http.Server{Addr: addr, Handler: handler}
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		stop()
		log.Printf("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		err := <-errCh
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func registerRoutes(mux *http.ServeMux, deps handler.Deps, store *state.Store, retentionDays int) {
	alerts := alertsHandler(deps)
	mux.HandleFunc("/alerts", alerts)
	mux.HandleFunc("/api/alerts", alerts)

	cleanup := cleanupHandler(store, retentionDays)
	mux.HandleFunc("/cleanup", cleanup)
	mux.HandleFunc("/api/cleanup", cleanup)

	health := healthHandler()
	mux.HandleFunc("/healthz", health)
	mux.HandleFunc("/api/healthz", health)
}

func serverPort() string {
	if port := os.Getenv("FUNCTIONS_CUSTOMHANDLER_PORT"); port != "" {
		return port
	}
	if port := os.Getenv("PORT"); port != "" {
		return port
	}
	return "8080"
}

// alertsHandler serves the HTTP-triggered "alerts" function. With
// enableForwardingHttpRequest the host forwards the original Action Group
// request, so the body is the raw Common Alert Schema payload.
func alertsHandler(deps handler.Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "cannot read body", http.StatusBadRequest)
			return
		}
		res := handler.HandleAlert(r.Context(), body, deps)
		w.WriteHeader(res.Status)
		_, _ = io.WriteString(w, res.Body)
	}
}

// cleanupHandler serves the timer-triggered "cleanup" function. The host sends
// the custom-handler payload envelope; we ignore it and run the purge.
func cleanupHandler(store *state.Store, retentionDays int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			log.Printf("state disabled; cleanup skipped")
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, "{}")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()
		if err := store.EnsureTable(ctx); err != nil {
			log.Printf("cleanup ensure table failed: %v", err)
			http.Error(w, "state store unavailable", http.StatusInternalServerError)
			return
		}
		cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
		deleted, err := store.PurgeOlderThan(ctx, cutoff)
		if err != nil {
			log.Printf("cleanup purge failed: %v", err)
			http.Error(w, "purge failed", http.StatusInternalServerError)
			return
		}
		log.Printf("cleanup removed %d record(s) older than %s", deleted, cutoff.Format(time.RFC3339))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{}")
	}
}

func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}
}
