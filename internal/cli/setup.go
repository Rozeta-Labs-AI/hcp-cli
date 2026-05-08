package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Rozeta-Labs-AI/hcp-cli/internal/config"
	"github.com/spf13/cobra"
)

type aiSetupChoice struct {
	Provider string
	Model    string
	Skipped  bool
}

func newSetupCommand(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure hcp CRM setup options",
	}
	cmd.AddCommand(newSetupModelCommand(app))
	return cmd
}

func newAICommand(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ai",
		Short: "Show AI assistant configuration",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show configured AI assistant mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := app.loadConfig()
			if err != nil {
				return err
			}
			if app.JSON {
				return writeJSON(app.Out, aiStatusPayload(cfg))
			}
			payload := aiStatusPayload(cfg)
			fmt.Fprintf(app.Out, "AI configured: %t\n", payload["configured"])
			if cfg.AI.Skipped {
				fmt.Fprintln(app.Out, "AI setup: skipped")
				fmt.Fprintln(app.Out, "Run `hcp setup model` when ready.")
				return nil
			}
			if cfg.AI.Provider == "" {
				fmt.Fprintln(app.Out, "AI provider: not configured")
				fmt.Fprintln(app.Out, "Run `hcp setup model`.")
				return nil
			}
			fmt.Fprintf(app.Out, "AI provider: %s\n", cfg.AI.Provider)
			fmt.Fprintf(app.Out, "AI model: %s\n", cfg.AI.Model)
			if cfg.AI.Provider == "chatgpt" {
				fmt.Fprintln(app.Out, "ChatGPT subscription path: hcp will call local Codex CLI; run `codex --login` first.")
			}
			if env := aiProviderEnvName(cfg.AI.Provider); env != "" {
				fmt.Fprintf(app.Out, "AI API key available: %t\n", configuredAIKey(cfg.AI) != "")
				fmt.Fprintf(app.Out, "Environment fallback: %s\n", env)
			}
			return nil
		},
	})
	return cmd
}

func newSetupModelCommand(app *App) *cobra.Command {
	var provider string
	var model string
	var apiKey string
	var skip bool

	cmd := &cobra.Command{
		Use:   "model",
		Short: "Choose the AI model path for hcp crm",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := app.loadConfig()
			if err != nil {
				return err
			}
			choice := aiSetupChoice{
				Provider: normalizeAIProvider(provider),
				Model:    strings.TrimSpace(model),
				Skipped:  skip,
			}
			var reader *bufio.Reader
			if choice.Provider == "" && !choice.Skipped {
				if app.NoInput {
					printAIModelPicker(app)
					fmt.Fprintln(app.Out, "Run again with --provider chatgpt, openrouter, anthropic, openai, ollama, or --skip.")
					return nil
				}
				reader = bufio.NewReader(cmd.InOrStdin())
				selected, err := promptAIModelChoice(app, reader)
				if err != nil {
					return err
				}
				choice = selected
			}
			if choice.Model == "" {
				choice.Model = defaultModelForProvider(choice.Provider)
			}
			cfg.AI.Provider = choice.Provider
			cfg.AI.Model = choice.Model
			cfg.AI.Skipped = choice.Skipped
			if strings.TrimSpace(apiKey) != "" {
				cfg.AI.APIKey = strings.TrimSpace(apiKey)
			}
			if choice.Provider == "chatgpt" {
				cfg.AI.APIKey = ""
			}
			if choice.Provider != "" && choice.Provider != "chatgpt" && choice.Provider != "ollama" && cfg.AI.APIKey == "" && !choice.Skipped && !app.NoInput {
				fmt.Fprintf(app.Out, "Paste %s API key, or press Enter to use environment variable later: ", strings.ToUpper(choice.Provider))
				if reader == nil {
					reader = bufio.NewReader(cmd.InOrStdin())
				}
				key, err := readerLine(reader)
				if err != nil {
					return err
				}
				cfg.AI.APIKey = strings.TrimSpace(key)
			}
			if err := config.Save(path, cfg); err != nil {
				return errorf(exitConfig, "%w", err)
			}
			printAISetupResult(app, cfg.AI)
			return nil
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "AI provider: chatgpt, openrouter, anthropic, openai, ollama")
	cmd.Flags().StringVar(&model, "model", "", "model name or provider-specific model slug")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for API-based providers; not used for ChatGPT subscription via Codex")
	cmd.Flags().BoolVar(&skip, "skip", false, "skip AI setup for now")
	return cmd
}

func hasAISetup(cfg config.Config) bool {
	return strings.TrimSpace(cfg.AI.Provider) != "" || cfg.AI.Skipped
}

func maybePromptAISetup(app *App, in *bufio.Reader) {
	if app.NoInput || app.Quiet {
		return
	}
	cfg, path, err := app.loadConfig()
	if err != nil || hasAISetup(cfg) {
		return
	}
	if strings.TrimSpace(cfg.APIKey()) == "" {
		return
	}
	fmt.Fprintln(app.Out, "No AI assistant mode configured.")
	printAIModelPicker(app)
	fmt.Fprint(app.Out, "Choose provider [1-6, Enter to skip]: ")
	choice, err := readAIModelChoice(in)
	if err != nil {
		fmt.Fprintf(app.Err, "AI setup skipped: %v\n", err)
		return
	}
	if choice.Provider == "" && !choice.Skipped {
		choice.Skipped = true
	}
	choice.Model = defaultModelForProvider(choice.Provider)
	cfg.AI.Provider = choice.Provider
	cfg.AI.Model = choice.Model
	cfg.AI.Skipped = choice.Skipped
	if err := config.Save(path, cfg); err != nil {
		fmt.Fprintf(app.Err, "AI setup save failed: %v\n", err)
		return
	}
	printAISetupResult(app, cfg.AI)
}

func printAIModelPicker(app *App) {
	fmt.Fprint(app.Out, setupModelLogo)
	fmt.Fprintln(app.Out, "AI Assistant Setup")
	fmt.Fprintln(app.Out)
	fmt.Fprintln(app.Out, "Choose how you want hcp crm to think.")
	fmt.Fprintln(app.Out, "HCP auth remains local; model credentials are stored only in your local config.")
	fmt.Fprintln(app.Out)
	fmt.Fprintln(app.Out, "  1. ChatGPT subscription via Codex")
	fmt.Fprintln(app.Out, "     Use your ChatGPT Plus/Pro login through Codex CLI. No OpenAI API key stored in hcp.")
	fmt.Fprintln(app.Out)
	fmt.Fprintln(app.Out, "  2. OpenRouter API key")
	fmt.Fprintln(app.Out, "     Model catalog path for Claude, OpenAI, Gemini, Llama, and other OpenRouter-hosted models.")
	fmt.Fprintln(app.Out, "     Default: openrouter/auto")
	fmt.Fprintln(app.Out)
	fmt.Fprintln(app.Out, "  3. Anthropic API key")
	fmt.Fprintln(app.Out, "     Direct Claude API path for teams standardizing on Anthropic.")
	fmt.Fprintln(app.Out, "     Default: claude-sonnet")
	fmt.Fprintln(app.Out)
	fmt.Fprintln(app.Out, "  4. OpenAI API key")
	fmt.Fprintln(app.Out, "     Direct OpenAI API path for API-billed usage.")
	fmt.Fprintln(app.Out, "     Default: gpt-4.1")
	fmt.Fprintln(app.Out)
	fmt.Fprintln(app.Out, "  5. Ollama local model")
	fmt.Fprintln(app.Out, "     Local model path for offline/dev workflows.")
	fmt.Fprintln(app.Out, "     Default: llama3.1")
	fmt.Fprintln(app.Out)
	fmt.Fprintln(app.Out, "  6. Skip for now")
	fmt.Fprintln(app.Out)
}

const setupModelLogo = `
██╗  ██╗ ██████╗██████╗     ███╗   ███╗ ██████╗ ██████╗ ███████╗██╗
██║  ██║██╔════╝██╔══██╗    ████╗ ████║██╔═══██╗██╔══██╗██╔════╝██║
███████║██║     ██████╔╝    ██╔████╔██║██║   ██║██║  ██║█████╗  ██║
██╔══██║██║     ██╔═══╝     ██║╚██╔╝██║██║   ██║██║  ██║██╔══╝  ██║
██║  ██║╚██████╗██║         ██║ ╚═╝ ██║╚██████╔╝██████╔╝███████╗███████╗
╚═╝  ╚═╝ ╚═════╝╚═╝         ╚═╝     ╚═╝ ╚═════╝ ╚═════╝ ╚══════╝╚══════╝
`

func promptAIModelChoice(app *App, reader *bufio.Reader) (aiSetupChoice, error) {
	printAIModelPicker(app)
	fmt.Fprint(app.Out, "Choose provider [1-6]: ")
	return readAIModelChoice(reader)
}

func readAIModelChoice(reader *bufio.Reader) (aiSetupChoice, error) {
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return aiSetupChoice{}, err
	}
	value := strings.TrimSpace(strings.ToLower(line))
	if value == "" {
		return aiSetupChoice{Skipped: true}, nil
	}
	if n, err := strconv.Atoi(value); err == nil {
		switch n {
		case 1:
			return aiSetupChoice{Provider: "chatgpt", Model: "codex-chatgpt"}, nil
		case 2:
			return aiSetupChoice{Provider: "openrouter", Model: "openrouter/auto"}, nil
		case 3:
			return aiSetupChoice{Provider: "anthropic", Model: "claude-sonnet"}, nil
		case 4:
			return aiSetupChoice{Provider: "openai", Model: "gpt-4.1"}, nil
		case 5:
			return aiSetupChoice{Provider: "ollama", Model: "llama3.1"}, nil
		case 6:
			return aiSetupChoice{Skipped: true}, nil
		}
	}
	provider := normalizeAIProvider(value)
	if provider == "" {
		return aiSetupChoice{}, fmt.Errorf("unknown provider %q", value)
	}
	return aiSetupChoice{Provider: provider, Model: defaultModelForProvider(provider)}, nil
}

func normalizeAIProvider(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "chatgpt", "codex":
		return "chatgpt"
	case "openrouter":
		return "openrouter"
	case "anthropic", "claude":
		return "anthropic"
	case "openai":
		return "openai"
	case "ollama", "local":
		return "ollama"
	default:
		return ""
	}
}

func defaultModelForProvider(provider string) string {
	switch provider {
	case "chatgpt":
		return "codex-chatgpt"
	case "openrouter":
		return "openrouter/auto"
	case "anthropic":
		return "claude-sonnet"
	case "openai":
		return "gpt-4.1"
	case "ollama":
		return "llama3.1"
	default:
		return ""
	}
}

func printAISetupResult(app *App, ai config.AIConfig) {
	if ai.Skipped {
		fmt.Fprintln(app.Out, "AI assistant setup skipped. Run `hcp setup model` when ready.")
		return
	}
	fmt.Fprintf(app.Out, "AI mode: %s\n", ai.Provider)
	if ai.Model != "" {
		fmt.Fprintf(app.Out, "Model: %s\n", ai.Model)
	}
	if ai.Provider == "chatgpt" {
		fmt.Fprintln(app.Out, "Next: run `codex --login` and choose Sign in with ChatGPT. hcp crm will call Codex inside this same shell.")
		return
	}
	if ai.Provider != "" && ai.APIKey == "" && ai.Provider != "ollama" {
		envName := aiProviderEnvName(ai.Provider)
		fmt.Fprintf(app.Out, "Next: set %s or rerun setup with --api-key.\n", envName)
	}
}

func aiStatusPayload(cfg config.Config) map[string]any {
	return map[string]any{
		"configured":        cfg.AI.Provider != "" && !cfg.AI.Skipped,
		"provider":          cfg.AI.Provider,
		"model":             cfg.AI.Model,
		"skipped":           cfg.AI.Skipped,
		"api_key":           config.Mask(cfg.AI.APIKey),
		"api_key_available": configuredAIKey(cfg.AI) != "",
	}
}

func aiProviderEnvName(provider string) string {
	switch provider {
	case "openrouter":
		return "OPENROUTER_API_KEY"
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "openai":
		return "OPENAI_API_KEY"
	default:
		return ""
	}
}

func readerLine(in io.Reader) (string, error) {
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return line, nil
}

func isTerminalReader(in io.Reader) bool {
	file, ok := in.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
