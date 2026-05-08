package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/Rozeta-Labs-AI/hcp-cli/internal/store"
	"github.com/spf13/cobra"
)

type reportRow struct {
	ID       string         `json:"id"`
	Name     string         `json:"name,omitempty"`
	Status   string         `json:"status,omitempty"`
	Customer string         `json:"customer,omitempty"`
	Amount   int64          `json:"amount,omitempty"`
	Reason   string         `json:"reason,omitempty"`
	Action   string         `json:"recommended_action,omitempty"`
	Raw      map[string]any `json:"raw,omitempty"`
}

type reportPayload struct {
	Name       string           `json:"name"`
	Period     string           `json:"period,omitempty"`
	Freshness  string           `json:"data_freshness"`
	Counts     map[string]int   `json:"counts"`
	Totals     map[string]int64 `json:"totals,omitempty"`
	Rows       []reportRow      `json:"rows,omitempty"`
	Deliveries []deliveryPlan   `json:"deliveries,omitempty"`
}

type deliveryPlan struct {
	Channel string `json:"channel"`
	DryRun  bool   `json:"dry_run"`
	Target  string `json:"target,omitempty"`
	Status  string `json:"status"`
}

func addReportSubcommands(app *App, cmd *cobra.Command, group string) {
	switch group {
	case "jobs":
		for _, spec := range []struct{ use, short string }{
			{"today", "Jobs scheduled today"},
			{"tomorrow", "Jobs scheduled tomorrow"},
			{"active", "Active jobs"},
			{"unscheduled", "Unscheduled jobs"},
			{"delayed", "Delayed jobs"},
			{"stalled", "Stalled jobs"},
			{"completed-not-invoiced", "Completed jobs without invoices"},
		} {
			cmd.AddCommand(newRowsReportCommand(app, spec.use, spec.short, "jobs"))
		}
	case "estimates":
		cmd.AddCommand(newRowsReportCommand(app, "open", "Open estimates", "estimates"))
		cmd.AddCommand(newRowsReportCommand(app, "unsold", "Unsold estimates", "estimates"))
		cmd.AddCommand(newRowsReportCommand(app, "stale", "Stale estimates", "estimates"))
		cmd.AddCommand(newRowsReportCommand(app, "high-value", "High-value estimates", "estimates"))
		cmd.AddCommand(newRowsReportCommand(app, "no-followup", "Estimates with no recent follow-up", "estimates"))
	case "leads":
		cmd.AddCommand(newRowsReportCommand(app, "stale", "Stale leads", "leads"))
	case "invoices":
		cmd.AddCommand(newRowsReportCommand(app, "open", "Open invoices", "invoices"))
		cmd.AddCommand(newRowsReportCommand(app, "overdue", "Overdue invoices", "invoices"))
		cmd.AddCommand(newRowsReportCommand(app, "aging", "Invoice aging", "invoices"))
	}
}

func newRowsReportCommand(app *App, use string, short string, resource string) *cobra.Command {
	var dbPath string
	var limit int
	var days int
	var minutes int
	var minAmount int
	var last string
	var uncontacted bool
	var unbooked bool
	var yesterday bool

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := store.Open(commandContext(cmd), dbPath)
			if err != nil {
				return errorf(exitConfig, "%w", err)
			}
			defer db.Close()

			rows, err := db.List(commandContext(cmd), resource, max(limit, 1000))
			if err != nil {
				return errorf(exitConfig, "%w", err)
			}
			if yesterday {
				last = "yesterday"
			}
			payload := buildRowsReport(cmd.CommandPath(), use, rows, limit, days, minutes, minAmount, last, uncontacted, unbooked, time.Now())
			return writeReport(app, payload)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	cmd.Flags().IntVar(&limit, "limit", 25, "maximum rows to print")
	cmd.Flags().IntVar(&days, "days", 3, "stale threshold in days")
	cmd.Flags().IntVar(&minutes, "minutes", 15, "stale threshold in minutes")
	cmd.Flags().IntVar(&minAmount, "min", 5000, "minimum amount for high-value reports")
	cmd.Flags().StringVar(&last, "last", "30d", "lookback window")
	cmd.Flags().BoolVar(&uncontacted, "uncontacted", false, "show uncontacted leads")
	cmd.Flags().BoolVar(&unbooked, "unbooked", false, "show unbooked leads")
	cmd.Flags().BoolVar(&yesterday, "yesterday", false, "use yesterday as the report period")
	return cmd
}

func buildRowsReport(name string, mode string, rows []map[string]any, limit int, days int, minutes int, minAmount int, last string, uncontacted bool, unbooked bool, now time.Time) reportPayload {
	out := reportPayload{Name: name, Freshness: now.UTC().Format(time.RFC3339), Counts: map[string]int{}, Totals: map[string]int64{}}
	for _, row := range rows {
		if !matchesMode(mode, row, days, minutes, minAmount, uncontacted, unbooked, now) {
			continue
		}
		report := rowForReport(mode, row, now)
		out.Rows = append(out.Rows, report)
		out.Totals["amount"] += report.Amount
		if limit > 0 && len(out.Rows) >= limit {
			break
		}
	}
	out.Counts["rows"] = len(out.Rows)
	if strings.TrimSpace(last) != "" {
		out.Period = last
	}
	return out
}

func matchesMode(mode string, row map[string]any, days int, minutes int, minAmount int, uncontacted bool, unbooked bool, now time.Time) bool {
	status := strings.ToLower(stringValue(row, "status", "work_status", "payment_status"))
	if uncontacted && stringValue(row, "contacted_at", "last_contacted_at") != "-" {
		return false
	}
	if unbooked && (stringValue(row, "job_id", "appointment_id", "scheduled_start") != "-" || containsAny(status, "booked", "scheduled")) {
		return false
	}
	switch mode {
	case "today":
		return sameDay(parseAnyTime(row, "scheduled_start", "start_time", "start"), now)
	case "tomorrow":
		return sameDay(parseAnyTime(row, "scheduled_start", "start_time", "start"), now.AddDate(0, 0, 1))
	case "active":
		return !containsAny(status, "complete", "completed", "canceled", "deleted")
	case "unscheduled":
		return status == "unscheduled" || stringValue(row, "scheduled_start", "start_time") == "-"
	case "delayed", "stalled":
		t := parseAnyTime(row, "scheduled_end", "scheduled_start", "updated_at")
		return !t.IsZero() && t.Before(now.Add(-24*time.Hour)) && !containsAny(status, "complete", "completed", "canceled", "deleted")
	case "completed-not-invoiced":
		return containsAny(status, "complete", "completed") && stringValue(row, "invoice_number") == "-"
	case "open":
		return !containsAny(status, "sold", "approved", "complete", "completed", "paid", "canceled", "declined")
	case "unsold":
		return !containsAny(status, "sold", "approved", "canceled", "declined")
	case "stale", "no-followup":
		threshold := time.Duration(days) * 24 * time.Hour
		if minutes > 0 && strings.Contains(strings.ToLower(status), "lead") {
			threshold = time.Duration(minutes) * time.Minute
		}
		t := parseAnyTime(row, "updated_at", "created_at")
		return !t.IsZero() && t.Before(now.Add(-threshold))
	case "high-value":
		return amountValue(row) >= int64(minAmount)
	case "overdue":
		due := parseAnyTime(row, "due_at")
		return !due.IsZero() && due.Before(now) && !containsAny(status, "paid", "void", "canceled")
	case "aging":
		return !containsAny(status, "paid", "void", "canceled")
	default:
		return true
	}
}

func rowForReport(mode string, row map[string]any, now time.Time) reportRow {
	reason := mode
	action := "Review in Housecall Pro"
	switch mode {
	case "stale":
		action = "Contact the lead or customer"
	case "no-followup":
		action = "Schedule estimate follow-up"
	case "completed-not-invoiced":
		action = "Create or collect invoice"
	case "overdue", "aging":
		action = "Follow up on payment"
	}
	return reportRow{
		ID:       stringValue(row, "id", "uuid", "number"),
		Name:     stringValue(row, "number", "name", "display_name", "description", "invoice_number"),
		Status:   stringValue(row, "status", "work_status", "payment_status"),
		Customer: customerValue(row),
		Amount:   amountValue(row),
		Reason:   reason,
		Action:   action,
		Raw:      row,
	}
}

func newBriefCommand(app *App) *cobra.Command {
	cmd := &cobra.Command{Use: "brief", Short: "Owner brief reports"}
	for _, period := range []string{"today", "yesterday", "week", "month"} {
		cmd.AddCommand(newBriefPeriodCommand(app, period))
	}
	return cmd
}

func newBriefPeriodCommand(app *App, period string) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   period,
		Short: fmt.Sprintf("Build the %s owner brief from local data", period),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := store.Open(commandContext(cmd), dbPath)
			if err != nil {
				return errorf(exitConfig, "%w", err)
			}
			defer db.Close()
			payload := reportPayload{Name: cmd.CommandPath(), Period: period, Freshness: time.Now().UTC().Format(time.RFC3339), Counts: map[string]int{}, Totals: map[string]int64{}}
			for _, resource := range []string{"leads", "appointments", "estimates", "jobs", "invoices"} {
				rows, _ := db.List(commandContext(cmd), resource, 1000)
				payload.Counts[resource] = len(rows)
				for _, row := range rows {
					payload.Totals[resource+"_amount"] += amountValue(row)
				}
			}
			return writeReport(app, payload)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}

func newFunnelCommand(app *App) *cobra.Command {
	var dbPath string
	var last string
	cmd := &cobra.Command{
		Use:   "funnel",
		Short: "Lead-to-cash funnel reports",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := store.Open(commandContext(cmd), dbPath)
			if err != nil {
				return errorf(exitConfig, "%w", err)
			}
			defer db.Close()
			payload := reportPayload{Name: cmd.CommandPath(), Period: last, Freshness: time.Now().UTC().Format(time.RFC3339), Counts: map[string]int{}, Totals: map[string]int64{}}
			for _, resource := range []string{"leads", "appointments", "estimates", "jobs", "invoices"} {
				rows, _ := db.List(commandContext(cmd), resource, 1000)
				payload.Counts[resource] = len(rows)
			}
			return writeReport(app, payload)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	cmd.Flags().StringVar(&last, "last", "30d", "lookback window")
	cmd.AddCommand(newRowsReportCommand(app, "leakage", "Find revenue leakage across the funnel", "leads"))
	cmd.AddCommand(newRowsReportCommand(app, "compare", "Compare funnel periods", "leads"))
	return cmd
}

func newCashCommand(app *App) *cobra.Command {
	cmd := &cobra.Command{Use: "cash", Short: "Cash collection reports"}
	cmd.AddCommand(newRowsReportCommand(app, "collected", "Collected cash", "invoices"))
	return cmd
}

func newReportCommand(app *App) *cobra.Command {
	var send string
	var dryRun bool
	var target string
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Render and deliver reports",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "owner-daily",
		Short: "Render owner daily report delivery payload",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := reportPayload{Name: cmd.CommandPath(), Period: "today", Freshness: time.Now().UTC().Format(time.RFC3339), Counts: map[string]int{}, Deliveries: []deliveryPlan{}}
			for _, channel := range splitCSV(send) {
				status := "dry_run_rendered"
				if !dryRun {
					status = "blocked_missing_provider"
				}
				payload.Deliveries = append(payload.Deliveries, deliveryPlan{Channel: channel, DryRun: dryRun, Target: target, Status: status})
			}
			return writeReport(app, payload)
		},
	})
	cmd.PersistentFlags().StringVar(&send, "send", "text", "delivery channel: text, slack, email")
	cmd.PersistentFlags().BoolVar(&dryRun, "dry-run", true, "render delivery payload without sending")
	cmd.PersistentFlags().StringVar(&target, "target", "", "delivery target")
	return cmd
}

func writeReport(app *App, payload reportPayload) error {
	if app.JSON {
		return writeJSON(app.Out, payload)
	}
	fmt.Fprintf(app.Out, "%s\n", payload.Name)
	fmt.Fprintf(app.Out, "Data freshness: %s\n", payload.Freshness)
	for key, value := range payload.Counts {
		fmt.Fprintf(app.Out, "%s: %d\n", key, value)
	}
	for key, value := range payload.Totals {
		if value != 0 {
			fmt.Fprintf(app.Out, "%s: %d\n", key, value)
		}
	}
	if len(payload.Rows) > 0 {
		rows := make([]map[string]any, 0, len(payload.Rows))
		for _, row := range payload.Rows {
			rows = append(rows, map[string]any{"id": row.ID, "number": row.Name, "status": row.Status, "customer_id": row.Customer, "updated_at": row.Reason})
		}
		return writeRowsSummary(app.Out, rows, len(rows))
	}
	for _, delivery := range payload.Deliveries {
		fmt.Fprintf(app.Out, "%s: %s\n", delivery.Channel, delivery.Status)
	}
	return nil
}

func parseAnyTime(row map[string]any, keys ...string) time.Time {
	for _, key := range keys {
		value := stringValue(row, key)
		if value == "-" {
			continue
		}
		for _, layout := range []string{time.RFC3339, "2006-01-02"} {
			if parsed, err := time.Parse(layout, value); err == nil {
				return parsed
			}
		}
	}
	return time.Time{}
}

func sameDay(a time.Time, b time.Time) bool {
	return !a.IsZero() && a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func amountValue(row map[string]any) int64 {
	for _, key := range []string{"total_amount", "amount", "amount_due", "due_amount"} {
		switch value := row[key].(type) {
		case int64:
			return value
		case int:
			return int64(value)
		case float64:
			return int64(value)
		}
	}
	return 0
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
