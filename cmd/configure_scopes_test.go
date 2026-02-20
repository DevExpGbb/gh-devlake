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
		{name: "org only", org: "my-org", want: "my-org"},
		{name: "enterprise and org", org: "my-org", enterprise: "my-enterprise", want: "my-enterprise/my-org"},
		{name: "enterprise only", enterprise: "my-enterprise", want: "my-enterprise"},
		{name: "enterprise with whitespace-only org", org: "   ", enterprise: "my-enterprise", want: "my-enterprise"},
		{name: "whitespace-only enterprise falls back to org", org: "my-org", enterprise: "   ", want: "my-org"},
		{name: "both with leading/trailing spaces", org: "  my-org  ", enterprise: "  my-ent  ", want: "my-ent/my-org"},
		{name: "both empty", want: ""},
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
	makeCmd := func() (*cobra.Command, *ScopeOpts) {
		opts := &ScopeOpts{}
		cmd := &cobra.Command{RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigureScopes(cmd, args, opts)
		}}
		cmd.Flags().StringVar(&opts.Plugin, "plugin", "", "")
		cmd.Flags().StringVar(&opts.Org, "org", "", "")
		cmd.Flags().StringVar(&opts.Repos, "repos", "", "")
		cmd.Flags().StringVar(&opts.ReposFile, "repos-file", "", "")
		cmd.Flags().IntVar(&opts.GHConnID, "github-connection-id", 0, "")
		cmd.Flags().IntVar(&opts.CopilotConnID, "copilot-connection-id", 0, "")
		return cmd, opts
	}

	t.Run("unknown plugin returns error", func(t *testing.T) {
		cmd, opts := makeCmd()
		opts.Plugin = "gitlab"
		_ = cmd.Flags().Set("plugin", "gitlab")
		err := runConfigureScopes(cmd, nil, opts)
		if err == nil || err.Error() != `unknown plugin "gitlab" — choose: github, gh-copilot` {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("flag mode without --plugin returns error", func(t *testing.T) {
		cmd, opts := makeCmd()
		_ = cmd.Flags().Set("org", "my-org")
		err := runConfigureScopes(cmd, nil, opts)
		if err == nil || err.Error() != "--plugin is required when using flags (use --plugin github or --plugin gh-copilot)" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("--plugin github sets skip-copilot", func(t *testing.T) {
		cmd, opts := makeCmd()
		opts.Plugin = "github"
		_ = cmd.Flags().Set("plugin", "github")
		_ = cmd.Flags().Set("org", "my-org")
		runConfigureScopes(cmd, nil, opts) //nolint:errcheck
		if !opts.SkipCopilot {
			t.Error("expected SkipCopilot=true after --plugin github")
		}
		if opts.SkipGitHub {
			t.Error("expected SkipGitHub=false after --plugin github")
		}
	})

	t.Run("--plugin gh-copilot sets skip-github", func(t *testing.T) {
		cmd, opts := makeCmd()
		opts.Plugin = "gh-copilot"
		_ = cmd.Flags().Set("plugin", "gh-copilot")
		_ = cmd.Flags().Set("org", "my-org")
		_ = cmd.Flags().Set("copilot-connection-id", "999")
		runConfigureScopes(cmd, nil, opts) //nolint:errcheck
		// After plugin resolution, SkipGitHub should be true.
		// The command will fail at connection resolution (ID 999 doesn't exist),
		// but the plugin flags are already set by then.
		if !opts.SkipGitHub {
			t.Error("expected SkipGitHub=true after --plugin gh-copilot")
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

	t.Run("empty plugin slug returns empty", func(t *testing.T) {
		got := filterChoicesByPlugin(choices, "")
		if len(got) != 0 {
			t.Errorf("expected 0 choices for empty plugin, got %d", len(got))
		}
	})
}
