package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestDeleteConnectionCmd_Registered(t *testing.T) {
	found := false
	for _, sub := range configureConnectionsCmd.Commands() {
		if sub.Use == "delete" {
			found = true
			break
		}
	}
	if !found {
		t.Error("'delete' subcommand not registered under configureConnectionsCmd")
	}
}

func TestRunDeleteConnection_UnknownPlugin(t *testing.T) {
	origPlugin := connDeletePlugin
	origID := connDeleteID
	t.Cleanup(func() {
		connDeletePlugin = origPlugin
		connDeleteID = origID
	})

	connDeletePlugin = "nonexistent-plugin"
	connDeleteID = 1

	cmd := &cobra.Command{RunE: runDeleteConnection}
	cmd.Flags().StringVar(&connDeletePlugin, "plugin", "", "")
	cmd.Flags().IntVar(&connDeleteID, "id", 0, "")
	_ = cmd.Flags().Set("plugin", "nonexistent-plugin")
	_ = cmd.Flags().Set("id", "1")

	err := runDeleteConnection(cmd, nil)
	if err == nil {
		t.Fatal("expected error for unknown plugin, got nil")
	}
	if !strings.Contains(err.Error(), "unknown plugin") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunDeleteConnection_PluginOnlyNoID(t *testing.T) {
	origPlugin := connDeletePlugin
	origID := connDeleteID
	t.Cleanup(func() {
		connDeletePlugin = origPlugin
		connDeleteID = origID
	})

	connDeletePlugin = "github"
	connDeleteID = 0

	cmd := &cobra.Command{RunE: runDeleteConnection}
	cmd.Flags().StringVar(&connDeletePlugin, "plugin", "", "")
	cmd.Flags().IntVar(&connDeleteID, "id", 0, "")
	_ = cmd.Flags().Set("plugin", "github")

	err := runDeleteConnection(cmd, nil)
	if err == nil {
		t.Fatal("expected error when only --plugin is provided, got nil")
	}
	if !strings.Contains(err.Error(), "both --plugin and --id must be provided together") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunDeleteConnection_ForceSkipsConfirm(t *testing.T) {
	origPlugin := connDeletePlugin
	origID := connDeleteID
	origForce := connDeleteForce
	t.Cleanup(func() {
		connDeletePlugin = origPlugin
		connDeleteID = origID
		connDeleteForce = origForce
	})

	connDeletePlugin = "github"
	connDeleteID = 1
	connDeleteForce = true

	cmd := &cobra.Command{RunE: runDeleteConnection}
	cmd.Flags().StringVar(&connDeletePlugin, "plugin", "", "")
	cmd.Flags().IntVar(&connDeleteID, "id", 0, "")
	cmd.Flags().BoolVar(&connDeleteForce, "force", false, "")
	_ = cmd.Flags().Set("plugin", "github")
	_ = cmd.Flags().Set("id", "1")
	_ = cmd.Flags().Set("force", "true")

	err := runDeleteConnection(cmd, nil)
	// The function should proceed past validation and the confirmation step,
	// failing at DevLake discovery (no instance running) — not hanging on a prompt.
	if err == nil {
		t.Fatal("expected error from DevLake discovery, got nil")
	}
	if strings.Contains(err.Error(), "both --plugin and --id") || strings.Contains(err.Error(), "unknown plugin") {
		t.Errorf("unexpected validation error (force flag should have passed validation): %v", err)
	}
}

func TestRunDeleteConnection_IDOnlyNoPlugin(t *testing.T) {
	origPlugin := connDeletePlugin
	origID := connDeleteID
	t.Cleanup(func() {
		connDeletePlugin = origPlugin
		connDeleteID = origID
	})

	connDeletePlugin = ""
	connDeleteID = 1

	cmd := &cobra.Command{RunE: runDeleteConnection}
	cmd.Flags().StringVar(&connDeletePlugin, "plugin", "", "")
	cmd.Flags().IntVar(&connDeleteID, "id", 0, "")
	_ = cmd.Flags().Set("id", "1")

	err := runDeleteConnection(cmd, nil)
	if err == nil {
		t.Fatal("expected error when only --id is provided, got nil")
	}
	if !strings.Contains(err.Error(), "both --plugin and --id must be provided together") {
		t.Errorf("unexpected error message: %v", err)
	}
}
