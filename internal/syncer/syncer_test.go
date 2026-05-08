package syncer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Rozeta-Labs-AI/hcp-cli/internal/hcp"
	"github.com/Rozeta-Labs-AI/hcp-cli/internal/store"
)

func TestRunSyncsCustomersIntoStore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/customers" {
			t.Fatalf("path = %s, want /customers", r.URL.Path)
		}
		if got, want := r.URL.Query().Get("page_size"), "100"; got != want {
			t.Fatalf("page_size = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"customers": [
				{"id":"cus_123","first_name":"Ada","last_name":"Lovelace","email":"ada@example.com"}
			],
			"page": 1,
			"page_size": 100,
			"total_pages": 1
		}`))
	}))
	defer server.Close()

	ctx := context.Background()
	client, err := hcp.New(hcp.Options{BaseURL: server.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}

	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "hcp.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	summary, err := Run(ctx, client, db, Options{
		Resources: []string{"customers"},
		PageSize:  100,
		MaxPages:  2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Resources) != 1 {
		t.Fatalf("resource summaries = %d, want 1", len(summary.Resources))
	}
	if summary.Resources[0].Rows != 1 {
		t.Fatalf("synced rows = %d, want 1", summary.Resources[0].Rows)
	}

	count, err := db.Count(ctx, "customers")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("customer count = %d, want 1", count)
	}
}

func TestRunAddsDefaultResourceQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pipeline/statuses" {
			t.Fatalf("path = %s, want /pipeline/statuses", r.URL.Path)
		}
		if got, want := r.URL.Query().Get("resource_type"), "lead"; got != want {
			t.Fatalf("resource_type = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"statuses": [
				{"id":"kcs_123","name":"New Lead","status_type":"lead_new_lead"}
			],
			"page": 1,
			"page_size": 100,
			"total_pages": 1
		}`))
	}))
	defer server.Close()

	ctx := context.Background()
	client, err := hcp.New(hcp.Options{BaseURL: server.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}

	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "hcp.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	summary, err := Run(ctx, client, db, Options{
		Resources: []string{"pipeline_statuses"},
		PageSize:  100,
		MaxPages:  2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := summary.Resources[0].Rows, 1; got != want {
		t.Fatalf("synced rows = %d, want %d", got, want)
	}
}
