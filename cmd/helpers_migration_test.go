package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
)

func TestTriggerAndWaitForMigrationWithClient_CompletesAfterTriggerTimeout(t *testing.T) {
	triggerCalls := 0
	pingCalls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/proceed-db-migration":
			triggerCalls++
			time.Sleep(25 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		case "/ping":
			pingCalls++
			if pingCalls == 1 {
				w.WriteHeader(http.StatusPreconditionRequired)
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := &devlake.Client{
		BaseURL: srv.URL,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Millisecond,
		},
	}

	err := triggerAndWaitForMigrationWithClient(srv.URL, client, 1, time.Millisecond, 3, time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if triggerCalls != 1 {
		t.Fatalf("trigger calls = %d, want 1", triggerCalls)
	}
	if pingCalls != 2 {
		t.Fatalf("ping calls = %d, want 2", pingCalls)
	}
}

func TestTriggerAndWaitForMigrationWithClient_RetriesBeforeWaiting(t *testing.T) {
	triggerCalls := 0
	pingCalls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/proceed-db-migration":
			triggerCalls++
			if triggerCalls == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
		case "/ping":
			pingCalls++
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := devlake.NewClient(srv.URL)

	err := triggerAndWaitForMigrationWithClient(srv.URL, client, 2, time.Millisecond, 2, time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if triggerCalls != 2 {
		t.Fatalf("trigger calls = %d, want 2", triggerCalls)
	}
	if pingCalls != 1 {
		t.Fatalf("ping calls = %d, want 1", pingCalls)
	}
}
