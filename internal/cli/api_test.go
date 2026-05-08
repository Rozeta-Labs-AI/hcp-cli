package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildAPIPlanInfersListCustomers(t *testing.T) {
	plan, err := buildAPIPlan("list customers limit 10 page 2", "", "", "", nil, nil, nil, "", "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Method != "GET" {
		t.Fatalf("method = %q, want GET", plan.Method)
	}
	if plan.Path != "/customers" {
		t.Fatalf("path = %q, want /customers", plan.Path)
	}
	if got, want := plan.Query["page_size"], "10"; got != want {
		t.Fatalf("page_size = %v, want %s", got, want)
	}
	if got, want := plan.Query["page"], "2"; got != want {
		t.Fatalf("page = %v, want %s", got, want)
	}
	if plan.Mutable {
		t.Fatal("GET plan should not be mutable")
	}
}

func TestBuildAPIPlanUsesExplicitPathForAnyEndpoint(t *testing.T) {
	plan, err := buildAPIPlan("approve estimate option", "POST", "/estimates/options/approve", `{"option_id":"opt_123"}`, nil, nil, nil, "", "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Method != "POST" {
		t.Fatalf("method = %q, want POST", plan.Method)
	}
	if plan.Path != "/estimates/options/approve" {
		t.Fatalf("path = %q, want /estimates/options/approve", plan.Path)
	}
	if !plan.Mutable {
		t.Fatal("POST plan should be mutable")
	}
	if plan.Risk != "mutating" {
		t.Fatalf("risk = %q, want mutating", plan.Risk)
	}
	body, ok := plan.Body.(map[string]any)
	if !ok {
		t.Fatalf("body type = %T, want map", plan.Body)
	}
	if got, want := body["option_id"], "opt_123"; got != want {
		t.Fatalf("option_id = %v, want %s", got, want)
	}
}

func TestBuildAPIPlanMarksDestructiveActions(t *testing.T) {
	plan, err := buildAPIPlan("delete /api/price_book/price_forms/form_123", "", "", "", nil, nil, nil, "", "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Method != "DELETE" {
		t.Fatalf("method = %q, want DELETE", plan.Method)
	}
	if plan.Risk != "destructive" {
		t.Fatalf("risk = %q, want destructive", plan.Risk)
	}
	if got, want := destructiveConfirmToken(plan), "delete:/api/price_book/price_forms/form_123"; got != want {
		t.Fatalf("confirm token = %q, want %q", got, want)
	}
}

func TestBuildAPIPlanMarksOperationalWrites(t *testing.T) {
	plan, err := buildAPIPlan("update schedule availability", "PUT", "/company/schedule_availability", `{"daily_schedule_windows":[]}`, nil, nil, nil, "", "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := plan.Risk, "operational"; got != want {
		t.Fatalf("risk = %q, want %q", got, want)
	}
	if !requiresConfirm(plan.Risk) {
		t.Fatal("operational writes should require confirmation")
	}
}

func TestAddVerificationPlan(t *testing.T) {
	plan, err := buildAPIPlan("update schedule availability", "PUT", "/company/schedule_availability", `{"daily_schedule_windows":[]}`, nil, nil, nil, "", "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := addVerificationPlan(&plan, "/company/schedule_availability", []string{"page_size=1"}, []string{"09:00", "17:00"}); err != nil {
		t.Fatal(err)
	}
	if plan.Verification == nil {
		t.Fatal("verification plan missing")
	}
	if got, want := plan.Verification.Path, "/company/schedule_availability"; got != want {
		t.Fatalf("verify path = %q, want %q", got, want)
	}
	if got, want := plan.Verification.Query["page_size"], "1"; got != want {
		t.Fatalf("verify query = %v, want %s", got, want)
	}
	if got, want := len(plan.Verification.Contains), 2; got != want {
		t.Fatalf("verify contains = %d, want %d", got, want)
	}
}

func TestBuildAPIPlanRejectsUnknownNaturalPath(t *testing.T) {
	_, err := buildAPIPlan("do the thing", "", "", "", nil, nil, nil, "", "", 0, 0)
	if err == nil {
		t.Fatal("expected path inference error")
	}
}

func TestBuildAPIPlanSubstitutesNaturalPathParams(t *testing.T) {
	plan, err := buildAPIPlan("create job appointment", "", "", `{"start_time":"2026-05-09T09:00:00Z"}`, nil, []string{"job_id=job_123"}, nil, "", "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := plan.Path, "/jobs/job_123/appointments"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	if plan.Method != "POST" {
		t.Fatalf("method = %q, want POST", plan.Method)
	}
}

func TestBuildAPIPlanReportsMissingNaturalPathParams(t *testing.T) {
	_, err := buildAPIPlan("list job appointments", "", "", "", nil, nil, nil, "", "", 0, 0)
	if err == nil {
		t.Fatal("expected missing param error")
	}
	if !strings.Contains(err.Error(), "job_id") {
		t.Fatalf("error = %v, want missing job_id", err)
	}
}

func TestBuildAPIPlanIncludesMultipartFiles(t *testing.T) {
	plan, err := buildAPIPlan("add job attachment", "", "", `{"description":"before photo"}`, nil, []string{"job_id=job_123"}, []string{"/tmp/photo.jpg"}, "attachment", "image/jpeg", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := plan.Path, "/jobs/job_123/attachments"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	if len(plan.Files) != 1 {
		t.Fatalf("files = %d, want 1", len(plan.Files))
	}
	if got, want := plan.Files[0].FieldName, "attachment"; got != want {
		t.Fatalf("field = %q, want %q", got, want)
	}
}

func TestBuildAPIPlanCoversEndpointFamilies(t *testing.T) {
	cases := []struct {
		request string
		params  []string
		path    string
	}{
		{"enable application", nil, "/application/enable"},
		{"list checklists", nil, "/checklists"},
		{"list customer addresses", []string{"customer_id=cus_123"}, "/customers/cus_123/addresses"},
		{"list employees", nil, "/employees"},
		{"bulk update job line items", []string{"job_id=job_123"}, "/jobs/job_123/line_items/bulk_update"},
		{"dispatch job", []string{"job_id=job_123"}, "/jobs/job_123/dispatch"},
		{"create webhook", nil, "/webhooks/subscription"},
		{"create estimate option note", []string{"estimate_id=est_123", "option_id=opt_123"}, "/estimates/est_123/options/opt_123/notes"},
		{"get booking windows", nil, "/company/schedule_availability/booking_windows"},
		{"get event id", []string{"event_id=evt_123"}, "/events/evt_123"},
		{"create tag", nil, "/tags"},
		{"convert lead", []string{"id=lead_123"}, "/leads/lead_123/convert"},
		{"list lead line items", []string{"lead_id=lead_123"}, "/leads/lead_123/line_items"},
		{"create lead source", nil, "/lead_sources"},
		{"create job type", nil, "/job_fields/job_types"},
		{"invoice preview", []string{"uuid=inv_123"}, "/api/invoices/inv_123/preview"},
		{"delete price form", nil, "/api/price_book/price_forms"},
		{"list service zones", nil, "/service_zones"},
		{"list routes", nil, "/routes"},
		{"update pipeline status", nil, "/pipeline/statuses"},
	}
	for _, tc := range cases {
		plan, err := buildAPIPlan(tc.request, "", "", "", nil, tc.params, nil, "", "", 0, 0)
		if err != nil {
			t.Fatalf("%q returned error: %v", tc.request, err)
		}
		if plan.Path != tc.path {
			t.Fatalf("%q path = %q, want %q", tc.request, plan.Path, tc.path)
		}
	}
}

func TestParseOpenAPICatalogFromSnapshot(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "HOUSECALL_PRO_OPENAPI_SNAPSHOT.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	ops, err := parseOpenAPICatalog(file)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(ops), 95; got != want {
		t.Fatalf("operation count = %d, want %d", got, want)
	}
	assertCatalogHas(t, ops, "GET", "/customers")
	assertCatalogHas(t, ops, "POST", "/jobs/{job_id}/appointments")
	assertCatalogHas(t, ops, "DELETE", "/api/price_book/materials/{uuid}")
}

func assertCatalogHas(t *testing.T, ops []apiCatalogOperation, method string, path string) {
	t.Helper()
	for _, op := range ops {
		if op.Method == method && op.Path == path {
			if strings.TrimSpace(op.Summary) == "" {
				t.Fatalf("%s %s has empty summary", method, path)
			}
			return
		}
	}
	t.Fatalf("catalog missing %s %s", method, path)
}

func TestAPIExamplesCoverMajorFamilies(t *testing.T) {
	examples := apiExamples()
	if len(examples) < 50 {
		t.Fatalf("examples = %d, want at least 50", len(examples))
	}
	for _, area := range []string{"customers", "jobs", "estimates", "leads", "invoices", "price_book", "pipeline", "webhooks"} {
		if got := filterExamples(examples, area); len(got) == 0 {
			t.Fatalf("missing examples for area %s", area)
		}
	}
}
