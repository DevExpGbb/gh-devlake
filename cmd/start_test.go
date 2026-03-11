package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── detectStartMode tests ────────────────────────────────────────────────────

func TestDetectStartMode_ExplicitAzureFlag(t *testing.T) {
	orig := startAzure
	startAzure = true
	t.Cleanup(func() { startAzure = orig })

	if got := detectStartMode(); got != "azure" {
		t.Errorf("expected azure, got %q", got)
	}
}

func TestDetectStartMode_ExplicitLocalFlag(t *testing.T) {
	orig := startLocal
	startLocal = true
	t.Cleanup(func() { startLocal = orig })

	if got := detectStartMode(); got != "local" {
		t.Errorf("expected local, got %q", got)
	}
}

func TestDetectStartMode_StateFile_AzureMethod(t *testing.T) {
	dir := t.TempDir()
	sf := filepath.Join(dir, "mystate.json")
	if err := os.WriteFile(sf, []byte(`{"method":"azure","resourceGroup":"my-rg"}`), 0644); err != nil {
		t.Fatal(err)
	}

	origState := startState
	startState = sf
	t.Cleanup(func() { startState = origState })

	if got := detectStartMode(); got != "azure" {
		t.Errorf("expected azure, got %q", got)
	}
}

func TestDetectStartMode_StateFile_LocalMethod(t *testing.T) {
	dir := t.TempDir()
	sf := filepath.Join(dir, "mystate.json")
	if err := os.WriteFile(sf, []byte(`{"method":"local"}`), 0644); err != nil {
		t.Fatal(err)
	}

	origState := startState
	startState = sf
	t.Cleanup(func() { startState = origState })

	if got := detectStartMode(); got != "local" {
		t.Errorf("expected local, got %q", got)
	}
}

// An Azure state file with a non-obvious filename (no "azure" in path) should still
// be detected as azure when it contains a resourceGroup field.
func TestDetectStartMode_StateFile_ResourceGroupFallback(t *testing.T) {
	dir := t.TempDir()
	sf := filepath.Join(dir, "state.json") // no "azure" in name
	if err := os.WriteFile(sf, []byte(`{"resourceGroup":"my-rg"}`), 0644); err != nil {
		t.Fatal(err)
	}

	origState := startState
	startState = sf
	t.Cleanup(func() { startState = origState })

	if got := detectStartMode(); got != "azure" {
		t.Errorf("expected azure from resourceGroup fallback, got %q", got)
	}
}

// A state file that can't be parsed should fall back to "local" rather than erroring.
func TestDetectStartMode_StateFile_UnparsableFallsBackToLocal(t *testing.T) {
	dir := t.TempDir()
	sf := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(sf, []byte(`not-json`), 0644); err != nil {
		t.Fatal(err)
	}

	origState := startState
	startState = sf
	t.Cleanup(func() { startState = origState })

	if got := detectStartMode(); got != "local" {
		t.Errorf("expected local fallback for unparsable state file, got %q", got)
	}
}

func TestDetectStartMode_AutoDetect_AzureJson(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origWd) })

	if err := os.WriteFile(".devlake-azure.json", []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	if got := detectStartMode(); got != "azure" {
		t.Errorf("expected azure from .devlake-azure.json, got %q", got)
	}
}

func TestDetectStartMode_AutoDetect_LocalJson(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origWd) })

	if err := os.WriteFile(".devlake-local.json", []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	if got := detectStartMode(); got != "local" {
		t.Errorf("expected local from .devlake-local.json, got %q", got)
	}
}

func TestDetectStartMode_AutoDetect_DockerCompose(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origWd) })

	if err := os.WriteFile("docker-compose.yml", []byte("version: '3'"), 0644); err != nil {
		t.Fatal(err)
	}

	if got := detectStartMode(); got != "local" {
		t.Errorf("expected local from docker-compose.yml, got %q", got)
	}
}

func TestDetectStartMode_NoDeployment(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origWd) })

	if got := detectStartMode(); got != "" {
		t.Errorf("expected empty string when no deployment found, got %q", got)
	}
}

// ── localCompanionURLs tests ──────────────────────────────────────────────────

func TestLocalCompanionURLs_Port8080(t *testing.T) {
	grafana, configUI := localCompanionURLs("http://localhost:8080")
	if grafana != "http://localhost:3002" {
		t.Errorf("unexpected grafana URL: %q", grafana)
	}
	if configUI != "http://localhost:4000" {
		t.Errorf("unexpected configUI URL: %q", configUI)
	}
}

func TestLocalCompanionURLs_Port8085(t *testing.T) {
	grafana, configUI := localCompanionURLs("http://localhost:8085")
	if grafana != "http://localhost:3004" {
		t.Errorf("unexpected grafana URL: %q", grafana)
	}
	if configUI != "http://localhost:4004" {
		t.Errorf("unexpected configUI URL: %q", configUI)
	}
}

// ── JSON output tests ─────────────────────────────────────────────────────────

// TestRunStart_JSONMode_NoDeployment verifies that when --json is set and no
// deployment exists, runStart returns an error without printing banners to stdout.
func TestRunStart_JSONMode_NoDeployment(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origWd) })

	origJSON := outputJSON
	outputJSON = true
	t.Cleanup(func() { outputJSON = origJSON })

	origAzure := startAzure
	origLocal := startLocal
	origState := startState
	startAzure = false
	startLocal = false
	startState = ""
	t.Cleanup(func() {
		startAzure = origAzure
		startLocal = origLocal
		startState = origState
	})

	var capturedErr error
	stdoutOut := captureStdout(func() {
		capturedErr = runStart(newStartCmd(), nil)
	})

	if capturedErr == nil {
		t.Fatal("expected error when no deployment found")
	}
	// stdout must be empty — no banners mixed into JSON output
	if strings.TrimSpace(stdoutOut) != "" {
		t.Errorf("expected no stdout output in JSON mode with no deployment, got: %q", stdoutOut)
	}
}

// TestRunStart_JSONPayload_LocalShape verifies the JSON payload structure for local mode.
func TestRunStart_JSONPayload_LocalShape(t *testing.T) {
	out := capturePrintJSON(t, map[string]string{"status": "started", "mode": "local"})
	var got map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v — got: %q", err, out)
	}
	if got["status"] != "started" {
		t.Errorf("expected status=started, got %q", got["status"])
	}
	if got["mode"] != "local" {
		t.Errorf("expected mode=local, got %q", got["mode"])
	}
}

// TestRunStart_JSONPayload_AzureShape verifies the JSON payload structure for azure mode.
func TestRunStart_JSONPayload_AzureShape(t *testing.T) {
	out := capturePrintJSON(t, map[string]string{"status": "started", "mode": "azure"})
	var got map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v — got: %q", err, out)
	}
	if got["status"] != "started" {
		t.Errorf("expected status=started, got %q", got["status"])
	}
	if got["mode"] != "azure" {
		t.Errorf("expected mode=azure, got %q", got["mode"])
	}
}
