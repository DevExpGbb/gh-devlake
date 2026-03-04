package devlake

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSaveStateAndLoadState tests the roundtrip behavior.
func TestSaveStateAndLoadState(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".devlake-test.json")

	originalState := &State{
		DeployedAt: time.Now().Format(time.RFC3339),
		Method:     "local",
		Endpoints: StateEndpoints{
			Backend:  "http://localhost:8080",
			Grafana:  "http://localhost:3002",
			ConfigUI: "http://localhost:4000",
		},
		Connections: []StateConnection{
			{
				Plugin:       "github",
				ConnectionID: 1,
				Name:         "my-github",
				Organization: "test-org",
			},
		},
		ConnectionsConfiguredAt: time.Now().Format(time.RFC3339),
		Project: &StateProject{
			Name:        "test-project",
			BlueprintID: 10,
			Repos:       []string{"org/repo1", "org/repo2"},
		},
		ScopesConfiguredAt: time.Now().Format(time.RFC3339),
	}

	// Save
	err := SaveState(path, originalState)
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// Load
	loadedState, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if loadedState == nil {
		t.Fatal("LoadState returned nil")
	}

	// Verify key fields
	if loadedState.Method != originalState.Method {
		t.Errorf("Method = %q, want %q", loadedState.Method, originalState.Method)
	}
	if loadedState.Endpoints.Backend != originalState.Endpoints.Backend {
		t.Errorf("Backend = %q, want %q", loadedState.Endpoints.Backend, originalState.Endpoints.Backend)
	}
	if len(loadedState.Connections) != len(originalState.Connections) {
		t.Fatalf("len(Connections) = %d, want %d", len(loadedState.Connections), len(originalState.Connections))
	}
	if loadedState.Connections[0].Plugin != originalState.Connections[0].Plugin {
		t.Errorf("Plugin = %q, want %q", loadedState.Connections[0].Plugin, originalState.Connections[0].Plugin)
	}
	if loadedState.Project == nil {
		t.Fatal("Project is nil")
	}
	if loadedState.Project.Name != originalState.Project.Name {
		t.Errorf("Project.Name = %q, want %q", loadedState.Project.Name, originalState.Project.Name)
	}
}

// TestSaveStateMerge tests the merge behavior with existing Azure metadata.
func TestSaveStateMerge(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".devlake-azure.json")

	// Write an existing state file with Azure-specific fields
	existingJSON := map[string]any{
		"deployedAt": "2024-01-01T00:00:00Z",
		"method":     "azure",
		"endpoints": map[string]any{
			"backend": "https://devlake.example.com",
			"grafana": "https://grafana.example.com",
		},
		"resourceGroup": "my-rg",
		"location":      "eastus",
		"resources": map[string]any{
			"containerApp": "devlake-app",
			"storage":      "devlakestorage",
		},
	}
	data, err := json.MarshalIndent(existingJSON, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal existing JSON: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write existing state file: %v", err)
	}

	// Create a new state with updated connections
	newState := &State{
		DeployedAt: "2024-01-02T00:00:00Z",
		Method:     "azure",
		Endpoints: StateEndpoints{
			Backend: "https://devlake.example.com",
			Grafana: "https://grafana.example.com",
		},
		Connections: []StateConnection{
			{Plugin: "github", ConnectionID: 1, Name: "conn1"},
		},
		ConnectionsConfiguredAt: "2024-01-02T12:00:00Z",
	}

	// Save should merge without clobbering Azure fields
	err = SaveState(path, newState)
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// Read the raw JSON
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify Azure fields are preserved
	if result["resourceGroup"] != "my-rg" {
		t.Errorf("resourceGroup = %v, want %q", result["resourceGroup"], "my-rg")
	}
	if result["location"] != "eastus" {
		t.Errorf("location = %v, want %q", result["location"], "eastus")
	}
	if result["resources"] == nil {
		t.Error("resources field was clobbered")
	}

	// Verify State fields are updated
	if result["deployedAt"] != "2024-01-02T00:00:00Z" {
		t.Errorf("deployedAt = %v, want %q", result["deployedAt"], "2024-01-02T00:00:00Z")
	}
	if result["connectionsConfiguredAt"] != "2024-01-02T12:00:00Z" {
		t.Errorf("connectionsConfiguredAt = %v, want %q", result["connectionsConfiguredAt"], "2024-01-02T12:00:00Z")
	}
}

// TestLoadStateNotFound tests loading a nonexistent file.
func TestLoadStateNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent.json")

	state, err := LoadState(path)
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if state != nil {
		t.Errorf("expected nil state, got %v", state)
	}
}

// TestLoadStateInvalidJSON tests loading an invalid JSON file.
func TestLoadStateInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.json")

	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatalf("failed to write invalid JSON file: %v", err)
	}

	state, err := LoadState(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if state != nil {
		t.Errorf("expected nil state on error, got %v", state)
	}
}

// TestLoadStateEmptyFile tests loading an empty file.
func TestLoadStateEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.json")

	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write empty JSON file: %v", err)
	}

	state, err := LoadState(path)
	if err == nil {
		t.Fatal("expected error for empty file, got nil")
	}
	if state != nil {
		t.Errorf("expected nil state on error, got %v", state)
	}
}

// TestFindStateFileMatchingEndpoint tests finding a state file by matching endpoint.
func TestFindStateFileMatchingEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	// Create Azure state with matching endpoint
	azureState := &State{
		Method: "azure",
		Endpoints: StateEndpoints{
			Backend: "https://devlake.example.com",
			Grafana: "https://grafana.example.com",
		},
	}
	azureData, err := json.MarshalIndent(azureState, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal Azure state: %v", err)
	}
	if err := os.WriteFile(".devlake-azure.json", azureData, 0644); err != nil {
		t.Fatalf("failed to write Azure state file: %v", err)
	}

	// Create local state with different endpoint
	localState := &State{
		Method: "local",
		Endpoints: StateEndpoints{
			Backend: "http://localhost:8080",
		},
	}
	localData, err := json.MarshalIndent(localState, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal local state: %v", err)
	}
	if err := os.WriteFile(".devlake-local.json", localData, 0644); err != nil {
		t.Fatalf("failed to write local state file: %v", err)
	}

	// FindStateFile should match the Azure one
	path, state := FindStateFile("https://devlake.example.com", "https://grafana.example.com")

	if !filepath.IsAbs(path) {
		t.Errorf("path is not absolute: %s", path)
	}
	if filepath.Base(path) != ".devlake-azure.json" {
		t.Errorf("path = %s, want .devlake-azure.json", filepath.Base(path))
	}
	if state.Method != "azure" {
		t.Errorf("Method = %q, want %q", state.Method, "azure")
	}
}

// TestFindStateFileFallbackToFirst tests falling back to first existing file.
func TestFindStateFileFallbackToFirst(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	// Only create Azure state (local doesn't exist)
	azureState := &State{
		Method: "azure",
		Endpoints: StateEndpoints{
			Backend: "https://devlake.example.com",
		},
	}
	azureData, err := json.MarshalIndent(azureState, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal azure state JSON: %v", err)
	}
	if err := os.WriteFile(".devlake-azure.json", azureData, 0644); err != nil {
		t.Fatalf("failed to write azure state file: %v", err)
	}

	// Search for a different endpoint — should fall back to first existing
	path, state := FindStateFile("http://localhost:9999", "")

	if filepath.Base(path) != ".devlake-azure.json" {
		t.Errorf("path = %s, want .devlake-azure.json", filepath.Base(path))
	}
	if state.Method != "azure" {
		t.Errorf("Method = %q, want %q", state.Method, "azure")
	}
}

// TestFindStateFileCreateNew tests creating a new local state when none exist.
func TestFindStateFileCreateNew(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	path, state := FindStateFile("http://localhost:8080", "http://localhost:3002")

	if filepath.Base(path) != ".devlake-local.json" {
		t.Errorf("path = %s, want .devlake-local.json", filepath.Base(path))
	}
	if state.Method != "local" {
		t.Errorf("Method = %q, want %q", state.Method, "local")
	}
	if state.Endpoints.Backend != "http://localhost:8080" {
		t.Errorf("Backend = %q, want %q", state.Endpoints.Backend, "http://localhost:8080")
	}

	// The file should not exist yet (FindStateFile doesn't write)
	if _, err := os.Stat(path); err == nil {
		t.Error("expected file to not exist yet")
	}
}

// TestUpdateConnections tests the UpdateConnections function.
func TestUpdateConnections(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".devlake-test.json")

	initialState := &State{
		DeployedAt: "2024-01-01T00:00:00Z",
		Method:     "local",
		Endpoints: StateEndpoints{
			Backend: "http://localhost:8080",
		},
	}
	if err := SaveState(path, initialState); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	newConns := []StateConnection{
		{Plugin: "github", ConnectionID: 1, Name: "conn1"},
		{Plugin: "gh-copilot", ConnectionID: 2, Name: "conn2"},
	}

	err := UpdateConnections(path, initialState, newConns)
	if err != nil {
		t.Fatalf("UpdateConnections failed: %v", err)
	}

	// Reload and verify
	loadedState, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if loadedState == nil {
		t.Fatal("LoadState returned nil state")
	}
	if len(loadedState.Connections) != 2 {
		t.Fatalf("len(Connections) = %d, want 2", len(loadedState.Connections))
	}
	if loadedState.Connections[0].Name != "conn1" {
		t.Errorf("Connections[0].Name = %q, want %q", loadedState.Connections[0].Name, "conn1")
	}
	if loadedState.ConnectionsConfiguredAt == "" {
		t.Error("ConnectionsConfiguredAt not set")
	}
}

// TestLoadStateFromCwd tests loading state from current working directory.
func TestLoadStateFromCwd(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	// No state files exist
	state, err := LoadStateFromCwd()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != nil {
		t.Errorf("expected nil state, got %v", state)
	}

	// Create local state
	localState := &State{
		Method: "local",
		Endpoints: StateEndpoints{
			Backend: "http://localhost:8080",
		},
	}
	data, err := json.MarshalIndent(localState, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal local state JSON: %v", err)
	}
	if err := os.WriteFile(".devlake-local.json", data, 0644); err != nil {
		t.Fatalf("failed to write local state file: %v", err)
	}

	// Should find it
	state, err = LoadStateFromCwd()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state == nil {
		t.Fatal("expected state, got nil")
	}
	if state.Method != "local" {
		t.Errorf("Method = %q, want %q", state.Method, "local")
	}
}

// TestSaveStatePreservesAndUpdatesFields tests that SaveState preserves unknown fields and updates known ones.
func TestSaveStatePreservesAndUpdatesFields(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".devlake-test.json")

	// Create initial state with extra fields
	existingJSON := map[string]any{
		"resourceGroup": "my-rg",
		"deployedAt":    "2024-01-01T00:00:00Z",
		"method":        "azure",
	}
	data, err := json.MarshalIndent(existingJSON, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal existing JSON: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write existing state file: %v", err)
	}

	// Update with new state
	newState := &State{
		DeployedAt: "2024-01-02T00:00:00Z",
		Method:     "azure",
		Endpoints: StateEndpoints{
			Backend: "https://example.com",
		},
	}
	if err := SaveState(path, newState); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// Read back and verify both old and new fields exist
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal state JSON: %v", err)
	}

	if result["resourceGroup"] != "my-rg" {
		t.Error("resourceGroup was not preserved")
	}
	if result["deployedAt"] != "2024-01-02T00:00:00Z" {
		t.Error("deployedAt was not updated")
	}
}

// TestSaveStateNilProject tests saving state with nil Project field.
func TestSaveStateNilProject(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".devlake-test.json")

	state := &State{
		DeployedAt: time.Now().Format(time.RFC3339),
		Method:     "local",
		Endpoints: StateEndpoints{
			Backend: "http://localhost:8080",
		},
		Project: nil,
	}

	err := SaveState(path, state)
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	loadedState, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if loadedState == nil {
		t.Fatal("expected non-nil state, got nil")
	}
	if loadedState.Project != nil {
		t.Errorf("expected nil Project, got %v", loadedState.Project)
	}
}

// TestSaveStateNilProjectDoesNotClearExisting verifies that saving with Project:nil
// over an existing state file that already has a project does not clear the
// existing project. This documents the merge behavior: because Project is
// tagged with omitempty, a nil Project does not remove an existing "project"
// field from the state file.
func TestSaveStateNilProjectDoesNotClearExisting(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".devlake-test.json")

	// First, save a state with a non-nil Project so the file contains a "project" key.
	initialState := &State{
		DeployedAt: time.Now().Format(time.RFC3339),
		Method:     "local",
		Endpoints: StateEndpoints{
			Backend: "http://localhost:8080",
		},
		Project: &StateProject{
			Name:        "test-project",
			BlueprintID: 1,
		},
	}

	if err := SaveState(path, initialState); err != nil {
		t.Fatalf("SaveState (initial) failed: %v", err)
	}

	loadedInitial, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState (initial) failed: %v", err)
	}
	if loadedInitial == nil {
		t.Fatal("expected non-nil initial state, got nil")
	}
	if loadedInitial.Project == nil {
		t.Fatal("expected non-nil initial Project, got nil")
	}
	if loadedInitial.Project.Name != "test-project" {
		t.Errorf("initial Project.Name = %s, want test-project", loadedInitial.Project.Name)
	}

	// Now save a new state with Project:nil. Because Project is omitempty,
	// this should not clear the existing "project" field in the file when
	// SaveState performs its merge behavior.
	updateState := &State{
		DeployedAt: loadedInitial.DeployedAt,
		Method:     loadedInitial.Method,
		Endpoints:  loadedInitial.Endpoints,
		Project:    nil, // explicitly nil
	}

	if err := SaveState(path, updateState); err != nil {
		t.Fatalf("SaveState (update) failed: %v", err)
	}

	loadedFinal, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState (final) failed: %v", err)
	}
	if loadedFinal == nil {
		t.Fatal("expected non-nil final state, got nil")
	}
	// The project should still be present because omitempty means nil fields
	// are not marshaled, so the merge preserves the existing "project" key.
	if loadedFinal.Project == nil {
		t.Error("expected Project to be preserved from initial state, got nil")
	} else if loadedFinal.Project.Name != "test-project" {
		t.Errorf("final Project.Name = %s, want test-project (preserved)", loadedFinal.Project.Name)
	}
}

// TestSaveStateEmptyConnections tests saving state with empty connections slice.
func TestSaveStateEmptyConnections(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".devlake-test.json")

	state := &State{
		DeployedAt:  time.Now().Format(time.RFC3339),
		Method:      "local",
		Endpoints:   StateEndpoints{Backend: "http://localhost:8080"},
		Connections: []StateConnection{},
	}

	err := SaveState(path, state)
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	loadedState, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if loadedState == nil {
		t.Fatal("expected non-nil state, got nil")
	}
	// Empty slice should be preserved or nil (both are valid JSON representations)
	if loadedState.Connections == nil {
		// This is acceptable - omitempty means nil and empty slice are equivalent
		return
	}
	if len(loadedState.Connections) != 0 {
		t.Errorf("expected empty slice, got %v", loadedState.Connections)
	}
}
