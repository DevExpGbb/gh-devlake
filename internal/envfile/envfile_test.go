package envfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_BasicParsing(t *testing.T) {
	content := `# DevLake secrets
GITHUB_TOKEN=ghp_test123
COPILOT_TOKEN=ghp_copilot456

# Commented out
# ADO_TOKEN=ado_xxx
EMPTY_VALUE=
QUOTED_DOUBLE="hello world"
QUOTED_SINGLE='single quoted'
`
	path := filepath.Join(t.TempDir(), ".devlake.env")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]string{
		"GITHUB_TOKEN":  "ghp_test123",
		"COPILOT_TOKEN": "ghp_copilot456",
		"QUOTED_DOUBLE": "hello world",
		"QUOTED_SINGLE": "single quoted",
	}

	for k, want := range tests {
		if got := result[k]; got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}

	// Empty values should not be stored
	if _, ok := result["EMPTY_VALUE"]; ok {
		t.Error("EMPTY_VALUE should not be in result")
	}
	// Commented lines should not be stored
	if _, ok := result["ADO_TOKEN"]; ok {
		t.Error("ADO_TOKEN should not be in result (commented out)")
	}
}

func TestLoad_FileNotExists(t *testing.T) {
	result, err := Load("/nonexistent/.devlake.env")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".devlake.env")
	if err := os.WriteFile(path, []byte("TOKEN=x"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := Delete(path); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should not exist after Delete")
	}

	// Delete on non-existent file should not error
	if err := Delete(path); err != nil {
		t.Fatal(err)
	}
}
