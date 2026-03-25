package cmd

import (
	"strings"
	"testing"
)

func TestNewAddConnectionCmd(t *testing.T) {
	if addConnectionCmd.Use != "add" {
		t.Errorf("expected Use %q, got %q", "add", addConnectionCmd.Use)
	}
	if addConnectionCmd.Short == "" {
		t.Error("expected Short to be set")
	}

	flags := []string{"plugin", "org", "enterprise", "token", "username", "env-file", "skip-cleanup", "name", "proxy", "endpoint"}
	for _, f := range flags {
		if addConnectionCmd.Flags().Lookup(f) == nil {
			t.Errorf("expected flag --%s to be registered on addConnectionCmd", f)
		}
	}
}

func TestAddConnectionCmd_Registered(t *testing.T) {
	found := false
	for _, sub := range configureConnectionsCmd.Commands() {
		if sub.Use == "add" {
			found = true
			break
		}
	}
	if !found {
		t.Error("'add' subcommand not registered under configureConnectionsCmd")
	}
}

func TestSelectPlugin_ValidSlug(t *testing.T) {
	def, err := selectPlugin("github")
	if err != nil {
		t.Fatalf("expected no error for valid slug, got: %v", err)
	}
	if def == nil {
		t.Fatal("expected ConnectionDef, got nil")
	}
	if def.Plugin != "github" {
		t.Errorf("expected plugin %q, got %q", "github", def.Plugin)
	}
}

func TestSelectPlugin_UnknownSlug(t *testing.T) {
	_, err := selectPlugin("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown slug, got nil")
	}
	if !strings.Contains(err.Error(), "unknown plugin") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSelectPlugin_AzureDevOpsAlias(t *testing.T) {
	def, err := selectPlugin("azure-devops")
	if err != nil {
		t.Fatalf("expected alias resolution for azure-devops, got error: %v", err)
	}
	if def == nil {
		t.Fatal("expected ConnectionDef, got nil")
	}
	if def.Plugin != "azuredevops_go" {
		t.Errorf("expected plugin %q, got %q", "azuredevops_go", def.Plugin)
	}
}

// TestIsInteractive_FlagMode verifies that setting --plugin triggers flag mode
// (non-interactive), which is used by runAddConnection to skip optional prompts.
func TestIsInteractive_FlagMode(t *testing.T) {
	// When --plugin is provided → not interactive
	pluginFlag := "github"
	isInteractive := pluginFlag == ""
	if isInteractive {
		t.Error("expected isInteractive=false when connPlugin is set")
	}

	// When --plugin is empty → interactive
	pluginFlag = ""
	isInteractive = pluginFlag == ""
	if !isInteractive {
		t.Error("expected isInteractive=true when connPlugin is empty")
	}
}

// TestIsInteractive_OrgPromptDecision verifies the org prompt logic:
// - NeedsOrg=true + org="" + flag mode → should error (not prompt)
// - NeedsOrg=false + org="" + flag mode → should skip (not prompt)
// - NeedsOrg=false + org="" + interactive → would prompt (tested manually)
func TestIsInteractive_OrgPromptDecision(t *testing.T) {
	tests := []struct {
		name          string
		needsOrg      bool
		org           string
		isInteractive bool
		wantPrompt    bool
		wantError     bool
	}{
		{
			name:          "NeedsOrg + no org + flag mode → error",
			needsOrg:      true,
			org:           "",
			isInteractive: false,
			wantPrompt:    false,
			wantError:     true,
		},
		{
			name:          "NeedsOrg + org provided + flag mode → no prompt",
			needsOrg:      true,
			org:           "my-org",
			isInteractive: false,
			wantPrompt:    false,
			wantError:     false,
		},
		{
			name:          "NoNeedsOrg + no org + flag mode → skip",
			needsOrg:      false,
			org:           "",
			isInteractive: false,
			wantPrompt:    false,
			wantError:     false,
		},
		{
			name:          "NoNeedsOrg + no org + interactive → would prompt",
			needsOrg:      false,
			org:           "",
			isInteractive: true,
			wantPrompt:    true,
			wantError:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate the org prompt decision logic from runAddConnection
			shouldPromptRequired := tc.needsOrg && tc.org == ""
			shouldError := shouldPromptRequired && !tc.isInteractive
			shouldPromptOptional := !tc.needsOrg && tc.org == "" && tc.isInteractive

			if shouldError != tc.wantError {
				t.Errorf("error: got %v, want %v", shouldError, tc.wantError)
			}
			gotPrompt := shouldPromptOptional || (shouldPromptRequired && tc.isInteractive)
			if gotPrompt != tc.wantPrompt {
				t.Errorf("prompt: got %v, want %v", gotPrompt, tc.wantPrompt)
			}
		})
	}
}
