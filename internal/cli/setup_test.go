package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rozeta-Labs-AI/hcp-cli/internal/config"
)

func TestSetupModelPickerStoresChatGPTChoice(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	var out bytes.Buffer
	cmd := newRootCommand("test", &out, &out)
	cmd.SetIn(bytes.NewBufferString("1\n"))
	cmd.SetArgs([]string{"--config", configPath, "setup", "model"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.AI.Provider, "chatgpt"; got != want {
		t.Fatalf("provider = %q, want %q", got, want)
	}
	if cfg.AI.APIKey != "" {
		t.Fatal("chatgpt setup should not store an api key")
	}
	if !strings.Contains(out.String(), "codex --login") {
		t.Fatalf("expected Codex guidance:\n%s", out.String())
	}
}

func TestSetupModelNoInputPrintsPickerWithoutBlocking(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	var out bytes.Buffer
	cmd := newRootCommand("test", &out, &out)
	cmd.SetArgs([]string{"--config", configPath, "--no-input", "setup", "model"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "AI Assistant Setup") {
		t.Fatalf("expected setup picker:\n%s", out.String())
	}
}

func TestAIStatusReportsConfiguredMode(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Default()
	cfg.AI.Provider = "chatgpt"
	cfg.AI.Model = "codex-chatgpt"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	cmd := newRootCommand("test", &out, &out)
	cmd.SetArgs([]string{"--config", configPath, "--json", "ai", "status"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if got, want := payload["provider"], "chatgpt"; got != want {
		t.Fatalf("provider = %v, want %s", got, want)
	}
	if got, want := payload["configured"], true; got != want {
		t.Fatalf("configured = %v, want %t", got, want)
	}
}
