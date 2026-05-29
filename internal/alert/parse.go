package alert

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

var severityNumber = map[string]int{
	"Sev0": 0, "Sev1": 1, "Sev2": 2, "Sev3": 3, "Sev4": 4,
}

// IsCommonAlertSchema reports whether the raw payload is an Azure Monitor
// common alert schema document.
func IsCommonAlertSchema(payload []byte) bool {
	var probe struct {
		SchemaID string          `json:"schemaId"`
		Data     json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(payload, &probe); err != nil {
		return false
	}
	return probe.SchemaID == CommonAlertSchemaID && len(probe.Data) > 0
}

// Options configures parsing (e.g. sovereign-cloud portal base).
type Options struct {
	PortalBase string
}

// ParseAlert normalizes a Common Alert Schema payload into a Model.
func ParseAlert(p CommonAlertSchema, opts Options) Model {
	portalBase := strings.TrimRight(orDefault(opts.PortalBase, "https://portal.azure.com"), "/")
	e := p.Data.Essentials

	ex := extractForSignal(e.MonitoringService, p.Data.AlertContext)
	resources := affectedResources(e)
	severity := orDefault(e.Severity, "Sev3")

	custom := p.Data.CustomProperties
	if custom == nil {
		custom = map[string]string{}
	}

	sevNum, ok := severityNumber[severity]
	if !ok {
		sevNum = 3
	}

	key := e.AlertID
	if key == "" {
		key = fingerprint(e, resources)
	}

	return Model{
		Key:               key,
		AlertID:           e.AlertID,
		AlertRule:         orDefault(e.AlertRule, "(unnamed alert rule)"),
		Severity:          severity,
		SeverityNumber:    sevNum,
		SignalType:        orDefault(e.SignalType, "Unknown"),
		MonitorCondition:  e.MonitorCondition,
		MonitoringService: e.MonitoringService,
		Description:       e.Description,
		FiredDateTime:     e.FiredDateTime,
		ResolvedDateTime:  e.ResolvedDateTime,
		AffectedResources: resources,
		ResourceGroupName: e.ResourceGroupName,
		Fields:            ex.fields,
		Labels:            ex.labels,
		Links:             ex.links,
		PortalAlertURL:    portalAlertURL(portalBase, e.AlertID),
		InvestigationLink: e.InvestigationLink,
		CustomProperties:  custom,
	}
}

func extractForSignal(service string, raw json.RawMessage) signalExtraction {
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	switch {
	case service == "Platform":
		return extractMetric(raw)
	case service == "Log Analytics" || service == "Log Alerts V2" || service == "Application Insights":
		return extractLog(raw)
	case strings.HasPrefix(service, "Activity Log"):
		return extractActivityLog(raw)
	case service == "ServiceHealth":
		return extractServiceHealth(raw)
	case service == "ResourceHealth" || service == "Resource Health":
		return extractResourceHealth(raw)
	case service == "Prometheus":
		return extractPrometheus(raw)
	default:
		return extractGeneric(raw)
	}
}

// affectedResources prefers alertTargetIDs, falling back to configurationItems.
func affectedResources(e Essentials) []string {
	if len(e.AlertTargetIDs) > 0 {
		return e.AlertTargetIDs
	}
	if len(e.ConfigurationItems) > 0 {
		return e.ConfigurationItems
	}
	return nil
}

// fingerprint is a deterministic fallback key when alertId is absent.
func fingerprint(e Essentials, resources []string) string {
	parts := append([]string{e.AlertRule}, append([]string(nil), resources...)...)
	sort.Strings(parts[1:])
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return "fp_" + fmt.Sprintf("%x", sum)[:32]
}

func portalAlertURL(portalBase, alertID string) string {
	if alertID == "" {
		return ""
	}
	return portalBase + "/#view/Microsoft_Azure_Monitoring/AlertDetailsTemplateBlade/alertId/" + url.QueryEscape(alertID)
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
