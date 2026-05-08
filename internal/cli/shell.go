package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

func newShellCommand(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell",
		Short: "Open the branded interactive Housecall Pro command center",
		Long:  "Open an interactive shell for running hcp commands with a branded operator-console prompt.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShell(app, cmd.InOrStdin())
		},
	}
	return cmd
}

func runShell(app *App, in io.Reader) error {
	if !app.Quiet {
		printShellBanner(app)
	}
	scanner := bufio.NewScanner(in)
	for {
		if !app.Quiet {
			fmt.Fprint(app.Out, "hcp> ")
		}
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if shouldExitShell(line) {
			fmt.Fprintln(app.Out, "bye")
			return nil
		}
		if line == "clear" {
			fmt.Fprint(app.Out, "\033[2J\033[H")
			printShellBanner(app)
			continue
		}
		if err := runShellLine(app, line); err != nil {
			fmt.Fprintln(app.Err, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func printShellBanner(app *App) {
	fmt.Fprint(app.Out, shellLogo)
	cfg, path, err := app.loadConfig()
	if err != nil {
		fmt.Fprintf(app.Out, "Config: unavailable (%v)\n", err)
	} else {
		fmt.Fprintf(app.Out, "Config: %s\n", path)
		fmt.Fprintf(app.Out, "Base URL: %s\n", cfg.BaseURL)
		if cfg.APIKey() != "" {
			fmt.Fprintln(app.Out, "Auth: configured")
		} else {
			fmt.Fprintln(app.Out, "Auth: missing; run auth login --api-key <key>")
		}
	}
	fmt.Fprintln(app.Out, "Mode: safe by default. Mutating API actions require --plan or --yes.")
	fmt.Fprintln(app.Out, "Try: status | api list customers --limit 5 --json | sync --resource customers --json | exit")
	fmt.Fprintln(app.Out)
}

const shellLogo = `
██╗  ██╗ ██████╗██████╗
██║  ██║██╔════╝██╔══██╗
███████║██║     ██████╔╝
██╔══██║██║     ██╔═══╝
██║  ██║╚██████╗██║
╚═╝  ╚═╝ ╚═════╝╚═╝

Housecall Pro Command Center
`

func shouldExitShell(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	return lower == "exit" || lower == "quit" || lower == ":q"
}

func runShellLine(app *App, line string) error {
	args, err := splitShellLine(line)
	if err != nil {
		return errorf(exitUsage, "%w", err)
	}
	if len(args) == 0 {
		return nil
	}
	if args[0] == "hcp" {
		args = args[1:]
	}
	if len(args) == 0 {
		return nil
	}
	if shouldShowShellHelp(line, args) {
		printShellHelp(app)
		return nil
	}
	if args[0] == "status" {
		args = []string{"auth", "doctor", "--endpoint", "/company"}
	} else if !isKnownShellCommand(args[0]) {
		if !isActionableShellLine(line) {
			printShellHelp(app)
			return nil
		}
		args = append([]string{"api"}, args...)
		if isLikelyMutatingShellLine(line) && !hasShellFlag(args, "--plan") && !hasShellFlag(args, "--dry-run") && !hasShellFlag(args, "--yes") {
			args = append(args, "--plan")
		}
	}
	var out bytes.Buffer
	child := newRootCommand(app.Version, &out, app.Err)
	child.SetArgs(args)
	if err := child.Execute(); err != nil {
		return err
	}
	if out.Len() > 0 {
		_, _ = app.Out.Write(out.Bytes())
		if !bytes.HasSuffix(out.Bytes(), []byte("\n")) {
			fmt.Fprintln(app.Out)
		}
	}
	return nil
}

func shouldShowShellHelp(line string, args []string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	if lower == "?" || lower == "help" || lower == "what can you do" || lower == "what do you do" || lower == "ok now what" || lower == "now what" {
		return true
	}
	if len(args) >= 2 && args[0] == "help" && args[1] == "shell" {
		return true
	}
	for _, phrase := range []string{"how does this work", "what should i do", "show me what", "getting started", "where do i start"} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

func printShellHelp(app *App) {
	fmt.Fprintln(app.Out, "This is a command-driven HCP shell, not a built-in chatbot.")
	fmt.Fprintln(app.Out, "Type commands at the hcp> prompt. Mutating requests plan first unless you explicitly use --yes.")
	fmt.Fprintln(app.Out)
	fmt.Fprintln(app.Out, "Safe next commands:")
	fmt.Fprintln(app.Out, "  status")
	fmt.Fprintln(app.Out, "  api get /company --json")
	fmt.Fprintln(app.Out, "  api list customers --limit 5 --json")
	fmt.Fprintln(app.Out, "  sync --resource customers --resource leads --page-size 10 --max-pages 1 --json")
	fmt.Fprintln(app.Out, "  customers list --limit 5 --json")
	fmt.Fprintln(app.Out, "  create lead source --body '{\"name\":\"Test Lead Source\"}'")
	fmt.Fprintln(app.Out, "  exit")
}

func isKnownShellCommand(command string) bool {
	switch command {
	case "api", "auth", "brief", "cash", "completion", "customers", "estimates", "funnel",
		"help", "invoices", "jobs", "leads", "marketing", "mcp", "report", "sync", "tech":
		return true
	default:
		return false
	}
}

func isActionableShellLine(line string) bool {
	lower := strings.ToLower(line)
	if strings.Contains(lower, "/") || strings.Contains(lower, "--") {
		return true
	}
	actionWords := []string{"list", "show", "get", "find", "search", "fetch", "create", "add", "update", "change", "delete", "remove", "enable", "disable", "dispatch", "convert", "approve", "decline"}
	resourceWords := []string{"application", "checklist", "company", "customer", "address", "employee", "job", "appointment", "invoice", "estimate", "event", "tag", "lead", "source", "type", "material", "price", "service", "route", "pipeline", "webhook", "schedule"}
	hasAction := false
	for _, word := range actionWords {
		if strings.Contains(lower, word) {
			hasAction = true
			break
		}
	}
	if !hasAction {
		return false
	}
	for _, word := range resourceWords {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}

func isLikelyMutatingShellLine(line string) bool {
	lower := strings.ToLower(line)
	for _, word := range []string{"create", "add", "update", "change", "delete", "remove", "enable", "disable", "dispatch", "convert", "approve", "decline"} {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}

func hasShellFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag || strings.HasPrefix(arg, flag+"=") {
			return true
		}
	}
	return false
}

func splitShellLine(line string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune
	escaped := false
	for _, r := range line {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ' ' || r == '\t' {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	if escaped {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args, nil
}
