package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStoreMigratesAndUpsertsCustomer(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "hcp.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	customer := map[string]any{
		"id":            "cus_123",
		"first_name":    "Ada",
		"last_name":     "Lovelace",
		"email":         "ada@example.com",
		"mobile_number": "555-0100",
		"created_at":    "2026-05-01T10:00:00Z",
		"updated_at":    "2026-05-02T10:00:00Z",
	}
	result, err := db.Upsert(ctx, "customers", customer)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Inserted {
		t.Fatal("first upsert should insert")
	}

	customer["email"] = "ada.updated@example.com"
	result, err = db.Upsert(ctx, "customers", customer)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Updated {
		t.Fatal("second upsert should update")
	}

	count, err := db.Count(ctx, "customers")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("customer count = %d, want 1", count)
	}
}

func TestStoreListReturnsRawRowsInRecentOrder(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "hcp.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Upsert(ctx, "customers", map[string]any{
		"id":         "cus_old",
		"first_name": "Old",
		"updated_at": "2026-05-01T10:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Upsert(ctx, "customers", map[string]any{
		"id":         "cus_new",
		"first_name": "New",
		"updated_at": "2026-05-02T10:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	rows, err := db.List(ctx, "customers", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if got, want := rows[0]["id"], "cus_new"; got != want {
		t.Fatalf("first id = %v, want %s", got, want)
	}
}
