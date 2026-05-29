package alert_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/bonddim/azure-alerts-proxy/internal/alert"
	"github.com/bonddim/azure-alerts-proxy/internal/fixtures"
)

func parse(t *testing.T, payload string) alert.Model {
	t.Helper()
	var s alert.CommonAlertSchema
	if err := json.Unmarshal([]byte(payload), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return alert.ParseAlert(s, alert.Options{})
}

func fieldValue(fields []alert.Field, label string) (string, bool) {
	for _, f := range fields {
		if f.Label == label {
			return f.Value, true
		}
	}
	return "", false
}

func TestIsCommonAlertSchema(t *testing.T) {
	if !alert.IsCommonAlertSchema([]byte(fixtures.MetricStaticFired)) {
		t.Error("expected valid common schema")
	}
	for _, bad := range []string{`{"schemaId":"other","data":{}}`, `not json`, `{}`} {
		if alert.IsCommonAlertSchema([]byte(bad)) {
			t.Errorf("expected %q to be rejected", bad)
		}
	}
}

func TestEssentials(t *testing.T) {
	m := parse(t, fixtures.MetricStaticFired)
	if m.AlertRule != "High CPU on web vm" {
		t.Errorf("alertRule = %q", m.AlertRule)
	}
	if m.SeverityNumber != 2 {
		t.Errorf("severityNumber = %d, want 2", m.SeverityNumber)
	}
	if m.MonitorCondition != "Fired" || m.Resolved() {
		t.Error("expected Fired")
	}
	// prefers alertTargetIDs over configurationItems
	if len(m.AffectedResources) != 1 || !strings.Contains(m.AffectedResources[0], "virtualmachines/web-vm-1") {
		t.Errorf("affectedResources = %v", m.AffectedResources)
	}
	if !strings.Contains(m.PortalAlertURL, "AlertDetailsTemplateBlade") {
		t.Errorf("portalAlertURL = %q", m.PortalAlertURL)
	}
	if m.CustomProperties["slack-channel"] != "#prod-incidents" {
		t.Errorf("slack-channel = %q", m.CustomProperties["slack-channel"])
	}
}

func TestKeyStableAcrossFiredResolved(t *testing.T) {
	if parse(t, fixtures.MetricStaticFired).Key != parse(t, fixtures.MetricResolved).Key {
		t.Error("key should match across fired/resolved")
	}
}

func TestFingerprintFallback(t *testing.T) {
	noID := strings.Replace(fixtures.MetricStaticFired,
		`"alertId": "/subscriptions/11111111-1111-1111-1111-111111111111/providers/Microsoft.AlertsManagement/alerts/aaaa1111-bbbb"`,
		`"alertId": ""`, 1)
	m := parse(t, noID)
	if !strings.HasPrefix(m.Key, "fp_") {
		t.Errorf("expected fingerprint key, got %q", m.Key)
	}
}

func TestPortalBaseOverride(t *testing.T) {
	var s alert.CommonAlertSchema
	_ = json.Unmarshal([]byte(fixtures.MetricStaticFired), &s)
	m := alert.ParseAlert(s, alert.Options{PortalBase: "https://portal.azure.us/"})
	if !strings.HasPrefix(m.PortalAlertURL, "https://portal.azure.us/#view") {
		t.Errorf("portalAlertURL = %q", m.PortalAlertURL)
	}
	if strings.Contains(m.PortalAlertURL, "azure.us//") {
		t.Error("trailing slash not trimmed")
	}
}

func TestMetricExtraction(t *testing.T) {
	m := parse(t, fixtures.MetricStaticFired)
	if v, _ := fieldValue(m.Fields, "Metric"); !strings.Contains(v, "Percentage CPU") {
		t.Errorf("Metric = %q", v)
	}
	if v, _ := fieldValue(m.Fields, "Observed value"); v != "97.5" {
		t.Errorf("Observed value = %q", v)
	}
	if v, _ := fieldValue(m.Fields, "Condition type"); v != "Static Threshold" {
		t.Errorf("Condition type = %q", v)
	}
	if v, _ := fieldValue(m.Fields, "Dimensions"); v != "host=web-vm-1" {
		t.Errorf("Dimensions = %q", v)
	}
}

func TestLogExtraction(t *testing.T) {
	m := parse(t, fixtures.LogSearchFired)
	if v, _ := fieldValue(m.Fields, "Query"); !strings.Contains(v, "ResultCode >= 500") {
		t.Errorf("Query = %q", v)
	}
	if v, _ := fieldValue(m.Fields, "Observed value"); v != "532" {
		t.Errorf("Observed value = %q", v)
	}
	if v, _ := fieldValue(m.Links, "Search results"); !strings.Contains(v, "portal.azure.com") {
		t.Errorf("Search results link = %q", v)
	}
}

func TestActivityAndHealthExtraction(t *testing.T) {
	a := parse(t, fixtures.ActivityLogAdministrative)
	if v, _ := fieldValue(a.Fields, "Operation"); !strings.Contains(v, "virtualMachines/delete") {
		t.Errorf("Operation = %q", v)
	}
	if v, _ := fieldValue(a.Fields, "Caller"); v != "alice@contoso.com" {
		t.Errorf("Caller = %q", v)
	}
	sh := parse(t, fixtures.ServiceHealthFired)
	if v, _ := fieldValue(sh.Fields, "Region"); v != "East US" {
		t.Errorf("Region = %q", v)
	}
	rh := parse(t, fixtures.ResourceHealthFired)
	if v, _ := fieldValue(rh.Fields, "Current status"); v != "Unavailable" {
		t.Errorf("Current status = %q", v)
	}
	if v, _ := fieldValue(rh.Fields, "Cause"); v != "PlatformInitiated" {
		t.Errorf("Cause = %q", v)
	}
}

func TestPrometheusExtraction(t *testing.T) {
	m := parse(t, fixtures.PrometheusFired)
	if v, _ := fieldValue(m.Fields, "Summary"); v != "Pod is crash looping" {
		t.Errorf("Summary = %q", v)
	}
	if v, _ := fieldValue(m.Fields, "Rule group"); v != "Prometheus Recommended Pod level Alerts - prod-aks" {
		t.Errorf("Rule group = %q (want decoded)", v)
	}
	if _, ok := fieldValue(m.Fields, "Labels"); ok {
		t.Error("labels should be in m.Labels, not m.Fields")
	}
	if v, _ := fieldValue(m.Labels, "namespace"); v != "payments" {
		t.Errorf("label namespace = %q", v)
	}
	for _, bad := range []string{"alertname", "severity", "microsoft.resourceid", "microsoft.subscriptionid"} {
		if _, ok := fieldValue(m.Labels, bad); ok {
			t.Errorf("label %q should be filtered out", bad)
		}
	}
	if !strings.Contains(m.InvestigationLink, "Investigate.ReactView") {
		t.Errorf("investigationLink = %q", m.InvestigationLink)
	}
}

func TestGenericExtraction(t *testing.T) {
	m := parse(t, fixtures.UnknownService)
	if v, _ := fieldValue(m.Fields, "someField"); v != "some value" {
		t.Errorf("someField = %q", v)
	}
	if v, _ := fieldValue(m.Fields, "count"); v != "42" {
		t.Errorf("count = %q", v)
	}
}
