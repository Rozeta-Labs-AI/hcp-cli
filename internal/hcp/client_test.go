package hcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestClientUsesTokenAuthAndCompanyHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Token test-key"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("X-Company-Id"), "company-123"; got != want {
			t.Fatalf("X-Company-Id = %q, want %q", got, want)
		}
		if got, want := r.URL.Query().Get("page"), "2"; got != want {
			t.Fatalf("page = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client, err := New(Options{
		BaseURL:   server.URL,
		APIKey:    "test-key",
		AuthMode:  "api_key",
		CompanyID: "company-123",
	})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := client.GetRaw(context.Background(), "/company", url.Values{"page": []string{"2"}})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"ok":true}` {
		t.Fatalf("raw = %s", raw)
	}
}

func TestClientUsesBearerAuthForOAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Bearer oauth-token"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client, err := New(Options{
		BaseURL:  server.URL,
		APIKey:   "oauth-token",
		AuthMode: "oauth",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.GetRaw(context.Background(), "/application", nil); err != nil {
		t.Fatal(err)
	}
}
