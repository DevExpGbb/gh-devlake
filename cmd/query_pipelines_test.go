package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/DevExpGBB/gh-devlake/internal/query"
)

func TestQueryPipelines_InvalidFormat(t *testing.T) {
	queryPipelinesFormat = "invalid"
	t.Cleanup(func() { queryPipelinesFormat = "json" })

	err := runQueryPipelines(nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid --format, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --format value") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestQueryPipelines_JSONOutputNoBanner(t *testing.T) {
	// Mock DevLake API
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/pipelines" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(devlake.PipelineListResponse{
				Pipelines: []devlake.Pipeline{
					{
						ID:            123,
						Status:        "TASK_COMPLETED",
						FinishedTasks: 10,
						TotalTasks:    10,
					},
				},
				Count: 1,
			})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	// Set URL to mock server
	origURL := cfgURL
	cfgURL = srv.URL
	t.Cleanup(func() { cfgURL = origURL })

	// Set format to JSON
	origFormat := queryPipelinesFormat
	queryPipelinesFormat = "json"
	t.Cleanup(func() { queryPipelinesFormat = origFormat })

	// Capture stdout
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig })

	// Run the command
	if err := runQueryPipelines(nil, nil); err != nil {
		t.Fatalf("runQueryPipelines returned error: %v", err)
	}

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	// Verify no discovery banners in output
	if strings.Contains(out, "Discovering DevLake") {
		t.Error("JSON output should not contain discovery banner")
	}
	if strings.Contains(out, "🔍") {
		t.Error("JSON output should not contain emoji banners")
	}

	// Verify valid JSON
	trimmed := strings.TrimSpace(out)
	var pipelines []query.PipelineResult
	if err := json.Unmarshal([]byte(trimmed), &pipelines); err != nil {
		t.Fatalf("output is not valid JSON: %v — got: %q", err, out)
	}

	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}
	if pipelines[0].ID != 123 {
		t.Errorf("expected ID=123, got %d", pipelines[0].ID)
	}
	if pipelines[0].Status != "TASK_COMPLETED" {
		t.Errorf("expected status=TASK_COMPLETED, got %q", pipelines[0].Status)
	}
}

func TestQueryPipelines_GlobalJSONFlag(t *testing.T) {
	// Mock DevLake API
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/pipelines" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(devlake.PipelineListResponse{
				Pipelines: []devlake.Pipeline{},
				Count:     0,
			})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	// Set URL to mock server
	origURL := cfgURL
	cfgURL = srv.URL
	t.Cleanup(func() { cfgURL = origURL })

	// Set global JSON flag
	origJSON := outputJSON
	outputJSON = true
	t.Cleanup(func() { outputJSON = origJSON })

	// Set format to table (should be overridden by --json)
	origFormat := queryPipelinesFormat
	queryPipelinesFormat = "table"
	t.Cleanup(func() { queryPipelinesFormat = origFormat })

	// Capture stdout
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig })

	// Run the command
	if err := runQueryPipelines(nil, nil); err != nil {
		t.Fatalf("runQueryPipelines returned error: %v", err)
	}

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	// Verify no discovery banners in output
	if strings.Contains(out, "Discovering DevLake") {
		t.Error("JSON output with --json should not contain discovery banner")
	}

	// Verify valid JSON
	trimmed := strings.TrimSpace(out)
	var pipelines []query.PipelineResult
	if err := json.Unmarshal([]byte(trimmed), &pipelines); err != nil {
		t.Fatalf("output is not valid JSON: %v — got: %q", err, out)
	}
}
