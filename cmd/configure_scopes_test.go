package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestCopilotScopeID(t *testing.T) {
	tests := []struct {
		name       string
		org        string
		enterprise string
		want       string
	}{
		{
			name: "org only",
			org:  "my-org",
			want: "my-org",
		},
		{
			name:       "enterprise and org",
			org:        "my-org",
			enterprise: "my-enterprise",
			want:       "my-enterprise/my-org",
		},
		{
			name:       "enterprise only",
			enterprise: "my-enterprise",
			want:       "my-enterprise",
		},
		{
			name:       "enterprise with whitespace-only org",
			org:        "   ",
			enterprise: "my-enterprise",
			want:       "my-enterprise",
		},
		{
			name:       "whitespace-only enterprise falls back to org",
			org:        "my-org",
			enterprise: "   ",
			want:       "my-org",
		},
		{
			name:       "both with leading/trailing spaces",
			org:        "  my-org  ",
			enterprise: "  my-ent  ",
			want:       "my-ent/my-org",
		},
		{
			name: "both empty",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := copilotScopeID(tt.org, tt.enterprise)
			if got != tt.want {
				t.Errorf("copilotScopeID(%q, %q) = %q, want %q", tt.org, tt.enterprise, got, tt.want)
			}
		})
	}
}

func TestRunConfigureScopes_PluginFlag(t *testing.T) {
	// Build a minimal cobra command that mirrors the real flag set so we can
	// call runConfigureScopes with controlled flag state.
	makeCmd := func() *cobra.Command {
		cmd := &cobra.Command{RunE: runConfigureScopes}
		cmd.Flags().StringVar(&scopePlugin, "plugin", "", "")
		cmd.Flags().StringVar(&scopeOrg, "org", "", "")
		cmd.Flags().StringVar(&scopeRepos, "repos", "", "")
		cmd.Flags().StringVar(&scopeReposFile, "repos-file", "", "")
		cmd.Flags().IntVar(&scopeGHConnID, "github-connection-id", 0, "")
		cmd.Flags().IntVar(&scopeCopilotConnID, "copilot-connection-id", 0, "")
		cmd.Flags().BoolVar(&scopeSkipCopilot, "skip-copilot", false, "")
		cmd.Flags().BoolVar(&scopeSkipGitHub, "skip-github", false, "")
		return cmd
	}

	t.Run("unknown plugin returns error", func(t *testing.T) {
		scopePlugin = "gitlab"
		scopeSkipCopilot = false
		scopeSkipGitHub = false
		cmd := makeCmd()
		_ = cmd.Flags().Set("plugin", "gitlab")
		err := runConfigureScopes(cmd, nil)
		if err == nil || err.Error() != `unknown plugin "gitlab" — choose: github, gh-copilot` {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("flag mode without --plugin returns error", func(t *testing.T) {
		scopePlugin = ""
		scopeSkipCopilot = false
		scopeSkipGitHub = false
		cmd := makeCmd()
		_ = cmd.Flags().Set("org", "my-org")
		err := runConfigureScopes(cmd, nil)
		if err == nil || err.Error() != "--plugin is required when using flags (use --plugin github or --plugin gh-copilot)" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("--plugin github sets skip-copilot", func(t *testing.T) {
		// We can't fully run without a DevLake instance; just verify the
		// plugin resolution code runs without immediate error beyond DevLake discovery.
		// The error we expect is from devlake.Discover, not from plugin resolution.
		scopePlugin = "github"
		scopeSkipCopilot = false
		scopeSkipGitHub = false
		cmd := makeCmd()
		_ = cmd.Flags().Set("plugin", "github")
		_ = cmd.Flags().Set("org", "my-org")
		// runConfigureScopes will fail at DevLake discovery, but skip flags should be set
		runConfigureScopes(cmd, nil) //nolint:errcheck
		if !scopeSkipCopilot {
			t.Error("expected scopeSkipCopilot=true after --plugin github")
		}
		if scopeSkipGitHub {
			t.Error("expected scopeSkipGitHub=false after --plugin github")
		}
	})

	t.Run("--plugin gh-copilot sets skip-github", func(t *testing.T) {
		scopePlugin = "gh-copilot"
		scopeSkipCopilot = false
		scopeSkipGitHub = false
		cmd := makeCmd()
		_ = cmd.Flags().Set("plugin", "gh-copilot")
		_ = cmd.Flags().Set("org", "my-org")
		runConfigureScopes(cmd, nil) //nolint:errcheck
		if scopeSkipCopilot {
			t.Error("expected scopeSkipCopilot=false after --plugin gh-copilot")
		}
		if !scopeSkipGitHub {
			t.Error("expected scopeSkipGitHub=true after --plugin gh-copilot")
		}
	})
}

func TestFilterChoicesByPlugin(t *testing.T) {
	choices := []connChoice{
		{plugin: "github", id: 1, label: "GitHub (ID: 1)"},
		{plugin: "gh-copilot", id: 2, label: "GitHub Copilot (ID: 2)"},
		{plugin: "github", id: 3, label: "GitHub (ID: 3)"},
	}

	t.Run("filter to github", func(t *testing.T) {
		got := filterChoicesByPlugin(choices, "github")
		if len(got) != 2 {
			t.Errorf("expected 2 github choices, got %d", len(got))
		}
		for _, c := range got {
			if c.plugin != "github" {
				t.Errorf("unexpected plugin %q", c.plugin)
			}
		}
	})

	t.Run("filter to gh-copilot", func(t *testing.T) {
		got := filterChoicesByPlugin(choices, "gh-copilot")
		if len(got) != 1 {
			t.Errorf("expected 1 copilot choice, got %d", len(got))
		}
	})

	t.Run("filter to unknown plugin returns empty", func(t *testing.T) {
		got := filterChoicesByPlugin(choices, "gitlab")
		if len(got) != 0 {
			t.Errorf("expected 0 choices, got %d", len(got))
		}
	})
}
