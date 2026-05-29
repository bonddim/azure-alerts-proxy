// Package handler orchestrates the alert→Slack flow, decoupled from the HTTP
// transport so it can be unit tested.
package handler

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/bonddim/azure-alerts-proxy/internal/alert"
	"github.com/bonddim/azure-alerts-proxy/internal/slack"
	"github.com/bonddim/azure-alerts-proxy/internal/state"
)

// channelProperty is the action-group custom property used to override the channel.
const channelProperty = "slack-channel"

// Store is the subset of the state store the handler needs (interface for tests).
type Store interface {
	EnsureTable(ctx context.Context) error
	Get(ctx context.Context, key string) (state.Record, bool, error)
	Save(ctx context.Context, r state.Record) error
}

// Deps are the handler's collaborators.
type Deps struct {
	Notifier       slack.Notifier
	Store          Store
	DefaultChannel string
	PortalBase     string
	Logf           func(string, ...any)
}

// Result is the HTTP-ish outcome of handling an alert.
type Result struct {
	Status int
	Body   string
}

func (d Deps) logf(format string, args ...any) {
	if d.Logf != nil {
		d.Logf(format, args...)
	}
}

// HandleAlert processes one Common Alert Schema payload: posts on Fired,
// updates the original message on Resolved, idempotent against retries.
func HandleAlert(ctx context.Context, payload []byte, d Deps) Result {
	if !alert.IsCommonAlertSchema(payload) {
		d.logf("rejected payload: not the Azure Monitor common alert schema")
		return Result{Status: 400, Body: "expected Azure Monitor common alert schema"}
	}

	var schema alert.CommonAlertSchema
	if err := json.Unmarshal(payload, &schema); err != nil {
		return Result{Status: 400, Body: "invalid JSON"}
	}

	model := alert.ParseAlert(schema, alert.Options{PortalBase: d.PortalBase})
	channel := d.DefaultChannel
	if c := model.CustomProperties[channelProperty]; c != "" {
		channel = c
	}
	msg := slack.Build(model)
	if storeDisabled(d.Store) {
		if _, err := d.Notifier.Post(ctx, channel, msg); err != nil {
			d.logf("slack post failed: %v", err)
			return Result{Status: 500, Body: "slack post failed"}
		}
		d.logf("posted message for alert %s without state", model.Key)
		return Result{Status: 200, Body: "posted"}
	}

	if err := d.Store.EnsureTable(ctx); err != nil {
		d.logf("ensure table failed: %v", err)
		return Result{Status: 500, Body: "state store unavailable"}
	}
	existing, found, err := d.Store.Get(ctx, model.Key)
	if err != nil {
		d.logf("state lookup failed: %v", err)
		return Result{Status: 500, Body: "state lookup failed"}
	}

	now := time.Now().UTC().Format(time.RFC3339)

	if model.Resolved() {
		if found {
			if err := d.Notifier.Update(ctx, existing.Channel, existing.MessageTS, msg); err != nil {
				d.logf("slack update failed: %v", err)
				return Result{Status: 500, Body: "slack update failed"}
			}
			resolvedAt := orNow(model.ResolvedDateTime, now)
			_ = d.Store.Save(ctx, state.Record{
				AlertKey: model.Key, Channel: existing.Channel, MessageTS: existing.MessageTS,
				MonitorCondition: "Resolved", FiredAt: existing.FiredAt, ResolvedAt: resolvedAt,
			})
			d.logf("updated resolved message for alert %s", model.Key)
			return Result{Status: 200, Body: "resolved"}
		}
		posted, err := d.Notifier.Post(ctx, channel, msg)
		if err != nil {
			d.logf("slack post failed: %v", err)
			return Result{Status: 500, Body: "slack post failed"}
		}
		_ = d.Store.Save(ctx, state.Record{
			AlertKey: model.Key, Channel: posted.Channel, MessageTS: posted.TS,
			MonitorCondition: "Resolved", ResolvedAt: orNow(model.ResolvedDateTime, now),
		})
		d.logf("posted standalone resolved message for alert %s", model.Key)
		return Result{Status: 200, Body: "resolved"}
	}

	// Fired (or any non-resolved condition): post once, update on retry.
	if found {
		if err := d.Notifier.Update(ctx, existing.Channel, existing.MessageTS, msg); err != nil {
			d.logf("slack update (retry) failed: %v", err)
			return Result{Status: 500, Body: "slack update failed"}
		}
		d.logf("updated existing fired message for alert %s (retry)", model.Key)
		return Result{Status: 200, Body: "updated"}
	}
	posted, err := d.Notifier.Post(ctx, channel, msg)
	if err != nil {
		d.logf("slack post failed: %v", err)
		return Result{Status: 500, Body: "slack post failed"}
	}
	_ = d.Store.Save(ctx, state.Record{
		AlertKey: model.Key, Channel: posted.Channel, MessageTS: posted.TS,
		MonitorCondition: model.MonitorCondition, FiredAt: orNow(model.FiredDateTime, now),
	})
	d.logf("posted fired message for alert %s in %s", model.Key, posted.Channel)
	return Result{Status: 200, Body: "posted"}
}

func storeDisabled(store Store) bool {
	if store == nil {
		return true
	}
	v := reflect.ValueOf(store)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func orNow(v, now string) string {
	if v != "" {
		return v
	}
	return now
}
