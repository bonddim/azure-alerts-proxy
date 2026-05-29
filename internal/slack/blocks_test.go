package slack_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/bonddim/azure-alerts-proxy/internal/alert"
	"github.com/bonddim/azure-alerts-proxy/internal/fixtures"
	slackpkg "github.com/bonddim/azure-alerts-proxy/internal/slack"
	"github.com/slack-go/slack"
)

func build(t *testing.T, payload string) slackpkg.Message {
	t.Helper()
	var s alert.CommonAlertSchema
	if err := json.Unmarshal([]byte(payload), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return slackpkg.Build(alert.ParseAlert(s, alert.Options{}))
}

func blockText(b slack.Block) string {
	switch v := b.(type) {
	case *slack.HeaderBlock:
		return v.Text.Text
	case *slack.SectionBlock:
		if v.Text != nil {
			return v.Text.Text
		}
	}
	return ""
}

func richTextContent(b slack.Block) string {
	rb, ok := b.(*slack.RichTextBlock)
	if !ok {
		return ""
	}
	var out strings.Builder
	for _, el := range rb.Elements {
		section, ok := el.(*slack.RichTextSection)
		if !ok {
			continue
		}
		for _, sectionEl := range section.Elements {
			if text, ok := sectionEl.(*slack.RichTextSectionTextElement); ok {
				out.WriteString(text.Text)
			}
		}
	}
	return out.String()
}

func actionLabels(att slack.Attachment) []string {
	var labels []string
	for _, b := range att.Blocks.BlockSet {
		if a, ok := b.(*slack.ActionBlock); ok {
			for _, el := range a.Elements.ElementSet {
				if btn, ok := el.(*slack.ButtonBlockElement); ok {
					labels = append(labels, btn.Text.Text)
				}
			}
		}
	}
	return labels
}

func actionURLs(att slack.Attachment) map[string]string {
	urls := make(map[string]string)
	for _, b := range att.Blocks.BlockSet {
		if a, ok := b.(*slack.ActionBlock); ok {
			for _, el := range a.Elements.ElementSet {
				if btn, ok := el.(*slack.ButtonBlockElement); ok {
					urls[btn.Text.Text] = btn.URL
				}
			}
		}
	}
	return urls
}

func countHeaders(att slack.Attachment) int {
	n := 0
	for _, b := range att.Blocks.BlockSet {
		if _, ok := b.(*slack.HeaderBlock); ok {
			n++
		}
	}
	return n
}

func countContexts(att slack.Attachment) int {
	n := 0
	for _, b := range att.Blocks.BlockSet {
		if _, ok := b.(*slack.ContextBlock); ok {
			n++
		}
	}
	return n
}

func contextText(att slack.Attachment) string {
	var out []string
	for _, b := range att.Blocks.BlockSet {
		if ctx, ok := b.(*slack.ContextBlock); ok {
			for _, el := range ctx.ContextElements.Elements {
				if text, ok := el.(*slack.TextBlockObject); ok {
					out = append(out, text.Text)
				}
			}
		}
	}
	return strings.Join(out, "\n")
}

func allRichText(att slack.Attachment) string {
	var out []string
	for _, b := range att.Blocks.BlockSet {
		if text := richTextContent(b); text != "" {
			out = append(out, text)
		}
	}
	return strings.Join(out, "\n")
}

func tableTexts(att slack.Attachment) []string {
	var tables []string
	for _, b := range att.Blocks.BlockSet {
		table, ok := b.(*slack.TableBlock)
		if !ok {
			continue
		}
		var rows []string
		for _, row := range table.Rows {
			var cells []string
			for _, cell := range row {
				cells = append(cells, richTextContent(cell))
			}
			rows = append(rows, strings.Join(cells, "\t"))
		}
		tables = append(tables, strings.Join(rows, "\n"))
	}
	return tables
}

func tableContaining(att slack.Attachment, text string) string {
	for _, table := range tableTexts(att) {
		if strings.Contains(table, text) {
			return table
		}
	}
	return ""
}

func countTables(att slack.Attachment) int {
	n := 0
	for _, b := range att.Blocks.BlockSet {
		if _, ok := b.(*slack.TableBlock); ok {
			n++
		}
	}
	return n
}

func TestFiredMessage(t *testing.T) {
	msg := build(t, fixtures.MetricStaticFired)
	if !strings.Contains(msg.Fallback, "Fired:") {
		t.Errorf("fallback = %q", msg.Fallback)
	}
	if msg.Attachment.Color != "#d39420" {
		t.Errorf("color = %q, want Sev2 amber", msg.Attachment.Color)
	}
	if strings.Contains(msg.Fallback, ":") && strings.Contains(msg.Fallback, "_") {
		t.Errorf("fallback should not include emoji: %q", msg.Fallback)
	}
	if n := countHeaders(msg.Attachment); n != 0 {
		t.Errorf("header count = %d, want 0", n)
	}
	title := blockText(msg.Attachment.Blocks.BlockSet[0])
	if !strings.Contains(title, "*<https://portal.azure.com/#view/Microsoft_Azure_Monitoring/AlertDetailsTemplateBlade/alertId/") ||
		!strings.Contains(title, "|Fired: High CPU on web vm>*") {
		t.Errorf("linked title = %q", title)
	}
	if n := countContexts(msg.Attachment); n != 2 {
		t.Errorf("context count = %d, want 2", n)
	}
	ctx := contextText(msg.Attachment)
	if !strings.Contains(ctx, "*Severity:* Warning") {
		t.Errorf("human-readable severity missing from context: %q", ctx)
	}
	if !strings.Contains(ctx, "*Fired:* 2026-05-27 10:00:00 UTC") {
		t.Errorf("human-readable fired time missing from context: %q", ctx)
	}
	if strings.Contains(ctx, ".000Z") {
		t.Errorf("context should not include sub-second timestamp precision: %q", ctx)
	}
	if strings.Contains(ctx, "*Signal:*") {
		t.Errorf("signal should not be rendered in context: %q", ctx)
	}
	if strings.Contains(ctx, "*Resource:*") {
		t.Errorf("resource should not be rendered in context: %q", ctx)
	}
	if richText := allRichText(msg.Attachment); richText != "" {
		t.Errorf("platform metric details should be skipped, got:\n%s", richText)
	}
	if !strings.Contains(ctx, "*Resource group:* prod-rg  |  *Resource Name:* web-vm-1") {
		t.Errorf("affected resource context missing: %q", ctx)
	}
	for i, b := range msg.Attachment.Blocks.BlockSet {
		if text := blockText(b); strings.Contains(text, "CPU is above the configured threshold") {
			if i < 2 {
				t.Errorf("description block index = %d, want after context blocks", i)
			}
			break
		}
	}
	if got := actionLabels(msg.Attachment); contains(got, "View alert") {
		t.Errorf("action labels = %v", got)
	}
	urls := actionURLs(msg.Attachment)
	if !strings.Contains(urls["Go to resource"], "%2Fsubscriptions%2F") ||
		!strings.Contains(urls["Go to resource"], "web-vm-1") {
		t.Errorf("Go to resource URL = %q", urls["Go to resource"])
	}
}

func TestResolvedMessage(t *testing.T) {
	msg := build(t, fixtures.MetricResolved)
	if !strings.Contains(msg.Fallback, "Resolved:") {
		t.Errorf("fallback = %q", msg.Fallback)
	}
	if msg.Attachment.Color != "#2EB67D" {
		t.Errorf("color = %q, want green", msg.Attachment.Color)
	}
	if ctx := contextText(msg.Attachment); !strings.Contains(ctx, "*Resolved:* 2026-05-27 10:30:00 UTC") {
		t.Errorf("human-readable resolved time missing from context: %q", ctx)
	}
}

func TestSev4Grey(t *testing.T) {
	sev4 := strings.Replace(fixtures.MetricStaticFired, `"severity": "Sev2"`, `"severity": "Sev4"`, 1)
	if c := build(t, sev4).Attachment.Color; c != "#868686" {
		t.Errorf("Sev4 colour = %q, want grey", c)
	}
}

func TestMarkdownLinkAndDedupe(t *testing.T) {
	// Inspect block text directly: json.Marshal escapes < and >, hiding the link.
	msg := build(t, fixtures.PrometheusFired)
	var all []string
	linkBlocks := 0
	const url = "aks-alerts/pod-level-recommended-alerts"
	for _, b := range msg.Attachment.Blocks.BlockSet {
		text := blockText(b)
		all = append(all, text)
		if strings.Contains(text, url) {
			linkBlocks++
		}
	}
	joined := strings.Join(all, "\n")
	if !strings.Contains(joined, "<https://aka.ms/aks-alerts/pod-level-recommended-alerts|link>") {
		t.Error("markdown link not converted to Slack mrkdwn")
	}
	if strings.Contains(joined, "](https://aka.ms") {
		t.Error("raw markdown link leaked through")
	}
	// The description (the only field carrying the link) must not be duplicated.
	if linkBlocks != 1 {
		t.Errorf("description rendered in %d blocks, want 1", linkBlocks)
	}
}

func TestPrometheusLabelsTableBlock(t *testing.T) {
	msg := build(t, fixtures.PrometheusFired)
	if n := countTables(msg.Attachment); n != 1 {
		t.Fatalf("table count = %d, want 1", n)
	}
	if ctx := contextText(msg.Attachment); !strings.Contains(ctx, "*Resource group:* prod-rg  |  *Resource Name:* prod-amw") {
		t.Errorf("affected resource context missing: %q", ctx)
	}
	table := tableContaining(msg.Attachment, "namespace")
	if table == "" {
		t.Fatal("no table block with labels found")
	}
	if strings.Contains(table, "```") {
		t.Errorf("table should not use a markdown code block:\n%s", table)
	}
	if !strings.Contains(table, "payments") {
		t.Error("label value missing")
	}
	if !strings.Contains(table, "name\tvalue") {
		t.Errorf("labels header missing:\n%s", table)
	}
	if !strings.Contains(table, "namespace\tpayments") {
		t.Errorf("labels not rendered as data table rows:\n%s", table)
	}
	if strings.Contains(table, "microsoft.resourceid") || strings.Contains(table, "alertname") {
		t.Error("noise labels should be filtered from parsed labels table")
	}
	if strings.Contains(table, "Rule group") || strings.Contains(table, "Expression") || strings.Contains(table, "Summary") {
		t.Errorf("prometheus details other than labels should be skipped:\n%s", table)
	}
}

func TestActivityLogDetailsTableBlock(t *testing.T) {
	msg := build(t, fixtures.ActivityLogAdministrative)
	if n := countTables(msg.Attachment); n != 1 {
		t.Fatalf("table count = %d, want 1", n)
	}
	table := tableContaining(msg.Attachment, "Operation")
	if table == "" {
		t.Fatal("no table block with activity log details found")
	}
	for _, want := range []string{
		"Operation\tMicrosoft.Compute/virtualMachines/delete",
		"Status\tSucceeded",
		"Level\tInformational",
		"Event source\tAdministrative",
		"Caller\talice@contoso.com",
		"Event time\t2026-05-27T12:59:30Z",
	} {
		if !strings.Contains(table, want) {
			t.Errorf("activity table missing %q:\n%s", want, table)
		}
	}
}

func TestBudgetAlertFormatting(t *testing.T) {
	msg := build(t, fixtures.BudgetFired)
	if first := blockText(msg.Attachment.Blocks.BlockSet[0]); !strings.Contains(first, "|Fired: Test_actual_cost_budget>*") {
		t.Fatalf("header = %q, want alert rule", first)
	}
	joined := strings.Join([]string{
		contextText(msg.Attachment),
		allRichText(msg.Attachment),
		strings.Join(tableTexts(msg.Attachment), "\n"),
	}, "\n")
	if strings.Contains(joined, "AlertData") || strings.Contains(joined, "AlertCategory") {
		t.Errorf("budget noise fields should be skipped:\n%s", joined)
	}
	var sections []string
	for _, b := range msg.Attachment.Blocks.BlockSet {
		if text := blockText(b); text != "" {
			sections = append(sections, text)
		}
	}
	if !strings.Contains(strings.Join(sections, "\n"), "Your spend for budget Test_actual_cost_budget") {
		t.Errorf("description should remain in place: %v", sections)
	}
}

func TestPrometheusButtons(t *testing.T) {
	msg := build(t, fixtures.PrometheusFired)
	labels := actionLabels(msg.Attachment)
	if !contains(labels, "Investigate with agent") {
		t.Errorf("missing Investigate button: %v", labels)
	}
	if !contains(labels, "Go to resource") {
		t.Errorf("missing Go to resource button: %v", labels)
	}
}

func TestLogSearchLinksAreButtonsAndDetailsSkipped(t *testing.T) {
	msg := build(t, fixtures.LogSearchFired)
	if richText := allRichText(msg.Attachment); richText != "" {
		t.Errorf("log alert details should be skipped, got:\n%s", richText)
	}
	labels := actionLabels(msg.Attachment)
	if !contains(labels, "Search results") {
		t.Errorf("missing Search results button: %v", labels)
	}
	urls := actionURLs(msg.Attachment)
	if urls["Search results"] != "https://portal.azure.com/#search/results" {
		t.Errorf("Search results URL = %q", urls["Search results"])
	}
}

func TestHeaderTruncated(t *testing.T) {
	long := strings.Replace(fixtures.MetricStaticFired, "High CPU on web vm", strings.Repeat("x", 300), 1)
	msg := build(t, long)
	title := blockText(msg.Attachment.Blocks.BlockSet[0])
	visible := title
	if pipe := strings.LastIndex(visible, "|"); pipe >= 0 {
		visible = visible[pipe+1:]
	}
	visible = strings.TrimSuffix(strings.TrimSuffix(visible, ">*"), "*")
	if len([]rune(visible)) > 150 {
		t.Errorf("visible title length = %d, want <= 150", len([]rune(visible)))
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
