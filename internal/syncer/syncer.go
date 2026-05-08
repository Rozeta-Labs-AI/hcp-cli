package syncer

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/Rozeta-Labs-AI/hcp-cli/internal/hcp"
	"github.com/Rozeta-Labs-AI/hcp-cli/internal/store"
)

type Resource struct {
	Name           string
	Singular       string
	Endpoint       string
	ParentResource string
	ParentParam    string
	DefaultQuery   url.Values
}

type Options struct {
	Resources   []string
	PageSize    int
	MaxPages    int
	LocationIDs []string
	Since       time.Time
}

type ResourceSummary struct {
	Resource string `json:"resource"`
	Pages    int    `json:"pages"`
	Rows     int    `json:"rows"`
	Inserted int    `json:"inserted"`
	Updated  int    `json:"updated"`
}

type Summary struct {
	DBPath    string            `json:"db_path"`
	Resources []ResourceSummary `json:"resources"`
}

var defaultResources = []Resource{
	{Name: "companies", Singular: "company", Endpoint: "/company"},
	{Name: "checklists", Singular: "checklist", Endpoint: "/checklists"},
	{Name: "employees", Singular: "employee", Endpoint: "/employees"},
	{Name: "customers", Singular: "customer", Endpoint: "/customers"},
	{Name: "customer_addresses", Singular: "addresses", Endpoint: "/customers/{customer_id}/addresses", ParentResource: "customers", ParentParam: "customer_id"},
	{Name: "lead_sources", Singular: "lead_source", Endpoint: "/lead_sources"},
	{Name: "leads", Singular: "lead", Endpoint: "/leads"},
	{Name: "lead_line_items", Singular: "line_items", Endpoint: "/leads/{lead_id}/line_items", ParentResource: "leads", ParentParam: "lead_id"},
	{Name: "jobs", Singular: "job", Endpoint: "/jobs"},
	{Name: "appointments", Singular: "appointment", Endpoint: "/jobs/{job_id}/appointments"},
	{Name: "job_line_items", Singular: "line_items", Endpoint: "/jobs/{job_id}/line_items", ParentResource: "jobs", ParentParam: "job_id"},
	{Name: "job_invoices", Singular: "invoices", Endpoint: "/jobs/{job_id}/invoices", ParentResource: "jobs", ParentParam: "job_id"},
	{Name: "job_input_materials", Singular: "job_input_material", Endpoint: "/jobs/{job_id}/job_input_materials", ParentResource: "jobs", ParentParam: "job_id"},
	{Name: "estimates", Singular: "estimate", Endpoint: "/estimates"},
	{Name: "estimate_option_line_items", Singular: "line_items", Endpoint: "/estimates/{estimate_id}/options/{option_id}/line_items", ParentResource: "estimate_options", ParentParam: "estimate_option_line_item"},
	{Name: "invoices", Singular: "invoice", Endpoint: "/invoices"},
	{Name: "tags", Singular: "tag", Endpoint: "/tags"},
	{Name: "events", Singular: "event", Endpoint: "/events"},
	{Name: "routes", Singular: "route", Endpoint: "/routes"},
	{Name: "service_zones", Singular: "service_zone", Endpoint: "/service_zones"},
	{Name: "pipeline_statuses", Singular: "statuses", Endpoint: "/pipeline/statuses", DefaultQuery: url.Values{"resource_type": []string{"lead"}}},
	{Name: "price_book_materials", Singular: "material", Endpoint: "/api/price_book/materials"},
	{Name: "price_book_material_categories", Singular: "material_category", Endpoint: "/api/price_book/material_categories"},
	{Name: "price_book_price_forms", Singular: "price_form", Endpoint: "/api/price_book/price_forms"},
	{Name: "price_book_services", Singular: "service", Endpoint: "/api/price_book/services"},
}

func DefaultResources() []Resource {
	out := make([]Resource, len(defaultResources))
	copy(out, defaultResources)
	return out
}

func Run(ctx context.Context, client *hcp.Client, db *store.Store, opts Options) (Summary, error) {
	if opts.PageSize <= 0 {
		opts.PageSize = 100
	}
	if opts.MaxPages <= 0 {
		opts.MaxPages = 10
	}

	resources, err := selectResources(opts.Resources)
	if err != nil {
		return Summary{}, err
	}

	summary := Summary{
		DBPath:    db.Path(),
		Resources: make([]ResourceSummary, 0, len(resources)),
	}

	for _, resource := range resources {
		result, err := syncResource(ctx, client, db, resource, opts)
		if err != nil {
			return summary, err
		}
		summary.Resources = append(summary.Resources, result)
	}

	return summary, nil
}

func syncResource(ctx context.Context, client *hcp.Client, db *store.Store, resource Resource, opts Options) (ResourceSummary, error) {
	if resource.Name == "appointments" {
		return syncAppointments(ctx, client, db, resource, opts)
	}
	if resource.ParentResource != "" {
		return syncNestedResource(ctx, client, db, resource, opts)
	}

	runID, err := db.BeginSyncRun(ctx, resource.Name)
	if err != nil {
		return ResourceSummary{}, err
	}

	result := ResourceSummary{Resource: resource.Name}
	var syncErr error
	defer func() {
		status := "success"
		if syncErr != nil {
			status = "failed"
		}
		_ = db.FinishSyncRun(ctx, runID, status, result.Pages, result.Rows, syncErr)
	}()

	for page := 1; page <= opts.MaxPages; page++ {
		req := hcp.PageRequest{Page: page, PageSize: opts.PageSize, LocationIDs: opts.LocationIDs}
		if resource.DefaultQuery != nil {
			req.Extra = resource.DefaultQuery
		}
		if !opts.Since.IsZero() {
			req.Since = opts.Since.Format(time.RFC3339)
		}
		if resource.Name == "jobs" {
			req.Expand = []string{"appointments"}
		}
		collection, err := client.ListResource(ctx, resource.Name, resource.Singular, resource.Endpoint, req)
		if err != nil {
			syncErr = fmt.Errorf("sync %s page %d: %w", resource.Name, page, err)
			return result, syncErr
		}
		items := collection.Items
		if len(items) == 0 {
			break
		}

		result.Pages++
		for _, item := range items {
			item = ensureItemID(item, resource.Name)
			upserted, err := db.Upsert(ctx, resource.Name, item)
			if err != nil {
				syncErr = fmt.Errorf("sync %s page %d: %w", resource.Name, page, err)
				return result, syncErr
			}
			result.Rows++
			if upserted.Inserted {
				result.Inserted++
			} else {
				result.Updated++
			}
			if resource.Name == "jobs" {
				for _, appointment := range nestedArray(item, "appointments") {
					appointment["job_id"] = store.TextValue(item, "id", "uuid")
					appointment["job_number"] = store.TextValue(item, "number")
					if upserted, err := db.Upsert(ctx, "appointments", appointment); err != nil {
						syncErr = fmt.Errorf("sync embedded appointments for job page %d: %w", page, err)
						return result, syncErr
					} else if upserted.Inserted {
						result.Inserted++
					} else {
						result.Updated++
					}
				}
			}
			if resource.Name == "estimates" {
				for _, option := range nestedArray(item, "options") {
					option["estimate_id"] = store.TextValue(item, "id", "uuid")
					if upserted, err := db.Upsert(ctx, "estimate_options", ensureItemID(option, "estimate_option")); err != nil {
						syncErr = fmt.Errorf("sync embedded estimate options page %d: %w", page, err)
						return result, syncErr
					} else if upserted.Inserted {
						result.Inserted++
					} else {
						result.Updated++
					}
				}
			}
		}

		if collection.TotalPages > 0 && page >= collection.TotalPages {
			break
		}
		if len(items) < opts.PageSize {
			break
		}
	}

	return result, nil
}

func replaceParentParam(endpoint string, param string, parentID string, parent map[string]any) string {
	if param == "estimate_option_line_item" {
		estimateID := store.TextValue(parent, "estimate_id")
		optionID := store.TextValue(parent, "id", "uuid")
		if estimateID == "" || optionID == "" {
			return ""
		}
		endpoint = strings.ReplaceAll(endpoint, "{estimate_id}", url.PathEscape(estimateID))
		return strings.ReplaceAll(endpoint, "{option_id}", url.PathEscape(optionID))
	}
	return strings.ReplaceAll(endpoint, "{"+param+"}", url.PathEscape(parentID))
}

func ensureItemID(item map[string]any, prefix string) map[string]any {
	if store.TextValue(item, "id", "uuid", "number") != "" {
		return item
	}
	if parent := store.TextValue(item, "parent_id", "job_id", "customer_id", "lead_id", "estimate_id"); parent != "" {
		item["id"] = prefix + "_" + parent + "_" + stableSuffix(item)
		return item
	}
	item["id"] = prefix + "_" + stableSuffix(item)
	return item
}

func stableSuffix(item map[string]any) string {
	for _, key := range []string{"name", "title", "created_at", "updated_at", "description"} {
		if value := store.TextValue(item, key); value != "" {
			return sanitizeID(value)
		}
	}
	return fmt.Sprintf("%d", len(item))
}

func sanitizeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "_", "/", "_", ":", "_", ".", "_")
	value = replacer.Replace(value)
	if value == "" {
		return "item"
	}
	return value
}

func syncNestedResource(ctx context.Context, client *hcp.Client, db *store.Store, resource Resource, opts Options) (ResourceSummary, error) {
	runID, err := db.BeginSyncRun(ctx, resource.Name)
	if err != nil {
		return ResourceSummary{}, err
	}
	result := ResourceSummary{Resource: resource.Name}
	var syncErr error
	defer func() {
		status := "success"
		if syncErr != nil {
			status = "failed"
		}
		_ = db.FinishSyncRun(ctx, runID, status, result.Pages, result.Rows, syncErr)
	}()

	parents, err := db.List(ctx, resource.ParentResource, opts.PageSize*opts.MaxPages)
	if err != nil {
		syncErr = fmt.Errorf("read %s for %s sync: %w", resource.ParentResource, resource.Name, err)
		return result, syncErr
	}
	for _, parent := range parents {
		parentID := store.TextValue(parent, "id", "uuid")
		if parentID == "" {
			continue
		}
		endpoint := replaceParentParam(resource.Endpoint, resource.ParentParam, parentID, parent)
		if endpoint == "" {
			continue
		}
		page, err := client.ListResource(ctx, resource.Name, resource.Singular, endpoint, hcp.PageRequest{Page: 1, PageSize: opts.PageSize})
		if err != nil {
			syncErr = fmt.Errorf("sync %s for parent %s: %w", resource.Name, parentID, err)
			return result, syncErr
		}
		result.Pages++
		for _, item := range page.Items {
			item["parent_id"] = parentID
			item[resource.ParentParam] = parentID
			upserted, err := db.Upsert(ctx, resource.Name, ensureItemID(item, resource.Name))
			if err != nil {
				syncErr = fmt.Errorf("sync %s for parent %s: %w", resource.Name, parentID, err)
				return result, syncErr
			}
			result.Rows++
			if upserted.Inserted {
				result.Inserted++
			} else {
				result.Updated++
			}
		}
	}
	return result, nil
}

func syncAppointments(ctx context.Context, client *hcp.Client, db *store.Store, resource Resource, opts Options) (ResourceSummary, error) {
	runID, err := db.BeginSyncRun(ctx, resource.Name)
	if err != nil {
		return ResourceSummary{}, err
	}
	result := ResourceSummary{Resource: resource.Name}
	var syncErr error
	defer func() {
		status := "success"
		if syncErr != nil {
			status = "failed"
		}
		_ = db.FinishSyncRun(ctx, runID, status, result.Pages, result.Rows, syncErr)
	}()

	jobs, err := db.List(ctx, "jobs", opts.PageSize*opts.MaxPages)
	if err != nil {
		syncErr = fmt.Errorf("read jobs for appointment sync: %w", err)
		return result, syncErr
	}
	for _, job := range jobs {
		jobID := store.TextValue(job, "id", "uuid")
		if jobID == "" {
			continue
		}
		page, err := client.ListJobAppointments(ctx, jobID, hcp.PageRequest{Page: 1, PageSize: opts.PageSize})
		if err != nil {
			syncErr = fmt.Errorf("sync appointments for job %s: %w", jobID, err)
			return result, syncErr
		}
		result.Pages++
		for _, item := range page.Items {
			item["job_id"] = jobID
			item["job_number"] = store.TextValue(job, "number")
			upserted, err := db.Upsert(ctx, "appointments", item)
			if err != nil {
				syncErr = fmt.Errorf("sync appointments for job %s: %w", jobID, err)
				return result, syncErr
			}
			result.Rows++
			if upserted.Inserted {
				result.Inserted++
			} else {
				result.Updated++
			}
		}
	}
	return result, nil
}

func selectResources(names []string) ([]Resource, error) {
	if len(names) == 0 {
		return DefaultResources(), nil
	}

	byName := map[string]Resource{}
	for _, resource := range defaultResources {
		byName[resource.Name] = resource
	}

	selected := make([]Resource, 0, len(names))
	for _, name := range names {
		resource, ok := byName[name]
		if !ok {
			valid := make([]string, 0, len(byName))
			for key := range byName {
				valid = append(valid, key)
			}
			sort.Strings(valid)
			return nil, fmt.Errorf("unknown sync resource %q; valid resources: %v", name, valid)
		}
		selected = append(selected, resource)
	}
	return selected, nil
}

func nestedArray(item map[string]any, key string) []map[string]any {
	values, ok := item[key].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if object, ok := value.(map[string]any); ok {
			out = append(out, object)
		}
	}
	return out
}
