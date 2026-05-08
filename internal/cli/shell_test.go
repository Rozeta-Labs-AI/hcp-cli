package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rozeta-Labs-AI/hcp-cli/internal/config"
)

func TestSplitShellLinePreservesQuotedJSON(t *testing.T) {
	args, err := splitShellLine(`api create lead source --body '{"name":"Spring Mailer"}' --plan`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"api", "create", "lead", "source", "--body", `{"name":"Spring Mailer"}`, "--plan"}
	if len(args) != len(want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg %d = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestShellRoutesUnknownMutatingLineToAPIPlan(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := &App{Version: "test", Out: &out, Err: &errOut, Quiet: true, ConfigPath: filepath.Join(t.TempDir(), "config.json")}

	if err := runShellLine(app, `create lead source --body '{"name":"Spring Mailer"}'`); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	for _, want := range []string{"POST /lead_sources", "mutable=true risk=mutating", "execute with --yes"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestShellBannerIncludesBrand(t *testing.T) {
	var out bytes.Buffer
	app := &App{Version: "test", Out: &out, Err: &out, Quiet: false, ConfigPath: "/tmp/missing-hcp-shell-test.json"}
	printShellBanner(app)
	for _, want := range []string{"HOUSE", "PRO", "Housecall Pro Command Center", "Tools", "Start"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("banner missing %q:\n%s", want, out.String())
		}
	}
}

func TestShellGuidancePhrasePrintsHelp(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := &App{Version: "test", Out: &out, Err: &errOut, Quiet: true, ConfigPath: filepath.Join(t.TempDir(), "config.json")}

	if err := runShellLine(app, "ok now what"); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	for _, want := range []string{"HCP command center", "status", "api get /company --json"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
	if errOut.Len() > 0 {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
}

func TestShellNonActionableUnknownPrintsHelp(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := &App{Version: "test", Out: &out, Err: &errOut, Quiet: true, ConfigPath: filepath.Join(t.TempDir(), "config.json")}

	if err := runShellLine(app, "tell me a joke"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "configure AI with `setup model`") {
		t.Fatalf("expected guidance output:\n%s", out.String())
	}
}

func TestShellActionableUnknownStillRoutesToAPI(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := &App{Version: "test", Out: &out, Err: &errOut, Quiet: true, ConfigPath: filepath.Join(t.TempDir(), "config.json")}

	if err := runShellLine(app, "list customers --limit 1 --json --plan"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"path": "/customers"`) {
		t.Fatalf("expected api output:\n%s", out.String())
	}
}

func TestShellAIChatGPTPrintsCodexGuide(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := &App{Version: "test", Out: &out, Err: &errOut, Quiet: true, ConfigPath: filepath.Join(t.TempDir(), "config.json")}

	if err := runShellLine(app, "ai chatgpt"); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	for _, want := range []string{"ChatGPT subscription mode uses Codex CLI", "hcp setup model", "ChatGPT subscription via Codex", "Housecall Pro API"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestShellSlashAIChatGPTPrintsCodexGuide(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := &App{Version: "test", Out: &out, Err: &errOut, Quiet: true, ConfigPath: filepath.Join(t.TempDir(), "config.json")}

	if err := runShellLine(app, "/ai chatgpt"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ChatGPT Plus/Pro -> Codex CLI -> hcp CLI") {
		t.Fatalf("expected slash ai guide:\n%s", out.String())
	}
}

func TestShellAIProvidersMentionsBacklogIssues(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := &App{Version: "test", Out: &out, Err: &errOut, Quiet: true, ConfigPath: filepath.Join(t.TempDir(), "config.json")}

	if err := runShellLine(app, "ai providers"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"OpenRouter", "Anthropic", "OpenAI", "local credentials"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestShellSetupModelRoutesToSetupCommand(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	configPath := t.TempDir() + "/config.json"
	app := &App{Version: "test", Out: &out, Err: &errOut, Quiet: true, ConfigPath: configPath, NoInput: true}

	if err := runShellLine(app, "setup model"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "AI Assistant Setup") {
		t.Fatalf("expected setup picker:\n%s", out.String())
	}
}

func TestShellStreamsInteractiveSetupModelCommand(t *testing.T) {
	if !shouldStreamShellCommand([]string{"setup", "model"}) {
		t.Fatal("expected setup model to stream output")
	}
	if !shouldStreamShellCommand([]string{"account", "auth"}) {
		t.Fatal("expected account auth to stream output")
	}
	if shouldStreamShellCommand([]string{"api", "list", "customers"}) {
		t.Fatal("expected non-interactive api command to stay buffered")
	}
}

func TestShellUsesConfiguredAIForConversationalLine(t *testing.T) {
	previous := callAI
	defer func() { callAI = previous }()
	callAI = func(ctx context.Context, cfg config.Config, userRequest string) (aiDecision, error) {
		if userRequest != "show my first 5 customers" {
			t.Fatalf("request = %q", userRequest)
		}
		return aiDecision{Type: "command", Command: "api list customers --limit 5 --json --plan", Explanation: "Listing customers."}, nil
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Default()
	cfg.AI.Provider = "chatgpt"
	cfg.AI.Model = "codex-chatgpt"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := &App{Version: "test", Out: &out, Err: &errOut, Quiet: false, ConfigPath: configPath}

	if err := runShellLine(app, "show my first 5 customers"); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	for _, want := range []string{"... Thinking...", "... Listing customers.", "... Proposed command: hcp api list customers", "... Running command...", `"path": "/customers"`, "... Done."} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestShellAIRetriesAfterBadCommand(t *testing.T) {
	previous := callAI
	defer func() { callAI = previous }()
	calls := 0
	callAI = func(ctx context.Context, cfg config.Config, userRequest string) (aiDecision, error) {
		calls++
		if calls == 1 {
			return aiDecision{Type: "command", Command: "api list customers --bad-flag", Explanation: "Trying the customer list."}, nil
		}
		if !strings.Contains(userRequest, "unknown flag") {
			t.Fatalf("retry request missing command error: %q", userRequest)
		}
		return aiDecision{Type: "answer", Text: "I hit an invalid flag and corrected course."}, nil
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Default()
	cfg.AI.Provider = "chatgpt"
	cfg.AI.Model = "codex-chatgpt"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := &App{Version: "test", Out: &out, Err: &errOut, Quiet: false, ConfigPath: configPath}

	if err := runShellLine(app, "show customers"); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	for _, want := range []string{"... Command failed:", "... Retrying once with the error context...", "I hit an invalid flag"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestShellAIStopsClearlyAfterRetryCommandFails(t *testing.T) {
	previous := callAI
	defer func() { callAI = previous }()
	callAI = func(ctx context.Context, cfg config.Config, userRequest string) (aiDecision, error) {
		return aiDecision{Type: "command", Command: "customers create --bad-flag", Explanation: "Planning the customer preview."}, nil
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Default()
	cfg.AI.Provider = "chatgpt"
	cfg.AI.Model = "codex-chatgpt"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := &App{Version: "test", Out: &out, Err: &errOut, Quiet: false, ConfigPath: configPath}

	if err := runShellLine(app, "create customer"); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	for _, want := range []string{"... Stopped: the corrected command also failed:", "... No write was executed unless you separately confirmed a command with --yes."} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestNormalizeAICommandAddsPlanToMutatingCommand(t *testing.T) {
	args := normalizeAICommandArgs([]string{"api", "create", "lead", "source", "--body", `{"name":"Test"}`})
	if !hasShellFlag(args, "--plan") {
		t.Fatalf("args = %#v, want --plan", args)
	}
}
