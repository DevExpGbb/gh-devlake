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
)

// capturePrintJSON captures stdout output from printJSON for testing.
func capturePrintJSON(t *testing.T, v any) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig })

	if err := printJSON(v); err != nil {
		t.Fatalf("printJSON: %v", err)
	}
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestPrintJSON_SingleLine(t *testing.T) {
	out := capturePrintJSON(t, map[string]string{"hello": "world"})
	// Must be a single line (no embedded newlines before the trailing newline)
	trimmed := strings.TrimRight(out, "\n")
	if strings.Contains(trimmed, "\n") {
		t.Errorf("printJSON should produce a single line, got: %q", out)
	}
}

func TestPrintJSON_ValidJSON(t *testing.T) {
	type sample struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	out := capturePrintJSON(t, sample{ID: 42, Name: "test"})
	var got sample
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v — output: %q", err, out)
	}
	if got.ID != 42 || got.Name != "test" {
		t.Errorf("unexpected values: %+v", got)
	}
}

func TestPrintJSON_EmptySlice(t *testing.T) {
	out := capturePrintJSON(t, []string{})
	trimmed := strings.TrimSpace(out)
	if trimmed != "[]" {
		t.Errorf("expected '[]', got %q", trimmed)
	}
}

func TestRunStatusJSON_NoState_DiscoveryError(t *testing.T) {
	origCfgURL := cfgURL
	cfgURL = "http://127.0.0.1:1" // unreachable — discover will fail
	t.Cleanup(func() { cfgURL = origCfgURL })

	// Discovery failure should surface as an error so Execute() can emit {"error":...}
	err := runStatusJSON(nil, "")
	if err == nil {
		t.Fatal("expected error when discovery fails, got nil")
	}
	if !strings.Contains(err.Error(), "discovering DevLake") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunStatusJSON_WithState(t *testing.T) {
	// Start a mock backend that responds 200 to /ping
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig })

	state := &devlake.State{
		Method:     "local",
		DeployedAt: "2024-01-01T00:00:00Z",
		Endpoints: devlake.StateEndpoints{
			Backend: srv.URL,
		},
		Connections: []devlake.StateConnection{
			{Plugin: "github", ConnectionID: 1, Name: "GitHub - my-org", Organization: "my-org"},
		},
		Project: &devlake.StateProject{
			Name:        "my-project",
			BlueprintID: 7,
		},
	}

	if err := runStatusJSON(state, ".devlake-local.json"); err != nil {
		t.Fatalf("runStatusJSON returned error: %v", err)
	}
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := strings.TrimSpace(buf.String())

	var got statusOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v — got: %q", err, out)
	}
	if got.Deployment == nil {
		t.Fatal("expected non-nil deployment")
	}
	if got.Deployment.Method != "local" {
		t.Errorf("expected method=local, got %q", got.Deployment.Method)
	}
	if got.Deployment.StateFile != ".devlake-local.json" {
		t.Errorf("expected stateFile=.devlake-local.json, got %q", got.Deployment.StateFile)
	}
	if len(got.Connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(got.Connections))
	}
	if got.Connections[0].Plugin != "github" {
		t.Errorf("expected plugin=github, got %q", got.Connections[0].Plugin)
	}
	if got.Connections[0].ID != 1 {
		t.Errorf("expected id=1, got %d", got.Connections[0].ID)
	}
	if got.Project == nil {
		t.Fatal("expected non-nil project")
	}
	if got.Project.Name != "my-project" {
		t.Errorf("expected project name=my-project, got %q", got.Project.Name)
	}
	if got.Project.BlueprintID != 7 {
		t.Errorf("expected blueprintId=7, got %d", got.Project.BlueprintID)
	}
	// Verify backend endpoint is included and healthy (mock server returns 200 to /ping)
	if len(got.Endpoints) == 0 {
		t.Error("expected at least one endpoint")
	} else if got.Endpoints[0].Name != "backend" {
		t.Errorf("expected first endpoint name=backend, got %q", got.Endpoints[0].Name)
	} else if !got.Endpoints[0].Healthy {
		t.Errorf("expected backend endpoint to be healthy")
	}
}

func TestConnectionListItem_JSONFields(t *testing.T) {
	item := connectionListItem{
		ID:           5,
		Plugin:       "github",
		Name:         "GitHub - test",
		Endpoint:     "https://api.github.com",
		Organization: "test-org",
	}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	out := string(data)
	for _, want := range []string{`"id":5`, `"plugin":"github"`, `"name":"GitHub - test"`, `"endpoint":"https://api.github.com"`, `"organization":"test-org"`} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in JSON output %q", want, out)
		}
	}
	// enterprise omitted when empty
	if strings.Contains(out, `"enterprise"`) {
		t.Errorf("enterprise field should be omitted when empty, got: %q", out)
	}
}

func TestScopeListItem_JSONFields(t *testing.T) {
	item := scopeListItem{
		ID:       "42",
		Name:     "my-repo",
		FullName: "org/my-repo",
	}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	out := string(data)
	for _, want := range []string{`"id":"42"`, `"name":"my-repo"`, `"fullName":"org/my-repo"`} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in JSON output %q", want, out)
		}
	}
	// fullName omitted when empty
	itemNoFull := scopeListItem{ID: "1", Name: "repo"}
	data2, err := json.Marshal(itemNoFull)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(data2), `"fullName"`) {
		t.Errorf("fullName should be omitted when empty, got: %q", string(data2))
	}
}

func TestRunScopeList_JSONRequiresFlags(t *testing.T) {
	origJSON := outputJSON
	outputJSON = true
	t.Cleanup(func() { outputJSON = origJSON })

	cmd := newScopeListCmd()
	err := runScopeList(cmd, nil)
	if err == nil {
		t.Fatal("expected error when --json used without --plugin/--connection-id")
	}
	if !strings.Contains(err.Error(), "--plugin") {
		t.Errorf("unexpected error: %v", err)
	}
}

