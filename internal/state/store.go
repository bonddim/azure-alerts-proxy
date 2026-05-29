// Package state persists the alert→Slack-message mapping in Azure Table Storage
// so a later "Resolved" notification can update the original message.
package state

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
)

// Record is the stored correlation between an alert and its Slack message.
type Record struct {
	AlertKey         string
	Channel          string
	FiredAt          string
	MessageTS        string
	MonitorCondition string
	ResolvedAt       string
}

type entity struct {
	AlertKey         string `json:"alertKey"`
	Channel          string `json:"channel"`
	FiredAt          string `json:"firedAt"`
	MessageTS        string `json:"messageTs"`
	MonitorCondition string `json:"monitorCondition"`
	PartitionKey     string `json:"PartitionKey"`
	ResolvedAt       string `json:"resolvedAt"`
	RowKey           string `json:"RowKey"`
	UpdatedAt        string `json:"updatedAt"` // RFC3339; lexicographically comparable
}

// Store wraps an Azure Table client.
type Store struct {
	client *aztables.Client
}

// New builds a Store. Uses a connection string when provided (local dev /
// Azurite); otherwise connects to the table endpoint with the function's
// managed identity.
func New(endpoint, tableName, connectionString string) (*Store, error) {
	if connectionString != "" {
		svc, err := aztables.NewServiceClientFromConnectionString(connectionString, nil)
		if err != nil {
			return nil, fmt.Errorf("table client from connection string: %w", err)
		}
		return &Store{client: svc.NewClient(tableName)}, nil
	}
	if endpoint == "" {
		return nil, errors.New("state store requires STATE_STORAGE_ENDPOINT or STATE_STORAGE_CONNECTION_STRING")
	}
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("default azure credential: %w", err)
	}
	svc, err := aztables.NewServiceClient(endpoint, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("table service client: %w", err)
	}
	return &Store{client: svc.NewClient(tableName)}, nil
}

func keysFor(alertKey string) (partitionKey, rowKey string) {
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte(alertKey)))
	return sum[:2], sum
}

func statusCode(err error) int {
	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) {
		return respErr.StatusCode
	}
	return 0
}

// EnsureTable creates the backing table if it does not yet exist.
func (s *Store) EnsureTable(ctx context.Context) error {
	_, err := s.client.CreateTable(ctx, nil)
	if err != nil && statusCode(err) != http.StatusConflict {
		return fmt.Errorf("create table: %w", err)
	}
	return nil
}

// Get returns the stored record for an alert key, or ok=false if none.
func (s *Store) Get(ctx context.Context, alertKey string) (Record, bool, error) {
	pk, rk := keysFor(alertKey)
	resp, err := s.client.GetEntity(ctx, pk, rk, nil)
	if err != nil {
		if statusCode(err) == http.StatusNotFound {
			return Record{}, false, nil
		}
		return Record{}, false, fmt.Errorf("get entity: %w", err)
	}
	var e entity
	if err := json.Unmarshal(resp.Value, &e); err != nil {
		return Record{}, false, fmt.Errorf("unmarshal entity: %w", err)
	}
	return Record{
		AlertKey:         e.AlertKey,
		Channel:          e.Channel,
		MessageTS:        e.MessageTS,
		MonitorCondition: e.MonitorCondition,
		FiredAt:          e.FiredAt,
		ResolvedAt:       e.ResolvedAt,
	}, true, nil
}

// Save persists (overwrites) the record for an alert key.
func (s *Store) Save(ctx context.Context, r Record) error {
	pk, rk := keysFor(r.AlertKey)
	e := entity{
		PartitionKey:     pk,
		RowKey:           rk,
		AlertKey:         r.AlertKey,
		Channel:          r.Channel,
		MessageTS:        r.MessageTS,
		MonitorCondition: r.MonitorCondition,
		FiredAt:          r.FiredAt,
		ResolvedAt:       r.ResolvedAt,
		UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
	}
	marshaled, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal entity: %w", err)
	}
	if _, err := s.client.UpsertEntity(ctx, marshaled, nil); err != nil {
		return fmt.Errorf("upsert entity: %w", err)
	}
	return nil
}

// PurgeOlderThan deletes records last updated before cutoff. Returns the count.
func (s *Store) PurgeOlderThan(ctx context.Context, cutoff time.Time) (int, error) {
	filter := fmt.Sprintf("UpdatedAt lt '%s'", cutoff.UTC().Format(time.RFC3339))
	pager := s.client.NewListEntitiesPager(&aztables.ListEntitiesOptions{Filter: &filter})
	deleted := 0
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return deleted, fmt.Errorf("list entities: %w", err)
		}
		for _, raw := range page.Entities {
			var e entity
			if err := json.Unmarshal(raw, &e); err != nil {
				continue
			}
			if _, err := s.client.DeleteEntity(ctx, e.PartitionKey, e.RowKey, nil); err != nil {
				return deleted, fmt.Errorf("delete entity: %w", err)
			}
			deleted++
		}
	}
	return deleted, nil
}
