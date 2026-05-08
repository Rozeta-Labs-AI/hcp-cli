package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/Rozeta-Labs-AI/hcp-cli/internal/store"
	"github.com/Rozeta-Labs-AI/hcp-cli/internal/syncer"
	"github.com/spf13/cobra"
)

func newSyncCommand(app *App) *cobra.Command {
	var since string
	var watch bool
	var watchInterval time.Duration
	var watchMaxRuns int
	var dryRun bool
	var dbPath string
	var resources []string
	var pageSize int
	var maxPages int
	var locationIDs []string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync Housecall Pro data into the local mirror",
		RunE: func(cmd *cobra.Command, args []string) error {
			sinceTime, err := parseSince(since, time.Now())
			if err != nil {
				return errorf(exitUsage, "%w", err)
			}
			if dryRun {
				_, cfg, path, err := app.newClient()
				if err != nil {
					return err
				}
				if app.JSON {
					return writeJSON(app.Out, map[string]any{
						"ok":          true,
						"mode":        "dry_run",
						"config_path": path,
						"base_url":    cfg.BaseURL,
						"since":       since,
						"watch":       watch,
						"resources":   resources,
						"page_size":   pageSize,
						"max_pages":   maxPages,
						"db_path":     dbPath,
					})
				}
				if !app.Quiet {
					fmt.Fprintf(app.Out, "Sync dry run passed using config %s\n", path)
				}
				return nil
			}

			client, cfg, _, err := app.newClient()
			if err != nil {
				return err
			}

			effectiveLocations := cleanStrings(locationIDs)
			if len(effectiveLocations) == 0 {
				effectiveLocations = cfg.Defaults.LocationIDs
			}

			db, err := store.Open(commandContext(cmd), dbPath)
			if err != nil {
				return errorf(exitConfig, "%w", err)
			}
			defer db.Close()

			runOnce := func() (syncer.Summary, error) {
				return syncer.Run(commandContext(cmd), client, db, syncer.Options{
					Resources:   cleanStrings(resources),
					PageSize:    pageSize,
					MaxPages:    maxPages,
					LocationIDs: effectiveLocations,
					Since:       sinceTime,
				})
			}

			if watch {
				runs := 0
				for {
					summary, err := runOnce()
					if err != nil {
						return errorf(exitAPI, "%w", err)
					}
					runs++
					if app.JSON {
						if err := writeJSON(app.Out, map[string]any{"watch": true, "run": runs, "summary": summary}); err != nil {
							return err
						}
					} else if !app.Quiet {
						fmt.Fprintf(app.Out, "Watch sync run %d completed into %s\n", runs, summary.DBPath)
					}
					if watchMaxRuns > 0 && runs >= watchMaxRuns {
						return nil
					}
					select {
					case <-commandContext(cmd).Done():
						return commandContext(cmd).Err()
					case <-time.After(watchInterval):
					}
				}
			}

			summary, err := runOnce()
			if err != nil {
				return errorf(exitAPI, "%w", err)
			}

			if app.JSON {
				return writeJSON(app.Out, summary)
			}

			if !app.Quiet {
				fmt.Fprintf(app.Out, "Synced Housecall Pro data into %s\n", summary.DBPath)
				for _, resource := range summary.Resources {
					fmt.Fprintf(app.Out, "- %s: %d rows across %d page(s)\n", resource.Resource, resource.Rows, resource.Pages)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "only sync records changed since this time")
	cmd.Flags().BoolVar(&watch, "watch", false, "keep syncing on an interval")
	cmd.Flags().DurationVar(&watchInterval, "watch-interval", 5*time.Minute, "interval for --watch polling mode")
	cmd.Flags().IntVar(&watchMaxRuns, "watch-max-runs", 0, "maximum watch sync runs before exit; 0 means run until interrupted")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate sync inputs without writing data")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	cmd.Flags().StringSliceVar(&resources, "resource", nil, "resource to sync; see `hcp api catalog` and sync docs for supported local mirror resources")
	cmd.Flags().IntVar(&pageSize, "page-size", 100, "API page size")
	cmd.Flags().IntVar(&maxPages, "max-pages", 10, "maximum pages per resource")
	cmd.Flags().StringSliceVar(&locationIDs, "location-id", nil, "Housecall Pro location ID; repeat or comma-separate")

	return cmd
}

func parseSince(value string, now time.Time) (time.Time, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return time.Time{}, nil
	}
	switch value {
	case "yesterday":
		return now.AddDate(0, 0, -1), nil
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), nil
	}
	if strings.HasSuffix(value, "d") {
		days, err := parsePositiveInt(strings.TrimSuffix(value, "d"))
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid --since %q", value)
		}
		return now.AddDate(0, 0, -days), nil
	}
	if strings.HasSuffix(value, "h") {
		hours, err := parsePositiveInt(strings.TrimSuffix(value, "h"))
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid --since %q", value)
		}
		return now.Add(-time.Duration(hours) * time.Hour), nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("invalid --since %q; use yesterday, today, 24h, 7d, or RFC3339", value)
}

func parsePositiveInt(value string) (int, error) {
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil || parsed <= 0 {
		return 0, fmt.Errorf("invalid positive integer")
	}
	return parsed, nil
}

func newTechCommand(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tech",
		Short: "Technician performance reports",
	}
	cmd.AddCommand(newNotImplementedCommand("scorecard", "Show technician scorecards"))
	return cmd
}

func newMarketingCommand(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "marketing",
		Short: "Marketing source performance reports",
	}
	cmd.AddCommand(newNotImplementedCommand("sources", "Show lead source performance"))
	return cmd
}

func newMCPCommand(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run the MCP server",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "serve",
		Short: "Serve hcp read-only tools over MCP",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := map[string]any{
				"protocol": "mcp",
				"name":     "hcp",
				"tools": []map[string]any{
					{"name": "brief", "read_only": true},
					{"name": "funnel", "read_only": true},
					{"name": "leads_stale", "read_only": true},
					{"name": "estimates_unsold", "read_only": true},
					{"name": "jobs_stalled", "read_only": true},
					{"name": "invoices_open", "read_only": true},
				},
				"mutating_actions": "excluded",
			}
			if app.JSON {
				return writeJSON(app.Out, payload)
			}
			fmt.Fprintln(app.Out, "hcp MCP server shell")
			fmt.Fprintln(app.Out, "Read-only tools: brief, funnel, leads_stale, estimates_unsold, jobs_stalled, invoices_open")
			return nil
		},
	})
	return cmd
}

func newNotImplementedCommand(use string, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return errorf(exitUsage, "%s is not implemented yet", cmd.CommandPath())
		},
	}
}
