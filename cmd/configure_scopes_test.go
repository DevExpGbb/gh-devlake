package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestParseBitbucketRepo(t *testing.T) {
	t.Run("uses payload fields when present", func(t *testing.T) {
		data, _ := json.Marshal(map[string]any{
			"bitbucketId": "workspace/api",
			"name":        "api",
			"fullName":    "workspace/api",
			"htmlUrl":     "https://bitbucket.org/workspace/api",
			"cloneUrl":    "https://bitbucket.org/workspace/api.git",
		})
		child := devlake.RemoteScopeChild{
			ID:       "ignored",
			Name:     "api-child",
			FullName: "workspace/api-child",
			Data:     data,
		}
		repo := parseBitbucketRepo(&child)
		if repo == nil {
			t.Fatal("expected repo, got nil")
		}
		if repo.BitbucketID != "workspace/api" {
			t.Fatalf("bitbucketId = %q, want %q", repo.BitbucketID, "workspace/api")
		}
		if repo.Name != "api" {
			t.Fatalf("name = %q, want %q", repo.Name, "api")
		}
		if repo.FullName != "workspace/api" {
			t.Fatalf("fullName = %q, want %q", repo.FullName, "workspace/api")
		}
		if repo.CloneURL != "https://bitbucket.org/workspace/api.git" {
			t.Fatalf("cloneUrl = %q, want https://bitbucket.org/workspace/api.git", repo.CloneURL)
		}
		if repo.HTMLURL != "https://bitbucket.org/workspace/api" {
			t.Fatalf("htmlUrl = %q, want https://bitbucket.org/workspace/api", repo.HTMLURL)
		}
	})

	t.Run("falls back to child fields when payload is sparse", func(t *testing.T) {
		child := devlake.RemoteScopeChild{
			Name:     "frontend",
			FullName: "team/frontend",
			Data:     []byte(`{"bitbucketId":"","name":"","fullName":""}`),
		}
		repo := parseBitbucketRepo(&child)
		if repo == nil {
			t.Fatal("expected repo, got nil")
		}
		if repo.BitbucketID != "team/frontend" {
			t.Fatalf("bitbucketId = %q, want %q", repo.BitbucketID, "team/frontend")
		}
		if repo.Name != "frontend" {
			t.Fatalf("name = %q, want %q", repo.Name, "frontend")
		}
		if repo.FullName != "team/frontend" {
			t.Fatalf("fullName = %q, want %q", repo.FullName, "team/frontend")
		}
	})

	t.Run("handles missing data by using child fields", func(t *testing.T) {
		child := devlake.RemoteScopeChild{
			Name:     "ui",
			FullName: "workspace/ui",
			Data:     nil,
		}
		repo := parseBitbucketRepo(&child)
		if repo == nil {
			t.Fatal("expected repo, got nil")
		}
		if repo.BitbucketID != "workspace/ui" {
			t.Fatalf("bitbucketId = %q, want %q", repo.BitbucketID, "workspace/ui")
		}
		if repo.Name != "ui" {
			t.Fatalf("name = %q, want %q", repo.Name, "ui")
		}
		if repo.FullName != "workspace/ui" {
			t.Fatalf("fullName = %q, want %q", repo.FullName, "workspace/ui")
		}
	})
}

func TestPagerDutyServiceFromChild_UsesData(t *testing.T) {
	data, _ := json.Marshal(map[string]any{
		"id":   "SVC123",
		"name": "Checkout",
		"url":  "https://api.pagerduty.com/services/SVC123",
	})
	child := &devlake.RemoteScopeChild{
		ID:       "fallback-id",
		Name:     "fallback-name",
		FullName: "fallback/full",
		Data:     data,
	}

	scope := pagerDutyServiceFromChild(child, 101)
	if scope.ID != "SVC123" {
		t.Fatalf("ID = %q, want SVC123", scope.ID)
	}
	if scope.Name != "Checkout" {
		t.Fatalf("Name = %q, want Checkout", scope.Name)
	}
	if scope.URL != "https://api.pagerduty.com/services/SVC123" {
		t.Fatalf("URL = %q, want https://api.pagerduty.com/services/SVC123", scope.URL)
	}
	if scope.ConnectionID != 101 {
		t.Fatalf("ConnectionID = %d, want 101", scope.ConnectionID)
	}
}

func TestPagerDutyServiceFromChild_Fallbacks(t *testing.T) {
	child := &devlake.RemoteScopeChild{
		ID:       "SVC999",
		FullName: "Platform/Incident",
	}

	scope := pagerDutyServiceFromChild(child, 7)
	if scope.ID != "SVC999" {
		t.Fatalf("ID = %q, want SVC999", scope.ID)
	}
	if scope.Name != "Platform/Incident" {
		t.Fatalf("Name = %q, want Platform/Incident", scope.Name)
	}
	if scope.URL != "" {
		t.Fatalf("URL = %q, want empty", scope.URL)
	}
	if scope.ConnectionID != 7 {
		t.Fatalf("ConnectionID = %d, want 7", scope.ConnectionID)
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

func TestScopeSonarQubeHandler_ProjectsFlag(t *testing.T) {
	origJSON := outputJSON
	outputJSON = false
	t.Cleanup(func() { outputJSON = origJSON })

	t.Run("valid project keys put scopes", func(t *testing.T) {
		var (
			putCalls   int
			captured   devlake.ScopeBatchRequest
			remoteResp = devlake.RemoteScopeResponse{
				Children: []devlake.RemoteScopeChild{
					{Type: "scope", ID: "proj-a", Name: "Project A"},
					{Type: "scope", ID: "proj-b", Name: "Project B"},
				},
			}
		)

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasPrefix(r.URL.Path, "/plugins/sonarqube/connections/123/remote-scopes"):
				data, _ := json.Marshal(remoteResp)
				_, _ = w.Write(data)
			case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/plugins/sonarqube/connections/123/scopes"):
				putCalls++
				if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
					t.Fatalf("decoding scopes payload: %v", err)
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		t.Cleanup(srv.Close)

		client := devlake.NewClient(srv.URL)
		client.HTTPClient = srv.Client()

		opts := &ScopeOpts{Projects: "proj-a, proj-b"}
		bp, err := scopeSonarQubeHandler(client, 123, "", "", opts)
		if err != nil {
			t.Fatalf("scopeSonarQubeHandler returned error: %v", err)
		}
		if putCalls != 1 {
			t.Fatalf("expected 1 PutScopes call, got %d", putCalls)
		}
		if len(captured.Data) != 2 {
			t.Fatalf("expected 2 scopes in payload, got %d", len(captured.Data))
		}

		assertScope := func(idx int, expectKey, expectName string) {
			item, ok := captured.Data[idx].(map[string]any)
			if !ok {
				t.Fatalf("scope %d type = %T, want map[string]any", idx, captured.Data[idx])
			}
			if got := item["projectKey"]; got != expectKey {
				t.Errorf("scope %d projectKey = %v, want %s", idx, got, expectKey)
			}
			if got := item["name"]; got != expectName {
				t.Errorf("scope %d name = %v, want %s", idx, got, expectName)
			}
			if got := item["connectionId"]; got != float64(123) { // JSON numbers decode as float64
				t.Errorf("scope %d connectionId = %v, want 123", idx, got)
			}
		}
		assertScope(0, "proj-a", "Project A")
		assertScope(1, "proj-b", "Project B")

		if bp == nil || bp.PluginName != "sonarqube" || bp.ConnectionID != 123 || len(bp.Scopes) != 2 {
			t.Fatalf("unexpected blueprint connection: %+v", bp)
		}
	})

	t.Run("invalid project key errors", func(t *testing.T) {
		var (
			putCalls   int
			remoteResp = devlake.RemoteScopeResponse{
				Children: []devlake.RemoteScopeChild{
					{Type: "scope", ID: "proj-a", Name: "Project A"},
				},
			}
		)

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasPrefix(r.URL.Path, "/plugins/sonarqube/connections/456/remote-scopes"):
				data, _ := json.Marshal(remoteResp)
				_, _ = w.Write(data)
			case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/plugins/sonarqube/connections/456/scopes"):
				putCalls++
				w.WriteHeader(http.StatusOK)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		t.Cleanup(srv.Close)

		client := devlake.NewClient(srv.URL)
		client.HTTPClient = srv.Client()

		opts := &ScopeOpts{Projects: "missing"}
		_, err := scopeSonarQubeHandler(client, 456, "", "", opts)
		if err == nil {
			t.Fatal("expected error for missing project key, got nil")
		}
		if !strings.Contains(err.Error(), `project key "missing" not found`) {
			t.Fatalf("unexpected error: %v", err)
		}
		if putCalls != 0 {
			t.Fatalf("expected no PutScopes calls on error, got %d", putCalls)
		}
	})
}
