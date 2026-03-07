package htmlbucket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientUploadSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method: %s", r.Method)
		}
		if r.URL.Path != "/v1/upload" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		if got := r.Header.Get("X-API-Key"); got != "k-123" {
			t.Fatalf("api key header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"url":"https://abc123.htmlbucket.com"}`))
	}))
	defer srv.Close()

	client := NewClientWithBaseURL("k-123", srv.URL, srv.Client())
	url, err := client.Upload(context.Background(), "<h1>Hello</h1>")
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if url != "https://abc123.htmlbucket.com" {
		t.Fatalf("unexpected url: %q", url)
	}
}

func TestClientUploadMissingURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := NewClientWithBaseURL("k-123", srv.URL, srv.Client())
	if _, err := client.Upload(context.Background(), "<h1>Hello</h1>"); err == nil {
		t.Fatalf("expected missing-url error")
	}
}

func TestClientUploadNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	client := NewClientWithBaseURL("k-123", srv.URL, srv.Client())
	_, err := client.Upload(context.Background(), "<h1>Hello</h1>")
	if err == nil {
		t.Fatalf("expected upload error")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Fatalf("expected status in error, got %q", err.Error())
	}
}
