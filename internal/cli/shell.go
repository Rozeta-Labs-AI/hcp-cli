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
	return newInteractiveCommand(app, "shell", "Open the branded interactive Housecall Pro command center", "Open an interactive shell for running hcp commands with a branded operator-console prompt. Use `hcp crm` as the friendlier user-facing command.")
}

func newCRMCommand(app *App) *cobra.Command {
	return newInteractiveCommand(app, "crm", "Open the branded Housecall Pro CRM command center", "Open the branded Housecall Pro CRM command center for running hcp commands with a persistent prompt.")
}

func newInteractiveCommand(app *App, use string, short string, long string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
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
	reader := bufio.NewReader(in)
	if isTerminalReader(in) {
		maybePromptAISetup(app, reader)
	}
	for {
		if !app.Quiet {
			fmt.Fprint(app.Out, "hcp> ")
		}
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}
		if err == io.EOF && line == "" {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			if err == io.EOF {
				break
			}
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
		if err == io.EOF {
			break
		}
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
	fmt.Fprintln(app.Out, "Try: setup model | status | api list customers --limit 5 --json | sync --resource customers --json | exit")
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
	if isShellAICommand(args) {
		printShellAIGuide(app, args)
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
	fmt.Fprintln(app.Out, "  ai chatgpt")
	fmt.Fprintln(app.Out, "  exit")
}

func isShellAICommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	first := strings.TrimPrefix(strings.ToLower(args[0]), "/")
	if first != "ai" {
		return false
	}
	if len(args) == 1 {
		return true
	}
	second := strings.ToLower(args[1])
	return second == "chatgpt" || second == "codex" || second == "openai" || second == "providers"
}

func printShellAIGuide(app *App, args []string) {
	mode := "chatgpt"
	if len(args) > 1 {
		mode = strings.ToLower(args[1])
	}
	if mode == "providers" || mode == "status" {
		fmt.Fprintln(app.Out, "AI modes:")
		fmt.Fprintln(app.Out, "  ChatGPT subscription: use Codex CLI as the AI layer, then have Codex run hcp.")
		fmt.Fprintln(app.Out, "  API providers: OpenRouter, Anthropic, and OpenAI API support is tracked for embedded shell chat.")
		fmt.Fprintln(app.Out, "  Linear: ENG-285 covers provider config; ENG-286 covers embedded AI chat.")
		return
	}
	fmt.Fprintln(app.Out, "ChatGPT subscription mode uses Codex CLI, not an hcp API key.")
	fmt.Fprintln(app.Out)
	fmt.Fprintln(app.Out, "Architecture:")
	fmt.Fprintln(app.Out, "  ChatGPT Plus/Pro -> Codex CLI -> hcp CLI -> Housecall Pro API")
	fmt.Fprintln(app.Out)
	fmt.Fprintln(app.Out, "Setup:")
	fmt.Fprintln(app.Out, "  npm install -g @openai/codex")
	fmt.Fprintln(app.Out, "  codex --login")
	fmt.Fprintln(app.Out, "  # choose Sign in with ChatGPT")
	fmt.Fprintln(app.Out)
	fmt.Fprintln(app.Out, "Then open Codex in any terminal where hcp is installed and ask:")
	fmt.Fprintln(app.Out, "  Use the hcp CLI to verify auth and list my first 5 Housecall Pro customers as JSON. Do not modify anything.")
	fmt.Fprintln(app.Out, "  Use hcp to plan creating a lead source called Spring Mailer. Show the plan only; do not execute it.")
	fmt.Fprintln(app.Out, "  Use hcp to sync customers and leads, then summarize stale opportunities. Do not modify anything.")
	fmt.Fprintln(app.Out)
	fmt.Fprintln(app.Out, "For OpenRouter, Anthropic, or OpenAI API keys, embedded shell chat is planned separately.")
}

func isKnownShellCommand(command string) bool {
	switch command {
	case "ai", "api", "auth", "brief", "cash", "completion", "customers", "estimates", "funnel",
		"help", "invoices", "jobs", "leads", "marketing", "mcp", "report", "setup", "sync", "tech":
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
