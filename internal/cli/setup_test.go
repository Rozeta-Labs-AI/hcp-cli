package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rozeta-Labs-AI/hcp-cli/internal/config"
)

func TestSetupModelPickerStoresChatGPTChoice(t *testing.T) {
	previousStatus := codexLoginStatusFunc
	previousAuth := runCodexDeviceAuthFunc
	defer func() {
		codexLoginStatusFunc = previousStatus
		runCodexDeviceAuthFunc = previousAuth
	}()
	codexLoginStatusFunc = func() (string, error) {
		return "Logged in using ChatGPT", nil
	}
	runCodexDeviceAuthFunc = func(app *App, in io.Reader) error {
		t.Fatal("device auth should not run when already logged in")
		return nil
	}
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
	if !strings.Contains(out.String(), "already connected through Codex") {
		t.Fatalf("expected connected Codex status:\n%s", out.String())
	}
}

func TestSetupModelStoresAPIProviderKey(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	var out bytes.Buffer
	cmd := newRootCommand("test", &out, &out)
	cmd.SetArgs([]string{"--config", configPath, "setup", "model", "--provider", "openrouter", "--model", "openrouter/auto", "--api-key", "router-secret"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.AI.Provider, "openrouter"; got != want {
		t.Fatalf("provider = %q, want %q", got, want)
	}
	if got, want := cfg.AI.APIKey, "router-secret"; got != want {
		t.Fatalf("api key = %q, want stored key", got)
	}
}

func TestSetupModelChatGPTRunsDeviceAuthWhenMissing(t *testing.T) {
	previousStatus := codexLoginStatusFunc
	previousAuth := runCodexDeviceAuthFunc
	defer func() {
		codexLoginStatusFunc = previousStatus
		runCodexDeviceAuthFunc = previousAuth
	}()
	statusCalls := 0
	codexLoginStatusFunc = func() (string, error) {
		statusCalls++
		if statusCalls == 1 {
			return "", errNotLoggedInForTest{}
		}
		return "Logged in using ChatGPT", nil
	}
	authRan := false
	runCodexDeviceAuthFunc = func(app *App, in io.Reader) error {
		authRan = true
		return nil
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	var out bytes.Buffer
	cmd := newRootCommand("test", &out, &out)
	cmd.SetIn(bytes.NewBufferString("1\n"))
	cmd.SetArgs([]string{"--config", configPath, "setup", "model"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !authRan {
		t.Fatal("expected HCP setup to run Codex device auth internally")
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.AI.Provider, "chatgpt"; got != want {
		t.Fatalf("provider = %q, want %q", got, want)
	}
	if !strings.Contains(out.String(), "Signing in to ChatGPT subscription mode") {
		t.Fatalf("expected HCP-owned login message:\n%s", out.String())
	}
}

type errNotLoggedInForTest struct{}

func (errNotLoggedInForTest) Error() string { return "not logged in" }

func TestSetupModelNoInputPrintsPickerWithoutBlocking(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	var out bytes.Buffer
	cmd := newRootCommand("test", &out, &out)
	cmd.SetArgs([]string{"--config", configPath, "--no-input", "setup", "model"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"AI Assistant Setup", "OpenRouter API key", "Anthropic API key", "ChatGPT subscription via Codex"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("expected setup picker to include %q:\n%s", want, out.String())
		}
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

func TestMaybePromptAISetupSkipsWhenHCPAuthMissing(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := &App{Version: "test", ConfigPath: configPath, Out: &out, Err: &errOut}

	maybePromptAISetup(app, bufio.NewReader(bytes.NewBufferString("1\n")))

	if strings.Contains(out.String(), "AI Assistant Setup") {
		t.Fatalf("AI setup should not prompt before HCP auth:\n%s", out.String())
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AI.Provider != "" || cfg.AI.Skipped {
		t.Fatalf("AI config changed before HCP auth: %#v", cfg.AI)
	}
}

func TestMaybePromptAISetupPromptsAfterHCPAuthConfigured(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Default()
	cfg.Auth.Mode = "api_key"
	cfg.Auth.APIKey = "test-key"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := &App{Version: "test", ConfigPath: configPath, Out: &out, Err: &errOut}

	maybePromptAISetup(app, bufio.NewReader(bytes.NewBufferString("1\n")))

	if !strings.Contains(out.String(), "AI Assistant Setup") {
		t.Fatalf("expected AI setup after HCP auth:\n%s", out.String())
	}
	updated, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := updated.AI.Provider, "chatgpt"; got != want {
		t.Fatalf("provider = %q, want %q", got, want)
	}
}
