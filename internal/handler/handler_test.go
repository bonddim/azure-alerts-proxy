package handler_test

import (
	"context"
	"strings"
	"testing"

	"github.com/bonddim/azure-alerts-proxy/internal/config"
	"github.com/bonddim/azure-alerts-proxy/internal/fixtures"
	"github.com/bonddim/azure-alerts-proxy/internal/handler"
	"github.com/bonddim/azure-alerts-proxy/internal/slack"
	"github.com/bonddim/azure-alerts-proxy/internal/state"
)

type post struct {
	channel string
	msg     slack.Message
}

type fakeNotifier struct {
	posts   []post
	updates []post
	ts      int
}

func (f *fakeNotifier) Post(_ context.Context, channel string, msg slack.Message) (slack.Posted, error) {
	f.posts = append(f.posts, post{channel, msg})
	f.ts++
	return slack.Posted{Channel: channel, TS: "100" + string(rune('0'+f.ts)) + ".0001"}, nil
}

func (f *fakeNotifier) Update(_ context.Context, channel, ts string, msg slack.Message) error {
	f.updates = append(f.updates, post{channel, msg})
	return nil
}

type fakeStore struct {
	records map[string]state.Record
}

func newFakeStore() *fakeStore { return &fakeStore{records: map[string]state.Record{}} }

func (s *fakeStore) EnsureTable(context.Context) error { return nil }
func (s *fakeStore) Get(_ context.Context, key string) (state.Record, bool, error) {
	r, ok := s.records[key]
	return r, ok, nil
}
func (s *fakeStore) Save(_ context.Context, r state.Record) error {
	s.records[r.AlertKey] = r
	return nil
}

func newDeps() (*fakeNotifier, *fakeStore, handler.Deps) {
	n := &fakeNotifier{}
	s := newFakeStore()
	return n, s, handler.Deps{Notifier: n, Store: s, DefaultChannel: "#default"}
}

func TestRejectsNonCommonSchema(t *testing.T) {
	n, _, deps := newDeps()
	res := handler.HandleAlert(context.Background(), []byte(`{"foo":"bar"}`), deps)
	if res.Status != 400 {
		t.Errorf("status = %d, want 400", res.Status)
	}
	if len(n.posts) != 0 {
		t.Error("should not post")
	}
}

func TestFiredPostsAndStores(t *testing.T) {
	n, s, deps := newDeps()
	res := handler.HandleAlert(context.Background(), []byte(fixtures.MetricStaticFired), deps)
	if res.Status != 200 {
		t.Fatalf("status = %d", res.Status)
	}
	if len(n.posts) != 1 {
		t.Fatalf("posts = %d, want 1", len(n.posts))
	}
	if n.posts[0].channel != "#prod-incidents" {
		t.Errorf("channel = %q, want custom-property routing", n.posts[0].channel)
	}
	if len(s.records) != 1 {
		t.Error("expected a stored record")
	}
}

func TestDefaultChannelFallback(t *testing.T) {
	n, _, deps := newDeps()
	noChannel := strings.Replace(fixtures.MetricStaticFired,
		`"customProperties": {"team": "platform", "slack-channel": "#prod-incidents"}`,
		`"customProperties": null`, 1)
	handler.HandleAlert(context.Background(), []byte(noChannel), deps)
	if n.posts[0].channel != "#default" {
		t.Errorf("channel = %q, want #default", n.posts[0].channel)
	}
}

func TestRoutesPrometheusByNamespaceLabel(t *testing.T) {
	n, _, deps := newDeps()
	deps.ChannelRoutes = []config.ChannelRoute{
		{Service: "Prometheus", Labels: map[string]string{"namespace": "payments"}, Channel: "#payments-alerts"},
	}
	res := handler.HandleAlert(context.Background(), []byte(fixtures.PrometheusFired), deps)
	if res.Status != 200 {
		t.Fatalf("status = %d", res.Status)
	}
	if n.posts[0].channel != "#payments-alerts" {
		t.Errorf("channel = %q, want #payments-alerts", n.posts[0].channel)
	}
}

func TestRoutesPrometheusQAAndProductionNamespaces(t *testing.T) {
	n, _, deps := newDeps()
	deps.Store = nil
	deps.ChannelRoutes = []config.ChannelRoute{
		{Service: "Prometheus", Labels: map[string]string{"namespace": "production"}, Channel: "#prod-alerts"},
		{Service: "Prometheus", Labels: map[string]string{"namespace": "qa"}, Channel: "#qa-alerts"},
	}
	prod := strings.Replace(fixtures.PrometheusFired, `"namespace": "payments"`, `"namespace": "production"`, 1)
	qa := strings.Replace(fixtures.PrometheusFired, `"namespace": "payments"`, `"namespace": "qa"`, 1)
	handler.HandleAlert(context.Background(), []byte(prod), deps)
	handler.HandleAlert(context.Background(), []byte(qa), deps)
	if len(n.posts) != 2 {
		t.Fatalf("posts = %d, want 2", len(n.posts))
	}
	if n.posts[0].channel != "#prod-alerts" {
		t.Errorf("prod channel = %q", n.posts[0].channel)
	}
	if n.posts[1].channel != "#qa-alerts" {
		t.Errorf("qa channel = %q", n.posts[1].channel)
	}
}

func TestUnmatchedChannelRouteFallsBackToDefault(t *testing.T) {
	n, _, deps := newDeps()
	deps.ChannelRoutes = []config.ChannelRoute{
		{Service: "Prometheus", Labels: map[string]string{"namespace": "production"}, Channel: "#prod-alerts"},
	}
	res := handler.HandleAlert(context.Background(), []byte(fixtures.PrometheusFired), deps)
	if res.Status != 200 {
		t.Fatalf("status = %d", res.Status)
	}
	if n.posts[0].channel != "#default" {
		t.Errorf("channel = %q, want #default", n.posts[0].channel)
	}
}

func TestCustomPropertyOverridesChannelRoute(t *testing.T) {
	n, _, deps := newDeps()
	deps.ChannelRoutes = []config.ChannelRoute{
		{Service: "Platform", Labels: map[string]string{"namespace": "production"}, Channel: "#prod-alerts"},
		{Channel: "#catch-all"},
	}
	res := handler.HandleAlert(context.Background(), []byte(fixtures.MetricStaticFired), deps)
	if res.Status != 200 {
		t.Fatalf("status = %d", res.Status)
	}
	if n.posts[0].channel != "#prod-incidents" {
		t.Errorf("channel = %q, want custom property channel", n.posts[0].channel)
	}
}

func TestNoStatePostsEveryFiredAlert(t *testing.T) {
	n, _, deps := newDeps()
	deps.Store = nil
	res := handler.HandleAlert(context.Background(), []byte(fixtures.MetricStaticFired), deps)
	if res.Status != 200 || res.Body != "posted" {
		t.Fatalf("result = %+v, want 200 posted", res)
	}
	res = handler.HandleAlert(context.Background(), []byte(fixtures.MetricStaticFired), deps)
	if res.Status != 200 || res.Body != "posted" {
		t.Fatalf("second result = %+v, want 200 posted", res)
	}
	if len(n.posts) != 2 || len(n.updates) != 0 {
		t.Errorf("posts=%d updates=%d, want 2/0", len(n.posts), len(n.updates))
	}
}

func TestTypedNilStorePostsWithoutPanic(t *testing.T) {
	n, _, deps := newDeps()
	var store *fakeStore
	deps.Store = store
	res := handler.HandleAlert(context.Background(), []byte(fixtures.MetricStaticFired), deps)
	if res.Status != 200 || res.Body != "posted" {
		t.Fatalf("result = %+v, want 200 posted", res)
	}
	if len(n.posts) != 1 || len(n.updates) != 0 {
		t.Errorf("posts=%d updates=%d, want 1/0", len(n.posts), len(n.updates))
	}
}

func TestNoStatePostsResolvedAlert(t *testing.T) {
	n, _, deps := newDeps()
	deps.Store = nil
	res := handler.HandleAlert(context.Background(), []byte(fixtures.MetricResolved), deps)
	if res.Status != 200 || res.Body != "posted" {
		t.Fatalf("result = %+v, want 200 posted", res)
	}
	if len(n.posts) != 1 || len(n.updates) != 0 {
		t.Errorf("posts=%d updates=%d, want 1/0", len(n.posts), len(n.updates))
	}
	if !strings.Contains(n.posts[0].msg.Fallback, "Resolved:") {
		t.Errorf("fallback = %q", n.posts[0].msg.Fallback)
	}
}

func TestResolveUpdatesInPlace(t *testing.T) {
	n, _, deps := newDeps()
	handler.HandleAlert(context.Background(), []byte(fixtures.MetricStaticFired), deps)
	res := handler.HandleAlert(context.Background(), []byte(fixtures.MetricResolved), deps)
	if res.Status != 200 {
		t.Fatalf("status = %d", res.Status)
	}
	if len(n.posts) != 1 {
		t.Errorf("posts = %d, want 1 (no second post)", len(n.posts))
	}
	if len(n.updates) != 1 {
		t.Fatalf("updates = %d, want 1", len(n.updates))
	}
	if !strings.Contains(n.updates[0].msg.Fallback, "Resolved:") {
		t.Errorf("update fallback = %q", n.updates[0].msg.Fallback)
	}
}

func TestResolvedAlertUpdatesStoredChannelDespiteRouteChange(t *testing.T) {
	n, s, deps := newDeps()
	deps.ChannelRoutes = []config.ChannelRoute{
		{Service: "Prometheus", Labels: map[string]string{"namespace": "payments"}, Channel: "#payments-alerts"},
	}
	handler.HandleAlert(context.Background(), []byte(fixtures.PrometheusFired), deps)
	deps.ChannelRoutes = []config.ChannelRoute{
		{Service: "Prometheus", Labels: map[string]string{"namespace": "payments"}, Channel: "#new-payments-alerts"},
	}
	resolved := strings.Replace(fixtures.PrometheusFired, `"monitorCondition": "Fired"`, `"monitorCondition": "Resolved"`, 1)
	res := handler.HandleAlert(context.Background(), []byte(resolved), deps)
	if res.Status != 200 {
		t.Fatalf("status = %d", res.Status)
	}
	if len(s.records) != 1 {
		t.Fatalf("records = %d, want 1", len(s.records))
	}
	if len(n.updates) != 1 {
		t.Fatalf("updates = %d, want 1", len(n.updates))
	}
	if n.updates[0].channel != "#payments-alerts" {
		t.Errorf("update channel = %q, want stored channel", n.updates[0].channel)
	}
}

func TestIdempotentDuplicateFired(t *testing.T) {
	n, _, deps := newDeps()
	handler.HandleAlert(context.Background(), []byte(fixtures.MetricStaticFired), deps)
	res := handler.HandleAlert(context.Background(), []byte(fixtures.MetricStaticFired), deps)
	if res.Body != "updated" {
		t.Errorf("body = %q, want updated", res.Body)
	}
	if len(n.posts) != 1 || len(n.updates) != 1 {
		t.Errorf("posts=%d updates=%d, want 1/1", len(n.posts), len(n.updates))
	}
}

func TestStandaloneResolved(t *testing.T) {
	n, _, deps := newDeps()
	res := handler.HandleAlert(context.Background(), []byte(fixtures.MetricResolved), deps)
	if res.Status != 200 {
		t.Fatalf("status = %d", res.Status)
	}
	if len(n.posts) != 1 || len(n.updates) != 0 {
		t.Errorf("posts=%d updates=%d, want 1/0", len(n.posts), len(n.updates))
	}
	if !strings.Contains(n.posts[0].msg.Fallback, "Resolved:") {
		t.Errorf("fallback = %q", n.posts[0].msg.Fallback)
	}
}
