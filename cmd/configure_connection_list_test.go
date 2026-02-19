package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunListConnections_UnknownPlugin(t *testing.T) {
	origPlugin := connListPlugin
	t.Cleanup(func() { connListPlugin = origPlugin })

	connListPlugin = "gitlab"
	cmd := &cobra.Command{RunE: runListConnections}
	cmd.Flags().StringVar(&connListPlugin, "plugin", "", "")
	_ = cmd.Flags().Set("plugin", "gitlab")

	err := runListConnections(cmd, nil)
	if err == nil {
		t.Fatal("expected error for unknown plugin, got nil")
	}
	if !strings.Contains(err.Error(), "unknown plugin") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAvailablePluginSlugs(t *testing.T) {
	slugs := availablePluginSlugs()
	if len(slugs) == 0 {
		t.Fatal("expected at least one available plugin slug")
	}
	// All returned slugs should correspond to available ConnectionDefs.
	for _, s := range slugs {
		def := FindConnectionDef(s)
		if def == nil {
			t.Errorf("slug %q has no ConnectionDef", s)
			continue
		}
		if !def.Available {
			t.Errorf("slug %q is marked unavailable but returned by availablePluginSlugs", s)
		}
	}
}

func TestListConnectionsCmd_Registered(t *testing.T) {
	// Verify the list subcommand is registered under configureConnectionsCmd.
	found := false
	for _, sub := range configureConnectionsCmd.Commands() {
		if sub.Use == "list" {
			found = true
			break
		}
	}
	if !found {
		t.Error("'list' subcommand not registered under configureConnectionsCmd")
	}
}

