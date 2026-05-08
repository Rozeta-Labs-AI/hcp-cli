package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/Rozeta-Labs-AI/hcp-cli/internal/store"
)

func TestRootHelpRegistersCommandGroups(t *testing.T) {
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	help := out.String()
	for _, want := range []string{
		"account",
		"auth",
		"doctor",
		"onboarding",
		"sync",
		"customers",
		"jobs",
		"estimates",
		"leads",
		"invoices",
		"brief",
		"funnel",
		"tech",
		"marketing",
		"mcp",
		"ai",
		"safety",
		"setup",
		"update",
		"crm",
		"shell",
	} {
		if !bytes.Contains([]byte(help), []byte(want)) {
			t.Fatalf("help output does not include %q:\n%s", want, help)
		}
	}
}

func TestTopLevelDoctorOfflineRequiresAPIKey(t *testing.T) {
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--config", t.TempDir() + "/missing.json", "doctor", "--offline"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected missing API key error")
	}
	if got, want := ExitCode(err), exitAuth; got != want {
		t.Fatalf("exit code = %d, want %d", got, want)
	}
}

func TestOnboardingPrintsFreshInstallPath(t *testing.T) {
	var out bytes.Buffer
	cmd := newRootCommand("test", &out, &out)
	cmd.SetArgs([]string{"onboarding"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"go install", "Go is just the installer", "hcp account auth", "hcp api catalog --json", "Codex", "hcp update"} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Fatalf("onboarding output missing %q:\n%s", want, out.String())
		}
	}
}

func TestUpdatePlanPrintsInstallerCommand(t *testing.T) {
	var out bytes.Buffer
	cmd := newRootCommand("test", &out, &out)
	cmd.SetArgs([]string{"update", "--plan"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Update plan", "go install github.com/Rozeta-Labs-AI/hcp-cli/cmd/hcp@latest", "without changing your local hcp config"} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Fatalf("update plan output missing %q:\n%s", want, out.String())
		}
	}
}

func TestAccountAuthStoresAPIKey(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	var out bytes.Buffer
	cmd := newRootCommand("test", &out, &out)
	cmd.SetArgs([]string{"--config", configPath, "--json", "account", "auth", "--api-key", "hcp-test-key"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if got, want := payload["api_key"], "****-key"; got != want {
		t.Fatalf("masked api key = %v, want %s", got, want)
	}
}

func TestCRMCommandOpensBrandedShell(t *testing.T) {
	var out bytes.Buffer
	cmd := newRootCommand("test", &out, &out)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(bytes.NewBufferString("exit\n"))
	cmd.SetArgs([]string{"crm"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("Housecall Pro Command Center")) {
		t.Fatalf("crm output missing branded shell:\n%s", out.String())
	}
}

func TestAuthDoctorOfflineRequiresAPIKey(t *testing.T) {
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--config", t.TempDir() + "/missing.json", "auth", "doctor", "--offline"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected missing API key error")
	}
	if got, want := ExitCode(err), exitAuth; got != want {
		t.Fatalf("exit code = %d, want %d", got, want)
	}
}

func TestCustomersListReadsFromLocalStore(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "hcp.sqlite")
	db, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Upsert(ctx, "customers", map[string]any{
		"id":         "cus_123",
		"first_name": "Ada",
		"last_name":  "Lovelace",
		"email":      "ada@example.com",
		"updated_at": "2026-05-02T10:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	cmd := newRootCommand("test", &out, &out)
	cmd.SetArgs([]string{"customers", "list", "--db", dbPath, "--json", "--limit", "1"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	var payload struct {
		DataSource string           `json:"data_source"`
		Resource   string           `json:"resource"`
		Count      int              `json:"count"`
		Rows       []map[string]any `json:"rows"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if payload.DataSource != "local" {
		t.Fatalf("data_source = %q, want local", payload.DataSource)
	}
	if payload.Resource != "customers" {
		t.Fatalf("resource = %q, want customers", payload.Resource)
	}
	if payload.Count != 1 {
		t.Fatalf("count = %d, want 1", payload.Count)
	}
	if got, want := payload.Rows[0]["id"], "cus_123"; got != want {
		t.Fatalf("row id = %v, want %s", got, want)
	}
}
