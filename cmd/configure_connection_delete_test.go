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

	connDeletePlugin = "gitlab"
	connDeleteID = 1

	cmd := &cobra.Command{RunE: runDeleteConnection}
	cmd.Flags().StringVar(&connDeletePlugin, "plugin", "", "")
	cmd.Flags().IntVar(&connDeleteID, "id", 0, "")
	_ = cmd.Flags().Set("plugin", "gitlab")
	_ = cmd.Flags().Set("id", "1")

	err := runDeleteConnection(cmd, nil)
	if err == nil {
		t.Fatal("expected error for unknown plugin, got nil")
	}
	if !strings.Contains(err.Error(), "unknown plugin") {
		t.Errorf("unexpected error message: %v", err)
	}
}
