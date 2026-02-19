package cmd

import (
	"strings"
	"testing"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/spf13/cobra"
)

func TestUpdateConnectionCmd_Registered(t *testing.T) {
	found := false
	for _, sub := range configureConnectionsCmd.Commands() {
		if sub.Use == "update" {
			found = true
			break
		}
	}
	if !found {
		t.Error("'update' subcommand not registered under configureConnectionsCmd")
	}
}

func TestRunUpdateConnection_UnknownPlugin(t *testing.T) {
	origPlugin := updateConnPlugin
	origID := updateConnID
	t.Cleanup(func() {
		updateConnPlugin = origPlugin
		updateConnID = origID
	})

	updateConnPlugin = "gitlab"
	updateConnID = 1

	cmd := &cobra.Command{RunE: runUpdateConnection}
	cmd.Flags().StringVar(&updateConnPlugin, "plugin", "", "")
	cmd.Flags().IntVar(&updateConnID, "id", 0, "")
	_ = cmd.Flags().Set("plugin", "gitlab")
	_ = cmd.Flags().Set("id", "1")

	err := runUpdateConnection(cmd, nil)
	if err == nil {
		t.Fatal("expected error for unknown plugin, got nil")
	}
	if !strings.Contains(err.Error(), "unknown plugin") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunUpdateConnection_MissingID(t *testing.T) {
	origPlugin := updateConnPlugin
	origID := updateConnID
	t.Cleanup(func() {
		updateConnPlugin = origPlugin
		updateConnID = origID
	})

	updateConnPlugin = "github"
	updateConnID = 0

	cmd := &cobra.Command{RunE: runUpdateConnection}
	cmd.Flags().StringVar(&updateConnPlugin, "plugin", "", "")
	cmd.Flags().IntVar(&updateConnID, "id", 0, "")
	_ = cmd.Flags().Set("plugin", "github")

	err := runUpdateConnection(cmd, nil)
	if err == nil {
		t.Fatal("expected error for missing --id, got nil")
	}
	if !strings.Contains(err.Error(), "--plugin and --id are both required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunUpdateConnection_MissingPlugin(t *testing.T) {
	origPlugin := updateConnPlugin
	origID := updateConnID
	t.Cleanup(func() {
		updateConnPlugin = origPlugin
		updateConnID = origID
	})

	updateConnPlugin = ""
	updateConnID = 5

	cmd := &cobra.Command{RunE: runUpdateConnection}
	cmd.Flags().StringVar(&updateConnPlugin, "plugin", "", "")
	cmd.Flags().IntVar(&updateConnID, "id", 0, "")
	_ = cmd.Flags().Set("id", "5")

	err := runUpdateConnection(cmd, nil)
	if err == nil {
		t.Fatal("expected error for missing --plugin, got nil")
	}
	if !strings.Contains(err.Error(), "--plugin and --id are both required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// resetUpdateConnFlags resets all package-level update connection flag variables to their zero values.
func resetUpdateConnFlags() {
	updateConnPlugin = ""
	updateConnID = 0
	updateConnToken = ""
	updateConnOrg = ""
	updateConnEnterprise = ""
	updateConnName = ""
	updateConnEndpoint = ""
	updateConnProxy = ""
}

func TestBuildUpdateRequestFromFlags_OnlyChangedFlags(t *testing.T) {
	resetUpdateConnFlags()
	t.Cleanup(resetUpdateConnFlags)

	cmd := &cobra.Command{}
	cmd.Flags().StringVar(&updateConnToken, "token", "", "")
	cmd.Flags().StringVar(&updateConnName, "name", "", "")
	cmd.Flags().StringVar(&updateConnOrg, "org", "", "")
	cmd.Flags().StringVar(&updateConnEndpoint, "endpoint", "", "")
	cmd.Flags().StringVar(&updateConnProxy, "proxy", "", "")
	cmd.Flags().StringVar(&updateConnEnterprise, "enterprise", "", "")

	// Only set --token
	_ = cmd.Flags().Set("token", "ghp_newtoken123")

	current := &devlake.Connection{
		ID:       1,
		Name:     "GitHub - my-org",
		Endpoint: "https://api.github.com/",
	}

	req := buildUpdateRequestFromFlags(cmd, current)

	if req.Token != "ghp_newtoken123" {
		t.Errorf("expected token to be set, got %q", req.Token)
	}
	if req.AuthMethod != "AccessToken" {
		t.Errorf("expected AuthMethod to be set when token is changed, got %q", req.AuthMethod)
	}
	if req.Name != "" {
		t.Errorf("expected Name to be empty (not changed), got %q", req.Name)
	}
	if req.Endpoint != "" {
		t.Errorf("expected Endpoint to be empty (not changed), got %q", req.Endpoint)
	}
}

func TestBuildUpdateRequestFromFlags_MultipleFlags(t *testing.T) {
	resetUpdateConnFlags()
	t.Cleanup(resetUpdateConnFlags)

	cmd := &cobra.Command{}
	cmd.Flags().StringVar(&updateConnToken, "token", "", "")
	cmd.Flags().StringVar(&updateConnName, "name", "", "")
	cmd.Flags().StringVar(&updateConnOrg, "org", "", "")
	cmd.Flags().StringVar(&updateConnEndpoint, "endpoint", "", "")
	cmd.Flags().StringVar(&updateConnProxy, "proxy", "", "")
	cmd.Flags().StringVar(&updateConnEnterprise, "enterprise", "", "")

	_ = cmd.Flags().Set("name", "New Name")
	_ = cmd.Flags().Set("org", "new-org")

	current := &devlake.Connection{
		ID:           1,
		Name:         "Old Name",
		Organization: "old-org",
	}

	req := buildUpdateRequestFromFlags(cmd, current)

	if req.Name != "New Name" {
		t.Errorf("expected Name=%q, got %q", "New Name", req.Name)
	}
	if req.Organization != "new-org" {
		t.Errorf("expected Organization=%q, got %q", "new-org", req.Organization)
	}
	if req.Token != "" {
		t.Errorf("expected Token to be empty, got %q", req.Token)
	}
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ghp_abcdefghij", "**********ghij"},
		{"abcd", "abcd"},
		{"ab", "ab"},
		{"", ""},
		{"12345", "*2345"},
	}
	for _, tt := range tests {
		got := maskToken(tt.input)
		if got != tt.expected {
			t.Errorf("maskToken(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
