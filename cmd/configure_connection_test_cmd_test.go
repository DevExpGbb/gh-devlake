package cmd

import (
	"testing"
)

func TestTestConnectionCmd_Registered(t *testing.T) {
	// Verify the test subcommand is registered under configureConnectionsCmd.
	found := false
	for _, sub := range configureConnectionsCmd.Commands() {
		if sub.Use == "test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("'test' subcommand not registered under configureConnectionsCmd")
	}
}

func TestFindConnectionDef_UnknownPlugin(t *testing.T) {
	// Test the plugin validation logic used by the test command.
	// This validates that FindConnectionDef correctly identifies unknown plugins.
	testCases := []struct {
		plugin      string
		shouldExist bool
	}{
		{"github", true},
		{"gh-copilot", true},
		{"nonexistent-plugin-xyz", false},
		{"invalid-xyz", false},
	}

	for _, tc := range testCases {
		def := FindConnectionDef(tc.plugin)
		exists := def != nil && def.Available
		if exists != tc.shouldExist {
			t.Errorf("plugin %q: expected exists=%v, got %v", tc.plugin, tc.shouldExist, exists)
		}
	}
}

func TestTestConnectionCmd_Flags(t *testing.T) {
	// Verify that the test command has the expected flags defined.
	if testConnectionCmd.Flags().Lookup("plugin") == nil {
		t.Error("expected --plugin flag to be defined")
	}
	if testConnectionCmd.Flags().Lookup("id") == nil {
		t.Error("expected --id flag to be defined")
	}

	// Verify default values
	idFlag := testConnectionCmd.Flags().Lookup("id")
	if idFlag != nil && idFlag.DefValue != "0" {
		t.Errorf("expected --id default to be 0, got %s", idFlag.DefValue)
	}
}