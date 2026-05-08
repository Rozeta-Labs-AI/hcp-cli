package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Rozeta-Labs-AI/hcp-cli/internal/config"
	"github.com/Rozeta-Labs-AI/hcp-cli/internal/hcp"
	"github.com/spf13/cobra"
)

type App struct {
	Version    string
	ConfigPath string
	BaseURL    string
	JSON       bool
	Quiet      bool
	NoInput    bool
	Out        io.Writer
	Err        io.Writer
	Safety     *safetySession
	Pending    *pendingShellAction
	ShellRunner func(app *App, args []string) error
}

type pendingShellAction struct {
	Args []string
}

func Execute(version string) error {
	return NewRootCommand(version).Execute()
}

func NewRootCommand(version string) *cobra.Command {
	return newRootCommand(version, os.Stdout, os.Stderr)
}

func newRootCommand(version string, out io.Writer, errOut io.Writer) *cobra.Command {
	app := &App{
		Version: version,
		Out:     out,
		Err:     errOut,
	}

	root := &cobra.Command{
		Use:           "hcp",
		Short:         "Housecall Pro operator CLI",
		Long:          "hcp turns Housecall Pro data into local operating intelligence for home-service teams.",
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       version,
	}
	root.SetOut(out)
	root.SetErr(errOut)

	root.PersistentFlags().StringVar(&app.ConfigPath, "config", "", "config file path")
	root.PersistentFlags().StringVar(&app.BaseURL, "base-url", "", "override Housecall Pro API base URL")
	root.PersistentFlags().BoolVar(&app.JSON, "json", false, "write machine-readable JSON")
	root.PersistentFlags().BoolVar(&app.Quiet, "quiet", false, "suppress non-essential output")
	root.PersistentFlags().BoolVar(&app.NoInput, "no-input", false, "fail instead of prompting for input")

	root.AddCommand(newAuthCommand(app))
	root.AddCommand(newAccountCommand(app))
	root.AddCommand(newDoctorCommand(app))
	root.AddCommand(newOnboardingCommand(app))
	root.AddCommand(newAPICommand(app))
	root.AddCommand(newSyncCommand(app))
	root.AddCommand(newResourceCommand(app, "customers", "customer", "/customers"))
	root.AddCommand(newResourceCommand(app, "jobs", "job", "/jobs"))
	root.AddCommand(newResourceCommand(app, "estimates", "estimate", "/estimates"))
	root.AddCommand(newResourceCommand(app, "leads", "lead", "/leads"))
	root.AddCommand(newResourceCommand(app, "invoices", "invoice", "/invoices"))
	root.AddCommand(newCashCommand(app))
	root.AddCommand(newBriefCommand(app))
	root.AddCommand(newFunnelCommand(app))
	root.AddCommand(newTechCommand(app))
	root.AddCommand(newMarketingCommand(app))
	root.AddCommand(newMCPCommand(app))
	root.AddCommand(newReportCommand(app))
	root.AddCommand(newSetupCommand(app))
	root.AddCommand(newAICommand(app))
	root.AddCommand(newUpdateCommand(app))
	root.AddCommand(newAuditCommand(app))
	root.AddCommand(newSafetyCommand(app))
	root.AddCommand(newCRMCommand(app))
	root.AddCommand(newShellCommand(app))

	return root
}

func (a *App) loadConfig() (config.Config, string, error) {
	path := a.ConfigPath
	if strings.TrimSpace(path) == "" {
		resolved, err := config.DefaultPath()
		if err != nil {
			return config.Config{}, "", errorf(exitConfig, "resolve config path: %w", err)
		}
		path = resolved
	}

	cfg, err := config.Load(path)
	if err != nil {
		return config.Config{}, path, errorf(exitConfig, "%w", err)
	}
	if strings.TrimSpace(a.BaseURL) != "" {
		cfg.BaseURL = strings.TrimSpace(a.BaseURL)
	}

	return cfg, path, nil
}

func (a *App) newClient() (*hcp.Client, config.Config, string, error) {
	cfg, path, err := a.loadConfig()
	if err != nil {
		return nil, cfg, path, err
	}

	key := cfg.APIKey()
	if strings.TrimSpace(key) == "" {
		return nil, cfg, path, errorf(exitAuth, "missing Housecall Pro API key; run `hcp auth login --api-key <key>` or set %s", config.EnvAPIKey)
	}

	client, err := hcp.New(hcp.Options{
		BaseURL:   cfg.BaseURL,
		APIKey:    key,
		AuthMode:  cfg.AuthMode(),
		CompanyID: cfg.Defaults.CompanyID,
		UserAgent: fmt.Sprintf("hcp-cli/%s", a.Version),
	})
	if err != nil {
		return nil, cfg, path, errorf(exitConfig, "%w", err)
	}

	return client, cfg, path, nil
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func commandContext(cmd *cobra.Command) context.Context {
	if ctx := cmd.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}
