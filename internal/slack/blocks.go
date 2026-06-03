// Package slack renders an alert Model into a Slack message and sends it.
package slack

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/bonddim/azure-alerts-proxy/internal/alert"
	"github.com/slack-go/slack"
)

// Message is a fully-formed Slack message: a notification fallback plus a
// single coloured attachment of blocks (no top-level text, to avoid a
// duplicate header line).
type Message struct {
	Fallback   string
	Attachment slack.Attachment
}

var mdLink = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^\s)]+)\)`)

// mdToSlack converts GitHub-style markdown links [text](url) to Slack <url|text>.
func mdToSlack(s string) string {
	return mdLink.ReplaceAllString(s, "<$2|$1>")
}

// colorFor: green when resolved, otherwise graded by severity (Sev4 = grey).
func colorFor(m alert.Model) string {
	if m.Resolved() {
		return "#2EB67D"
	}
	switch m.SeverityNumber {
	case 0, 1:
		return "#E01E5A"
	case 2:
		return "#d39420"
	case 3:
		return "#4263c0"
	default:
		return "#868686"
	}
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

func richTextDetails(facts, labels []alert.Field) *slack.RichTextBlock {
	var b strings.Builder
	writeRows := func(rows []alert.Field) {
		for _, row := range rows {
			b.WriteString(row.Label)
			b.WriteString("\t\t")
			b.WriteString(row.Value)
			b.WriteByte('\n')
		}
	}

	writeRows(facts)
	if len(labels) > 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		writeRows(labels)
	}

	text := strings.TrimRight(truncate(b.String(), 3000), "\n")
	return slack.NewRichTextBlock("",
		slack.NewRichTextSection(slack.NewRichTextSectionTextElement(text, nil)))
}

func richTextCell(text string) *slack.RichTextBlock {
	return slack.NewRichTextBlock("",
		slack.NewRichTextSection(slack.NewRichTextSectionTextElement(text, nil)))
}

func labelsTable(labels []alert.Field) *slack.TableBlock {
	return fieldsTable([]alert.Field{{Label: "name", Value: "value"}}, labels)
}

func fieldsTable(header, rows []alert.Field) *slack.TableBlock {
	table := slack.NewTableBlock("").WithColumnSettings(
		slack.ColumnSetting{Align: slack.ColumnAlignmentLeft, IsWrapped: true},
		slack.ColumnSetting{Align: slack.ColumnAlignmentLeft, IsWrapped: true},
	)
	for _, row := range header {
		table.AddRow(richTextCell(row.Label), richTextCell(row.Value))
	}
	for _, row := range rows {
		table.AddRow(richTextCell(row.Label), richTextCell(row.Value))
	}
	return table
}

func affectedResourceContexts(m alert.Model) []slack.Block {
	blocks := make([]slack.Block, 0, len(m.AffectedResources))
	for _, resource := range m.AffectedResources {
		group, name := resourceParts(resource)
		if group == "" {
			group = m.ResourceGroupName
		}
		blocks = append(blocks, slack.NewContextBlock("", mrkdwn(
			fmt.Sprintf("*Resource group:* %s  |  *Resource Name:* %s", group, name))))
	}
	return blocks
}

func resourceParts(resource string) (string, string) {
	parts := strings.Split(resource, "/")
	name := resource
	if len(parts) > 0 && parts[len(parts)-1] != "" {
		name = parts[len(parts)-1]
	}
	group := ""
	for i := 0; i < len(parts)-1; i++ {
		if strings.EqualFold(parts[i], "resourceGroups") {
			group = parts[i+1]
			break
		}
	}
	return group, name
}

func detailFields(m alert.Model) []alert.Field {
	switch m.MonitoringService {
	case "Prometheus", "Platform", "Log Analytics", "Log Alerts V2", "Application Insights":
		return nil
	}

	facts := make([]alert.Field, 0, len(m.Fields)+1)
	if m.ResourceGroupName != "" {
		facts = append(facts, alert.Field{Label: "Resource group", Value: m.ResourceGroupName})
	}
	for _, f := range m.Fields {
		if !isSkippedDetailField(f) && f.Value != m.Description {
			facts = append(facts, f)
		}
	}
	return facts
}

func isSkippedDetailField(f alert.Field) bool {
	switch f.Label {
	case "Resource", "AlertData", "AlertCategory":
		return true
	default:
		return false
	}
}

func mrkdwn(text string) *slack.TextBlockObject {
	return slack.NewTextBlockObject(slack.MarkdownType, text, false, false)
}

func titleBlock(condition string, m alert.Model) *slack.SectionBlock {
	title := truncate(fmt.Sprintf("%s: %s", condition, m.AlertRule), 150)
	if isHTTPURL(m.PortalAlertURL) {
		title = fmt.Sprintf("<%s|%s>", m.PortalAlertURL, title)
	}
	return slack.NewSectionBlock(mrkdwn("*"+title+"*"), nil, nil)
}

func severityText(m alert.Model) string {
	switch m.SeverityNumber {
	case 0:
		return "Critical"
	case 1:
		return "Error"
	case 2:
		return "Warning"
	case 3:
		return "Informational"
	case 4:
		return "Verbose"
	default:
		if m.Severity == "" {
			return "Unknown"
		}
		return m.Severity
	}
}

func formatAlertTime(v string) string {
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return v
	}
	return t.UTC().Format("2006-01-02 15:04:05 UTC")
}

// Build constructs the Slack message for an alert. Used for both the initial
// Fired post and the Resolved update.
func Build(m alert.Model) Message {
	condition := "Fired"
	if m.Resolved() {
		condition = "Resolved"
	}

	var blocks []slack.Block

	blocks = append(blocks, titleBlock(condition, m))

	ctx := []string{
		"*Severity:* " + severityText(m),
	}
	if m.MonitoringService != "" {
		ctx = append(ctx, "*Service:* "+m.MonitoringService)
	}
	if m.Resolved() && m.ResolvedDateTime != "" {
		ctx = append(ctx, "*Resolved:* "+formatAlertTime(m.ResolvedDateTime))
	}
	if m.FiredDateTime != "" {
		ctx = append(ctx, "*Fired:* "+formatAlertTime(m.FiredDateTime))
	}
	blocks = append(blocks, slack.NewContextBlock("", mrkdwn(strings.Join(ctx, "  |  "))))

	blocks = append(blocks, affectedResourceContexts(m)...)

	if m.Description != "" {
		blocks = append(blocks, slack.NewSectionBlock(mrkdwn(truncate(mdToSlack(m.Description), 3000)), nil, nil))
	}

	facts := detailFields(m)
	if m.MonitoringService == "Prometheus" && len(m.Labels) > 0 {
		blocks = append(blocks, labelsTable(m.Labels))
	} else if strings.HasPrefix(m.MonitoringService, "Activity Log") && len(facts) > 0 {
		blocks = append(blocks, fieldsTable(nil, facts))
	} else if len(facts) > 0 || len(m.Labels) > 0 {
		blocks = append(blocks, richTextDetails(facts, m.Labels))
	}

	var buttons []slack.BlockElement
	if resourceURL := firstResourceURL(m); resourceURL != "" {
		btn := slack.NewButtonBlockElement("go_to_resource", "",
			slack.NewTextBlockObject(slack.PlainTextType, "Go to resource", true, false))
		btn.URL = resourceURL
		buttons = append(buttons, btn)
	}
	for i, l := range m.Links {
		if !isHTTPURL(l.Value) {
			continue
		}
		label := truncate(l.Label, 75)
		if label == "" {
			label = "Open link"
		}
		btn := slack.NewButtonBlockElement(fmt.Sprintf("alert_link_%d", i), "",
			slack.NewTextBlockObject(slack.PlainTextType, label, true, false))
		btn.URL = l.Value
		buttons = append(buttons, btn)
	}
	if isHTTPURL(m.InvestigationLink) {
		btn := slack.NewButtonBlockElement("investigate", "",
			slack.NewTextBlockObject(slack.PlainTextType, "Investigate with agent", true, false))
		btn.URL = m.InvestigationLink
		buttons = append(buttons, btn)
	}
	if len(buttons) > 0 {
		blocks = append(blocks, slack.NewActionBlock("", buttons...))
	}

	fallback := fmt.Sprintf("%s: %s (%s)", condition, m.AlertRule, severityText(m))
	return Message{
		Fallback: fallback,
		Attachment: slack.Attachment{
			Color:    colorFor(m),
			Fallback: fmt.Sprintf("%s: %s", condition, m.AlertRule),
			Blocks:   slack.Blocks{BlockSet: blocks},
		},
	}
}

func isHTTPURL(v string) bool {
	return strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://")
}

func firstResourceURL(m alert.Model) string {
	for _, candidate := range m.AffectedResources {
		if isAzureResourceID(candidate) {
			return portalBase(m) + "/#view/HubsExtension/ResourceMenuBlade/~/overview/id/" + url.QueryEscape(candidate)
		}
	}
	for _, f := range m.Fields {
		if f.Label == "Resource" && isAzureResourceID(f.Value) {
			return portalBase(m) + "/#view/HubsExtension/ResourceMenuBlade/~/overview/id/" + url.QueryEscape(f.Value)
		}
	}
	return ""
}

func isAzureResourceID(v string) bool {
	return strings.HasPrefix(strings.ToLower(v), "/subscriptions/")
}

func portalBase(m alert.Model) string {
	if i := strings.Index(m.PortalAlertURL, "/#"); i > 0 {
		return m.PortalAlertURL[:i]
	}
	return "https://portal.azure.com"
}
