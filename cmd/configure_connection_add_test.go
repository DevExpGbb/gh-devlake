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

	flags := []string{"plugin", "org", "enterprise", "token", "env-file", "skip-cleanup", "name", "proxy", "endpoint"}
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

func TestSelectPlugin_UnavailablePlugin(t *testing.T) {
	_, err := selectPlugin("gitlab")
	if err == nil {
		t.Fatal("expected error for unavailable plugin, got nil")
	}
	if !strings.Contains(err.Error(), "coming soon") {
		t.Errorf("unexpected error message: %v", err)
	}
}
