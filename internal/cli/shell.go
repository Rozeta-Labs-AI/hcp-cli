package cli

import (
	"bufio"
	"bytes"
	"context"
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
	fmt.Fprintln(app.Out, "HOUSECALL PRO")
	fmt.Fprint(app.Out, shellLogo)
	fmt.Fprintf(app.Out, "hcp-cli %s  |  Housecall Pro Command Center\n", app.Version)
	fmt.Fprintln(app.Out, strings.Repeat("-", 72))
	cfg, path, err := app.loadConfig()
	if err != nil {
		fmt.Fprintf(app.Out, "Config      unavailable (%v)\n", err)
	} else {
		fmt.Fprintf(app.Out, "Config      %s\n", path)
		fmt.Fprintf(app.Out, "Base URL    %s\n", cfg.BaseURL)
		if cfg.APIKey() != "" {
			fmt.Fprintln(app.Out, "HCP Auth    configured")
		} else {
			fmt.Fprintln(app.Out, "HCP Auth    missing; run auth login --api-key <key>")
		}
		if hasAISetup(cfg) {
			fmt.Fprintf(app.Out, "AI Mode     %s\n", shellAIModeLabel(cfg.AI.Provider, cfg.AI.Skipped))
		} else if cfg.APIKey() != "" {
			fmt.Fprintln(app.Out, "AI Mode     not configured; run setup model")
		} else {
			fmt.Fprintln(app.Out, "AI Mode     deferred until HCP auth is configured")
		}
	}
	fmt.Fprintln(app.Out, "Safety      plans first; writes require --yes; high-risk writes need tokens")
	fmt.Fprintln(app.Out)
	fmt.Fprintln(app.Out, "Tools       api catalog | api examples | sync | reports | audit | safety")
	fmt.Fprintln(app.Out, "Start       onboarding | auth login --api-key <key> | doctor | crm")
	fmt.Fprintln(app.Out, "Try         status | api list customers --limit 5 --json | setup model | exit")
	fmt.Fprintln(app.Out)
}

const shellLogo = `
██╗  ██╗ ██████╗ ██╗   ██╗███████╗███████╗ ██████╗ █████╗ ██╗     ██╗
██║  ██║██╔═══██╗██║   ██║██╔════╝██╔════╝██╔════╝██╔══██╗██║     ██║
███████║██║   ██║██║   ██║███████╗█████╗  ██║     ███████║██║     ██║
██╔══██║██║   ██║██║   ██║╚════██║██╔══╝  ██║     ██╔══██║██║     ██║
██║  ██║╚██████╔╝╚██████╔╝███████║███████╗╚██████╗██║  ██║███████╗███████╗
╚═╝  ╚═╝ ╚═════╝  ╚═════╝ ╚══════╝╚══════╝ ╚═════╝╚═╝  ╚═╝╚══════╝╚══════╝

██████╗ ██████╗  ██████╗
██╔══██╗██╔══██╗██╔═══██╗
██████╔╝██████╔╝██║   ██║
██╔═══╝ ██╔══██╗██║   ██║
██║     ██║  ██║╚██████╔╝
╚═╝     ╚═╝  ╚═╝ ╚═════╝

`

func shellAIModeLabel(provider string, skipped bool) string {
	if skipped {
		return "skipped"
	}
	switch provider {
	case "chatgpt":
		return "ChatGPT subscription via Codex"
	case "openrouter":
		return "OpenRouter API"
	case "anthropic":
		return "Anthropic API"
	case "openai":
		return "OpenAI API"
	default:
		return "not configured"
	}
}

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
	if isPendingConfirmation(line) {
		return runPendingShellAction(app)
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
	} else if len(args) >= 2 && args[0] == "safety" && args[1] == "status" {
		return newSafetyStatusCommand(app).Execute()
	} else if !isKnownShellCommand(args[0]) {
		if handled, err := runShellAI(app, line); handled || err != nil {
			return err
		}
		if !isActionableShellLine(line) {
			printShellHelp(app)
			return nil
		}
		if isCompoundMutatingText(line) {
			return errorf(exitUsage, "compound mutating request detected; split it into separate reviewed plans")
		}
		args = append([]string{"api"}, args...)
		if isLikelyMutatingShellLine(line) && !hasShellFlag(args, "--plan") && !hasShellFlag(args, "--dry-run") && !hasShellFlag(args, "--yes") {
			args = append(args, "--plan")
		}
	}
	return runShellArgsWithOverride(app, args)
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
	fmt.Fprintln(app.Out, "This is the HCP command center. Type commands directly, or configure AI with `setup model` for natural-language chat.")
	fmt.Fprintln(app.Out, "Mutating requests plan first unless you explicitly use --yes.")
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
		fmt.Fprintln(app.Out, "  ChatGPT subscription: hcp setup model connects the local Codex auth session for you.")
		fmt.Fprintln(app.Out, "  OpenRouter, Anthropic, OpenAI: hcp crm calls provider APIs directly with local credentials.")
		return
	}
	fmt.Fprintln(app.Out, "ChatGPT subscription mode uses Codex CLI, not an hcp API key.")
	fmt.Fprintln(app.Out)
	fmt.Fprintln(app.Out, "Architecture:")
	fmt.Fprintln(app.Out, "  ChatGPT Plus/Pro -> Codex CLI -> hcp CLI -> Housecall Pro API")
	fmt.Fprintln(app.Out)
	fmt.Fprintln(app.Out, "Setup:")
	fmt.Fprintln(app.Out, "  hcp setup model")
	fmt.Fprintln(app.Out, "  # choose ChatGPT subscription via Codex")
	fmt.Fprintln(app.Out)
	fmt.Fprintln(app.Out, "Then stay inside hcp crm and type natural-language requests at hcp>.")
	fmt.Fprintln(app.Out, "  Show my first 5 customers as JSON.")
	fmt.Fprintln(app.Out, "  Plan creating a lead source called Spring Mailer.")
	fmt.Fprintln(app.Out, "  Sync customers and leads, then summarize stale opportunities.")
	fmt.Fprintln(app.Out)
	fmt.Fprintln(app.Out, "All model-proposed HCP actions still run through hcp safety gates.")
}

func isKnownShellCommand(command string) bool {
	switch command {
	case "ai", "api", "audit", "auth", "brief", "cash", "completion", "customers", "doctor", "estimates", "funnel",
		"help", "invoices", "jobs", "leads", "marketing", "mcp", "onboarding", "report", "safety", "setup", "sync", "tech":
		return true
	default:
		return false
	}
}

func runShellAI(app *App, line string) (bool, error) {
	cfg, _, err := app.loadConfig()
	if err != nil {
		return false, err
	}
	if cfg.AI.Provider == "" || cfg.AI.Skipped {
		return false, nil
	}
	printAIStage(app, "Thinking...")
	decision, err := callAI(context.Background(), cfg, line)
	if err != nil {
		return true, err
	}
	if handled, err := runAIDecision(app, decision); handled || err != nil {
		if err == nil {
			return true, nil
		}
		printAIStage(app, fmt.Sprintf("Command failed: %v", err))
		printAIStage(app, "Retrying once with the error context...")
		retry, retryErr := callAI(context.Background(), cfg, aiRetryRequest(line, decision, err))
		if retryErr != nil {
			return true, fmt.Errorf("%v; retry failed: %w", err, retryErr)
		}
		_, retryErr = runAIDecision(app, retry)
		if retryErr != nil {
			printAIStage(app, fmt.Sprintf("Stopped: the corrected command also failed: %v", retryErr))
			printAIStage(app, "No write was executed unless you separately confirmed a command with --yes.")
			return true, nil
		}
		return true, retryErr
	}
	return true, nil
}

func printAIStage(app *App, message string) {
	if !app.Quiet {
		fmt.Fprintf(app.Out, "... %s\n", message)
	}
}

func runAIDecision(app *App, decision aiDecision) (bool, error) {
	switch decision.Type {
	case "answer":
		fmt.Fprintln(app.Out, decision.Text)
		return true, nil
	case "command":
		if decision.Explanation != "" {
			printAIStage(app, decision.Explanation)
		}
		args, err := splitShellLine(decision.Command)
		if err != nil {
			return true, err
		}
		args = normalizeAICommandArgs(args)
		if len(args) == 0 {
			return true, fmt.Errorf("AI returned an empty command")
		}
		printAIStage(app, "Proposed command: hcp "+strings.Join(args, " "))
		if storesPendingPlan(args) {
			app.Pending = &pendingShellAction{Args: append([]string(nil), args...)}
		}
		printAIStage(app, "Running command...")
		if err := runShellArgsWithOverride(app, args); err != nil {
			return true, err
		}
		if storesPendingPlan(args) {
			printAIStage(app, "Preview only. Say \"yes\" or \"run it\" to execute this exact plan.")
		}
		printAIStage(app, "Done.")
		return true, nil
	default:
		return true, fmt.Errorf("unsupported AI decision %q", decision.Type)
	}
}

func isPendingConfirmation(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	switch lower {
	case "yes", "y", "run it", "execute it", "do it", "create it", "create that customer", "execute that", "run that", "yes execute", "yes run it":
		return true
	default:
		return false
	}
}

func runPendingShellAction(app *App) error {
	if app.Pending == nil || len(app.Pending.Args) == 0 {
		return errorf(exitUsage, "no pending plan to execute")
	}
	args := makeExecutableArgs(app.Pending.Args)
	app.Pending = nil
	printAIStage(app, "Executing the previous plan.")
	printAIStage(app, "Running command: hcp "+strings.Join(args, " "))
	if err := runShellArgsWithOverride(app, args); err != nil {
		printAIStage(app, fmt.Sprintf("Execution failed: %v", err))
		return nil
	}
	printAIStage(app, "Execution complete.")
	return nil
}

func storesPendingPlan(args []string) bool {
	return hasShellFlag(args, "--plan") && isLikelyMutatingShellLine(strings.Join(args, " "))
}

func makeExecutableArgs(args []string) []string {
	out := make([]string, 0, len(args)+1)
	hasYes := false
	for _, arg := range args {
		if arg == "--plan" || strings.HasPrefix(arg, "--plan=") {
			continue
		}
		if arg == "--yes" || strings.HasPrefix(arg, "--yes=") {
			hasYes = true
		}
		out = append(out, arg)
	}
	if !hasYes {
		out = append(out, "--yes")
	}
	return out
}

func aiRetryRequest(original string, previous aiDecision, err error) string {
	return fmt.Sprintf(`The previous hcp command failed. Propose one corrected hcp CLI command or explain why it cannot be done.

Original user request:
%s

Previous proposed command:
%s

Command error:
%v`, original, previous.Command, err)
}

func normalizeAICommandArgs(args []string) []string {
	if len(args) > 0 && args[0] == "hcp" {
		args = args[1:]
	}
	if len(args) == 0 {
		return args
	}
	if isLikelyMutatingShellLine(strings.Join(args, " ")) && !hasShellFlag(args, "--plan") && !hasShellFlag(args, "--dry-run") && !hasShellFlag(args, "--yes") {
		args = append(args, "--plan")
	}
	return args
}

func runShellArgs(app *App, args []string) error {
	if err := recordShellSafetyAttempt(app, args); err != nil {
		return err
	}
	var out bytes.Buffer
	childOut := io.Writer(&out)
	if shouldStreamShellCommand(args) {
		childOut = app.Out
	}
	child := newRootCommand(app.Version, childOut, app.Err)
	if strings.TrimSpace(app.ConfigPath) != "" {
		args = append([]string{"--config", app.ConfigPath}, args...)
	}
	if strings.TrimSpace(app.BaseURL) != "" {
		args = append([]string{"--base-url", app.BaseURL}, args...)
	}
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

func runShellArgsWithOverride(app *App, args []string) error {
	if app.ShellRunner != nil {
		return app.ShellRunner(app, args)
	}
	return runShellArgs(app, args)
}

func shouldStreamShellCommand(args []string) bool {
	if len(args) >= 2 && args[0] == "setup" && args[1] == "model" {
		return true
	}
	if len(args) >= 2 && args[0] == "account" && args[1] == "auth" {
		return true
	}
	if len(args) >= 2 && args[0] == "auth" && args[1] == "login" {
		return true
	}
	return false
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
