package cmd

import (
	"errors"
	"testing"
)

func TestExtractPortFromError(t *testing.T) {
	tests := []struct {
		name     string
		errStr   string
		wantPort string
	}{
		{
			name:     "bind for pattern",
			errStr:   "Error response from daemon: driver failed: Bind for 0.0.0.0:8080 failed: port is already allocated",
			wantPort: "8080",
		},
		{
			name:     "exposing port pattern",
			errStr:   "Error response from daemon: Ports are not available: exposing port TCP 0.0.0.0:8080",
			wantPort: "8080",
		},
		{
			name:     "ipv6 listening pattern with colon",
			errStr:   "bind: address already in use (listening on [::]:8080)",
			wantPort: "8080",
		},
		{
			name:     "ipv6 listening pattern without colon",
			errStr:   "bind: address already in use (listening on [::] 3002)",
			wantPort: "3002",
		},
		{
			name:     "port in error context",
			errStr:   "failed programming external connectivity on endpoint devlake (8080/tcp)",
			wantPort: "8080",
		},
		{
			name:     "alternate port 8085",
			errStr:   "Error response from daemon: driver failed: Bind for 0.0.0.0:8085 failed: port is already allocated",
			wantPort: "8085",
		},
		{
			name:     "grafana port 3002",
			errStr:   "Bind for 0.0.0.0:3002: failed: port is already allocated",
			wantPort: "3002",
		},
		{
			name:     "config-ui port 4000",
			errStr:   "Ports are not available: exposing port TCP 0.0.0.0:4000",
			wantPort: "4000",
		},
		{
			name:     "no port in error",
			errStr:   "Error response from daemon: some other error",
			wantPort: "",
		},
		{
			name:     "empty error",
			errStr:   "",
			wantPort: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPortFromError(tt.errStr)
			if got != tt.wantPort {
				t.Errorf("extractPortFromError() = %q, want %q", got, tt.wantPort)
			}
		})
	}
}

func TestClassifyDockerComposeError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantClass  DeployErrorClass
		wantPort   string
		wantNilErr bool
	}{
		{
			name:      "port already allocated",
			err:       errors.New("Error response from daemon: driver failed: Bind for 0.0.0.0:8080 failed: port is already allocated"),
			wantClass: ErrorClassDockerPortConflict,
			wantPort:  "8080",
		},
		{
			name:      "ports are not available",
			err:       errors.New("Error response from daemon: Ports are not available: exposing port TCP 0.0.0.0:8080"),
			wantClass: ErrorClassDockerPortConflict,
			wantPort:  "8080",
		},
		{
			name:      "address already in use",
			err:       errors.New("bind: address already in use (listening on [::]:8080)"),
			wantClass: ErrorClassDockerPortConflict,
			wantPort:  "8080",
		},
		{
			name:      "failed programming external connectivity",
			err:       errors.New("failed programming external connectivity on endpoint devlake"),
			wantClass: ErrorClassDockerPortConflict,
			wantPort:  "",
		},
		{
			name:      "unknown docker error",
			err:       errors.New("Error response from daemon: some other error"),
			wantClass: ErrorClassUnknown,
			wantPort:  "",
		},
		{
			name:       "nil error",
			err:        nil,
			wantNilErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyDockerComposeError(tt.err)

			if tt.wantNilErr {
				if got != nil {
					t.Errorf("classifyDockerComposeError() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("classifyDockerComposeError() returned nil, want non-nil")
			}

			if got.Class != tt.wantClass {
				t.Errorf("classifyDockerComposeError().Class = %v, want %v", got.Class, tt.wantClass)
			}

			if got.Port != tt.wantPort {
				t.Errorf("classifyDockerComposeError().Port = %q, want %q", got.Port, tt.wantPort)
			}

			if got.OriginalErr != tt.err {
				t.Errorf("classifyDockerComposeError().OriginalErr = %v, want %v", got.OriginalErr, tt.err)
			}
		})
	}
}

func TestClassifyDockerComposeError_AllPatterns(t *testing.T) {
	// Test that all documented port conflict patterns are recognized
	patterns := []string{
		"port is already allocated",
		"Bind for 0.0.0.0:8080",
		"ports are not available",
		"address already in use",
		"failed programming external connectivity",
	}

	for _, pattern := range patterns {
		t.Run(pattern, func(t *testing.T) {
			err := errors.New("Error: " + pattern)
			got := classifyDockerComposeError(err)

			if got == nil {
				t.Fatal("classifyDockerComposeError() returned nil, want non-nil")
			}

			if got.Class != ErrorClassDockerPortConflict {
				t.Errorf("Pattern %q not classified as port conflict, got %v", pattern, got.Class)
			}
		})
	}
}
