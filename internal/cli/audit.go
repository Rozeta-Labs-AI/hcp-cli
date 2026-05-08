package cli

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type auditRecord struct {
	ID           string                 `json:"id"`
	Timestamp    string                 `json:"timestamp"`
	Action       string                 `json:"action"`
	Status       string                 `json:"status"`
	Method       string                 `json:"method,omitempty"`
	Path         string                 `json:"path,omitempty"`
	Query        map[string]any         `json:"query,omitempty"`
	Body         any                    `json:"body,omitempty"`
	Risk         string                 `json:"risk,omitempty"`
	Safety       apiSafety              `json:"safety,omitempty"`
	Verification map[string]any         `json:"verification,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

func newAuditCommand(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Inspect local hcp audit logs",
	}
	cmd.AddCommand(newAuditListCommand(app))
	cmd.AddCommand(newAuditShowCommand(app))
	return cmd
}

func newAuditListCommand(app *App) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent audit log entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			records, path, err := app.readAudit(limit)
			if err != nil {
				return err
			}
			if app.JSON {
				return writeJSON(app.Out, map[string]any{"path": path, "count": len(records), "records": records})
			}
			if len(records) == 0 {
				fmt.Fprintf(app.Out, "No audit entries at %s\n", path)
				return nil
			}
			for _, record := range records {
				fmt.Fprintf(app.Out, "%s %s %s %s %s risk=%s blast=%s\n", record.ID, record.Timestamp, record.Status, record.Method, record.Path, record.Risk, record.Safety.BlastRadius)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum audit entries to show")
	return cmd
}

func newAuditShowCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show one audit log entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			records, _, err := app.readAudit(0)
			if err != nil {
				return err
			}
			for _, record := range records {
				if record.ID == args[0] {
					return writeJSON(app.Out, record)
				}
			}
			return errorf(exitUsage, "audit entry %s not found", args[0])
		},
	}
}

func (a *App) auditPath() (string, error) {
	cfgPath := a.ConfigPath
	if strings.TrimSpace(cfgPath) == "" {
		resolved, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolve user config dir: %w", err)
		}
		return filepath.Join(resolved, "hcp", "audit.jsonl"), nil
	}
	return filepath.Join(filepath.Dir(cfgPath), "audit.jsonl"), nil
}

func (a *App) writeAudit(record auditRecord) error {
	path, err := a.auditPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create audit dir: %w", err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer file.Close()
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode audit record: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write audit record: %w", err)
	}
	return nil
}

func (a *App) readAudit(limit int) ([]auditRecord, string, error) {
	path, err := a.auditPath()
	if err != nil {
		return nil, "", err
	}
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, path, nil
	}
	if err != nil {
		return nil, path, fmt.Errorf("open audit log: %w", err)
	}
	defer file.Close()
	var records []auditRecord
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record auditRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err == nil {
			records = append(records, record)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, path, err
	}
	if limit > 0 && len(records) > limit {
		records = records[len(records)-limit:]
	}
	return records, path, nil
}

func apiAuditRecord(action string, plan apiPlan, status string, verification *apiVerificationResult, err error) auditRecord {
	record := auditRecord{
		ID:        newAuditID(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Action:    action,
		Status:    status,
		Method:    plan.Method,
		Path:      plan.Path,
		Query:     redactMap(plan.Query),
		Body:      redactValue(plan.Body),
		Risk:      plan.Risk,
		Safety:    plan.Safety,
	}
	if verification != nil {
		record.Verification = map[string]any{
			"method": verification.Method,
			"path":   verification.Path,
			"ok":     verification.OK,
		}
	}
	if err != nil {
		record.Error = err.Error()
	}
	return record
}

func redactMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]any, len(values))
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out[key] = redactKeyValue(key, values[key])
	}
	return out
}

func redactValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return redactMap(typed)
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, redactValue(item))
		}
		return out
	default:
		return value
	}
}

func redactKeyValue(key string, value any) any {
	lower := strings.ToLower(key)
	if strings.Contains(lower, "key") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "authorization") {
		return "[redacted]"
	}
	return redactValue(value)
}

func newAuditID() string {
	var data [4]byte
	if _, err := rand.Read(data[:]); err != nil {
		return fmt.Sprintf("aud_%d", time.Now().UnixNano())
	}
	return "aud_" + hex.EncodeToString(data[:])
}
