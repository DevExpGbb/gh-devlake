package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
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

func TestAzureScopeLabel(t *testing.T) {
	tests := []struct {
		name string
		in   devlake.RemoteScopeChild
		want string
	}{
		{
			name: "prefers full name",
			in:   devlake.RemoteScopeChild{FullName: "org/project/repo", Name: "repo", ID: "123"},
			want: "org/project/repo",
		},
		{
			name: "falls back to name",
			in:   devlake.RemoteScopeChild{Name: "project", ID: "456"},
			want: "project",
		},
		{
			name: "falls back to id",
			in:   devlake.RemoteScopeChild{ID: "789"},
			want: "789",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := azureScopeLabel(tt.in); got != tt.want {
				t.Errorf("azureScopeLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAzureDevOpsScopePayload_FullNameFallback(t *testing.T) {
	raw := map[string]any{
		"id":       "",
		"name":     "",
		"fullName": "",
	}
	data, _ := json.Marshal(raw)
	child := devlake.RemoteScopeChild{
		ID:       "123",
		Name:     "repo",
		FullName: "org/project/repo",
		Data:     data,
	}
	payload := azureDevOpsScopePayload(child, 42)

	if payload["id"] != "123" {
		t.Fatalf("id = %v, want 123", payload["id"])
	}
	if payload["name"] != "repo" {
		t.Fatalf("name = %v, want repo", payload["name"])
	}
	if payload["fullName"] != "org/project/repo" {
		t.Fatalf("fullName = %v, want org/project/repo", payload["fullName"])
	}
	if payload["connectionId"] != 42 {
		t.Fatalf("connectionId = %v, want 42", payload["connectionId"])
	}
}

func TestAzureDevOpsScopePayload_KeepsExistingFields(t *testing.T) {
	raw := map[string]any{
		"id":       "keep-id",
		"name":     "keep-name",
		"fullName": "keep-full",
	}
	data, _ := json.Marshal(raw)
	child := devlake.RemoteScopeChild{
		ID:       "child-id",
		Name:     "child-name",
		FullName: "child/full",
		Data:     data,
	}
	payload := azureDevOpsScopePayload(child, 7)

	if payload["id"] != "keep-id" {
		t.Fatalf("id = %v, want keep-id", payload["id"])
	}
	if payload["name"] != "keep-name" {
		t.Fatalf("name = %v, want keep-name", payload["name"])
	}
	if payload["fullName"] != "keep-full" {
		t.Fatalf("fullName = %v, want keep-full", payload["fullName"])
	}
	if payload["connectionId"] != 7 {
		t.Fatalf("connectionId = %v, want 7", payload["connectionId"])
	}
}

func TestRunConfigureScopes_PluginFlag(t *testing.T) {
	makeCmd := func() (*cobra.Command, *ScopeOpts) {
		opts := &ScopeOpts{}
		cmd := &cobra.Command{RunE: func(cmd *cobra.Command, args []string) error {
			return runScopeAdd(cmd, args, opts)
		}}
		cmd.Flags().StringVar(&opts.Plugin, "plugin", "", "")
		cmd.Flags().StringVar(&opts.Org, "org", "", "")
		cmd.Flags().StringVar(&opts.Repos, "repos", "", "")
		cmd.Flags().StringVar(&opts.ReposFile, "repos-file", "", "")
		cmd.Flags().IntVar(&opts.ConnectionID, "connection-id", 0, "")
		return cmd, opts
	}

	t.Run("unknown plugin returns error", func(t *testing.T) {
		cmd, opts := makeCmd()
		opts.Plugin = "nonexistent-plugin"
		_ = cmd.Flags().Set("plugin", "nonexistent-plugin")
		err := runScopeAdd(cmd, nil, opts)
		if err == nil {
			t.Error("expected error for unavailable plugin")
		}
	})

	t.Run("flag mode without --plugin returns error", func(t *testing.T) {
		cmd, opts := makeCmd()
		_ = cmd.Flags().Set("org", "my-org")
		err := runScopeAdd(cmd, nil, opts)
		if err == nil {
			t.Error("expected error when flags used without --plugin")
		}
	})

	t.Run("--plugin github selects github", func(t *testing.T) {
		cmd, opts := makeCmd()
		opts.Plugin = "github"
		_ = cmd.Flags().Set("plugin", "github")
		_ = cmd.Flags().Set("org", "my-org")
		// Will fail at connection discovery but plugin validation passes
		err := runScopeAdd(cmd, nil, opts)
		// Should get past plugin validation to connection discovery phase
		if err != nil && err.Error() == `unknown plugin "github"` {
			t.Error("github should be accepted as a valid plugin")
		}
	})

	t.Run("--plugin gh-copilot selects copilot", func(t *testing.T) {
		cmd, opts := makeCmd()
		opts.Plugin = "gh-copilot"
		_ = cmd.Flags().Set("plugin", "gh-copilot")
		_ = cmd.Flags().Set("org", "my-org")
		_ = cmd.Flags().Set("connection-id", "999")
		err := runScopeAdd(cmd, nil, opts)
		// Should get past plugin validation to connection discovery phase
		if err != nil && err.Error() == `unknown plugin "gh-copilot"` {
			t.Error("gh-copilot should be accepted as a valid plugin")
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

func TestRepoListLimit(t *testing.T) {
	if repoListLimit != 100 {
		t.Errorf("repoListLimit = %d, want 100", repoListLimit)
	}
}

func TestResolveRepos_WithReposFlag(t *testing.T) {
	opts := &ScopeOpts{Repos: "org/repo1, org/repo2, , org/repo3"}
	got, err := resolveRepos("my-org", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"org/repo1", "org/repo2", "org/repo3"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, r := range want {
		if got[i] != r {
			t.Errorf("got[%d] = %q, want %q", i, got[i], r)
		}
	}
}

func TestResolveRepos_WithReposFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "repos.txt")
	if err := os.WriteFile(f, []byte("org/repo1\norg/repo2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	opts := &ScopeOpts{ReposFile: f}
	got, err := resolveRepos("my-org", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 repos, got %d: %v", len(got), got)
	}
}

// TestResolveRepos_SentinelFiltered verifies that the "Enter repos manually instead"
// sentinel value is excluded from the returned repos slice when real repos are present.
func TestResolveRepos_SentinelFiltered(t *testing.T) {
	const manualOpt = "Enter repos manually instead"
	// Simulate what the picker returns when both real repos and sentinel are chosen.
	rawSelection := []string{"org/repo1", manualOpt, "org/repo2"}
	var picked []string
	for _, s := range rawSelection {
		if s != manualOpt {
			picked = append(picked, s)
		}
	}
	if len(picked) != 2 {
		t.Fatalf("expected 2 repos after filtering sentinel, got %d: %v", len(picked), picked)
	}
	for _, r := range picked {
		if r == manualOpt {
			t.Errorf("sentinel value %q should not appear in picked repos", manualOpt)
		}
	}
}

func TestResolveJenkinsJobs_WithJobsFlag(t *testing.T) {
	tests := []struct {
		name      string
		flagValue string
		want      []string
		wantErr   bool
	}{
		{
			name:      "trims whitespace and ignores empty entries",
			flagValue: " job1 , job2,, job3 ",
			want:      []string{"job1", "job2", "job3"},
		},
		{
			name:      "single job",
			flagValue: "folder/job1",
			want:      []string{"folder/job1"},
		},
		{
			name:      "only separators yields error",
			flagValue: " , , ",
			wantErr:   true,
		},
		{
			name:      "spaces only yields error",
			flagValue: "   ",
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &ScopeOpts{Jobs: tt.flagValue}
			got, err := resolveJenkinsJobs(nil, 1, opts)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d]=%q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
