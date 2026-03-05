package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunScopeDelete_ForceSkipsConfirm(t *testing.T) {
	origPlugin := scopeDeletePlugin
	origConnID := scopeDeleteConnID
	origScopeID := scopeDeleteScopeID
	origForce := scopeDeleteForce
	t.Cleanup(func() {
		scopeDeletePlugin = origPlugin
		scopeDeleteConnID = origConnID
		scopeDeleteScopeID = origScopeID
		scopeDeleteForce = origForce
	})

	scopeDeletePlugin = "github"
	scopeDeleteConnID = 1
	scopeDeleteScopeID = "12345678"
	scopeDeleteForce = true

	cmd := &cobra.Command{RunE: runScopeDelete}
	cmd.Flags().StringVar(&scopeDeletePlugin, "plugin", "", "")
	cmd.Flags().IntVar(&scopeDeleteConnID, "connection-id", 0, "")
	cmd.Flags().StringVar(&scopeDeleteScopeID, "scope-id", "", "")
	cmd.Flags().BoolVar(&scopeDeleteForce, "force", false, "")
	_ = cmd.Flags().Set("plugin", "github")
	_ = cmd.Flags().Set("connection-id", "1")
	_ = cmd.Flags().Set("scope-id", "12345678")
	_ = cmd.Flags().Set("force", "true")

	err := runScopeDelete(cmd, nil)
	// The function should proceed past validation and the confirmation step,
	// failing at DevLake discovery (no instance running) — not hanging on a prompt.
	if err == nil {
		t.Fatal("expected error from DevLake discovery, got nil")
	}
	if strings.Contains(err.Error(), "--plugin, --connection-id, and --scope-id must all be provided together") ||
		strings.Contains(err.Error(), "unknown plugin") {
		t.Errorf("unexpected validation error (force flag should have passed validation): %v", err)
	}
}
