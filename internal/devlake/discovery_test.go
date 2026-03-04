package devlake

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestInferLocalCompanionURLs tests the inferLocalCompanionURLs function.
func TestInferLocalCompanionURLs(t *testing.T) {
	tests := []struct {
		name            string
		backendURL      string
		wantGrafana     string
		wantConfigUI    string
	}{
		{
			name:         "port 8080",
			backendURL:   "http://localhost:8080",
			wantGrafana:  "http://localhost:3002",
			wantConfigUI: "http://localhost:4000",
		},
		{
			name:         "port 8080 with path",
			backendURL:   "http://localhost:8080/api",
			wantGrafana:  "http://localhost:3002",
			wantConfigUI: "http://localhost:4000",
		},
		{
			name:         "port 8085",
			backendURL:   "http://localhost:8085",
			wantGrafana:  "http://localhost:3004",
			wantConfigUI: "http://localhost:4004",
		},
		{
			name:         "port 8085 with path",
			backendURL:   "http://localhost:8085/v1",
			wantGrafana:  "http://localhost:3004",
			wantConfigUI: "http://localhost:4004",
		},
		{
			name:         "unknown port",
			backendURL:   "http://localhost:9999",
			wantGrafana:  "",
			wantConfigUI: "",
		},
		{
			name:         "remote URL",
			backendURL:   "https://devlake.example.com",
			wantGrafana:  "",
			wantConfigUI: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grafana, configUI := inferLocalCompanionURLs(tt.backendURL)
			if grafana != tt.wantGrafana {
				t.Errorf("grafana = %q, want %q", grafana, tt.wantGrafana)
			}
			if configUI != tt.wantConfigUI {
				t.Errorf("configUI = %q, want %q", configUI, tt.wantConfigUI)
			}
		})
	}
}

// TestDiscoverExplicitURL tests discovery with an explicit URL.
func TestDiscoverExplicitURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	result, err := Discover(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.URL != srv.URL {
		t.Errorf("URL = %q, want %q", result.URL, srv.URL)
	}
	if result.Source != "parameter" {
		t.Errorf("Source = %q, want %q", result.Source, "parameter")
	}
}

// TestDiscoverExplicitURLUnreachable tests discovery with unreachable explicit URL.
func TestDiscoverExplicitURLUnreachable(t *testing.T) {
	result, err := Discover("http://127.0.0.1:1")
	if err == nil {
		t.Fatal("expected error for unreachable URL, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

// TestDiscoverFromStateFile tests discovery from state files.
func TestDiscoverFromStateFile(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	// Start mock server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	// Create state file with reachable backend
	state := &State{
		Method: "local",
		Endpoints: StateEndpoints{
			Backend:  srv.URL,
			Grafana:  "http://localhost:3002",
			ConfigUI: "http://localhost:4000",
		},
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal state JSON: %v", err)
	}
	if err := os.WriteFile(".devlake-local.json", data, 0644); err != nil {
		t.Fatalf("failed to write state file: %v", err)
	}

	result, err := Discover("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.URL != srv.URL {
		t.Errorf("URL = %q, want %q", result.URL, srv.URL)
	}
	if result.Source != "statefile" {
		t.Errorf("Source = %q, want %q", result.Source, "statefile")
	}
	if result.GrafanaURL != "http://localhost:3002" {
		t.Errorf("GrafanaURL = %q, want %q", result.GrafanaURL, "http://localhost:3002")
	}
}

// TestTryStateFileUnreachable tests tryStateFile with an unreachable backend.
func TestTryStateFileUnreachable(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".devlake-local.json")

	state := &State{
		Method: "local",
		Endpoints: StateEndpoints{
			Backend: "http://127.0.0.1:1", // Valid but unreachable port
		},
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal state JSON: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write state file: %v", err)
	}

	result := tryStateFile(path)
	if result != nil {
		t.Errorf("expected nil for unreachable backend, got %v", result)
	}
}

// TestTryStateFileNoBackend tests tryStateFile with empty backend URL.
func TestTryStateFileNoBackend(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".devlake-local.json")

	state := &State{
		Method: "local",
		Endpoints: StateEndpoints{
			Backend: "",
		},
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write state file: %v", err)
	}

	result := tryStateFile(path)
	if result != nil {
		t.Errorf("expected nil for empty backend, got %v", result)
	}
}

// TestTryStateFileNonExistent tests tryStateFile with a nonexistent file.
func TestTryStateFileNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent.json")

	result := tryStateFile(path)
	if result != nil {
		t.Errorf("expected nil for nonexistent file, got %v", result)
	}
}

// TestTryStateFileInvalidJSON tests tryStateFile with invalid JSON.
func TestTryStateFileInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".devlake-local.json")

	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatalf("failed to write invalid JSON file: %v", err)
	}

	result := tryStateFile(path)
	if result != nil {
		t.Errorf("expected nil for invalid JSON, got %v", result)
	}
}

// TestDiscoverPriorityOrder tests the discovery priority order.
func TestDiscoverPriorityOrder(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	// Create two mock servers
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ping" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv1.Close()

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ping" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv2.Close()

	// Create state file pointing to srv2
	state := &State{
		Method: "local",
		Endpoints: StateEndpoints{
			Backend: srv2.URL,
		},
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal state JSON: %v", err)
	}
	if err := os.WriteFile(".devlake-local.json", data, 0644); err != nil {
		t.Fatalf("failed to write state file: %v", err)
	}

	// Explicit URL should take priority
	result, err := Discover(srv1.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.URL != srv1.URL {
		t.Errorf("URL = %q, want %q (explicit should win)", result.URL, srv1.URL)
	}
	if result.Source != "parameter" {
		t.Errorf("Source = %q, want %q", result.Source, "parameter")
	}

	// State file should take priority over localhost scanning
	result, err = Discover("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.URL != srv2.URL {
		t.Errorf("URL = %q, want %q (statefile should win)", result.URL, srv2.URL)
	}
	if result.Source != "statefile" {
		t.Errorf("Source = %q, want %q", result.Source, "statefile")
	}
}

// TestDiscoverTrailingSlash tests that trailing slashes are trimmed.
func TestDiscoverTrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	result, err := Discover(srv.URL + "/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.URL != srv.URL {
		t.Errorf("URL = %q, want %q (trailing slash should be trimmed)", result.URL, srv.URL)
	}
}

// TestDiscoverMultipleStateFiles tests priority when both Azure and local state exist.
func TestDiscoverMultipleStateFiles(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	// Create Azure state first (but unreachable)
	azureState := &State{
		Method: "azure",
		Endpoints: StateEndpoints{
			Backend: "http://127.0.0.1:1", // Valid but unreachable port
		},
	}
	azureData, err := json.MarshalIndent(azureState, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal azure state JSON: %v", err)
	}
	if err := os.WriteFile(".devlake-azure.json", azureData, 0644); err != nil {
		t.Fatalf("failed to write azure state file: %v", err)
	}

	// Create local state (reachable)
	localState := &State{
		Method: "local",
		Endpoints: StateEndpoints{
			Backend: srv.URL,
		},
	}
	localData, err := json.MarshalIndent(localState, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal local state JSON: %v", err)
	}
	if err := os.WriteFile(".devlake-local.json", localData, 0644); err != nil {
		t.Fatalf("failed to write local state file: %v", err)
	}

	// Should skip unreachable Azure and use reachable local
	result, err := Discover("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.URL != srv.URL {
		t.Errorf("URL = %q, want %q", result.URL, srv.URL)
	}
	if result.Source != "statefile" {
		t.Errorf("Source = %q, want %q", result.Source, "statefile")
	}
}

// TestPingURLSuccess tests that pingURL succeeds on a 200 OK response.
func TestPingURLSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/ping" {
			t.Errorf("path = %s, want /ping", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := pingURL(srv.URL)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestPingURLNon200Status tests pingURL with non-200 status.
func TestPingURLNon200Status(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/ping" {
			t.Errorf("path = %s, want /ping", r.URL.Path)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	err := pingURL(srv.URL)
	if err == nil {
		t.Error("expected error for non-200 status, got nil")
	}
}
