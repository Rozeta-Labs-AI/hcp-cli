package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const EnvDBPath = "HCP_DB"

type Store struct {
	db   *sql.DB
	path string
}

type UpsertResult struct {
	Inserted bool `json:"inserted"`
	Updated  bool `json:"updated"`
}

func DefaultPath() (string, error) {
	if configured := strings.TrimSpace(os.Getenv(EnvDBPath)); configured != "" {
		return configured, nil
	}

	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache dir: %w", err)
	}

	return filepath.Join(dir, "hcp", "hcp.sqlite"), nil
}

func Open(ctx context.Context, path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		resolved, err := DefaultPath()
		if err != nil {
			return nil, err
		}
		path = resolved
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create database dir: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	db.SetMaxOpenConns(1)

	store := &Store{db: db, path: path}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	statements := []string{
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA journal_mode = WAL`,
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS sync_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			started_at TEXT NOT NULL,
			finished_at TEXT,
			resource TEXT NOT NULL,
			status TEXT NOT NULL,
			page_count INTEGER NOT NULL DEFAULT 0,
			row_count INTEGER NOT NULL DEFAULT 0,
			error TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS customers (
			id TEXT PRIMARY KEY,
			first_name TEXT,
			last_name TEXT,
			email TEXT,
			mobile_number TEXT,
			home_number TEXT,
			work_number TEXT,
			company TEXT,
			company_name TEXT,
			lead_source TEXT,
			created_at TEXT,
			updated_at TEXT,
			raw_json TEXT NOT NULL,
			synced_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS leads (
			id TEXT PRIMARY KEY,
			number TEXT,
			status TEXT,
			pipeline_status TEXT,
			lead_source TEXT,
			customer_id TEXT,
			customer_name TEXT,
			assigned_employee_id TEXT,
			assigned_employee_name TEXT,
			lost_at TEXT,
			created_at TEXT,
			updated_at TEXT,
			raw_json TEXT NOT NULL,
			synced_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			number TEXT,
			description TEXT,
			work_status TEXT,
			customer_id TEXT,
			customer_name TEXT,
			invoice_number TEXT,
			scheduled_start TEXT,
			scheduled_end TEXT,
			completed_at TEXT,
			canceled_at TEXT,
			deleted_at TEXT,
			created_at TEXT,
			updated_at TEXT,
			raw_json TEXT NOT NULL,
			synced_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS estimates (
			id TEXT PRIMARY KEY,
			number TEXT,
			status TEXT,
			customer_id TEXT,
			customer_name TEXT,
			total_amount INTEGER,
			created_at TEXT,
			updated_at TEXT,
			raw_json TEXT NOT NULL,
			synced_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS invoices (
			id TEXT PRIMARY KEY,
			uuid TEXT,
			invoice_number TEXT,
			status TEXT,
			customer_id TEXT,
			customer_name TEXT,
			total_amount INTEGER,
			amount_due INTEGER,
			due_at TEXT,
			paid_at TEXT,
			created_at TEXT,
			updated_at TEXT,
			raw_json TEXT NOT NULL,
			synced_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS employees (
			id TEXT PRIMARY KEY,
			first_name TEXT,
			last_name TEXT,
			email TEXT,
			mobile_number TEXT,
			role TEXT,
			created_at TEXT,
			updated_at TEXT,
			raw_json TEXT NOT NULL,
			synced_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS appointments (
			id TEXT PRIMARY KEY,
			job_id TEXT,
			job_number TEXT,
			start_time TEXT,
			end_time TEXT,
			arrival_window TEXT,
			assigned_employee_ids TEXT,
			created_at TEXT,
			updated_at TEXT,
			raw_json TEXT NOT NULL,
			synced_at TEXT NOT NULL
		)`,
		genericTableSQL("companies"),
		genericTableSQL("checklists"),
		genericTableSQL("customer_addresses"),
		genericTableSQL("lead_line_items"),
		genericTableSQL("job_line_items"),
		genericTableSQL("job_invoices"),
		genericTableSQL("job_input_materials"),
		genericTableSQL("job_tags"),
		genericTableSQL("job_attachments"),
		genericTableSQL("job_notes"),
		genericTableSQL("estimate_options"),
		genericTableSQL("estimate_option_line_items"),
		genericTableSQL("estimate_option_notes"),
		genericTableSQL("estimate_option_attachments"),
		genericTableSQL("invoice_items"),
		genericTableSQL("invoice_taxes"),
		genericTableSQL("invoice_discounts"),
		genericTableSQL("invoice_payments"),
		genericTableSQL("lead_sources"),
		genericTableSQL("tags"),
		genericTableSQL("events"),
		genericTableSQL("routes"),
		genericTableSQL("service_zones"),
		genericTableSQL("pipeline_statuses"),
		genericTableSQL("price_book_materials"),
		genericTableSQL("price_book_material_categories"),
		genericTableSQL("price_book_price_forms"),
		genericTableSQL("price_book_services"),
		genericTableSQL("activity_log"),
		genericTableSQL("daily_metrics"),
		`CREATE TABLE IF NOT EXISTS raw_api_payloads (
			id TEXT PRIMARY KEY,
			resource TEXT NOT NULL,
			api_path TEXT,
			parent_id TEXT,
			raw_json TEXT NOT NULL,
			synced_at TEXT NOT NULL
		)`,
	}

	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("run migration: %w", err)
		}
	}

	return nil
}

func (s *Store) BeginSyncRun(ctx context.Context, resource string) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO sync_runs (started_at, resource, status)
		VALUES (?, ?, ?)
	`, time.Now().UTC().Format(time.RFC3339), resource, "running")
	if err != nil {
		return 0, fmt.Errorf("begin sync run: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read sync run id: %w", err)
	}
	return id, nil
}

func (s *Store) FinishSyncRun(ctx context.Context, id int64, status string, pageCount int, rowCount int, syncErr error) error {
	var errText any
	if syncErr != nil {
		errText = syncErr.Error()
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE sync_runs
		SET finished_at = ?, status = ?, page_count = ?, row_count = ?, error = ?
		WHERE id = ?
	`, time.Now().UTC().Format(time.RFC3339), status, pageCount, rowCount, errText, id)
	if err != nil {
		return fmt.Errorf("finish sync run: %w", err)
	}
	return nil
}

func genericTableSQL(name string) string {
	return `CREATE TABLE IF NOT EXISTS ` + name + ` (
		id TEXT PRIMARY KEY,
		name TEXT,
		status TEXT,
		created_at TEXT,
		updated_at TEXT,
		raw_json TEXT NOT NULL,
		synced_at TEXT NOT NULL
	)`
}

func (s *Store) Upsert(ctx context.Context, resource string, item map[string]any) (UpsertResult, error) {
	if idValue(item) == "" {
		return UpsertResult{}, fmt.Errorf("%s item missing id, uuid, or number", resource)
	}

	raw, err := json.Marshal(item)
	if err != nil {
		return UpsertResult{}, fmt.Errorf("encode raw json: %w", err)
	}
	inserted, err := s.isInsert(ctx, resource, idValue(item))
	if err != nil {
		return UpsertResult{}, err
	}

	switch resource {
	case "customers":
		err = s.upsertCustomer(ctx, item, string(raw))
	case "leads":
		err = s.upsertLead(ctx, item, string(raw))
	case "jobs":
		err = s.upsertJob(ctx, item, string(raw))
	case "estimates":
		err = s.upsertEstimate(ctx, item, string(raw))
	case "invoices":
		err = s.upsertInvoice(ctx, item, string(raw))
	case "employees":
		err = s.upsertEmployee(ctx, item, string(raw))
	case "appointments":
		err = s.upsertAppointment(ctx, item, string(raw))
	case "companies", "checklists", "customer_addresses", "lead_line_items",
		"job_line_items", "job_invoices", "job_input_materials", "job_tags", "job_attachments", "job_notes",
		"estimate_options", "estimate_option_line_items", "estimate_option_notes", "estimate_option_attachments",
		"invoice_items", "invoice_taxes", "invoice_discounts", "invoice_payments",
		"lead_sources", "tags", "events", "routes", "service_zones", "pipeline_statuses",
		"price_book_materials", "price_book_material_categories", "price_book_price_forms", "price_book_services",
		"activity_log", "daily_metrics", "raw_api_payloads":
		err = s.upsertGeneric(ctx, resource, item, string(raw))
	default:
		err = fmt.Errorf("unsupported resource %q", resource)
	}
	if err != nil {
		return UpsertResult{}, err
	}
	return UpsertResult{Inserted: inserted, Updated: !inserted}, nil
}

func (s *Store) Count(ctx context.Context, table string) (int, error) {
	if !isSafeTable(table) {
		return 0, fmt.Errorf("unsupported table %q", table)
	}

	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
		return 0, fmt.Errorf("count %s: %w", table, err)
	}
	return count, nil
}

func (s *Store) List(ctx context.Context, resource string, limit int) ([]map[string]any, error) {
	if limit < 1 {
		return nil, fmt.Errorf("limit must be greater than 0")
	}
	if !isSafeTable(resource) || resource == "sync_runs" {
		return nil, fmt.Errorf("unsupported resource %q", resource)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT raw_json
		FROM `+resource+`
		ORDER BY COALESCE(NULLIF(updated_at, ''), NULLIF(created_at, ''), synced_at) DESC, id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", resource, err)
	}
	defer rows.Close()

	items := make([]map[string]any, 0, limit)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan %s row: %w", resource, err)
		}

		var item map[string]any
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode %s row json: %w", resource, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read %s rows: %w", resource, err)
	}

	return items, nil
}

func isSafeTable(table string) bool {
	switch table {
	case "customers", "leads", "jobs", "estimates", "invoices", "sync_runs":
		return true
	case "employees", "appointments", "companies", "checklists", "customer_addresses", "lead_line_items":
		return true
	case "job_line_items", "job_invoices", "job_input_materials", "job_tags", "job_attachments", "job_notes":
		return true
	case "estimate_options", "estimate_option_line_items", "estimate_option_notes", "estimate_option_attachments":
		return true
	case "invoice_items", "invoice_taxes", "invoice_discounts", "invoice_payments":
		return true
	case "lead_sources", "tags", "events", "routes", "service_zones", "pipeline_statuses":
		return true
	case "price_book_materials", "price_book_material_categories", "price_book_price_forms", "price_book_services":
		return true
	case "activity_log", "daily_metrics", "raw_api_payloads":
		return true
	default:
		return false
	}
}

func (s *Store) isInsert(ctx context.Context, table string, id string) (bool, error) {
	if !isSafeTable(table) || table == "sync_runs" {
		return false, fmt.Errorf("unsupported table %q", table)
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE id = ?", id).Scan(&exists); err != nil {
		return false, fmt.Errorf("check existing %s row: %w", table, err)
	}
	return exists == 0, nil
}

func (s *Store) upsertCustomer(ctx context.Context, item map[string]any, raw string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO customers (
			id, first_name, last_name, email, mobile_number, home_number, work_number,
			company, company_name, lead_source, created_at, updated_at, raw_json, synced_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			first_name = excluded.first_name,
			last_name = excluded.last_name,
			email = excluded.email,
			mobile_number = excluded.mobile_number,
			home_number = excluded.home_number,
			work_number = excluded.work_number,
			company = excluded.company,
			company_name = excluded.company_name,
			lead_source = excluded.lead_source,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			raw_json = excluded.raw_json,
			synced_at = excluded.synced_at
	`, idValue(item), text(item, "first_name"), text(item, "last_name"), text(item, "email"),
		text(item, "mobile_number"), text(item, "home_number"), text(item, "work_number"),
		text(item, "company"), text(item, "company_name"), text(item, "lead_source"),
		text(item, "created_at"), text(item, "updated_at"), raw, syncedAt())
	return wrapExec("upsert customer", err)
}

func (s *Store) upsertLead(ctx context.Context, item map[string]any, raw string) error {
	customer := object(item, "customer")
	employee := object(item, "assigned_employee")
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO leads (
			id, number, status, pipeline_status, lead_source, customer_id, customer_name,
			assigned_employee_id, assigned_employee_name, lost_at, created_at, updated_at, raw_json, synced_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			number = excluded.number,
			status = excluded.status,
			pipeline_status = excluded.pipeline_status,
			lead_source = excluded.lead_source,
			customer_id = excluded.customer_id,
			customer_name = excluded.customer_name,
			assigned_employee_id = excluded.assigned_employee_id,
			assigned_employee_name = excluded.assigned_employee_name,
			lost_at = excluded.lost_at,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			raw_json = excluded.raw_json,
			synced_at = excluded.synced_at
	`, idValue(item), text(item, "number"), text(item, "status"), nestedText(item, "pipeline_status", "name"),
		leadSource(item), text(customer, "id"), personName(customer),
		text(employee, "id"), personName(employee), text(item, "lost_at"),
		text(item, "created_at"), text(item, "updated_at"), raw, syncedAt())
	return wrapExec("upsert lead", err)
}

func (s *Store) upsertJob(ctx context.Context, item map[string]any, raw string) error {
	customer := object(item, "customer")
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO jobs (
			id, number, description, work_status, customer_id, customer_name, invoice_number,
			scheduled_start, scheduled_end, completed_at, canceled_at, deleted_at,
			created_at, updated_at, raw_json, synced_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			number = excluded.number,
			description = excluded.description,
			work_status = excluded.work_status,
			customer_id = excluded.customer_id,
			customer_name = excluded.customer_name,
			invoice_number = excluded.invoice_number,
			scheduled_start = excluded.scheduled_start,
			scheduled_end = excluded.scheduled_end,
			completed_at = excluded.completed_at,
			canceled_at = excluded.canceled_at,
			deleted_at = excluded.deleted_at,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			raw_json = excluded.raw_json,
			synced_at = excluded.synced_at
	`, idValue(item), text(item, "number"), text(item, "description"), text(item, "work_status"),
		text(customer, "id"), personName(customer), text(item, "invoice_number"),
		text(item, "scheduled_start"), text(item, "scheduled_end"), text(item, "completed_at"),
		text(item, "canceled_at"), text(item, "deleted_at"), text(item, "created_at"),
		text(item, "updated_at"), raw, syncedAt())
	return wrapExec("upsert job", err)
}

func (s *Store) upsertEstimate(ctx context.Context, item map[string]any, raw string) error {
	customer := object(item, "customer")
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO estimates (
			id, number, status, customer_id, customer_name, total_amount,
			created_at, updated_at, raw_json, synced_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			number = excluded.number,
			status = excluded.status,
			customer_id = excluded.customer_id,
			customer_name = excluded.customer_name,
			total_amount = excluded.total_amount,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			raw_json = excluded.raw_json,
			synced_at = excluded.synced_at
	`, idValue(item), text(item, "number"), text(item, "status"),
		text(customer, "id"), personName(customer), integer(item, "total_amount", "amount"),
		text(item, "created_at"), text(item, "updated_at"), raw, syncedAt())
	return wrapExec("upsert estimate", err)
}

func (s *Store) upsertInvoice(ctx context.Context, item map[string]any, raw string) error {
	customer := object(item, "customer")
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO invoices (
			id, uuid, invoice_number, status, customer_id, customer_name, total_amount,
			amount_due, due_at, paid_at, created_at, updated_at, raw_json, synced_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			uuid = excluded.uuid,
			invoice_number = excluded.invoice_number,
			status = excluded.status,
			customer_id = excluded.customer_id,
			customer_name = excluded.customer_name,
			total_amount = excluded.total_amount,
			amount_due = excluded.amount_due,
			due_at = excluded.due_at,
			paid_at = excluded.paid_at,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			raw_json = excluded.raw_json,
			synced_at = excluded.synced_at
	`, idValue(item), text(item, "uuid"), text(item, "invoice_number", "number"),
		text(item, "status"), text(customer, "id"), personName(customer),
		integer(item, "total_amount", "amount"), integer(item, "amount_due", "due_amount"),
		text(item, "due_at"), text(item, "paid_at"), text(item, "created_at"),
		text(item, "updated_at"), raw, syncedAt())
	return wrapExec("upsert invoice", err)
}

func (s *Store) upsertEmployee(ctx context.Context, item map[string]any, raw string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO employees (
			id, first_name, last_name, email, mobile_number, role, created_at, updated_at, raw_json, synced_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			first_name = excluded.first_name,
			last_name = excluded.last_name,
			email = excluded.email,
			mobile_number = excluded.mobile_number,
			role = excluded.role,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			raw_json = excluded.raw_json,
			synced_at = excluded.synced_at
	`, idValue(item), text(item, "first_name"), text(item, "last_name"), text(item, "email"),
		text(item, "mobile_number"), text(item, "role"), text(item, "created_at"), text(item, "updated_at"), raw, syncedAt())
	return wrapExec("upsert employee", err)
}

func (s *Store) upsertAppointment(ctx context.Context, item map[string]any, raw string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO appointments (
			id, job_id, job_number, start_time, end_time, arrival_window,
			assigned_employee_ids, created_at, updated_at, raw_json, synced_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			job_id = excluded.job_id,
			job_number = excluded.job_number,
			start_time = excluded.start_time,
			end_time = excluded.end_time,
			arrival_window = excluded.arrival_window,
			assigned_employee_ids = excluded.assigned_employee_ids,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			raw_json = excluded.raw_json,
			synced_at = excluded.synced_at
	`, idValue(item), text(item, "job_id"), text(item, "job_number"), text(item, "start_time", "start", "scheduled_start"),
		text(item, "end_time", "end", "scheduled_end"), text(item, "arrival_window"), jsonText(item, "assigned_employee_ids", "employee_ids", "employees"),
		text(item, "created_at"), text(item, "updated_at"), raw, syncedAt())
	return wrapExec("upsert appointment", err)
}

func (s *Store) upsertGeneric(ctx context.Context, resource string, item map[string]any, raw string) error {
	if !isSafeTable(resource) || resource == "sync_runs" {
		return fmt.Errorf("unsupported resource %q", resource)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO `+resource+` (id, name, status, created_at, updated_at, raw_json, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			status = excluded.status,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			raw_json = excluded.raw_json,
			synced_at = excluded.synced_at
	`, idValue(item), text(item, "name", "title"), text(item, "status"), text(item, "created_at"), text(item, "updated_at"), raw, syncedAt())
	return wrapExec("upsert "+resource, err)
}

func wrapExec(action string, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return nil
}

func syncedAt() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func idValue(item map[string]any) string {
	if value := text(item, "id"); value != "" {
		return value
	}
	if value := text(item, "uuid"); value != "" {
		return value
	}
	return text(item, "number")
}

func text(item map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := item[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			return strings.TrimSpace(typed)
		case json.Number:
			return typed.String()
		case float64:
			return fmt.Sprintf("%.0f", typed)
		case bool:
			return fmt.Sprintf("%t", typed)
		}
	}
	return ""
}

func TextValue(item map[string]any, keys ...string) string {
	return text(item, keys...)
}

func jsonText(item map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := item[key]
		if !ok || value == nil {
			continue
		}
		if textValue := text(item, key); textValue != "" {
			return textValue
		}
		data, err := json.Marshal(value)
		if err == nil {
			return string(data)
		}
	}
	return ""
}

func integer(item map[string]any, keys ...string) any {
	for _, key := range keys {
		value, ok := item[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case int:
			return typed
		case int64:
			return typed
		case float64:
			return int64(typed)
		case json.Number:
			i, err := typed.Int64()
			if err == nil {
				return i
			}
		}
	}
	return nil
}

func object(item map[string]any, key string) map[string]any {
	if value, ok := item[key].(map[string]any); ok {
		return value
	}
	return nil
}

func nestedText(item map[string]any, key string, nestedKeys ...string) string {
	if value := text(item, key); value != "" {
		return value
	}
	return text(object(item, key), nestedKeys...)
}

func leadSource(item map[string]any) string {
	if value := text(item, "lead_source"); value != "" {
		return value
	}
	return text(object(item, "lead_source"), "name", "id")
}

func personName(item map[string]any) string {
	if len(item) == 0 {
		return ""
	}
	if value := text(item, "name", "display_name", "company", "company_name"); value != "" {
		return value
	}

	parts := []string{text(item, "first_name"), text(item, "last_name")}
	return strings.TrimSpace(strings.Join(nonEmpty(parts), " "))
}

func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, strings.TrimSpace(value))
		}
	}
	return out
}
