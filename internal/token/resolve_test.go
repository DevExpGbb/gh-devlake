package token

import (
	"os"
	"path/filepath"
	"testing"
)

// helper to build ResolveOpts for GitHub-family plugins
func ghOpts(flagValue, envFile string) ResolveOpts {
	return ResolveOpts{
		FlagValue:   flagValue,
		EnvFilePath: envFile,
		EnvFileKeys: []string{"GITHUB_PAT", "GITHUB_TOKEN", "GH_TOKEN"},
		EnvVarNames: []string{"GITHUB_TOKEN", "GH_TOKEN"},
		DisplayName: "GitHub",
	}
}

func glOpts(flagValue, envFile string) ResolveOpts {
	return ResolveOpts{
		FlagValue:   flagValue,
		EnvFilePath: envFile,
		EnvFileKeys: []string{"GITLAB_TOKEN"},
		EnvVarNames: []string{"GITLAB_TOKEN"},
		DisplayName: "GitLab",
	}
}

func adoOpts(flagValue, envFile string) ResolveOpts {
	return ResolveOpts{
		FlagValue:   flagValue,
		EnvFilePath: envFile,
		EnvFileKeys: []string{"AZURE_DEVOPS_PAT"},
		EnvVarNames: []string{"AZURE_DEVOPS_PAT"},
		DisplayName: "Azure DevOps",
	}
}

func TestResolve_FlagTakesPriority(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env-token")
	result, err := Resolve(ghOpts("flag-token", ""))
	if err != nil {
		t.Fatal(err)
	}
	if result.Token != "flag-token" || result.Source != "flag" {
		t.Errorf("got token=%q source=%q, want token=%q source=%q", result.Token, result.Source, "flag-token", "flag")
	}
}

func TestResolve_GitHub_EnvVar(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_test")
	result, err := Resolve(ghOpts("", ""))
	if err != nil {
		t.Fatal(err)
	}
	if result.Token != "ghp_test" || result.Source != "environment" {
		t.Errorf("got token=%q source=%q, want token=%q source=%q", result.Token, result.Source, "ghp_test", "environment")
	}
}

func TestResolve_GHCopilot_EnvVar(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_copilot")
	// Copilot uses same env vars as GitHub
	result, err := Resolve(ghOpts("", ""))
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
	result, err := Resolve(ghOpts("", "/nonexistent/.devlake.env"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Token != "gho_fallback" || result.Source != "environment" {
		t.Errorf("got token=%q source=%q, want token=%q source=%q", result.Token, result.Source, "gho_fallback", "environment")
	}
}

func TestResolve_GitLab_EnvVar(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "glpat_test")
	result, err := Resolve(glOpts("", ""))
	if err != nil {
		t.Fatal(err)
	}
	if result.Token != "glpat_test" || result.Source != "environment" {
		t.Errorf("got token=%q source=%q, want token=%q source=%q", result.Token, result.Source, "glpat_test", "environment")
	}
}

func TestResolve_GitLab_NotPickedUpByGitHub(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "glpat_test")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	_, err := Resolve(ghOpts("", "/nonexistent/.devlake.env"))
	if err == nil {
		t.Error("expected error when GITLAB_TOKEN set but resolving with GitHub env vars")
	}
}

func TestResolve_AzureDevOps_EnvVar(t *testing.T) {
	t.Setenv("AZURE_DEVOPS_PAT", "ado_test")
	result, err := Resolve(adoOpts("", ""))
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
	result, err := Resolve(ghOpts("", envFile))
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
	result, err := Resolve(glOpts("", envFile))
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
		name      string
		opts      ResolveOpts
		wantToken string
	}{
		{"github", ghOpts("", envFile), "ghp_multi"},
		{"gitlab", glOpts("", envFile), "glpat_multi"},
		{"azure-devops", adoOpts("", envFile), "ado_multi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Resolve(tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if result.Token != tt.wantToken {
				t.Errorf("got token=%q, want %q", result.Token, tt.wantToken)
			}
			if result.Source != "envfile" {
				t.Errorf("got source=%q, want envfile", result.Source)
			}
		})
	}
}
