package cli

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

const updateInstallTarget = "github.com/Rozeta-Labs-AI/hcp-cli/cmd/hcp@latest"

var runSelfUpdate = func(app *App) error {
	cmd := exec.Command("go", "install", updateInstallTarget)
	cmd.Stdout = app.Out
	cmd.Stderr = app.Err
	return cmd.Run()
}

func newUpdateCommand(app *App) *cobra.Command {
	var plan bool
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update hcp to the latest public release",
		RunE: func(cmd *cobra.Command, args []string) error {
			if plan || dryRun {
				fmt.Fprintln(app.Out, "Update plan:")
				fmt.Fprintf(app.Out, "  go install %s\n", updateInstallTarget)
				fmt.Fprintln(app.Out)
				fmt.Fprintln(app.Out, "This updates the hcp binary in Go's bin directory without changing your local hcp config.")
				return nil
			}
			fmt.Fprintln(app.Out, "Updating hcp...")
			if err := runSelfUpdate(app); err != nil {
				return fmt.Errorf("hcp update failed: %w", err)
			}
			fmt.Fprintln(app.Out, "hcp update complete. Open a new terminal if your shell cached the old binary.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&plan, "plan", false, "show the update command without running it")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show the update command without running it")
	return cmd
}
