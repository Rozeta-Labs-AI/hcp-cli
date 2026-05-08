package cli

import (
	"strings"
	"testing"

	"github.com/Rozeta-Labs-AI/hcp-cli/internal/config"
)

func TestParseAIDecisionCommand(t *testing.T) {
	decision, err := parseAIDecision(`{"type":"command","command":"api list customers --limit 5 --json","explanation":"List customers"}`)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := decision.Type, "command"; got != want {
		t.Fatalf("type = %q, want %q", got, want)
	}
	if !strings.Contains(decision.Command, "api list customers") {
		t.Fatalf("command = %q", decision.Command)
	}
}

func TestParseAIDecisionAnswerFromMarkdownFence(t *testing.T) {
	decision, err := parseAIDecision("```json\n{\"type\":\"answer\",\"text\":\"Done\"}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := decision.Text, "Done"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}

func TestEmbeddedAIPromptRequiresPlanForWrites(t *testing.T) {
	prompt := embeddedAIPrompt("create a lead source")
	for _, want := range []string{"Mutating actions must use --plan", "Return exactly one JSON object", "Do not ask for or reveal API keys", "Do not restate request classifications", "Keep explanations concise and operational"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestConfiguredAIKeyUsesProviderEnvironment(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "router-key")
	got := configuredAIKey(config.AIConfig{Provider: "openrouter"})
	if got != "router-key" {
		t.Fatalf("key = %q, want env key", got)
	}
}
