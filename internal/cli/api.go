package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Rozeta-Labs-AI/hcp-cli/internal/hcp"
	"github.com/spf13/cobra"
)

type apiPlan struct {
	Method  string         `json:"method"`
	Path    string         `json:"path"`
	Query   map[string]any `json:"query,omitempty"`
	Body    any            `json:"body,omitempty"`
	Files   []apiPlanFile  `json:"files,omitempty"`
	Mutable bool           `json:"mutable"`
	Risk    string         `json:"risk"`
}

type apiPlanFile struct {
	FieldName   string `json:"field_name"`
	Path        string `json:"path"`
	ContentType string `json:"content_type,omitempty"`
}

type apiResource struct {
	Name       string
	Singular   string
	Collection string
	ItemPath   string
}

var apiResources = []apiResource{
	{Name: "application", Singular: "application", Collection: "/application"},
	{Name: "checklists", Singular: "checklist", Collection: "/checklists"},
	{Name: "customers", Singular: "customer", Collection: "/customers", ItemPath: "/customers/{id}"},
	{Name: "employees", Singular: "employee", Collection: "/employees", ItemPath: "/employees/{id}"},
	{Name: "jobs", Singular: "job", Collection: "/jobs", ItemPath: "/jobs/{id}"},
	{Name: "estimates", Singular: "estimate", Collection: "/estimates", ItemPath: "/estimates/{id}"},
	{Name: "company", Singular: "company", Collection: "/company"},
	{Name: "events", Singular: "event", Collection: "/events", ItemPath: "/events/{id}"},
	{Name: "tags", Singular: "tag", Collection: "/tags", ItemPath: "/tags/{id}"},
	{Name: "leads", Singular: "lead", Collection: "/leads", ItemPath: "/leads/{id}"},
	{Name: "lead sources", Singular: "lead source", Collection: "/lead_sources", ItemPath: "/lead_sources/{id}"},
	{Name: "job types", Singular: "job type", Collection: "/job_fields/job_types", ItemPath: "/job_fields/job_types/{id}"},
	{Name: "invoices", Singular: "invoice", Collection: "/invoices", ItemPath: "/invoices/{id}"},
	{Name: "materials", Singular: "material", Collection: "/api/price_book/materials", ItemPath: "/api/price_book/materials/{id}"},
	{Name: "material categories", Singular: "material category", Collection: "/api/price_book/material_categories", ItemPath: "/api/price_book/material_categories/{id}"},
	{Name: "price forms", Singular: "price form", Collection: "/api/price_book/price_forms", ItemPath: "/api/price_book/price_forms/{id}"},
	{Name: "services", Singular: "service", Collection: "/api/price_book/services", ItemPath: "/api/price_book/services/{id}"},
	{Name: "service zones", Singular: "service zone", Collection: "/service_zones", ItemPath: "/service_zones/{id}"},
	{Name: "routes", Singular: "route", Collection: "/routes", ItemPath: "/routes/{id}"},
	{Name: "pipeline statuses", Singular: "pipeline status", Collection: "/pipeline/statuses"},
}

func newAPICommand(app *App) *cobra.Command {
	var method string
	var path string
	var body string
	var queryPairs []string
	var paramPairs []string
	var filePaths []string
	var fileField string
	var contentType string
	var planOnly bool
	var yes bool
	var dryRun bool
	var confirm string
	var limit int
	var page int

	cmd := &cobra.Command{
		Use:   "api [natural language request]",
		Short: "Call any Housecall Pro API endpoint from natural language or explicit request flags",
		Long: "Call Housecall Pro directly. Examples:\n" +
			"  hcp api list customers --limit 10\n" +
			"  hcp api get /company\n" +
			"  hcp api create customer --body '{\"first_name\":\"Ada\"}' --yes\n" +
			"  hcp api --method PATCH --path /customers/cus_123 --body '{\"last_name\":\"Lovelace\"}' --yes",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			request := strings.Join(args, " ")
			plan, err := buildAPIPlan(request, method, path, body, queryPairs, paramPairs, filePaths, fileField, contentType, limit, page)
			if err != nil {
				return errorf(exitUsage, "%w", err)
			}

			if dryRun {
				planOnly = true
			}
			if planOnly {
				return writeAPIPlan(app, plan)
			}
			if plan.Mutable && !yes {
				return errorf(exitUsage, "refusing to run %s %s without --yes; inspect first with --plan", plan.Method, plan.Path)
			}
			if plan.Risk == "destructive" && confirm != destructiveConfirmToken(plan) {
				return errorf(exitUsage, "refusing destructive %s %s without --confirm %s", plan.Method, plan.Path, destructiveConfirmToken(plan))
			}

			client, _, _, err := app.newClient()
			if err != nil {
				return err
			}

			var raw json.RawMessage
			if len(plan.Files) > 0 {
				raw, err = client.DoMultipart(commandContext(cmd), plan.Method, plan.Path, valuesFromPlan(plan.Query), multipartFields(plan.Body), multipartFiles(plan.Files))
			} else {
				raw, err = client.DoRaw(commandContext(cmd), plan.Method, plan.Path, valuesFromPlan(plan.Query), plan.Body)
			}
			if err != nil {
				return errorf(exitAPI, "%w", err)
			}

			if app.JSON {
				var value any
				if err := json.Unmarshal(raw, &value); err != nil {
					return fmt.Errorf("decode json response: %w", err)
				}
				return writeJSON(app.Out, map[string]any{
					"request":  plan,
					"response": value,
				})
			}

			fmt.Fprintf(app.Out, "%s %s\n", plan.Method, plan.Path)
			return prettyPrintRawJSON(app.Out, raw)
		},
	}

	cmd.Flags().StringVar(&method, "method", "", "HTTP method to use")
	cmd.Flags().StringVar(&path, "path", "", "Housecall Pro API path, for example /customers")
	cmd.Flags().StringVar(&body, "body", "", "JSON request body for POST, PATCH, or PUT")
	cmd.Flags().StringArrayVar(&queryPairs, "query", nil, "query parameter as key=value; repeat for multiple values")
	cmd.Flags().StringArrayVar(&paramPairs, "param", nil, "path parameter as key=value, for example job_id=job_123")
	cmd.Flags().StringArrayVar(&filePaths, "file", nil, "file path to upload as multipart data; repeat for multiple files")
	cmd.Flags().StringVar(&fileField, "file-field", "file", "multipart file field name")
	cmd.Flags().StringVar(&contentType, "content-type", "", "file content type hint for plans")
	cmd.Flags().BoolVar(&planOnly, "plan", false, "print the resolved API request without executing it")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "alias for --plan; do not send the API request")
	cmd.Flags().BoolVar(&yes, "yes", false, "execute mutating API requests")
	cmd.Flags().StringVar(&confirm, "confirm", "", "confirmation token required for destructive actions")
	cmd.Flags().IntVar(&limit, "limit", 0, "page_size alias for natural-language list requests")
	cmd.Flags().IntVar(&page, "page", 0, "page query parameter for natural-language list requests")

	cmd.AddCommand(newAPICatalogCommand(app))
	cmd.AddCommand(newAPIExamplesCommand(app))

	return cmd
}

type apiExample struct {
	Area        string `json:"area"`
	Intent      string `json:"intent"`
	Command     string `json:"command"`
	Mutable     bool   `json:"mutable"`
	Description string `json:"description,omitempty"`
}

func newAPIExamplesCommand(app *App) *cobra.Command {
	var area string
	cmd := &cobra.Command{
		Use:   "examples",
		Short: "Show agent-ready examples for Housecall Pro API actions",
		RunE: func(cmd *cobra.Command, args []string) error {
			examples := filterExamples(apiExamples(), area)
			if app.JSON {
				return writeJSON(app.Out, map[string]any{"count": len(examples), "examples": examples})
			}
			for _, example := range examples {
				fmt.Fprintf(app.Out, "%s: %s\n  %s\n", example.Area, example.Intent, example.Command)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&area, "area", "", "filter by API area")
	return cmd
}

func apiExamples() []apiExample {
	return []apiExample{
		{"application", "Get application status", "hcp api get /application", false, ""},
		{"application", "Enable application", "hcp api post /application/enable --plan", true, ""},
		{"application", "Disable application", "hcp api post /application/disable --plan", true, ""},
		{"checklists", "List checklists for jobs", "hcp api --method GET --path /checklists --query job_uuids[]=job_123 --limit 25", false, ""},
		{"customers", "List customers", "hcp api list customers --limit 25", false, ""},
		{"customers", "Create customer", "hcp api create customer --body '{\"first_name\":\"Ada\",\"last_name\":\"Lovelace\"}' --plan", true, ""},
		{"customers", "Update customer", "hcp api --method PUT --path /customers/cus_123 --body '{\"last_name\":\"Lovelace\"}' --plan", true, ""},
		{"customer_addresses", "List customer addresses", "hcp api --method GET --path /customers/cus_123/addresses", false, ""},
		{"customer_addresses", "Create customer address", "hcp api --method POST --path /customers/cus_123/addresses --body '{\"street\":\"1 Main St\"}' --plan", true, ""},
		{"employees", "List employees", "hcp api list employees --limit 25", false, ""},
		{"jobs", "List jobs", "hcp api list jobs --limit 25", false, ""},
		{"jobs", "Create job", "hcp api create job --body '{\"customer_id\":\"cus_123\"}' --plan", true, ""},
		{"job_line_items", "List job line items", "hcp api --method GET --path /jobs/job_123/line_items", false, ""},
		{"job_line_items", "Bulk update job line items", "hcp api --method PUT --path /jobs/job_123/line_items/bulk_update --body '{\"line_items\":[]}' --plan", true, ""},
		{"job_appointments", "List job appointments", "hcp api --method GET --path /jobs/job_123/appointments", false, ""},
		{"job_appointments", "Create job appointment", "hcp api --method POST --path /jobs/job_123/appointments --body '{\"start_time\":\"2026-05-09T09:00:00Z\"}' --plan", true, ""},
		{"job_schedule", "Update job schedule", "hcp api --method PUT --path /jobs/job_123/schedule --body '{\"scheduled_start\":\"2026-05-09T09:00:00Z\"}' --plan", true, ""},
		{"job_dispatch", "Dispatch job", "hcp api --method PUT --path /jobs/job_123/dispatch --body '{\"employee_ids\":[\"emp_123\"]}' --plan", true, ""},
		{"job_invoices", "List job invoices", "hcp api --method GET --path /jobs/job_123/invoices", false, ""},
		{"job_notes", "Add job note", "hcp api --method POST --path /jobs/job_123/notes --body '{\"note\":\"Called customer\"}' --plan", true, ""},
		{"job_tags", "Add job tag", "hcp api --method POST --path /jobs/job_123/tags --body '{\"tag_id\":\"tag_123\"}' --plan", true, ""},
		{"job_links", "Create job link", "hcp api --method POST --path /jobs/job_123/links --body '{\"url\":\"https://example.com\"}' --plan", true, ""},
		{"job_locks", "Lock one job", "hcp api --method POST --path /jobs/job_123/lock --body '{\"locked_at\":\"2026-05-09T00:00:00Z\"}' --plan", true, ""},
		{"webhooks", "Create webhook subscription", "hcp api --method POST --path /webhooks/subscription --body '{\"url\":\"https://example.com/webhook\"}' --plan", true, ""},
		{"webhooks", "Delete webhook subscription", "hcp api --method DELETE --path /webhooks/subscription --plan", true, ""},
		{"estimates", "List estimates", "hcp api list estimates --limit 25", false, ""},
		{"estimates", "Create estimate", "hcp api create estimate --body '{\"customer_id\":\"cus_123\"}' --plan", true, ""},
		{"estimate_options", "Create estimate option", "hcp api --method POST --path /estimates/est_123/options --body '{\"name\":\"Option A\"}' --plan", true, ""},
		{"estimate_line_items", "List estimate option line items", "hcp api --method GET --path /estimates/est_123/options/opt_123/line_items", false, ""},
		{"estimate_schedule", "Update estimate option schedule", "hcp api --method PUT --path /estimates/est_123/options/opt_123/schedule --body '{\"scheduled_start\":\"2026-05-09T09:00:00Z\"}' --plan", true, ""},
		{"estimate_notes", "Create estimate option note", "hcp api --method POST --path /estimates/est_123/options/opt_123/notes --body '{\"note\":\"Follow up Friday\"}' --plan", true, ""},
		{"estimate_approval", "Approve estimate options", "hcp api --method POST --path /estimates/options/approve --body '{\"option_ids\":[\"opt_123\"]}' --plan", true, ""},
		{"company", "Get company", "hcp api get /company", false, ""},
		{"company", "Update franchise info", "hcp api --method PATCH --path /company/franchise_info --body '{\"franchise_name\":\"Example\"}' --plan", true, ""},
		{"schedule", "Get schedule availability", "hcp api get /company/schedule_availability", false, ""},
		{"schedule", "Update schedule availability", "hcp api --method PUT --path /company/schedule_availability --body '{\"daily_schedule_windows\":[]}' --plan", true, ""},
		{"events", "List events", "hcp api list events --limit 25", false, ""},
		{"tags", "List tags", "hcp api list tags --limit 25", false, ""},
		{"tags", "Create tag", "hcp api create tag --body '{\"name\":\"VIP\"}' --plan", true, ""},
		{"leads", "List leads", "hcp api list leads --limit 25", false, ""},
		{"leads", "Create lead", "hcp api create lead --body '{\"customer_id\":\"cus_123\"}' --plan", true, ""},
		{"leads", "Convert lead", "hcp api --method POST --path /leads/lead_123/convert --body '{\"convert_to\":\"job\"}' --plan", true, ""},
		{"lead_line_items", "List lead line items", "hcp api --method GET --path /leads/lead_123/line_items", false, ""},
		{"lead_sources", "List lead sources", "hcp api list lead_sources", false, ""},
		{"lead_sources", "Create lead source", "hcp api create lead source --body '{\"name\":\"Google Ads\"}' --plan", true, ""},
		{"job_types", "List job types", "hcp api --method GET --path /job_fields/job_types", false, ""},
		{"job_types", "Create job type", "hcp api --method POST --path /job_fields/job_types --body '{\"name\":\"Install\"}' --plan", true, ""},
		{"invoices", "List invoices", "hcp api list invoices --limit 25", false, ""},
		{"invoices", "Get invoice by UUID", "hcp api --method GET --path /api/invoices/inv_uuid", false, ""},
		{"invoices", "Preview invoice by UUID", "hcp api --method GET --path /api/invoices/inv_uuid/preview", false, ""},
		{"price_book", "List materials", "hcp api --method GET --path /api/price_book/materials --query material_category_uuid=cat_uuid", false, ""},
		{"price_book", "Create material", "hcp api --method POST --path /api/price_book/materials --body '{\"material_category_uuid\":\"cat_uuid\",\"name\":\"Filter\"}' --plan", true, ""},
		{"price_book", "Update material category", "hcp api --method PUT --path /api/price_book/material_categories/cat_uuid --body '{\"name\":\"Parts\"}' --plan", true, ""},
		{"price_book", "Delete price form", "hcp api --method DELETE --path /api/price_book/price_forms/form_uuid --plan", true, ""},
		{"service_zones", "List service zones", "hcp api --method GET --path /service_zones", false, ""},
		{"routes", "List routes", "hcp api --method GET --path /routes", false, ""},
		{"pipeline", "List pipeline statuses", "hcp api --method GET --path /pipeline/statuses --query resource_type=lead", false, ""},
		{"pipeline", "Update pipeline status", "hcp api --method PUT --path /pipeline/statuses --body '{\"resource_type\":\"lead\",\"statuses\":[]}' --plan", true, ""},
	}
}

func filterExamples(examples []apiExample, area string) []apiExample {
	area = strings.ToLower(strings.TrimSpace(area))
	if area == "" {
		return examples
	}
	out := make([]apiExample, 0, len(examples))
	for _, example := range examples {
		if strings.Contains(strings.ToLower(example.Area), area) || strings.Contains(strings.ToLower(example.Intent), area) {
			out = append(out, example)
		}
	}
	return out
}

type apiCatalogOperation struct {
	Area        string   `json:"area"`
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	Summary     string   `json:"summary"`
	OperationID string   `json:"operation_id,omitempty"`
	Mutable     bool     `json:"mutable"`
	PathParams  []string `json:"path_params,omitempty"`
}

func newAPICatalogCommand(app *App) *cobra.Command {
	var specPath string
	var area string
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "List documented Housecall Pro API operations",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := strings.TrimSpace(specPath)
			if path == "" {
				path = "HOUSECALL_PRO_OPENAPI_SNAPSHOT.yaml"
			}
			file, err := os.Open(path)
			if err != nil && !filepath.IsAbs(path) {
				file, err = os.Open(filepath.Join("..", "..", path))
			}
			if err != nil {
				return errorf(exitConfig, "open API snapshot: %w", err)
			}
			defer file.Close()

			ops, err := parseOpenAPICatalog(file)
			if err != nil {
				return errorf(exitConfig, "%w", err)
			}
			ops = filterCatalog(ops, area)

			if app.JSON {
				return writeJSON(app.Out, map[string]any{
					"count":      len(ops),
					"operations": ops,
				})
			}

			for _, op := range ops {
				fmt.Fprintf(app.Out, "%-6s %-55s %s\n", op.Method, op.Path, op.Summary)
			}
			fmt.Fprintf(app.Out, "\n%d operation(s)\n", len(ops))
			return nil
		},
	}
	cmd.Flags().StringVar(&specPath, "spec", "", "OpenAPI snapshot path")
	cmd.Flags().StringVar(&area, "area", "", "filter by API area, for example jobs, estimates, price_book")
	return cmd
}

func parseOpenAPICatalog(r io.Reader) ([]apiCatalogOperation, error) {
	scanner := bufio.NewScanner(r)
	var currentPath string
	var current *apiCatalogOperation
	var ops []apiCatalogOperation

	flush := func() {
		if current != nil {
			current.PathParams = pathParams(current.Path)
			ops = append(ops, *current)
			current = nil
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(line, "  /") || strings.HasPrefix(line, "  '/") {
			flush()
			currentPath = strings.Trim(strings.TrimSuffix(trimmed, ":"), "'\"")
			continue
		}
		if currentPath == "" {
			continue
		}
		if method := openAPIMethodLine(line); method != "" {
			flush()
			current = &apiCatalogOperation{
				Area:    areaFromPath(currentPath),
				Method:  method,
				Path:    currentPath,
				Mutable: method != http.MethodGet,
			}
			continue
		}
		if current == nil {
			continue
		}
		if strings.HasPrefix(trimmed, "summary:") && current.Summary == "" {
			current.Summary = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "summary:")), "'\"")
		}
		if strings.HasPrefix(trimmed, "operationId:") && current.OperationID == "" {
			current.OperationID = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "operationId:")), "'\"")
		}
	}
	flush()
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read OpenAPI snapshot: %w", err)
	}
	return ops, nil
}

func openAPIMethodLine(line string) string {
	if !strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "      ") {
		return ""
	}
	switch strings.TrimSpace(line) {
	case "get:":
		return http.MethodGet
	case "post:":
		return http.MethodPost
	case "put:":
		return http.MethodPut
	case "patch:":
		return http.MethodPatch
	case "delete:":
		return http.MethodDelete
	default:
		return ""
	}
}

func filterCatalog(ops []apiCatalogOperation, area string) []apiCatalogOperation {
	area = strings.ToLower(strings.TrimSpace(area))
	if area == "" {
		return ops
	}
	out := make([]apiCatalogOperation, 0, len(ops))
	for _, op := range ops {
		if strings.Contains(strings.ToLower(op.Area), area) || strings.Contains(strings.ToLower(op.Path), area) {
			out = append(out, op)
		}
	}
	return out
}

func areaFromPath(path string) string {
	path = strings.Trim(path, "/")
	switch {
	case strings.HasPrefix(path, "api/price_book/"):
		return "price_book"
	case strings.HasPrefix(path, "api/invoices"):
		return "invoices"
	case strings.HasPrefix(path, "job_fields/"):
		return "job_fields"
	}
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "root"
	}
	return parts[0]
}

func pathParams(path string) []string {
	matches := regexp.MustCompile(`\{([^}]+)\}`).FindAllStringSubmatch(path, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) == 2 {
			out = append(out, match[1])
		}
	}
	return out
}

func buildAPIPlan(request string, explicitMethod string, explicitPath string, rawBody string, queryPairs []string, paramPairs []string, filePaths []string, fileField string, contentType string, limit int, page int) (apiPlan, error) {
	query, err := parseQueryPairs(queryPairs)
	if err != nil {
		return apiPlan{}, err
	}
	params, err := parseParamPairs(paramPairs)
	if err != nil {
		return apiPlan{}, err
	}

	body, err := parseBody(rawBody)
	if err != nil {
		return apiPlan{}, err
	}
	files := planFiles(filePaths, fileField, contentType)

	method := strings.ToUpper(strings.TrimSpace(explicitMethod))
	path := normalizePath(explicitPath)
	if method == "" {
		method = methodFromWords(request, body != nil)
	}
	if path == "" {
		path = pathFromWords(request)
	}
	if path == "" {
		return apiPlan{}, fmt.Errorf("could not infer API path; include an endpoint like /customers or pass --path")
	}
	path, err = substitutePathParams(path, params)
	if err != nil {
		return apiPlan{}, err
	}
	if method == "" {
		method = http.MethodGet
	}
	if err := validateMethod(method); err != nil {
		return apiPlan{}, err
	}

	if method == http.MethodGet {
		addPaginationFromWords(query, request)
		if limit > 0 {
			query["page_size"] = fmt.Sprintf("%d", limit)
		}
		if page > 0 {
			query["page"] = fmt.Sprintf("%d", page)
		}
	}

	return apiPlan{
		Method:  method,
		Path:    path,
		Query:   query,
		Body:    body,
		Files:   files,
		Mutable: method != http.MethodGet,
		Risk:    riskFor(method, path),
	}, nil
}

func planFiles(paths []string, field string, contentType string) []apiPlanFile {
	field = strings.TrimSpace(field)
	if field == "" {
		field = "file"
	}
	out := make([]apiPlanFile, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		out = append(out, apiPlanFile{FieldName: field, Path: path, ContentType: strings.TrimSpace(contentType)})
	}
	return out
}

func multipartFields(body any) map[string]string {
	fields := map[string]string{}
	object, ok := body.(map[string]any)
	if !ok {
		return fields
	}
	for key, value := range object {
		switch typed := value.(type) {
		case string:
			fields[key] = typed
		default:
			data, err := json.Marshal(typed)
			if err == nil {
				fields[key] = string(data)
			}
		}
	}
	return fields
}

func multipartFiles(files []apiPlanFile) []hcp.FilePart {
	out := make([]hcp.FilePart, 0, len(files))
	for _, file := range files {
		out = append(out, hcp.FilePart{FieldName: file.FieldName, Path: file.Path, ContentType: file.ContentType})
	}
	return out
}

func writeAPIPlan(app *App, plan apiPlan) error {
	if app.JSON {
		return writeJSON(app.Out, plan)
	}
	fmt.Fprintf(app.Out, "%s %s\n", plan.Method, plan.Path)
	if len(plan.Query) > 0 {
		keys := make([]string, 0, len(plan.Query))
		for key := range plan.Query {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(app.Out, "query.%s=%v\n", key, plan.Query[key])
		}
	}
	if plan.Body != nil {
		fmt.Fprintln(app.Out, "body:")
		_ = writeJSON(app.Out, plan.Body)
	}
	if plan.Mutable {
		fmt.Fprintf(app.Out, "mutable=true risk=%s; execute with --yes", plan.Risk)
		if plan.Risk == "destructive" {
			fmt.Fprintf(app.Out, " --confirm %s", destructiveConfirmToken(plan))
		}
		fmt.Fprintln(app.Out)
	}
	return nil
}

func riskFor(method string, path string) string {
	if method == http.MethodGet {
		return "read"
	}
	lower := strings.ToLower(path)
	if method == http.MethodDelete ||
		strings.Contains(lower, "/disable") ||
		strings.Contains(lower, "/decline") ||
		strings.Contains(lower, "/lock") ||
		strings.Contains(lower, "/schedule") ||
		strings.Contains(lower, "/webhooks/subscription") {
		return "destructive"
	}
	return "mutating"
}

func destructiveConfirmToken(plan apiPlan) string {
	return strings.ToLower(plan.Method) + ":" + plan.Path
}

func parseBody(raw string) (any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var body any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return nil, fmt.Errorf("decode --body JSON: %w", err)
	}
	return body, nil
}

func parseQueryPairs(pairs []string) (map[string]any, error) {
	query := map[string]any{}
	for _, pair := range pairs {
		key, value, ok := strings.Cut(pair, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("--query must be key=value")
		}
		if existing, ok := query[key]; ok {
			switch values := existing.(type) {
			case []string:
				query[key] = append(values, strings.TrimSpace(value))
			case string:
				query[key] = []string{values, strings.TrimSpace(value)}
			}
			continue
		}
		query[key] = strings.TrimSpace(value)
	}
	return query, nil
}

func parseParamPairs(pairs []string) (map[string]string, error) {
	params := map[string]string{}
	for _, pair := range pairs {
		key, value, ok := strings.Cut(pair, "=")
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if !ok || key == "" || value == "" {
			return nil, fmt.Errorf("--param must be key=value")
		}
		params[key] = value
	}
	return params, nil
}

func substitutePathParams(path string, params map[string]string) (string, error) {
	missing := []string{}
	for _, name := range pathParams(path) {
		value := strings.TrimSpace(params[name])
		if value == "" {
			missing = append(missing, name)
			continue
		}
		path = strings.ReplaceAll(path, "{"+name+"}", url.PathEscape(value))
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("missing path param(s) for %s: pass %s", path, paramHelp(missing))
	}
	return path, nil
}

func paramHelp(names []string) string {
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, "--param "+name+"=<value>")
	}
	return strings.Join(parts, " ")
}

func methodFromWords(request string, hasBody bool) string {
	lower := strings.ToLower(request)
	switch {
	case regexp.MustCompile(`\b(delete|remove|destroy)\b`).MatchString(lower):
		return http.MethodDelete
	case regexp.MustCompile(`\b(update|change|patch|modify)\b`).MatchString(lower):
		return http.MethodPatch
	case regexp.MustCompile(`\b(create|add|post|enable|approve|decline)\b`).MatchString(lower):
		return http.MethodPost
	case regexp.MustCompile(`\b(put|replace)\b`).MatchString(lower):
		return http.MethodPut
	case regexp.MustCompile(`\b(get|show|list|find|search|fetch)\b`).MatchString(lower):
		return http.MethodGet
	case hasBody:
		return http.MethodPost
	default:
		return http.MethodGet
	}
}

func pathFromWords(request string) string {
	if path := firstPathToken(request); path != "" {
		return path
	}
	lower := strings.ToLower(request)

	if path := specialPathFromWords(lower); path != "" {
		return path
	}

	for _, rule := range apiPathRules() {
		if rule.matches(lower) {
			return rule.Path
		}
	}

	if strings.Contains(lower, "schedule availability") {
		if strings.Contains(lower, "booking window") {
			return "/company/schedule_availability/booking_windows"
		}
		return "/company/schedule_availability"
	}
	if strings.Contains(lower, "franchise") {
		return "/company/franchise_info"
	}
	if strings.Contains(lower, "approve estimate option") {
		return "/estimates/options/approve"
	}
	if strings.Contains(lower, "decline estimate option") {
		return "/estimates/options/decline"
	}
	if strings.Contains(lower, "webhook") {
		return "/webhooks/subscription"
	}

	for _, resource := range apiResources {
		if containsResource(lower, resource) {
			if id := idFromWords(lower); id != "" && resource.ItemPath != "" {
				return strings.ReplaceAll(resource.ItemPath, "{id}", id)
			}
			return resource.Collection
		}
	}
	return ""
}

type apiPathRule struct {
	Terms []string
	Path  string
}

func (r apiPathRule) matches(lower string) bool {
	for _, term := range r.Terms {
		if !strings.Contains(lower, term) {
			return false
		}
	}
	return true
}

func specialPathFromWords(lower string) string {
	switch {
	case strings.Contains(lower, "customer address") || strings.Contains(lower, "customer addresses"):
		if strings.Contains(lower, " address id") || strings.Contains(lower, "address_id") {
			return "/customers/{customer_id}/addresses/{address_id}"
		}
		return "/customers/{customer_id}/addresses"
	case strings.Contains(lower, "job line item"):
		if strings.Contains(lower, "bulk") {
			return "/jobs/{job_id}/line_items/bulk_update"
		}
		if strings.Contains(lower, "delete") || strings.Contains(lower, "update single") {
			return "/jobs/{job_id}/line_items/{id}"
		}
		return "/jobs/{job_id}/line_items"
	case strings.Contains(lower, "job attachment"):
		return "/jobs/{job_id}/attachments"
	case strings.Contains(lower, "job appointment"):
		if strings.Contains(lower, "delete") || strings.Contains(lower, "update") {
			return "/jobs/{job_id}/appointments/{appointment_id}"
		}
		return "/jobs/{job_id}/appointments"
	case strings.Contains(lower, "job schedule"):
		return "/jobs/{job_id}/schedule"
	case strings.Contains(lower, "job dispatch") || strings.Contains(lower, "dispatch job"):
		return "/jobs/{job_id}/dispatch"
	case strings.Contains(lower, "job invoice"):
		return "/jobs/{job_id}/invoices"
	case strings.Contains(lower, "job input material"):
		if strings.Contains(lower, "bulk") || strings.Contains(lower, "update") {
			return "/jobs/{job_id}/job_input_materials/bulk_update"
		}
		return "/jobs/{job_id}/job_input_materials"
	case strings.Contains(lower, "job note"):
		if strings.Contains(lower, "delete") {
			return "/jobs/{job_id}/notes/{note_id}"
		}
		return "/jobs/{job_id}/notes"
	case strings.Contains(lower, "job tag"):
		if strings.Contains(lower, "delete") || strings.Contains(lower, "remove") {
			return "/jobs/{job_id}/tags/{tag_id}"
		}
		return "/jobs/{job_id}/tags"
	case strings.Contains(lower, "job link"):
		return "/jobs/{job_id}/links"
	case strings.Contains(lower, "lock jobs"):
		return "/jobs/lock"
	case strings.Contains(lower, "job lock") || strings.Contains(lower, "lock job"):
		return "/jobs/{job_id}/lock"
	case strings.Contains(lower, "estimate option attachment"):
		return "/estimates/{estimate_id}/options/{option_id}/attachments"
	case strings.Contains(lower, "estimate option line item"):
		if strings.Contains(lower, "bulk") || strings.Contains(lower, "update") {
			return "/estimates/{estimate_id}/options/{option_id}/line_items/bulk_update"
		}
		return "/estimates/{estimate_id}/options/{option_id}/line_items"
	case strings.Contains(lower, "estimate option schedule"):
		return "/estimates/{estimate_id}/options/{option_id}/schedule"
	case strings.Contains(lower, "estimate option note"):
		if strings.Contains(lower, "delete") {
			return "/estimates/{estimate_id}/options/{option_id}/notes/{note_id}"
		}
		return "/estimates/{estimate_id}/options/{option_id}/notes"
	case strings.Contains(lower, "estimate option link"):
		return "/estimates/{estimate_id}/options/{option_id}/links"
	case strings.Contains(lower, "estimate option"):
		return "/estimates/{estimate_id}/options"
	case strings.Contains(lower, "lead line item"):
		return "/leads/{lead_id}/line_items"
	case strings.Contains(lower, "convert lead") || strings.Contains(lower, "lead conversion"):
		return "/leads/{id}/convert"
	case strings.Contains(lower, "invoice preview"):
		return "/api/invoices/{uuid}/preview"
	case strings.Contains(lower, "invoice uuid") || strings.Contains(lower, "invoice by uuid"):
		return "/api/invoices/{uuid}"
	}
	return ""
}

func apiPathRules() []apiPathRule {
	return []apiPathRule{
		{[]string{"application", "enable"}, "/application/enable"},
		{[]string{"application", "disable"}, "/application/disable"},
		{[]string{"application"}, "/application"},
		{[]string{"checklist"}, "/checklists"},
		{[]string{"webhook"}, "/webhooks/subscription"},
		{[]string{"franchise"}, "/company/franchise_info"},
		{[]string{"booking window"}, "/company/schedule_availability/booking_windows"},
		{[]string{"schedule availability"}, "/company/schedule_availability"},
		{[]string{"event", "id"}, "/events/{event_id}"},
		{[]string{"event"}, "/events"},
		{[]string{"lead source"}, "/lead_sources"},
		{[]string{"job type"}, "/job_fields/job_types"},
		{[]string{"material category"}, "/api/price_book/material_categories"},
		{[]string{"material"}, "/api/price_book/materials"},
		{[]string{"price form"}, "/api/price_book/price_forms"},
		{[]string{"price book service"}, "/api/price_book/services"},
		{[]string{"service zone"}, "/service_zones"},
		{[]string{"route"}, "/routes"},
		{[]string{"pipeline status"}, "/pipeline/statuses"},
	}
}

func containsResource(lower string, resource apiResource) bool {
	terms := []string{resource.Name, resource.Singular}
	for _, term := range terms {
		if term != "" && regexp.MustCompile(`\b`+regexp.QuoteMeta(term)+`\b`).MatchString(lower) {
			return true
		}
	}
	return false
}

func firstPathToken(request string) string {
	for _, token := range strings.Fields(request) {
		token = strings.Trim(token, ".,;:()[]{}\"'")
		if strings.HasPrefix(token, "/") {
			return normalizePath(token)
		}
	}
	return ""
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func idFromWords(lower string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`\bid\s+([a-z0-9][a-z0-9_\-]+)`),
		regexp.MustCompile(`\bfor\s+([a-z0-9][a-z0-9_\-]+)`),
	}
	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(lower)
		if len(matches) == 2 {
			return strings.Trim(matches[1], ".,;:")
		}
	}
	return ""
}

func addPaginationFromWords(query map[string]any, request string) {
	lower := strings.ToLower(request)
	if _, ok := query["page_size"]; !ok {
		if value := numberAfter(lower, "limit"); value > 0 {
			query["page_size"] = fmt.Sprintf("%d", value)
		}
	}
	if _, ok := query["page"]; !ok {
		if value := numberAfter(lower, "page"); value > 0 {
			query["page"] = fmt.Sprintf("%d", value)
		}
	}
}

func numberAfter(lower string, word string) int {
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(word) + `\s+([0-9]+)\b`)
	matches := pattern.FindStringSubmatch(lower)
	if len(matches) != 2 {
		return 0
	}
	var value int
	_, _ = fmt.Sscanf(matches[1], "%d", &value)
	return value
}

func valuesFromPlan(query map[string]any) url.Values {
	values := url.Values{}
	for key, value := range query {
		switch typed := value.(type) {
		case []string:
			for _, item := range typed {
				values.Add(key, item)
			}
		case string:
			values.Set(key, typed)
		default:
			values.Set(key, fmt.Sprint(typed))
		}
	}
	return values
}

func validateMethod(method string) error {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
		return nil
	default:
		return fmt.Errorf("unsupported method %q; use GET, POST, PATCH, PUT, or DELETE", method)
	}
}

func prettyPrintRawJSON(w interface{ Write([]byte) (int, error) }, raw json.RawMessage) error {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		_, writeErr := w.Write(append(raw, '\n'))
		return writeErr
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}
