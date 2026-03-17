package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewritePoetryInstallLine_RewritesInstallerLine(t *testing.T) {
	input := "FROM python:3.9-slim-bookworm\nRUN curl -sSL https://install.python-poetry.org | python3 -\n"

	got, changed := rewritePoetryInstallLine(input, "2.2.1")
	if !changed {
		t.Fatalf("expected rewrite to report change")
	}
	want := "FROM python:3.9-slim-bookworm\nRUN curl -sSL https://install.python-poetry.org | python3 - --version 2.2.1\n"
	if got != want {
		t.Fatalf("unexpected rewrite result\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestRewritePoetryInstallLine_NoChangeWhenPinned(t *testing.T) {
	input := "RUN curl -sSL https://install.python-poetry.org | python3 - --version 2.2.1\n"

	got, changed := rewritePoetryInstallLine(input, "2.2.1")
	if changed {
		t.Fatalf("expected no change for already pinned content")
	}
	if got != input {
		t.Fatalf("content changed unexpectedly")
	}
}

func TestRewritePoetryInstallLine_NoChangeWhenLineMissing(t *testing.T) {
	input := "RUN echo hello\n"

	got, changed := rewritePoetryInstallLine(input, "2.2.1")
	if changed {
		t.Fatalf("expected no change when installer line is missing")
	}
	if got != input {
		t.Fatalf("content changed unexpectedly")
	}
}

func TestRewriteComposePorts(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantContain []string
		wantErr     bool
	}{
		{
			name: "standard docker-compose format",
			input: `version: '3'
services:
  devlake:
    ports:
      - 8080:8080
  grafana:
    ports:
      - 3002:3002
  config-ui:
    ports:
      - 4000:4000
`,
			wantContain: []string{"8085:8080", "3004:3002", "4004:4000"},
			wantErr:     false,
		},
		{
			name: "quoted port mappings",
			input: `version: '3'
services:
  devlake:
    ports:
      - "8080:8080"
  grafana:
    ports:
      - "3002:3002"
  config-ui:
    ports:
      - "4000:4000"
`,
			wantContain: []string{"8085:8080", "3004:3002", "4004:4000"},
			wantErr:     false,
		},
		{
			name: "single-quoted port mappings",
			input: `version: '3'
services:
  devlake:
    ports:
      - '8080:8080'
  grafana:
    ports:
      - '3002:3002'
  config-ui:
    ports:
      - '4000:4000'
`,
			wantContain: []string{"8085:8080", "3004:3002", "4004:4000"},
			wantErr:     false,
		},
		{
			name: "mixed port formats",
			input: `version: '3'
services:
  devlake:
    ports:
      - 8080:8080
  grafana:
    ports:
      - "3002:3002"
  config-ui:
    ports:
      - '4000:4000'
`,
			wantContain: []string{"8085:8080", "3004:3002", "4004:4000"},
			wantErr:     false,
		},
		{
			name: "no matching ports",
			input: `version: '3'
services:
  mysql:
    ports:
      - 3306:3306
`,
			wantErr: true,
		},
		{
			name: "already rewritten ports (alternate bundle)",
			input: `version: '3'
services:
  devlake:
    ports:
      - 8085:8080
  grafana:
    ports:
      - 3004:3002
  config-ui:
    ports:
      - 4004:4000
`,
			wantErr: true, // No changes made
		},
		{
			name: "custom host port should not be rewritten",
			input: `version: '3'
services:
  devlake:
    ports:
      - 18080:8080
  grafana:
    ports:
      - 13002:3002
  config-ui:
    ports:
      - 14000:4000
`,
			wantErr: true, // No matching ports to rewrite
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			composePath := filepath.Join(tmpDir, "docker-compose.yml")
			if err := os.WriteFile(composePath, []byte(tt.input), 0644); err != nil {
				t.Fatalf("Failed to write test compose file: %v", err)
			}

			// Run rewrite
			err := rewriteComposePorts(composePath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("rewriteComposePorts() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("rewriteComposePorts() unexpected error: %v", err)
				return
			}

			// Read result
			result, err := os.ReadFile(composePath)
			if err != nil {
				t.Fatalf("Failed to read result: %v", err)
			}
			resultStr := string(result)

			// Check expected content
			for _, want := range tt.wantContain {
				if !strings.Contains(resultStr, want) {
					t.Errorf("rewriteComposePorts() result missing %q\nResult:\n%s", want, resultStr)
				}
			}

			// Ensure old ports are gone
			oldPorts := []string{"8080:8080", "3002:3002", "4000:4000"}
			for _, old := range oldPorts {
				if strings.Contains(resultStr, old) {
					t.Errorf("rewriteComposePorts() result still contains old port %q", old)
				}
			}
		})
	}
}

func TestRewriteComposePorts_FileNotFound(t *testing.T) {
	err := rewriteComposePorts("/nonexistent/path/docker-compose.yml")
	if err == nil {
		t.Error("rewriteComposePorts() expected error for nonexistent file, got nil")
	}
	if !strings.Contains(err.Error(), "reading compose file") {
		t.Errorf("rewriteComposePorts() error = %v, want error about reading compose file", err)
	}
}

func TestDetectPortBundle(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   portBundle
	}{
		{
			name: "default port bundle",
			input: `version: '3'
services:
  devlake:
    ports:
      - 8080:8080
  grafana:
    ports:
      - 3002:3002
  config-ui:
    ports:
      - 4000:4000
`,
			want: portBundleDefault,
		},
		{
			name: "alternate port bundle",
			input: `version: '3'
services:
  devlake:
    ports:
      - 8085:8080
  grafana:
    ports:
      - 3004:3002
  config-ui:
    ports:
      - 4004:4000
`,
			want: portBundleAlternate,
		},
		{
			name: "custom port bundle",
			input: `version: '3'
services:
  devlake:
    ports:
      - 18080:8080
  grafana:
    ports:
      - 13002:3002
  config-ui:
    ports:
      - 14000:4000
`,
			want: portBundleCustom,
		},
		{
			name: "mixed custom and unrelated ports",
			input: `version: '3'
services:
  mysql:
    ports:
      - 3306:3306
  devlake:
    ports:
      - 9090:8080
`,
			want: portBundleCustom,
		},
		{
			name: "partial default bundle (has at least one default port)",
			input: `version: '3'
services:
  devlake:
    ports:
      - 8080:8080
`,
			want: portBundleDefault,
		},
		{
			name: "partial alternate bundle (has at least one alternate port)",
			input: `version: '3'
services:
  devlake:
    ports:
      - 8085:8080
`,
			want: portBundleAlternate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			composePath := filepath.Join(tmpDir, "docker-compose.yml")
			if err := os.WriteFile(composePath, []byte(tt.input), 0644); err != nil {
				t.Fatalf("Failed to write test compose file: %v", err)
			}

			got := detectPortBundle(composePath)
			if got != tt.want {
				t.Errorf("detectPortBundle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectPortBundle_FileNotFound(t *testing.T) {
	got := detectPortBundle("/nonexistent/path/docker-compose.yml")
	if got != portBundleDefault {
		t.Errorf("detectPortBundle() for nonexistent file = %v, want %v (default)", got, portBundleDefault)
	}
}

func TestExtractServicePorts_MissingFile(t *testing.T) {
	// Should return empty map for non-existent file
	ports := extractServicePorts("/nonexistent/docker-compose.yml", "devlake")
	if len(ports) != 0 {
		t.Errorf("extractServicePorts() for nonexistent file returned %v, want empty map", ports)
	}
}

