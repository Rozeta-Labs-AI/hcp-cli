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
	if !strings.Contains(out.String(), "Housecall Pro Command Center") {
		t.Fatalf("banner missing brand:\n%s", out.String())
	}
}
