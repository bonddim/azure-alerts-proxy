// Package fixtures holds sample Common Alert Schema payloads for tests.
// Modelled on Microsoft's "Sample alert payloads" documentation.
package fixtures

// MetricStaticFired is a static-threshold metric alert (Sev2, Platform).
const MetricStaticFired = `{
  "schemaId": "azureMonitorCommonAlertSchema",
  "data": {
    "essentials": {
      "alertId": "/subscriptions/11111111-1111-1111-1111-111111111111/providers/Microsoft.AlertsManagement/alerts/aaaa1111-bbbb",
      "alertRule": "High CPU on web vm",
      "severity": "Sev2",
      "signalType": "Metric",
      "monitorCondition": "Fired",
      "monitoringService": "Platform",
      "alertTargetIDs": ["/subscriptions/11111111-1111-1111-1111-111111111111/resourcegroups/prod-rg/providers/microsoft.compute/virtualmachines/web-vm-1"],
      "configurationItems": ["web-vm-1"],
      "firedDateTime": "2026-05-27T10:00:00.000Z",
      "description": "CPU is above the configured threshold.",
      "resourceGroupName": "prod-rg"
    },
    "alertContext": {
      "conditionType": "Static Threshold",
      "condition": {
        "windowSize": "PT5M",
        "allOf": [{
          "metricName": "Percentage CPU",
          "metricNamespace": "Microsoft.Compute/virtualMachines",
          "operator": "GreaterThan",
          "threshold": "90",
          "timeAggregation": "Average",
          "dimensions": [{"name": "host", "value": "web-vm-1"}],
          "metricValue": 97.5
        }]
      }
    },
    "customProperties": {"team": "platform", "slack-channel": "#prod-incidents"}
  }
}`

// MetricResolved is MetricStaticFired in the Resolved condition.
const MetricResolved = `{
  "schemaId": "azureMonitorCommonAlertSchema",
  "data": {
    "essentials": {
      "alertId": "/subscriptions/11111111-1111-1111-1111-111111111111/providers/Microsoft.AlertsManagement/alerts/aaaa1111-bbbb",
      "alertRule": "High CPU on web vm",
      "severity": "Sev2",
      "signalType": "Metric",
      "monitorCondition": "Resolved",
      "monitoringService": "Platform",
      "configurationItems": ["web-vm-1"],
      "firedDateTime": "2026-05-27T10:00:00.000Z",
      "resolvedDateTime": "2026-05-27T10:30:00.000Z",
      "description": "CPU is above the configured threshold.",
      "resourceGroupName": "prod-rg"
    },
    "alertContext": {"conditionType": "Static Threshold", "condition": {"allOf": []}},
    "customProperties": {"slack-channel": "#prod-incidents"}
  }
}`

// PrometheusFired is a managed-Prometheus alert with noisy labels + a markdown
// link in the description and an investigation link.
const PrometheusFired = `{
  "schemaId": "azureMonitorCommonAlertSchema",
  "data": {
    "essentials": {
      "alertId": "/subscriptions/11111111-1111-1111-1111-111111111111/providers/Microsoft.AlertsManagement/alerts/prom-1",
      "alertRule": "KubePodCrashLooping",
      "severity": "Sev2",
      "signalType": "Metric",
      "monitorCondition": "Fired",
      "monitoringService": "Prometheus",
      "alertTargetIDs": ["/subscriptions/11111111-1111-1111-1111-111111111111/resourcegroups/prod-rg/providers/microsoft.monitor/accounts/prod-amw"],
      "configurationItems": ["prod-aks"],
      "firedDateTime": "2026-05-27T16:00:00.000Z",
      "resourceGroupName": "prod-rg",
      "description": "Pod payments/payments-7d9f-xyz is crash looping. See this [link](https://aka.ms/aks-alerts/pod-level-recommended-alerts).",
      "investigationLink": "https://portal.azure.com/#view/Microsoft_Azure_Monitoring_Alerts/Investigate.ReactView/alertId/prom-1"
    },
    "alertContext": {
      "labels": {
        "alertname": "KubePodCrashLooping",
        "severity": "warning",
        "namespace": "payments",
        "pod": "payments-7d9f-xyz",
        "microsoft.resourceid": "/subscriptions/11111111-1111-1111-1111-111111111111/resourcegroups/prod-rg/providers/microsoft.containerservice/managedclusters/prod-aks",
        "microsoft.subscriptionid": "11111111-1111-1111-1111-111111111111"
      },
      "annotations": {
        "summary": "Pod is crash looping",
        "description": "Pod payments/payments-7d9f-xyz is crash looping. See this [link](https://aka.ms/aks-alerts/pod-level-recommended-alerts)."
      },
      "ruleGroup": "/subscriptions/11111111-1111-1111-1111-111111111111/resourcegroups/prod-rg/providers/microsoft.alertsmanagement/prometheusrulegroups/Prometheus%20Recommended%20Pod%20level%20Alerts%20-%20prod-aks",
      "expression": "rate(kube_pod_container_status_restarts_total[10m]) > 0"
    },
    "customProperties": null
  }
}`

// LogSearchFired is a log search alert (Log Alerts V2).
const LogSearchFired = `{
  "schemaId": "azureMonitorCommonAlertSchema",
  "data": {
    "essentials": {
      "alertId": "/subscriptions/11111111-1111-1111-1111-111111111111/providers/Microsoft.AlertsManagement/alerts/log-1",
      "alertRule": "Too many 5xx in logs",
      "severity": "Sev1",
      "signalType": "Log",
      "monitorCondition": "Fired",
      "monitoringService": "Log Alerts V2",
      "configurationItems": ["prod-law"],
      "firedDateTime": "2026-05-27T12:00:00.000Z",
      "resourceGroupName": "prod-rg"
    },
    "alertContext": {
      "condition": {
        "windowSize": "PT10M",
        "allOf": [{
          "searchQuery": "AppRequests | where ResultCode >= 500 | summarize count()",
          "metricMeasureColumn": "count_",
          "operator": "GreaterThan",
          "threshold": "100",
          "timeAggregation": "Count",
          "metricValue": 532,
          "linkToSearchResultsUI": "https://portal.azure.com/#search/results"
        }]
      }
    },
    "customProperties": null
  }
}`

// ActivityLogAdministrative is an activity-log administrative alert.
const ActivityLogAdministrative = `{
  "schemaId": "azureMonitorCommonAlertSchema",
  "data": {
    "essentials": {
      "alertId": "/subscriptions/11111111-1111-1111-1111-111111111111/providers/Microsoft.AlertsManagement/alerts/activity-1",
      "alertRule": "VM deleted",
      "severity": "Sev4",
      "signalType": "Activity Log",
      "monitorCondition": "Fired",
      "monitoringService": "Activity Log - Administrative",
      "configurationItems": ["web-vm-1"],
      "firedDateTime": "2026-05-27T13:00:00.000Z",
      "resourceGroupName": "prod-rg"
    },
    "alertContext": {
      "operationName": "Microsoft.Compute/virtualMachines/delete",
      "status": "Succeeded",
      "level": "Informational",
      "eventSource": "Administrative",
      "caller": "alice@contoso.com",
      "resourceProviderName": "Microsoft.Compute",
      "eventTimestamp": "2026-05-27T12:59:30Z"
    },
    "customProperties": null
  }
}`

// BudgetFired is a Cost Management budget alert payload with verbose generic
// fields that should not be rendered as Slack detail rows.
const BudgetFired = `{
  "schemaId": "azureMonitorCommonAlertSchema",
  "data": {
    "essentials": {
      "alertId": "/subscriptions/11111111-1111-1111-1111-111111111111/providers/Microsoft.AlertsManagement/alerts/budget-1",
      "alertRule": "Test_actual_cost_budget",
      "severity": "Sev3",
      "signalType": "Metric",
      "monitorCondition": "Fired",
      "monitoringService": "CostAlerts",
      "configurationItems": ["budgets"],
      "firedDateTime": "2026-05-29T14:23:37.000Z",
      "description": "Your spend for budget Test_actual_cost_budget is now $11,111.00 exceeding your specified threshold $25.00.",
      "resourceGroupName": ""
    },
    "alertContext": {
      "AlertCategory": "budgets",
      "AlertData": {
        "BudgetCreator": "test@sample.test",
        "BudgetId": "/subscriptions/11111111-1111-1111-1111-111111111111/providers/Microsoft.Consumption/budgets/Test_actual_cost_budget",
        "BudgetName": "Test_actual_cost_budget",
        "BudgetStartDate": "2022-11-01",
        "BudgetThreshold": "$50.00",
        "BudgetType": "Cost"
      }
    },
    "customProperties": null
  }
}`

// ServiceHealthFired is a Service Health alert.
const ServiceHealthFired = `{
  "schemaId": "azureMonitorCommonAlertSchema",
  "data": {
    "essentials": {
      "alertId": "/subscriptions/11111111-1111-1111-1111-111111111111/providers/Microsoft.AlertsManagement/alerts/sh-1",
      "alertRule": "Service issue in East US",
      "severity": "Sev3",
      "signalType": "Activity Log",
      "monitorCondition": "Fired",
      "monitoringService": "ServiceHealth",
      "firedDateTime": "2026-05-27T14:00:00.000Z"
    },
    "alertContext": {
      "status": "Active",
      "eventTimestamp": "2026-05-27T13:55:00Z",
      "properties": {
        "incidentType": "Incident",
        "service": "Virtual Machines",
        "region": "East US",
        "stage": "Active",
        "communicationId": "comm-123"
      }
    },
    "customProperties": null
  }
}`

// ResourceHealthFired is a Resource Health alert.
const ResourceHealthFired = `{
  "schemaId": "azureMonitorCommonAlertSchema",
  "data": {
    "essentials": {
      "alertId": "/subscriptions/11111111-1111-1111-1111-111111111111/providers/Microsoft.AlertsManagement/alerts/rh-1",
      "alertRule": "Resource became unavailable",
      "severity": "Sev1",
      "signalType": "Activity Log",
      "monitorCondition": "Fired",
      "monitoringService": "ResourceHealth",
      "configurationItems": ["sql-db-1"],
      "firedDateTime": "2026-05-27T15:00:00.000Z",
      "resourceGroupName": "prod-rg"
    },
    "alertContext": {
      "resourceId": "/subscriptions/11111111-1111-1111-1111-111111111111/resourcegroups/prod-rg/providers/microsoft.sql/servers/srv/databases/sql-db-1",
      "eventTimestamp": "2026-05-27T14:58:00Z",
      "properties": {
        "currentHealthStatus": "Unavailable",
        "previousHealthStatus": "Available",
        "type": "Downtime",
        "cause": "PlatformInitiated"
      }
    },
    "customProperties": null
  }
}`

// UnknownService exercises the generic fallback extractor.
const UnknownService = `{
  "schemaId": "azureMonitorCommonAlertSchema",
  "data": {
    "essentials": {
      "alertId": "/subscriptions/11111111-1111-1111-1111-111111111111/providers/Microsoft.AlertsManagement/alerts/unknown-1",
      "alertRule": "Future signal type",
      "severity": "Sev3",
      "signalType": "Unknown",
      "monitorCondition": "Fired",
      "monitoringService": "SomeFutureService",
      "configurationItems": ["thing-1"],
      "firedDateTime": "2026-05-27T17:00:00.000Z"
    },
    "alertContext": {"someField": "some value", "count": 42, "nested": {"a": 1}},
    "customProperties": null
  }
}`
