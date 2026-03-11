package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── detectStopMode tests ─────────────────────────────────────────────────────

func TestDetectStopMode_ExplicitAzureFlag(t *testing.T) {
	orig := stopAzure
	stopAzure = true
	t.Cleanup(func() { stopAzure = orig })

	if got := detectStopMode(); got != "azure" {
		t.Errorf("expected azure, got %q", got)
	}
}

func TestDetectStopMode_ExplicitLocalFlag(t *testing.T) {
	orig := stopLocal
	stopLocal = true
	t.Cleanup(func() { stopLocal = orig })

	if got := detectStopMode(); got != "local" {
		t.Errorf("expected local, got %q", got)
	}
}

func TestDetectStopMode_StateFile_AzureMethod(t *testing.T) {
	dir := t.TempDir()
	sf := filepath.Join(dir, "mystate.json")
	if err := os.WriteFile(sf, []byte(`{"method":"azure","resourceGroup":"my-rg"}`), 0644); err != nil {
		t.Fatal(err)
	}

	origState := stopState
	stopState = sf
	t.Cleanup(func() { stopState = origState })

	if got := detectStopMode(); got != "azure" {
		t.Errorf("expected azure, got %q", got)
	}
}

func TestDetectStopMode_StateFile_LocalMethod(t *testing.T) {
	dir := t.TempDir()
	sf := filepath.Join(dir, "mystate.json")
	if err := os.WriteFile(sf, []byte(`{"method":"local"}`), 0644); err != nil {
		t.Fatal(err)
	}

	origState := stopState
	stopState = sf
	t.Cleanup(func() { stopState = origState })

	if got := detectStopMode(); got != "local" {
		t.Errorf("expected local, got %q", got)
	}
}

func TestDetectStopMode_StateFile_ResourceGroupFallback(t *testing.T) {
	dir := t.TempDir()
	sf := filepath.Join(dir, "state.json")
	if err := os.WriteFile(sf, []byte(`{"resourceGroup":"my-rg"}`), 0644); err != nil {
		t.Fatal(err)
	}

	origState := stopState
	stopState = sf
	t.Cleanup(func() { stopState = origState })

	if got := detectStopMode(); got != "azure" {
		t.Errorf("expected azure from resourceGroup fallback, got %q", got)
	}
}

func TestDetectStopMode_StateFile_UnparsableFallsBackToLocal(t *testing.T) {
	dir := t.TempDir()
	sf := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(sf, []byte(`not-json`), 0644); err != nil {
		t.Fatal(err)
	}

	origState := stopState
	stopState = sf
	t.Cleanup(func() { stopState = origState })

	if got := detectStopMode(); got != "local" {
		t.Errorf("expected local fallback for unparsable state file, got %q", got)
	}
}

func TestDetectStopMode_AutoDetect_AzureJson(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origWd) })

	if err := os.WriteFile(".devlake-azure.json", []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	if got := detectStopMode(); got != "azure" {
		t.Errorf("expected azure from .devlake-azure.json, got %q", got)
	}
}

func TestDetectStopMode_AutoDetect_LocalJson(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origWd) })

	if err := os.WriteFile(".devlake-local.json", []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	if got := detectStopMode(); got != "local" {
		t.Errorf("expected local from .devlake-local.json, got %q", got)
	}
}

func TestDetectStopMode_AutoDetect_DockerCompose(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origWd) })

	if err := os.WriteFile("docker-compose.yml", []byte("version: '3'"), 0644); err != nil {
		t.Fatal(err)
	}

	if got := detectStopMode(); got != "local" {
		t.Errorf("expected local from docker-compose.yml, got %q", got)
	}
}

func TestDetectStopMode_NoDeployment(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origWd) })

	if got := detectStopMode(); got != "" {
		t.Errorf("expected empty string when no deployment found, got %q", got)
	}
}

// ── JSON output tests ─────────────────────────────────────────────────────────

// TestRunStop_JSONMode_NoDeployment verifies that when --json is set and no
// deployment exists, runStop returns an error without printing banners to stdout.
func TestRunStop_JSONMode_NoDeployment(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origWd) })

	origJSON := outputJSON
	outputJSON = true
	t.Cleanup(func() { outputJSON = origJSON })

	origAzure := stopAzure
	origLocal := stopLocal
	origState := stopState
	stopAzure = false
	stopLocal = false
	stopState = ""
	t.Cleanup(func() {
		stopAzure = origAzure
		stopLocal = origLocal
		stopState = origState
	})

	var capturedErr error
	stdoutOut := captureStdout(func() {
		capturedErr = runStop(newStopCmd(), nil)
	})

	if capturedErr == nil {
		t.Fatal("expected error when no deployment found")
	}
	// stdout must be empty — no banners mixed into JSON output
	if strings.TrimSpace(stdoutOut) != "" {
		t.Errorf("expected no stdout output in JSON mode with no deployment, got: %q", stdoutOut)
	}
}

// TestRunStop_JSONPayload_LocalShape verifies the JSON payload structure for local mode.
func TestRunStop_JSONPayload_LocalShape(t *testing.T) {
	out := capturePrintJSON(t, map[string]string{"status": "stopped", "mode": "local"})
	var got map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v — got: %q", err, out)
	}
	if got["status"] != "stopped" {
		t.Errorf("expected status=stopped, got %q", got["status"])
	}
	if got["mode"] != "local" {
		t.Errorf("expected mode=local, got %q", got["mode"])
	}
}

// TestRunStop_JSONPayload_AzureShape verifies the JSON payload structure for azure mode.
func TestRunStop_JSONPayload_AzureShape(t *testing.T) {
	out := capturePrintJSON(t, map[string]string{"status": "stopped", "mode": "azure"})
	var got map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v — got: %q", err, out)
	}
	if got["status"] != "stopped" {
		t.Errorf("expected status=stopped, got %q", got["status"])
	}
	if got["mode"] != "azure" {
		t.Errorf("expected mode=azure, got %q", got["mode"])
	}
}
