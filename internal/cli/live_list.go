package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Rozeta-Labs-AI/hcp-cli/internal/store"
	"github.com/spf13/cobra"
)

func newResourceCommand(app *App, group string, singular string, endpoint string) *cobra.Command {
	var dbPath string
	var limit int
	var uncontacted bool
	var unbooked bool
	cmd := &cobra.Command{
		Use:   group,
		Short: fmt.Sprintf("Work with Housecall Pro %s", group),
	}
	if group == "leads" {
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			if !uncontacted && !unbooked {
				return cmd.Help()
			}
			db, err := store.Open(commandContext(cmd), dbPath)
			if err != nil {
				return errorf(exitConfig, "%w", err)
			}
			defer db.Close()
			rows, err := db.List(commandContext(cmd), "leads", max(limit, 1000))
			if err != nil {
				return errorf(exitConfig, "%w", err)
			}
			payload := buildRowsReport(cmd.CommandPath(), "stale", rows, limit, 3, 15, 0, "30d", uncontacted, unbooked, time.Now())
			return writeReport(app, payload)
		}
		cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
		cmd.Flags().IntVar(&limit, "limit", 25, "maximum rows to print")
		cmd.Flags().BoolVar(&uncontacted, "uncontacted", false, "show uncontacted leads")
		cmd.Flags().BoolVar(&unbooked, "unbooked", false, "show unbooked leads")
	}

	cmd.AddCommand(newLiveListCommand(app, group, singular, endpoint))
	addReportSubcommands(app, cmd, group)
	return cmd
}

func newLiveListCommand(app *App, group string, singular string, endpoint string) *cobra.Command {
	var page int
	var pageSize int
	var limit int
	var dataSource string
	var dbPath string
	var locationIDs []string

	cmd := &cobra.Command{
		Use:   "list",
		Short: fmt.Sprintf("List %s from the local mirror or Housecall Pro API", group),
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit < 1 {
				return errorf(exitUsage, "--limit must be greater than 0")
			}

			switch strings.ToLower(strings.TrimSpace(dataSource)) {
			case "local":
				return runLocalList(cmd, app, group, dbPath, limit)
			case "live":
				return runLiveList(cmd, app, group, singular, endpoint, page, pageSize, limit, locationIDs)
			default:
				return errorf(exitUsage, "--data-source must be local or live")
			}
		},
	}

	cmd.Flags().StringVar(&dataSource, "data-source", "local", "data source: local or live")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path for local data")
	cmd.Flags().IntVar(&page, "page", 1, "API page number for live data")
	cmd.Flags().IntVar(&pageSize, "page-size", 25, "API page size for live data")
	cmd.Flags().IntVar(&limit, "limit", 25, "maximum rows to print")
	cmd.Flags().StringSliceVar(&locationIDs, "location-id", nil, "Housecall Pro location ID for live data; repeat or comma-separate")

	return cmd
}

func runLocalList(cmd *cobra.Command, app *App, group string, dbPath string, limit int) error {
	db, err := store.Open(commandContext(cmd), dbPath)
	if err != nil {
		return errorf(exitConfig, "%w", err)
	}
	defer db.Close()

	rows, err := db.List(commandContext(cmd), group, limit)
	if err != nil {
		return errorf(exitConfig, "%w", err)
	}

	if app.JSON {
		return writeJSON(app.Out, map[string]any{
			"data_source": "local",
			"resource":    group,
			"limit":       limit,
			"count":       len(rows),
			"rows":        rows,
		})
	}

	if len(rows) == 0 {
		fmt.Fprintf(app.Out, "No local %s found. Run `hcp sync --resource %s` first, or use `--data-source live`.\n", group, group)
		return nil
	}

	return writeRowsSummary(app.Out, rows, limit)
}

func runLiveList(cmd *cobra.Command, app *App, group string, singular string, endpoint string, page int, pageSize int, limit int, locationIDs []string) error {
	client, cfg, _, err := app.newClient()
	if err != nil {
		return err
	}

	if page < 1 {
		return errorf(exitUsage, "--page must be greater than 0")
	}
	if pageSize < 1 || pageSize > 200 {
		return errorf(exitUsage, "--page-size must be between 1 and 200")
	}

	query := url.Values{}
	query.Set("page", fmt.Sprintf("%d", page))
	query.Set("page_size", fmt.Sprintf("%d", pageSize))

	effectiveLocations := cleanStrings(locationIDs)
	if len(effectiveLocations) == 0 {
		effectiveLocations = cfg.Defaults.LocationIDs
	}
	for _, id := range effectiveLocations {
		query.Add("location_ids[]", id)
	}

	raw, err := client.GetRaw(commandContext(cmd), endpoint, query)
	if err != nil {
		return errorf(exitAPI, "%w", err)
	}

	if app.JSON {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("decode json response: %w", err)
		}
		return writeJSON(app.Out, value)
	}

	return writeCollectionSummary(app.Out, group, singular, raw, limit)
}

func writeCollectionSummary(w io.Writer, group string, singular string, raw json.RawMessage, limit int) error {
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode json response: %w", err)
	}

	items := findCollection(group, singular, payload)
	if len(items) == 0 {
		_, err := fmt.Fprintln(w, "No rows returned.")
		return err
	}

	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		row, _ := item.(map[string]any)
		rows = append(rows, row)
	}

	return writeRowsSummary(w, rows, limit)
}

func writeRowsSummary(w io.Writer, rows []map[string]any, limit int) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME/NUMBER\tSTATUS\tCUSTOMER\tUPDATED")

	for i, row := range rows {
		if i >= limit {
			break
		}
		fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\n",
			stringValue(row, "id", "uuid"),
			stringValue(row, "number", "name", "display_name", "description"),
			stringValue(row, "status", "work_status", "payment_status"),
			customerValue(row),
			stringValue(row, "updated_at", "created_at", "scheduled_start"),
		)
	}

	if err := tw.Flush(); err != nil {
		return err
	}

	if len(rows) > limit {
		fmt.Fprintf(w, "\nShowing %d of %d rows. Increase --limit to print more from this page.\n", limit, len(rows))
	}

	return nil
}

func findCollection(group string, singular string, payload any) []any {
	if arr, ok := payload.([]any); ok {
		return arr
	}

	obj, ok := payload.(map[string]any)
	if !ok {
		return nil
	}

	for _, key := range []string{group, singular, "data", "items", "results"} {
		if arr, ok := obj[key].([]any); ok {
			return arr
		}
	}

	keys := make([]string, 0, len(obj))
	for key := range obj {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if arr, ok := obj[key].([]any); ok {
			return arr
		}
	}

	return nil
}

func stringValue(row map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := row[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return typed
			}
		case float64:
			return fmt.Sprintf("%.0f", typed)
		case bool:
			return fmt.Sprintf("%t", typed)
		}
	}
	return "-"
}

func customerValue(row map[string]any) string {
	customer, ok := row["customer"].(map[string]any)
	if !ok {
		return stringValue(row, "customer_id", "customer_uuid")
	}

	name := strings.TrimSpace(strings.Join([]string{
		stringValue(customer, "first_name"),
		stringValue(customer, "last_name"),
	}, " "))
	name = strings.ReplaceAll(name, "- -", "-")
	name = strings.Trim(name, " -")
	if name != "" {
		return name
	}

	return stringValue(customer, "name", "company", "company_name", "id")
}
