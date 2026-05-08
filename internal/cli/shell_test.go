package cli

import (
	"bytes"
	"strings"
	"testing"
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
	app := &App{Version: "test", Out: &out, Err: &errOut, Quiet: true}

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
	app := &App{Version: "test", Out: &out, Err: &errOut, Quiet: true}

	if err := runShellLine(app, "ok now what"); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	for _, want := range []string{"command-driven HCP shell", "status", "api get /company --json"} {
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
	app := &App{Version: "test", Out: &out, Err: &errOut, Quiet: true}

	if err := runShellLine(app, "tell me a joke"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "not a built-in chatbot") {
		t.Fatalf("expected guidance output:\n%s", out.String())
	}
}

func TestShellActionableUnknownStillRoutesToAPI(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := &App{Version: "test", Out: &out, Err: &errOut, Quiet: true}

	if err := runShellLine(app, "list customers --limit 1 --json"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"path": "/customers"`) {
		t.Fatalf("expected api output:\n%s", out.String())
	}
}

func TestShellAIChatGPTPrintsCodexGuide(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := &App{Version: "test", Out: &out, Err: &errOut, Quiet: true}

	if err := runShellLine(app, "ai chatgpt"); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	for _, want := range []string{"ChatGPT subscription mode uses Codex CLI", "codex --login", "Sign in with ChatGPT", "Housecall Pro API"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestShellSlashAIChatGPTPrintsCodexGuide(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := &App{Version: "test", Out: &out, Err: &errOut, Quiet: true}

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
	app := &App{Version: "test", Out: &out, Err: &errOut, Quiet: true}

	if err := runShellLine(app, "ai providers"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"OpenRouter", "Anthropic", "ENG-285", "ENG-286"} {
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
