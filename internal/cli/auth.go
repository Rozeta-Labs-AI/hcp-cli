package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/Rozeta-Labs-AI/hcp-cli/internal/config"
	"github.com/spf13/cobra"
)

func newAuthCommand(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Housecall Pro authentication",
	}

	cmd.AddCommand(newAuthLoginCommand(app))
	cmd.AddCommand(newAuthStatusCommand(app))
	cmd.AddCommand(newAuthDoctorCommand(app))

	return cmd
}

func newAuthLoginCommand(app *App) *cobra.Command {
	var apiKey string
	var companyID string
	var locationIDs []string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store a Housecall Pro API key locally",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := app.loadConfig()
			if err != nil {
				return err
			}

			key := strings.TrimSpace(apiKey)
			if key == "" {
				key = strings.TrimSpace(os.Getenv(config.EnvAPIKey))
			}
			if key == "" {
				return errorf(exitAuth, "missing API key; pass --api-key or set %s", config.EnvAPIKey)
			}

			cfg.Auth.Mode = "api_key"
			cfg.Auth.APIKey = key
			if strings.TrimSpace(companyID) != "" {
				cfg.Defaults.CompanyID = strings.TrimSpace(companyID)
			}
			if len(locationIDs) > 0 {
				cfg.Defaults.LocationIDs = cleanStrings(locationIDs)
			}

			if err := config.Save(path, cfg); err != nil {
				return errorf(exitConfig, "%w", err)
			}

			if app.JSON {
				return writeJSON(app.Out, map[string]any{
					"config_path": path,
					"base_url":    cfg.BaseURL,
					"auth_mode":   cfg.Auth.Mode,
					"api_key":     config.Mask(cfg.Auth.APIKey),
				})
			}
			if !app.Quiet {
				fmt.Fprintf(app.Out, "Configured Housecall Pro API auth at %s\n", path)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&apiKey, "api-key", "", "Housecall Pro API key")
	cmd.Flags().StringVar(&companyID, "company-id", "", "default X-Company-Id for multi-company accounts")
	cmd.Flags().StringSliceVar(&locationIDs, "location-id", nil, "default Housecall Pro location ID; repeat or comma-separate")

	return cmd
}

func newAuthStatusCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show local auth configuration without revealing secrets",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := app.loadConfig()
			if err != nil {
				return err
			}

			status := map[string]any{
				"config_path":        path,
				"base_url":           cfg.BaseURL,
				"auth_mode":          cfg.AuthMode(),
				"api_key_configured": cfg.APIKey() != "",
				"stored_api_key":     config.Mask(cfg.Auth.APIKey),
				"env_api_key":        os.Getenv(config.EnvAPIKey) != "",
				"company_id":         cfg.Defaults.CompanyID,
				"location_ids":       cfg.Defaults.LocationIDs,
			}

			if app.JSON {
				return writeJSON(app.Out, status)
			}

			fmt.Fprintf(app.Out, "Config: %s\n", path)
			fmt.Fprintf(app.Out, "Base URL: %s\n", cfg.BaseURL)
			fmt.Fprintf(app.Out, "Auth mode: %s\n", cfg.AuthMode())
			fmt.Fprintf(app.Out, "API key configured: %t\n", cfg.APIKey() != "")
			if cfg.Defaults.CompanyID != "" {
				fmt.Fprintf(app.Out, "Company ID: %s\n", cfg.Defaults.CompanyID)
			}
			if len(cfg.Defaults.LocationIDs) > 0 {
				fmt.Fprintf(app.Out, "Location IDs: %s\n", strings.Join(cfg.Defaults.LocationIDs, ", "))
			}
			return nil
		},
	}
}

func newAuthDoctorCommand(app *App) *cobra.Command {
	var offline bool
	var endpoint string

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Validate auth configuration and optionally call the API",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := app.loadConfig()
			if err != nil {
				return err
			}

			checks := []map[string]any{
				{"name": "config_path", "ok": path != "", "value": path},
				{"name": "base_url", "ok": cfg.BaseURL != "", "value": cfg.BaseURL},
				{"name": "auth_mode", "ok": cfg.AuthMode() != "", "value": cfg.AuthMode()},
				{"name": "api_key", "ok": cfg.APIKey() != "", "value": config.Mask(cfg.APIKey())},
			}

			if cfg.APIKey() == "" {
				if app.JSON {
					_ = writeJSON(app.Out, map[string]any{"ok": false, "checks": checks})
				}
				return errorf(exitAuth, "missing Housecall Pro API key; run `hcp auth login --api-key <key>` or set %s", config.EnvAPIKey)
			}

			if offline {
				if app.JSON {
					return writeJSON(app.Out, map[string]any{"ok": true, "mode": "offline", "checks": checks})
				}
				if !app.Quiet {
					fmt.Fprintln(app.Out, "Config and auth look usable.")
				}
				return nil
			}

			client, _, _, err := app.newClient()
			if err != nil {
				return err
			}

			raw, err := client.GetRaw(commandContext(cmd), endpoint, url.Values{})
			if err != nil {
				return errorf(exitAPI, "%w", err)
			}

			if app.JSON {
				var body any
				if err := json.Unmarshal(raw, &body); err != nil {
					body = json.RawMessage(raw)
				}
				return writeJSON(app.Out, map[string]any{
					"ok":       true,
					"endpoint": endpoint,
					"checks":   checks,
					"response": body,
				})
			}

			if !app.Quiet {
				fmt.Fprintf(app.Out, "API check passed: GET %s\n", endpoint)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&offline, "offline", false, "only validate local configuration")
	cmd.Flags().StringVar(&endpoint, "endpoint", "/company", "low-risk endpoint to call")

	return cmd
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
