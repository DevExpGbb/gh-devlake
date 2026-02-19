package token

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolve_FlagTakesPriority(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env-token")
	result, err := Resolve("github", "flag-token", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Token != "flag-token" || result.Source != "flag" {
		t.Errorf("got token=%q source=%q, want token=%q source=%q", result.Token, result.Source, "flag-token", "flag")
	}
}

func TestResolve_GitHub_EnvVar(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_test")
	result, err := Resolve("github", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Token != "ghp_test" || result.Source != "environment" {
		t.Errorf("got token=%q source=%q, want token=%q source=%q", result.Token, result.Source, "ghp_test", "environment")
	}
}

func TestResolve_GHCopilot_EnvVar(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_copilot")
	result, err := Resolve("gh-copilot", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Token != "ghp_copilot" || result.Source != "environment" {
		t.Errorf("got token=%q source=%q, want token=%q source=%q", result.Token, result.Source, "ghp_copilot", "environment")
	}
}

func TestResolve_GHToken_Fallback(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "gho_fallback")
	result, err := Resolve("github", "", "/nonexistent/.devlake.env", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Token != "gho_fallback" || result.Source != "environment" {
		t.Errorf("got token=%q source=%q, want token=%q source=%q", result.Token, result.Source, "gho_fallback", "environment")
	}
}

func TestResolve_GitLab_EnvVar(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "glpat_test")
	result, err := Resolve("gitlab", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Token != "glpat_test" || result.Source != "environment" {
		t.Errorf("got token=%q source=%q, want token=%q source=%q", result.Token, result.Source, "glpat_test", "environment")
	}
}

func TestResolve_GitLab_NotPickedUpByGitHub(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "glpat_test")
	// Ensure GITHUB_TOKEN and GH_TOKEN are unset
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	_, err := Resolve("github", "", "/nonexistent/.devlake.env", "")
	// Should fail (no token found, non-terminal)
	if err == nil {
		t.Error("expected error when GITLAB_TOKEN set but resolving github plugin")
	}
}

func TestResolve_AzureDevOps_EnvVar(t *testing.T) {
	t.Setenv("AZURE_DEVOPS_PAT", "ado_test")
	result, err := Resolve("azure-devops", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Token != "ado_test" || result.Source != "environment" {
		t.Errorf("got token=%q source=%q, want token=%q source=%q", result.Token, result.Source, "ado_test", "environment")
	}
}

func TestResolve_EnvFile_GitHubPAT(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".devlake.env")
	if err := os.WriteFile(envFile, []byte("GITHUB_PAT=ghp_envfile\n"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Resolve("github", "", envFile, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Token != "ghp_envfile" || result.Source != "envfile" {
		t.Errorf("got token=%q source=%q, want token=%q source=%q", result.Token, result.Source, "ghp_envfile", "envfile")
	}
	if result.EnvFilePath != envFile {
		t.Errorf("got EnvFilePath=%q, want %q", result.EnvFilePath, envFile)
	}
}

func TestResolve_EnvFile_GitLabToken(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".devlake.env")
	if err := os.WriteFile(envFile, []byte("GITLAB_TOKEN=glpat_envfile\n"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Resolve("gitlab", "", envFile, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Token != "glpat_envfile" || result.Source != "envfile" {
		t.Errorf("got token=%q source=%q, want token=%q source=%q", result.Token, result.Source, "glpat_envfile", "envfile")
	}
}

func TestResolve_EnvFile_MultiplePlugins(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".devlake.env")
	content := "GITHUB_TOKEN=ghp_multi\nGITLAB_TOKEN=glpat_multi\nAZURE_DEVOPS_PAT=ado_multi\n"
	if err := os.WriteFile(envFile, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		plugin    string
		wantToken string
	}{
		{"github", "ghp_multi"},
		{"gh-copilot", "ghp_multi"},
		{"gitlab", "glpat_multi"},
		{"azure-devops", "ado_multi"},
	}

	for _, tt := range tests {
		t.Run(tt.plugin, func(t *testing.T) {
			result, err := Resolve(tt.plugin, "", envFile, "")
			if err != nil {
				t.Fatal(err)
			}
			if result.Token != tt.wantToken {
				t.Errorf("plugin=%q: got token=%q, want %q", tt.plugin, result.Token, tt.wantToken)
			}
			if result.Source != "envfile" {
				t.Errorf("plugin=%q: got source=%q, want envfile", tt.plugin, result.Source)
			}
		})
	}
}

func TestResolve_UnknownPlugin_FallsBackToGitHub(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_unknown_plugin")
	result, err := Resolve("some-unknown-plugin", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Token != "ghp_unknown_plugin" {
		t.Errorf("got token=%q, want %q", result.Token, "ghp_unknown_plugin")
	}
}

func TestPluginEnvFileKeys(t *testing.T) {
	tests := []struct {
		plugin string
		want   []string
	}{
		{"github", []string{"GITHUB_PAT", "GITHUB_TOKEN", "GH_TOKEN"}},
		{"gh-copilot", []string{"GITHUB_PAT", "GITHUB_TOKEN", "GH_TOKEN"}},
		{"gitlab", []string{"GITLAB_TOKEN"}},
		{"azure-devops", []string{"AZURE_DEVOPS_PAT"}},
		{"unknown", []string{"GITHUB_PAT", "GITHUB_TOKEN", "GH_TOKEN"}},
	}
	for _, tt := range tests {
		got := pluginEnvFileKeys(tt.plugin)
		if len(got) != len(tt.want) {
			t.Errorf("pluginEnvFileKeys(%q) = %v, want %v", tt.plugin, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("pluginEnvFileKeys(%q)[%d] = %q, want %q", tt.plugin, i, got[i], tt.want[i])
			}
		}
	}
}

func TestPluginEnvVarKeys(t *testing.T) {
	tests := []struct {
		plugin string
		want   []string
	}{
		{"github", []string{"GITHUB_TOKEN", "GH_TOKEN"}},
		{"gh-copilot", []string{"GITHUB_TOKEN", "GH_TOKEN"}},
		{"gitlab", []string{"GITLAB_TOKEN"}},
		{"azure-devops", []string{"AZURE_DEVOPS_PAT"}},
		{"unknown", []string{"GITHUB_TOKEN", "GH_TOKEN"}},
	}
	for _, tt := range tests {
		got := pluginEnvVarKeys(tt.plugin)
		if len(got) != len(tt.want) {
			t.Errorf("pluginEnvVarKeys(%q) = %v, want %v", tt.plugin, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("pluginEnvVarKeys(%q)[%d] = %q, want %q", tt.plugin, i, got[i], tt.want[i])
			}
		}
	}
}
