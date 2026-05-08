package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDoctorCommand(app *App) *cobra.Command {
	cmd := newAuthDoctorCommand(app)
	cmd.Use = "doctor"
	cmd.Short = "Validate hcp installation, local auth, and Housecall Pro API access"
	return cmd
}

func newOnboardingCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "onboarding",
		Short: "Show fresh-install setup steps for hcp",
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.JSON {
				return writeJSON(app.Out, map[string]any{
					"steps": []map[string]string{
						{"name": "install", "command": "go install github.com/Rozeta-Labs-AI/hcp-cli/cmd/hcp@latest"},
						{"name": "path", "command": `export PATH="$PATH:$(go env GOPATH)/bin"`},
						{"name": "auth", "command": "hcp account auth --api-key <your-housecall-pro-api-key>"},
						{"name": "verify", "command": "hcp doctor"},
						{"name": "model", "command": "hcp setup model"},
						{"name": "open_crm", "command": "hcp crm"},
						{"name": "chatgpt_subscription", "command": "codex --login"},
					},
				})
			}
			fmt.Fprintln(app.Out, "Fresh hcp setup")
			fmt.Fprintln(app.Out)
			fmt.Fprintln(app.Out, "1. Install hcp:")
			fmt.Fprintln(app.Out, "   go install github.com/Rozeta-Labs-AI/hcp-cli/cmd/hcp@latest")
			fmt.Fprintln(app.Out)
			fmt.Fprintln(app.Out, "   Plain English: Go is just the installer for this CLI. It gives you the global `hcp` command, similar to how npm installs Node-based CLIs.")
			fmt.Fprintln(app.Out)
			fmt.Fprintln(app.Out, "2. Make sure Go's bin directory is on PATH:")
			fmt.Fprintln(app.Out, `   export PATH="$PATH:$(go env GOPATH)/bin"`)
			fmt.Fprintln(app.Out)
			fmt.Fprintln(app.Out, "3. Connect your Housecall Pro account:")
			fmt.Fprintln(app.Out, "   hcp account auth --api-key <your-housecall-pro-api-key>")
			fmt.Fprintln(app.Out)
			fmt.Fprintln(app.Out, "4. Verify the install and API connection:")
			fmt.Fprintln(app.Out, "   hcp doctor")
			fmt.Fprintln(app.Out)
			fmt.Fprintln(app.Out, "5. Connect your model:")
			fmt.Fprintln(app.Out, "   hcp setup model")
			fmt.Fprintln(app.Out)
			fmt.Fprintln(app.Out, "6. Open the branded command center:")
			fmt.Fprintln(app.Out, "   hcp crm")
			fmt.Fprintln(app.Out)
			fmt.Fprintln(app.Out, "Then type natural-language requests directly at hcp>.")
			fmt.Fprintln(app.Out, "For ChatGPT subscription mode, run `codex --login` once before choosing ChatGPT/Codex in setup model.")
			fmt.Fprintln(app.Out)
			fmt.Fprintln(app.Out, "If the GitHub repo is made private again later, also run:")
			fmt.Fprintln(app.Out, "   gh auth login")
			fmt.Fprintln(app.Out, "   go env -w GOPRIVATE=github.com/Rozeta-Labs-AI/*")
			return nil
		},
	}
}
