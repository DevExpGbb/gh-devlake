package cmd

import (
	"fmt"
	"os"
	"testing"
)

func TestBuildCreateRequest_RateLimit(t *testing.T) {
	tests := []struct {
		name     string
		rate     int
		wantRate int
	}{
		{"github uses 4500", 4500, 4500},
		{"gh-copilot uses 5000", 5000, 5000},
		{"zero uses default 4500", 0, 4500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := &ConnectionDef{Endpoint: "https://api.github.com/", RateLimitPerHour: tt.rate}
			req := def.BuildCreateRequest("test", ConnectionParams{Token: "tok"})
			if req.RateLimitPerHour != tt.wantRate {
				t.Errorf("got rate limit %d, want %d", req.RateLimitPerHour, tt.wantRate)
			}
		})
	}
}

func TestBuildCreateRequest_EnterpriseOrg(t *testing.T) {
	def := &ConnectionDef{
		Plugin:          "gh-copilot",
		Endpoint:        "https://api.github.com/",
		NeedsOrg:        true,
		NeedsEnterprise: true,
	}

	t.Run("org and enterprise", func(t *testing.T) {
		req := def.BuildCreateRequest("test", ConnectionParams{
			Token:      "tok",
			Org:        "my-org",
			Enterprise: "my-ent",
		})
		if req.Organization != "my-org" {
			t.Errorf("got Organization %q, want %q", req.Organization, "my-org")
		}
		if req.Enterprise != "my-ent" {
			t.Errorf("got Enterprise %q, want %q", req.Enterprise, "my-ent")
		}
	})

	t.Run("org only", func(t *testing.T) {
		req := def.BuildCreateRequest("test", ConnectionParams{
			Token: "tok",
			Org:   "my-org",
		})
		if req.Organization != "my-org" {
			t.Errorf("got Organization %q, want %q", req.Organization, "my-org")
		}
		if req.Enterprise != "" {
			t.Errorf("got Enterprise %q, want empty", req.Enterprise)
		}
	})

	t.Run("enterprise only", func(t *testing.T) {
		req := def.BuildCreateRequest("test", ConnectionParams{
			Token:      "tok",
			Enterprise: "my-ent",
		})
		if req.Organization != "" {
			t.Errorf("got Organization %q, want empty", req.Organization)
		}
		if req.Enterprise != "my-ent" {
			t.Errorf("got Enterprise %q, want %q", req.Enterprise, "my-ent")
		}
	})
}

func TestBuildTestRequest_CopilotFields(t *testing.T) {
	def := &ConnectionDef{
		Plugin:           "gh-copilot",
		Endpoint:         "https://api.github.com/",
		NeedsOrg:         true,
		NeedsEnterprise:  true,
		RateLimitPerHour: 5000,
	}

	t.Run("includes org and enterprise", func(t *testing.T) {
		req := def.BuildTestRequest("test", ConnectionParams{
			Token:      "tok",
			Org:        "my-org",
			Enterprise: "my-ent",
		})
		if req.Organization != "my-org" {
			t.Errorf("got Organization %q, want %q", req.Organization, "my-org")
		}
		if req.Enterprise != "my-ent" {
			t.Errorf("got Enterprise %q, want %q", req.Enterprise, "my-ent")
		}
		if req.RateLimitPerHour != 5000 {
			t.Errorf("got rate limit %d, want 5000", req.RateLimitPerHour)
		}
		if req.Name != "test" {
			t.Errorf("got Name %q, want %q", req.Name, "test")
		}
	})

	t.Run("github does not include org/enterprise", func(t *testing.T) {
		ghDef := &ConnectionDef{
			Plugin:           "github",
			Endpoint:         "https://api.github.com/",
			SupportsTest:     true,
			RateLimitPerHour: 4500,
			EnableGraphql:    true,
		}
		req := ghDef.BuildTestRequest("test", ConnectionParams{
			Token:      "tok",
			Org:        "ignored",
			Enterprise: "ignored",
		})
		if req.Organization != "" {
			t.Errorf("github test request should not have Organization, got %q", req.Organization)
		}
		if req.Enterprise != "" {
			t.Errorf("github test request should not have Enterprise, got %q", req.Enterprise)
		}
		if req.RateLimitPerHour != 4500 {
			t.Errorf("got rate limit %d, want 4500", req.RateLimitPerHour)
		}
		if !req.EnableGraphql {
			t.Error("github test request should have EnableGraphql=true")
		}
	})
}

// TestAvailablePluginsScopeHints verifies that all available plugins have non-empty
// RequiredScopes and ScopeHint fields so users always see what PAT scopes are needed,
// except for plugins that use API tokens instead of OAuth (e.g., Jira, Jenkins).
func TestAvailablePluginsScopeHints(t *testing.T) {
	for _, def := range AvailableConnections() {
		// Plugins using BasicAuth (e.g., Jenkins) don't have OAuth PAT scopes
		if def.AuthMethod == "BasicAuth" {
			continue
		}
		// Plugins that use API tokens instead of OAuth PATs may have empty scopes
		if len(def.RequiredScopes) == 0 {
			continue
		}
		if def.ScopeHint == "" {
			t.Errorf("plugin %q has empty ScopeHint", def.Plugin)
		}
	}
}

// TestJiraConnectionDef verifies the Jira plugin registry entry.
func TestJiraConnectionDef(t *testing.T) {
	def := FindConnectionDef("jira")
	if def == nil {
		t.Fatal("jira plugin not found in registry")
	}

	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"Plugin", def.Plugin, "jira"},
		{"DisplayName", def.DisplayName, "Jira"},
		{"Available", def.Available, true},
		{"Endpoint", def.Endpoint, ""},
		{"SupportsTest", def.SupportsTest, true},
		{"AuthMethod", def.AuthMethod, "AccessToken"},
		{"ScopeIDField", def.ScopeIDField, "boardId"},
		{"HasRepoScopes", def.HasRepoScopes, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s: got %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}

	if def.ScopeFunc == nil {
		t.Error("ScopeFunc should not be nil")
	}

	// Jira API tokens don't use OAuth/PAT scopes
	if len(def.RequiredScopes) != 0 {
		t.Errorf("RequiredScopes should be empty for Jira API tokens, got %v", def.RequiredScopes)
	}
	if def.ScopeHint != "" {
		t.Errorf("ScopeHint should be empty for Jira API tokens, got %q", def.ScopeHint)
	}

	expectedEnvVars := []string{"JIRA_TOKEN", "JIRA_API_TOKEN"}
	if len(def.EnvVarNames) != len(expectedEnvVars) {
		t.Errorf("EnvVarNames length: got %d, want %d", len(def.EnvVarNames), len(expectedEnvVars))
	} else {
		for i, v := range expectedEnvVars {
			if def.EnvVarNames[i] != v {
				t.Errorf("EnvVarNames[%d]: got %q, want %q", i, def.EnvVarNames[i], v)
			}
		}
	}

	expectedEnvFileKeys := []string{"JIRA_TOKEN", "JIRA_API_TOKEN"}
	if len(def.EnvFileKeys) != len(expectedEnvFileKeys) {
		t.Errorf("EnvFileKeys length: got %d, want %d", len(def.EnvFileKeys), len(expectedEnvFileKeys))
	} else {
		for i, v := range expectedEnvFileKeys {
			if def.EnvFileKeys[i] != v {
				t.Errorf("EnvFileKeys[%d]: got %q, want %q", i, def.EnvFileKeys[i], v)
			}
		}
	}
}

// TestAzureDevOpsRegistryEntry verifies the Azure DevOps plugin registry entry.
func TestAzureDevOpsRegistryEntry(t *testing.T) {
	def := FindConnectionDef("azuredevops_go")
	if def == nil {
		t.Fatal("azuredevops_go plugin not found in registry")
	}

	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"Plugin", def.Plugin, "azuredevops_go"},
		{"DisplayName", def.DisplayName, "Azure DevOps"},
		{"Available", def.Available, true},
		{"Endpoint", def.Endpoint, ""},
		{"NeedsOrg", def.NeedsOrg, true},
		{"SupportsTest", def.SupportsTest, true},
		{"AuthMethod", def.authMethod(), "AccessToken"},
		{"ScopeIDField", def.ScopeIDField, "id"},
		{"HasRepoScopes", def.HasRepoScopes, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s: got %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}

	if def.ScopeFunc == nil {
		t.Error("ScopeFunc should not be nil")
	}
	if len(def.RequiredScopes) != 0 {
		t.Errorf("RequiredScopes should be empty, got %v", def.RequiredScopes)
	}
	if def.ScopeHint != "" {
		t.Errorf("ScopeHint should be empty, got %q", def.ScopeHint)
	}

	expectedEnvVars := []string{"AZURE_DEVOPS_PAT", "AZDO_PAT"}
	if len(def.EnvVarNames) != len(expectedEnvVars) {
		t.Fatalf("EnvVarNames length: got %d, want %d", len(def.EnvVarNames), len(expectedEnvVars))
	}
	for i, v := range expectedEnvVars {
		if def.EnvVarNames[i] != v {
			t.Errorf("EnvVarNames[%d]: got %q, want %q", i, def.EnvVarNames[i], v)
		}
	}

	expectedEnvFileKeys := []string{"AZURE_DEVOPS_PAT", "AZDO_PAT"}
	if len(def.EnvFileKeys) != len(expectedEnvFileKeys) {
		t.Fatalf("EnvFileKeys length: got %d, want %d", len(def.EnvFileKeys), len(expectedEnvFileKeys))
	}
	for i, v := range expectedEnvFileKeys {
		if def.EnvFileKeys[i] != v {
			t.Errorf("EnvFileKeys[%d]: got %q, want %q", i, def.EnvFileKeys[i], v)
		}
	}
}

// TestBuildCreateRequest_AuthMethod verifies that AuthMethod defaults to "AccessToken"
// when empty, and uses the configured value when set.
func TestBuildCreateRequest_AuthMethod(t *testing.T) {
	tests := []struct {
		name           string
		authMethod     string
		wantAuthMethod string
	}{
		{"empty defaults to AccessToken", "", "AccessToken"},
		{"explicit AccessToken", "AccessToken", "AccessToken"},
		{"BasicAuth", "BasicAuth", "BasicAuth"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := &ConnectionDef{
				Endpoint:   "https://example.com/",
				AuthMethod: tt.authMethod,
			}
			req := def.BuildCreateRequest("test", ConnectionParams{Token: "tok"})
			if req.AuthMethod != tt.wantAuthMethod {
				t.Errorf("got AuthMethod %q, want %q", req.AuthMethod, tt.wantAuthMethod)
			}
		})
	}
}

// TestBuildTestRequest_AuthMethod mirrors TestBuildCreateRequest_AuthMethod for the test request.
func TestBuildTestRequest_AuthMethod(t *testing.T) {
	tests := []struct {
		name           string
		authMethod     string
		wantAuthMethod string
	}{
		{"empty defaults to AccessToken", "", "AccessToken"},
		{"explicit AccessToken", "AccessToken", "AccessToken"},
		{"BasicAuth", "BasicAuth", "BasicAuth"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := &ConnectionDef{
				Endpoint:   "https://example.com/",
				AuthMethod: tt.authMethod,
			}
			req := def.BuildTestRequest("test", ConnectionParams{Token: "tok"})
			if req.AuthMethod != tt.wantAuthMethod {
				t.Errorf("got AuthMethod %q, want %q", req.AuthMethod, tt.wantAuthMethod)
			}
		})
	}
}

// TestBuildCreateRequest_BasicAuth verifies that Username and Password are set
// when NeedsUsername is true and a Username is provided in ConnectionParams.
func TestBuildCreateRequest_BasicAuth(t *testing.T) {
	def := &ConnectionDef{
		Plugin:        "jenkins",
		Endpoint:      "https://jenkins.example.com/",
		AuthMethod:    "BasicAuth",
		NeedsUsername: true,
	}

	t.Run("username and password populated, token empty", func(t *testing.T) {
		req := def.BuildCreateRequest("test", ConnectionParams{
			Token:    "mypassword",
			Username: "admin",
		})
		if req.Username != "admin" {
			t.Errorf("got Username %q, want %q", req.Username, "admin")
		}
		if req.Password != "mypassword" {
			t.Errorf("got Password %q, want %q", req.Password, "mypassword")
		}
		if req.Token != "" {
			t.Errorf("expected empty Token for BasicAuth, got %q", req.Token)
		}
		if req.AuthMethod != "BasicAuth" {
			t.Errorf("got AuthMethod %q, want %q", req.AuthMethod, "BasicAuth")
		}
	})

	t.Run("no username = token field used instead", func(t *testing.T) {
		req := def.BuildCreateRequest("test", ConnectionParams{Token: "tok"})
		if req.Username != "" {
			t.Errorf("expected empty Username, got %q", req.Username)
		}
		if req.Password != "" {
			t.Errorf("expected empty Password, got %q", req.Password)
		}
		if req.Token != "tok" {
			t.Errorf("expected Token %q, got %q", "tok", req.Token)
		}
	})
}

// TestBuildTestRequest_BasicAuth mirrors TestBuildCreateRequest_BasicAuth for the test request.
func TestBuildTestRequest_BasicAuth(t *testing.T) {
	def := &ConnectionDef{
		Plugin:        "jenkins",
		Endpoint:      "https://jenkins.example.com/",
		AuthMethod:    "BasicAuth",
		NeedsUsername: true,
	}

	t.Run("username and password populated, token empty", func(t *testing.T) {
		req := def.BuildTestRequest("test", ConnectionParams{
			Token:    "mypassword",
			Username: "admin",
		})
		if req.Username != "admin" {
			t.Errorf("got Username %q, want %q", req.Username, "admin")
		}
		if req.Password != "mypassword" {
			t.Errorf("got Password %q, want %q", req.Password, "mypassword")
		}
		if req.Token != "" {
			t.Errorf("expected empty Token for BasicAuth, got %q", req.Token)
		}
		if req.AuthMethod != "BasicAuth" {
			t.Errorf("got AuthMethod %q, want %q", req.AuthMethod, "BasicAuth")
		}
	})

	t.Run("no username = token field used instead", func(t *testing.T) {
		req := def.BuildTestRequest("test", ConnectionParams{Token: "tok"})
		if req.Username != "" {
			t.Errorf("expected empty Username, got %q", req.Username)
		}
		if req.Password != "" {
			t.Errorf("expected empty Password, got %q", req.Password)
		}
		if req.Token != "tok" {
			t.Errorf("expected Token %q, got %q", "tok", req.Token)
		}
	})
}

// TestGitLabRegistryEntry verifies the GitLab plugin is correctly defined in the registry.
func TestGitLabRegistryEntry(t *testing.T) {
	def := FindConnectionDef("gitlab")
	if def == nil {
		t.Fatal("gitlab not found in registry")
	}

	t.Run("is available", func(t *testing.T) {
		if !def.Available {
			t.Error("gitlab should be Available=true")
		}
	})

	t.Run("has correct endpoint", func(t *testing.T) {
		want := "https://gitlab.com/api/v4/"
		if def.Endpoint != want {
			t.Errorf("got Endpoint %q, want %q", def.Endpoint, want)
		}
	})

	t.Run("supports test", func(t *testing.T) {
		if !def.SupportsTest {
			t.Error("gitlab should have SupportsTest=true")
		}
	})

	t.Run("has scope func", func(t *testing.T) {
		if def.ScopeFunc == nil {
			t.Error("gitlab ScopeFunc should not be nil")
		}
	})

	t.Run("has correct scope id field", func(t *testing.T) {
		if def.ScopeIDField != "gitlabId" {
			t.Errorf("got ScopeIDField %q, want %q", def.ScopeIDField, "gitlabId")
		}
	})

	t.Run("has repo scopes", func(t *testing.T) {
		if !def.HasRepoScopes {
			t.Error("gitlab should have HasRepoScopes=true")
		}
	})

	t.Run("has required scopes", func(t *testing.T) {
		if len(def.RequiredScopes) == 0 {
			t.Error("gitlab should have RequiredScopes")
		}
	})

	t.Run("has scope hint", func(t *testing.T) {
		if def.ScopeHint == "" {
			t.Error("gitlab should have a non-empty ScopeHint")
		}
	})

	t.Run("has env var names", func(t *testing.T) {
		if len(def.EnvVarNames) == 0 {
			t.Error("gitlab should have EnvVarNames")
		}
	})

	t.Run("uses access token auth", func(t *testing.T) {
		if def.authMethod() != "AccessToken" {
			t.Errorf("got AuthMethod %q, want %q", def.authMethod(), "AccessToken")
		}
	})

	t.Run("build create request uses token field", func(t *testing.T) {
		req := def.BuildCreateRequest("test-conn", ConnectionParams{Token: "glpat-test123"})
		if req.Token != "glpat-test123" {
			t.Errorf("got Token %q, want %q", req.Token, "glpat-test123")
		}
		if req.Endpoint != def.Endpoint {
			t.Errorf("got Endpoint %q, want %q", req.Endpoint, def.Endpoint)
		}
		if req.RateLimitPerHour != 3600 {
			t.Errorf("got RateLimitPerHour %d, want 3600", req.RateLimitPerHour)
		}
	})

	t.Run("build create request accepts custom endpoint", func(t *testing.T) {
		customEndpoint := "https://gitlab.mycompany.com/api/v4/"
		req := def.BuildCreateRequest("self-hosted", ConnectionParams{
			Token:    "glpat-test",
			Endpoint: customEndpoint,
		})
		if req.Endpoint != customEndpoint {
			t.Errorf("got Endpoint %q, want %q", req.Endpoint, customEndpoint)
		}
	})
}

// TestNeedsTokenExpiry verifies that the NeedsTokenExpiry field is set on the
// github and gh-copilot registry entries (and not on others).
func TestNeedsTokenExpiry(t *testing.T) {
	tests := []struct {
		plugin string
		want   bool
	}{
		{"github", true},
		{"gh-copilot", true},
		{"gitlab", false},
		{"azuredevops_go", false},
	}
	for _, tt := range tests {
		t.Run(tt.plugin, func(t *testing.T) {
			def := FindConnectionDef(tt.plugin)
			if def == nil {
				t.Fatalf("plugin %q not found in registry", tt.plugin)
			}
			if def.NeedsTokenExpiry != tt.want {
				t.Errorf("plugin %q: NeedsTokenExpiry=%v, want %v", tt.plugin, def.NeedsTokenExpiry, tt.want)
			}
		})
	}
}

// TestLooksLikeZeroDateTokenExpiresAt verifies the helper detects zero-date errors.
func TestLooksLikeZeroDateTokenExpiresAt(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"unrelated error", fmt.Errorf("something else"), false},
		{"zero date error", fmt.Errorf("token_expires_at: 0000-00-00 is invalid"), true},
		{"only token_expires_at", fmt.Errorf("token_expires_at is bad"), false},
		{"only 0000-00-00", fmt.Errorf("date 0000-00-00 is not valid"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeZeroDateTokenExpiresAt(tt.err)
			if got != tt.want {
				t.Errorf("got %v, want %v for error: %v", got, tt.want, tt.err)
			}
		})
	}
}

// TestResolveUsername covers the flag ΓåÆ env file ΓåÆ env var resolution paths for resolveUsername.
func TestResolveUsername(t *testing.T) {
	def := &ConnectionDef{
		Plugin:              "jenkins",
		DisplayName:         "Jenkins",
		NeedsUsername:       true,
		UsernameEnvVars:     []string{"JENKINS_USER", "JENKINS_USERNAME"},
		UsernameEnvFileKeys: []string{"JENKINS_USER"},
	}

	t.Run("flag value takes priority over everything", func(t *testing.T) {
		t.Setenv("JENKINS_USER", "env-user")
		got := resolveUsername(def, "flag-user", "/nonexistent/.devlake.env")
		if got != "flag-user" {
			t.Errorf("got %q, want %q", got, "flag-user")
		}
	})

	t.Run("env file key resolved when no flag", func(t *testing.T) {
		envFile := t.TempDir() + "/.devlake.env"
		if err := os.WriteFile(envFile, []byte("JENKINS_USER=envfile-user\n"), 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("JENKINS_USER", "")
		t.Setenv("JENKINS_USERNAME", "")
		got := resolveUsername(def, "", envFile)
		if got != "envfile-user" {
			t.Errorf("got %q, want %q", got, "envfile-user")
		}
	})

	t.Run("first env var used when no flag or env file", func(t *testing.T) {
		t.Setenv("JENKINS_USER", "primary-env-user")
		t.Setenv("JENKINS_USERNAME", "secondary-env-user")
		got := resolveUsername(def, "", "/nonexistent/.devlake.env")
		if got != "primary-env-user" {
			t.Errorf("got %q, want %q", got, "primary-env-user")
		}
	})

	t.Run("second env var used when first is empty", func(t *testing.T) {
		t.Setenv("JENKINS_USER", "")
		t.Setenv("JENKINS_USERNAME", "secondary-env-user")
		got := resolveUsername(def, "", "/nonexistent/.devlake.env")
		if got != "secondary-env-user" {
			t.Errorf("got %q, want %q", got, "secondary-env-user")
		}
	})

	t.Run("env file takes priority over env vars", func(t *testing.T) {
		envFile := t.TempDir() + "/.devlake.env"
		if err := os.WriteFile(envFile, []byte("JENKINS_USER=envfile-wins\n"), 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("JENKINS_USER", "env-loses")
		got := resolveUsername(def, "", envFile)
		if got != "envfile-wins" {
			t.Errorf("got %q, want %q", got, "envfile-wins")
		}
	})
}

// TestConnectionRegistry_Bitbucket verifies the Bitbucket Cloud plugin registry entry.
func TestConnectionRegistry_Bitbucket(t *testing.T) {
	def := FindConnectionDef("bitbucket")
	if def == nil {
		t.Fatal("bitbucket connection def not found")
	}
	if !def.Available {
		t.Errorf("bitbucket should be available")
	}
	if def.AuthMethod != "BasicAuth" {
		t.Errorf("bitbucket AuthMethod = %q, want BasicAuth", def.AuthMethod)
	}
	if !def.NeedsUsername {
		t.Errorf("bitbucket NeedsUsername should be true")
	}
	if def.ScopeIDField != "bitbucketId" {
		t.Errorf("bitbucket ScopeIDField = %q, want %q", def.ScopeIDField, "bitbucketId")
	}
	if !def.HasRepoScopes {
		t.Errorf("bitbucket HasRepoScopes should be true")
	}
	if def.ScopeFunc == nil {
		t.Errorf("bitbucket ScopeFunc should be set")
	}
	wantEnvVars := []string{"BITBUCKET_TOKEN", "BITBUCKET_APP_PASSWORD"}
	if len(def.EnvVarNames) != len(wantEnvVars) {
		t.Errorf("bitbucket EnvVarNames length: got %d, want %d", len(def.EnvVarNames), len(wantEnvVars))
	} else {
		for i, v := range wantEnvVars {
			if def.EnvVarNames[i] != v {
				t.Errorf("bitbucket EnvVarNames[%d]: got %q, want %q", i, def.EnvVarNames[i], v)
			}
		}
	}
	wantUserEnvVars := []string{"BITBUCKET_USER", "BITBUCKET_USERNAME"}
	if len(def.UsernameEnvVars) != len(wantUserEnvVars) {
		t.Errorf("bitbucket UsernameEnvVars length: got %d, want %d", len(def.UsernameEnvVars), len(wantUserEnvVars))
	} else {
		for i, v := range wantUserEnvVars {
			if def.UsernameEnvVars[i] != v {
				t.Errorf("bitbucket UsernameEnvVars[%d]: got %q, want %q", i, def.UsernameEnvVars[i], v)
			}
		}
	}

	// ScopeFlags: repos and repos-file flags must be registered
	foundRepos, foundReposFile := false, false
	for _, f := range def.ScopeFlags {
		switch f.Name {
		case "repos":
			foundRepos = true
		case "repos-file":
			foundReposFile = true
		}
	}
	if !foundRepos {
		t.Errorf("bitbucket ScopeFlags should include repos flag")
	}
	if !foundReposFile {
		t.Errorf("bitbucket ScopeFlags should include repos-file flag")
	}

	// BasicAuth: BuildCreateRequest puts credentials into username/password, not token
	req := def.BuildCreateRequest("test-conn", ConnectionParams{
		Token:    "app-password",
		Username: "myuser",
	})
	if req.Username != "myuser" {
		t.Errorf("bitbucket create request Username = %q, want %q", req.Username, "myuser")
	}
	if req.Password != "app-password" {
		t.Errorf("bitbucket create request Password = %q, want %q", req.Password, "app-password")
	}
	if req.Token != "" {
		t.Errorf("bitbucket create request Token should be empty for BasicAuth, got %q", req.Token)
	}
}

func TestConnectionRegistry_Jenkins(t *testing.T) {
	def := FindConnectionDef("jenkins")
	if def == nil {
		t.Fatal("jenkins connection def not found")
	}
	if !def.Available {
		t.Errorf("jenkins should be available")
	}
	if def.AuthMethod != "BasicAuth" {
		t.Errorf("jenkins AuthMethod = %q, want BasicAuth", def.AuthMethod)
	}
	if !def.NeedsUsername {
		t.Errorf("jenkins NeedsUsername should be true")
	}
	if def.ScopeIDField != "fullName" {
		t.Errorf("jenkins ScopeIDField = %q, want %q", def.ScopeIDField, "fullName")
	}
	if def.ScopeFunc == nil {
		t.Errorf("jenkins ScopeFunc should be set")
	}
	foundJobs := false
	for _, f := range def.ScopeFlags {
		if f.Name == "jobs" {
			foundJobs = true
			break
		}
	}
	if !foundJobs {
		t.Errorf("jenkins ScopeFlags should include jobs flag")
	}
}

// TestConnectionRegistry_SonarQube verifies the SonarQube plugin registry entry.
func TestConnectionRegistry_SonarQube(t *testing.T) {
	def := FindConnectionDef("sonarqube")
	if def == nil {
		t.Fatal("sonarqube plugin not found in registry")
	}

	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"Plugin", def.Plugin, "sonarqube"},
		{"DisplayName", def.DisplayName, "SonarQube"},
		{"Available", def.Available, true},
		{"Endpoint", def.Endpoint, ""},
		{"SupportsTest", def.SupportsTest, true},
		{"AuthMethod", def.AuthMethod, "AccessToken"},
		{"ScopeIDField", def.ScopeIDField, "projectKey"},
		{"HasRepoScopes", def.HasRepoScopes, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s: got %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}

	if def.ScopeFunc == nil {
		t.Error("ScopeFunc should not be nil")
	}

	foundProjectsFlag := false
	for _, f := range def.ScopeFlags {
		if f.Name == "projects" {
			foundProjectsFlag = true
			break
		}
	}
	if !foundProjectsFlag {
		t.Errorf("ScopeFlags should include projects flag")
	}

	// SonarQube uses API tokens, not OAuth/PAT scopes
	if len(def.RequiredScopes) != 0 {
		t.Errorf("RequiredScopes should be empty for SonarQube API tokens, got %v", def.RequiredScopes)
	}
	if def.ScopeHint != "" {
		t.Errorf("ScopeHint should be empty for SonarQube API tokens, got %q", def.ScopeHint)
	}

	expectedEnvVars := []string{"SONARQUBE_TOKEN", "SONAR_TOKEN"}
	if len(def.EnvVarNames) != len(expectedEnvVars) {
		t.Errorf("EnvVarNames length: got %d, want %d", len(def.EnvVarNames), len(expectedEnvVars))
	} else {
		for i, v := range expectedEnvVars {
			if def.EnvVarNames[i] != v {
				t.Errorf("EnvVarNames[%d]: got %q, want %q", i, def.EnvVarNames[i], v)
			}
		}
	}

	expectedEnvFileKeys := []string{"SONARQUBE_TOKEN", "SONAR_TOKEN"}
	if len(def.EnvFileKeys) != len(expectedEnvFileKeys) {
		t.Errorf("EnvFileKeys length: got %d, want %d", len(def.EnvFileKeys), len(expectedEnvFileKeys))
	} else {
		for i, v := range expectedEnvFileKeys {
			if def.EnvFileKeys[i] != v {
				t.Errorf("EnvFileKeys[%d]: got %q, want %q", i, def.EnvFileKeys[i], v)
			}
		}
	}
}

// TestConnectionRegistry_ArgoCD verifies the ArgoCD plugin registry entry.
func TestConnectionRegistry_ArgoCD(t *testing.T) {
	def := FindConnectionDef("argocd")
	if def == nil {
		t.Fatal("argocd plugin not found in registry")
	}

	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"Plugin", def.Plugin, "argocd"},
		{"DisplayName", def.DisplayName, "ArgoCD"},
		{"Available", def.Available, true},
		{"Endpoint", def.Endpoint, ""},
		{"SupportsTest", def.SupportsTest, true},
		{"AuthMethod", def.AuthMethod, "AccessToken"},
		{"ScopeIDField", def.ScopeIDField, "name"},
		{"HasRepoScopes", def.HasRepoScopes, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s: got %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}

	if def.ScopeFunc == nil {
		t.Error("ScopeFunc should not be nil")
	}

	// ArgoCD uses auth tokens, not OAuth/PAT scopes
	if len(def.RequiredScopes) != 0 {
		t.Errorf("RequiredScopes should be empty for ArgoCD auth tokens, got %v", def.RequiredScopes)
	}
	if def.ScopeHint != "" {
		t.Errorf("ScopeHint should be empty for ArgoCD auth tokens, got %q", def.ScopeHint)
	}

	expectedEnvVars := []string{"ARGOCD_TOKEN", "ARGOCD_AUTH_TOKEN"}
	if len(def.EnvVarNames) != len(expectedEnvVars) {
		t.Errorf("EnvVarNames length: got %d, want %d", len(def.EnvVarNames), len(expectedEnvVars))
	} else {
		for i, v := range expectedEnvVars {
			if def.EnvVarNames[i] != v {
				t.Errorf("EnvVarNames[%d]: got %q, want %q", i, def.EnvVarNames[i], v)
			}
		}
	}

	expectedEnvFileKeys := []string{"ARGOCD_TOKEN", "ARGOCD_AUTH_TOKEN"}
	if len(def.EnvFileKeys) != len(expectedEnvFileKeys) {
		t.Errorf("EnvFileKeys length: got %d, want %d", len(def.EnvFileKeys), len(expectedEnvFileKeys))
	} else {
		for i, v := range expectedEnvFileKeys {
			if def.EnvFileKeys[i] != v {
				t.Errorf("EnvFileKeys[%d]: got %q, want %q", i, def.EnvFileKeys[i], v)
			}
		}
	}
}
