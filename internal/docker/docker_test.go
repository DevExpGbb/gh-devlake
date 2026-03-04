package docker

import (
	"os"
	"os/exec"
	"reflect"
	"testing"
)

// fakeExecCommand returns a function that replaces execCommand and captures its arguments
// without executing the actual command. The returned function records the full argv
// (name + args) into capturedArgs and runs a no-op subprocess that exits 0.
func fakeExecCommand(capturedArgs *[]string) func(string, ...string) *exec.Cmd {
	return func(name string, args ...string) *exec.Cmd {
		*capturedArgs = append([]string{name}, args...)
		// Use the test binary itself as a no-op process.
		cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--")
		cmd.Env = append(os.Environ(), "GO_TEST_HELPER_NOOP=1")
		return cmd
	}
}

// TestHelperProcess implements the TestHelperProcess pattern for subprocess testing.
// It exits 0 immediately when GO_TEST_HELPER_NOOP=1 is set, serving as a no-op
// stand-in for docker commands during tests.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_TEST_HELPER_NOOP") != "1" {
		return
	}
	os.Exit(0)
}

func TestComposeUp_CommandArgs(t *testing.T) {
	tests := []struct {
		name     string
		build    bool
		services []string
		wantArgs []string
	}{
		{
			name:     "no build no services",
			build:    false,
			services: nil,
			wantArgs: []string{"docker", "compose", "up", "-d"},
		},
		{
			name:     "with build flag",
			build:    true,
			services: nil,
			wantArgs: []string{"docker", "compose", "up", "-d", "--build"},
		},
		{
			name:     "with services no build",
			build:    false,
			services: []string{"devlake", "grafana"},
			wantArgs: []string{"docker", "compose", "up", "-d", "devlake", "grafana"},
		},
		{
			name:     "with build and services",
			build:    true,
			services: []string{"devlake"},
			wantArgs: []string{"docker", "compose", "up", "-d", "--build", "devlake"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured []string
			execCommand = fakeExecCommand(&captured)
			t.Cleanup(func() { execCommand = exec.Command })

			_ = ComposeUp(t.TempDir(), tt.build, tt.services...)

			if !reflect.DeepEqual(captured, tt.wantArgs) {
				t.Errorf("args = %v, want %v", captured, tt.wantArgs)
			}
		})
	}
}

func TestComposeDown_CommandArgs(t *testing.T) {
	tests := []struct {
		name          string
		removeVolumes bool
		wantArgs      []string
	}{
		{
			name:          "default (keep volumes)",
			removeVolumes: false,
			wantArgs:      []string{"docker", "compose", "down", "--rmi", "local"},
		},
		{
			name:          "remove volumes",
			removeVolumes: true,
			wantArgs:      []string{"docker", "compose", "down", "--rmi", "local", "-v"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured []string
			execCommand = fakeExecCommand(&captured)
			t.Cleanup(func() { execCommand = exec.Command })

			_ = ComposeDown(t.TempDir(), tt.removeVolumes)

			if !reflect.DeepEqual(captured, tt.wantArgs) {
				t.Errorf("args = %v, want %v", captured, tt.wantArgs)
			}
		})
	}
}

func TestBuild_CommandArgs(t *testing.T) {
	tests := []struct {
		name       string
		tag        string
		dockerfile string
		context    string
		wantArgs   []string
	}{
		{
			name:       "standard build",
			tag:        "myimage:latest",
			dockerfile: "Dockerfile",
			context:    ".",
			wantArgs:   []string{"docker", "build", "-t", "myimage:latest", "-f", "Dockerfile", "."},
		},
		{
			name:       "custom context",
			tag:        "app:v1",
			dockerfile: "build/Dockerfile.prod",
			context:    "./app",
			wantArgs:   []string{"docker", "build", "-t", "app:v1", "-f", "build/Dockerfile.prod", "./app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured []string
			execCommand = fakeExecCommand(&captured)
			t.Cleanup(func() { execCommand = exec.Command })

			_ = Build(tt.tag, tt.dockerfile, tt.context)

			if !reflect.DeepEqual(captured, tt.wantArgs) {
				t.Errorf("args = %v, want %v", captured, tt.wantArgs)
			}
		})
	}
}
