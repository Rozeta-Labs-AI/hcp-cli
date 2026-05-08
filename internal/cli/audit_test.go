package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditRedactsSensitiveFields(t *testing.T) {
	app := &App{ConfigPath: filepath.Join(t.TempDir(), "config.json")}
	plan, err := buildAPIPlan("create lead source", "POST", "/lead_sources", `{"name":"Test","api_key":"secret","nested":{"token":"abc"}}`, []string{"authorization=Bearer abc"}, nil, nil, "", "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.writeAudit(apiAuditRecord("plan", plan, "planned", nil, nil)); err != nil {
		t.Fatal(err)
	}

	records, _, err := app.readAudit(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	body, ok := records[0].Body.(map[string]any)
	if !ok {
		t.Fatalf("body type = %T, want map", records[0].Body)
	}
	if got := body["api_key"]; got != "[redacted]" {
		t.Fatalf("api_key = %v, want redacted", got)
	}
	nested, ok := body["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested type = %T, want map", body["nested"])
	}
	if got := nested["token"]; got != "[redacted]" {
		t.Fatalf("nested token = %v, want redacted", got)
	}
	if got := records[0].Query["authorization"]; got != "[redacted]" {
		t.Fatalf("authorization = %v, want redacted", got)
	}
}

func TestAuditCommandListsPlanEntries(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	cmd := newRootCommand("test", &out, &out)
	cmd.SetArgs([]string{
		"--config", filepath.Join(dir, "config.json"),
		"--json",
		"api", "create", "lead", "source",
		"--body", `{"name":"Audit Test"}`,
		"--plan",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	cmd = newRootCommand("test", &out, &out)
	cmd.SetArgs([]string{"--config", filepath.Join(dir, "config.json"), "--json", "audit", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	var payload struct {
		Count   int           `json:"count"`
		Records []auditRecord `json:"records"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Count != 1 {
		t.Fatalf("count = %d, want 1; output=%s", payload.Count, out.String())
	}
	record := payload.Records[0]
	if got, want := record.Status, "planned"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if got, want := record.Path, "/lead_sources"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	if !strings.Contains(record.Safety.WorstCase, "record") {
		t.Fatalf("worst case = %q, want record context", record.Safety.WorstCase)
	}
}
