// Package alert parses the Azure Monitor Common Alert Schema into a
// signal-agnostic model the Slack layer renders.
//
// Reference: https://learn.microsoft.com/azure/azure-monitor/alerts/alerts-common-schema
package alert

import "encoding/json"

// CommonAlertSchemaID is the schemaId value every common-schema payload carries.
const CommonAlertSchemaID = "azureMonitorCommonAlertSchema"

// CommonAlertSchema is the top-level webhook payload.
type CommonAlertSchema struct {
	SchemaID string `json:"schemaId"`
	Data     struct {
		AlertContext     json.RawMessage   `json:"alertContext"`
		CustomProperties map[string]string `json:"customProperties"`
		Essentials       Essentials        `json:"essentials"`
	} `json:"data"`
}

// Essentials holds the standardized fields present for every alert type.
type Essentials struct {
	AlertID            string   `json:"alertId"`
	AlertRule          string   `json:"alertRule"`
	AlertTargetIDs     []string `json:"alertTargetIDs"`
	ConfigurationItems []string `json:"configurationItems"`
	Description        string   `json:"description"`
	FiredDateTime      string   `json:"firedDateTime"`
	InvestigationLink  string   `json:"investigationLink"`
	MonitorCondition   string   `json:"monitorCondition"`
	MonitoringService  string   `json:"monitoringService"`
	ResolvedDateTime   string   `json:"resolvedDateTime"`
	ResourceGroupName  string   `json:"resourceGroupName"`
	Severity           string   `json:"severity"`
	SignalType         string   `json:"signalType"`
}

// Field is a single label/value fact rendered in the Slack message.
type Field struct {
	Label string
	Value string
}

// Model is the signal-agnostic representation produced by ParseAlert.
type Model struct {
	AffectedResources []string
	AlertID           string
	AlertRule         string
	CustomProperties  map[string]string
	Description       string
	Fields            []Field // signal-specific key facts
	FiredDateTime     string
	InvestigationLink string
	Key               string
	Labels            []Field // key/value pairs rendered as an aligned table
	Links             []Field // named links (e.g. log search results)
	MonitorCondition  string
	MonitoringService string
	PortalAlertURL    string
	ResolvedDateTime  string
	ResourceGroupName string
	Severity          string
	SeverityNumber    int
	SignalType        string
}

// Resolved reports whether the alert's monitor condition is Resolved.
func (m *Model) Resolved() bool { return m.MonitorCondition == "Resolved" }

// signalExtraction is what a per-signal extractor returns.
type signalExtraction struct {
	fields []Field
	links  []Field
	labels []Field
}
