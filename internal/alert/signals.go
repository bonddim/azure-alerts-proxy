package alert

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type dimension struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// --- Metric (monitoringService = "Platform") ---

type metricContext struct {
	ConditionType string `json:"conditionType"`
	Condition     struct {
		WindowSize string `json:"windowSize"`
		AllOf      []struct {
			MetricName      string      `json:"metricName"`
			MetricNamespace string      `json:"metricNamespace"`
			Operator        string      `json:"operator"`
			Threshold       string      `json:"threshold"`
			TimeAggregation string      `json:"timeAggregation"`
			Dimensions      []dimension `json:"dimensions"`
			MetricValue     *float64    `json:"metricValue"`
			WebTestName     string      `json:"webTestName"`
		} `json:"allOf"`
		WindowStartTime string `json:"windowStartTime"`
		WindowEndTime   string `json:"windowEndTime"`
	} `json:"condition"`
}

func extractMetric(raw json.RawMessage) signalExtraction {
	var c metricContext
	_ = json.Unmarshal(raw, &c)
	var fields []Field
	for _, cond := range c.Condition.AllOf {
		if cond.WebTestName != "" {
			fields = append(fields, Field{"Web test", cond.WebTestName})
			continue
		}
		if cond.MetricName != "" {
			ns := ""
			if cond.MetricNamespace != "" {
				ns = " (" + cond.MetricNamespace + ")"
			}
			fields = append(fields, Field{"Metric", cond.MetricName + ns})
		}
		if cond.Operator != "" || cond.Threshold != "" {
			fields = append(fields, Field{"Condition",
				strings.TrimSpace(fmt.Sprintf("%s %s %s", cond.TimeAggregation, cond.Operator, cond.Threshold))})
		}
		if cond.MetricValue != nil {
			fields = append(fields, Field{"Observed value", trimFloat(*cond.MetricValue)})
		}
		if d := formatDimensions(cond.Dimensions); d != "" {
			fields = append(fields, Field{"Dimensions", d})
		}
	}
	if c.ConditionType != "" {
		fields = append(fields, Field{"Condition type", c.ConditionType})
	}
	if c.Condition.WindowSize != "" {
		fields = append(fields, Field{"Window size", c.Condition.WindowSize})
	}
	return signalExtraction{fields: fields}
}

// --- Log search (Log Analytics / Log Alerts V2 / Application Insights) ---

type logContext struct {
	Condition struct {
		WindowSize string `json:"windowSize"`
		AllOf      []struct {
			SearchQuery                   string      `json:"searchQuery"`
			MetricMeasureColumn           string      `json:"metricMeasureColumn"`
			Operator                      string      `json:"operator"`
			Threshold                     string      `json:"threshold"`
			TimeAggregation               string      `json:"timeAggregation"`
			Dimensions                    []dimension `json:"dimensions"`
			MetricValue                   *float64    `json:"metricValue"`
			LinkToSearchResultsUI         string      `json:"linkToSearchResultsUI"`
			LinkToFilteredSearchResultsUI string      `json:"linkToFilteredSearchResultsUI"`
		} `json:"allOf"`
	} `json:"condition"`
}

func extractLog(raw json.RawMessage) signalExtraction {
	var c logContext
	_ = json.Unmarshal(raw, &c)
	var fields, links []Field
	for _, cond := range c.Condition.AllOf {
		if cond.SearchQuery != "" {
			fields = append(fields, Field{"Query", truncateRunes(cond.SearchQuery, 400)})
		}
		if cond.Operator != "" || cond.Threshold != "" {
			col := cond.MetricMeasureColumn
			if col == "" {
				col = "results"
			}
			fields = append(fields, Field{"Condition",
				strings.TrimSpace(fmt.Sprintf("%s %s %s %s", cond.TimeAggregation, col, cond.Operator, cond.Threshold))})
		}
		if cond.MetricValue != nil {
			fields = append(fields, Field{"Observed value", trimFloat(*cond.MetricValue)})
		}
		if d := formatDimensions(cond.Dimensions); d != "" {
			fields = append(fields, Field{"Dimensions", d})
		}
		if cond.LinkToSearchResultsUI != "" {
			links = append(links, Field{"Search results", cond.LinkToSearchResultsUI})
		} else if cond.LinkToFilteredSearchResultsUI != "" {
			links = append(links, Field{"Filtered results", cond.LinkToFilteredSearchResultsUI})
		}
	}
	if c.Condition.WindowSize != "" {
		fields = append(fields, Field{"Window size", c.Condition.WindowSize})
	}
	return signalExtraction{fields: fields, links: links}
}

// --- Activity log (Administrative/Policy/Autoscale/Security) ---

type activityLogContext struct {
	OperationName        string            `json:"operationName"`
	Status               string            `json:"status"`
	SubStatus            string            `json:"subStatus"`
	Level                string            `json:"level"`
	EventSource          string            `json:"eventSource"`
	Caller               string            `json:"caller"`
	ResourceProviderName string            `json:"resourceProviderName"`
	ResourceID           string            `json:"resourceId"`
	EventTimestamp       string            `json:"eventTimestamp"`
	Properties           map[string]string `json:"properties"`
}

func extractActivityLog(raw json.RawMessage) signalExtraction {
	var c activityLogContext
	_ = json.Unmarshal(raw, &c)
	var fields []Field
	add := func(label, value string) {
		if value != "" {
			fields = append(fields, Field{label, value})
		}
	}
	add("Operation", c.OperationName)
	add("Status", c.Status)
	add("Sub-status", c.SubStatus)
	add("Level", c.Level)
	add("Event source", c.EventSource)
	add("Provider", c.ResourceProviderName)
	add("Caller", c.Caller)
	add("Event time", c.EventTimestamp)
	return signalExtraction{fields: fields}
}

func extractServiceHealth(raw json.RawMessage) signalExtraction {
	var c activityLogContext
	_ = json.Unmarshal(raw, &c)
	var fields []Field
	add := func(label, value string) {
		if value != "" {
			fields = append(fields, Field{label, value})
		}
	}
	p := c.Properties
	add("Incident type", p["incidentType"])
	add("Service", p["service"])
	add("Region", p["region"])
	add("Impacted services", p["impactedServices"])
	add("Stage", p["stage"])
	add("Communication", p["communicationId"])
	add("Status", c.Status)
	add("Event time", c.EventTimestamp)
	return signalExtraction{fields: fields}
}

func extractResourceHealth(raw json.RawMessage) signalExtraction {
	var c activityLogContext
	_ = json.Unmarshal(raw, &c)
	var fields []Field
	add := func(label, value string) {
		if value != "" {
			fields = append(fields, Field{label, value})
		}
	}
	p := c.Properties
	add("Current status", p["currentHealthStatus"])
	add("Previous status", p["previousHealthStatus"])
	add("Type", p["type"])
	add("Cause", p["cause"])
	add("Resource", c.ResourceID)
	add("Event time", c.EventTimestamp)
	return signalExtraction{fields: fields}
}

// --- Managed Prometheus (monitoringService = "Prometheus") ---

type prometheusContext struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	RuleGroup   string            `json:"ruleGroup"`
	Expression  string            `json:"expression"`
}

func extractPrometheus(raw json.RawMessage) signalExtraction {
	var c prometheusContext
	_ = json.Unmarshal(raw, &c)
	var fields []Field
	add := func(label, value string) {
		if value != "" {
			fields = append(fields, Field{label, value})
		}
	}
	add("Alert name", c.Labels["alertname"])
	add("Summary", c.Annotations["summary"])
	add("Description", c.Annotations["description"])
	add("Expression", c.Expression)
	add("Rule group", ruleGroupName(c.RuleGroup))

	// Functional labels, sorted, excluding internal/verbose noise.
	keys := make([]string, 0, len(c.Labels))
	for k := range c.Labels {
		if isNoiseLabel(k) {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	labels := make([]Field, 0, len(keys))
	for _, k := range keys {
		labels = append(labels, Field{k, c.Labels[k]})
	}
	return signalExtraction{fields: fields, labels: labels}
}

func isNoiseLabel(name string) bool {
	return name == "alertname" || name == "severity" || strings.HasPrefix(name, "microsoft.")
}

func ruleGroupName(ruleGroup string) string {
	if ruleGroup == "" {
		return ""
	}
	parts := strings.Split(ruleGroup, "/")
	last := parts[len(parts)-1]
	if decoded, err := url.QueryUnescape(last); err == nil {
		return decoded
	}
	return last
}

// --- Generic fallback for unknown / future monitoringService values ---

func extractGeneric(raw json.RawMessage) signalExtraction {
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var fields []Field
	for _, k := range keys {
		v := m[k]
		if v == nil {
			continue
		}
		rendered := display(v)
		if rendered == "" {
			continue
		}
		fields = append(fields, Field{k, rendered})
		if len(fields) >= 10 {
			break
		}
	}
	return signalExtraction{fields: fields}
}

func display(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return trimFloat(t)
	case bool:
		return fmt.Sprintf("%t", t)
	case map[string]any:
		if len(t) == 0 {
			return ""
		}
		b, _ := json.Marshal(t)
		return truncateRunes(string(b), 300)
	case []any:
		if len(t) == 0 {
			return ""
		}
		b, _ := json.Marshal(t)
		return truncateRunes(string(b), 300)
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

func formatDimensions(dims []dimension) string {
	if len(dims) == 0 {
		return ""
	}
	parts := make([]string, 0, len(dims))
	for _, d := range dims {
		parts = append(parts, d.Name+"="+d.Value)
	}
	return strings.Join(parts, ", ")
}

// trimFloat renders a float without a trailing ".0" for whole numbers.
func trimFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
