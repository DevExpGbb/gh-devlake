package cmd

import (
	"fmt"
	"strings"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/DevExpGBB/gh-devlake/internal/prompt"
)

// ScopeHandler is a function that configures scopes for a connection.
// It receives the client, connection ID, org, enterprise, and an options struct.
// opts may be nil when called interactively; the handler is responsible for
// prompting for defaults in that case.
// It returns the BlueprintConnection entry (for project creation) and an error.
type ScopeHandler func(client *devlake.Client, connID int, org, enterprise string, opts *ScopeOpts) (*devlake.BlueprintConnection, error)

// ConnectionDef describes a plugin connection type and how to create it.
type ConnectionDef struct {
	Plugin           string
	DisplayName      string
	Available        bool // false = coming soon
	Endpoint         string
	NeedsOrg         bool
	NeedsEnterprise  bool
	NeedsOrgOrEnt    bool
	SupportsTest     bool
	RateLimitPerHour int          // default rate limit; 0 = use 4500
	EnableGraphql    bool         // send enableGraphql=true in create/test payloads
	RequiredScopes   []string     // PAT scopes needed for this plugin
	ScopeHint        string       // short hint for error messages
	TokenPrompt      string       // label for masked token prompt (e.g. "GitHub PAT")
	OrgPrompt        string       // label for org prompt; empty = not prompted
	EnterprisePrompt string       // label for enterprise prompt; empty = not prompted
	EnvVarNames      []string     // environment variable names for token resolution
	EnvFileKeys      []string     // .devlake.env keys for token resolution
	ScopeFunc        ScopeHandler // nil = scope configuration not yet supported
}

// MenuLabel returns the label for interactive menus.
func (d *ConnectionDef) MenuLabel() string {
	if !d.Available {
		return fmt.Sprintf("%s (coming soon)", d.DisplayName)
	}
	return d.DisplayName
}

// scopeHintSuffix returns a formatted scope hint string for appending to error messages,
// or an empty string if no ScopeHint is set.
func (d *ConnectionDef) scopeHintSuffix() string {
	if d.ScopeHint == "" {
		return ""
	}
	return fmt.Sprintf("\n   💡 Ensure your PAT has these scopes: %s", d.ScopeHint)
}

// defaultConnName returns the default connection name for this plugin + org.
func (d *ConnectionDef) defaultConnName(org string) string {
	if org != "" {
		return fmt.Sprintf("%s - %s", d.DisplayName, org)
	}
	return d.DisplayName
}

// ConnectionParams holds user-supplied values for a single connection.
type ConnectionParams struct {
	Token      string
	Org        string
	Enterprise string
	Name       string // override default connection name
	Proxy      string // HTTP proxy URL
	Endpoint   string // override default endpoint (e.g. GitHub Enterprise Server)
}

// rateLimitOrDefault returns the configured rate limit or a sensible default.
func (d *ConnectionDef) rateLimitOrDefault() int {
	if d.RateLimitPerHour > 0 {
		return d.RateLimitPerHour
	}
	return 4500
}

// BuildCreateRequest constructs the API payload for creating this connection.
func (d *ConnectionDef) BuildCreateRequest(name string, params ConnectionParams) *devlake.ConnectionCreateRequest {
	endpoint := d.Endpoint
	if params.Endpoint != "" {
		endpoint = params.Endpoint
	}
	req := &devlake.ConnectionCreateRequest{
		Name:             name,
		Endpoint:         endpoint,
		Proxy:            params.Proxy,
		AuthMethod:       "AccessToken",
		Token:            params.Token,
		RateLimitPerHour: d.rateLimitOrDefault(),
		EnableGraphql:    d.EnableGraphql,
	}
	if (d.NeedsOrg || d.NeedsOrgOrEnt) && params.Org != "" {
		req.Organization = params.Org
	}
	if (d.NeedsEnterprise || d.NeedsOrgOrEnt) && params.Enterprise != "" {
		req.Enterprise = params.Enterprise
	}
	return req
}

// BuildTestRequest constructs the API payload for testing this connection.
func (d *ConnectionDef) BuildTestRequest(name string, params ConnectionParams) *devlake.ConnectionTestRequest {
	endpoint := d.Endpoint
	if params.Endpoint != "" {
		endpoint = params.Endpoint
	}
	req := &devlake.ConnectionTestRequest{
		Name:             name,
		Endpoint:         endpoint,
		AuthMethod:       "AccessToken",
		Token:            params.Token,
		RateLimitPerHour: d.rateLimitOrDefault(),
		Proxy:            params.Proxy,
		EnableGraphql:    d.EnableGraphql,
	}
	if (d.NeedsOrg || d.NeedsOrgOrEnt) && params.Org != "" {
		req.Organization = params.Org
	}
	if (d.NeedsEnterprise || d.NeedsOrgOrEnt) && params.Enterprise != "" {
		req.Enterprise = params.Enterprise
	}
	return req
}

// connectionRegistry is the ordered list of all known plugin connection types.
var connectionRegistry = []*ConnectionDef{
	{
		Plugin:           "github",
		DisplayName:      "GitHub",
		Available:        true,
		Endpoint:         "https://api.github.com/",
		SupportsTest:     true,
		RateLimitPerHour: 4500,
		EnableGraphql:    true,
		RequiredScopes:   []string{"repo", "read:org", "read:user"},
		ScopeHint:        "repo, read:org, read:user",
		TokenPrompt:      "GitHub PAT",
		EnvVarNames:      []string{"GITHUB_PAT", "GITHUB_TOKEN", "GH_TOKEN"},
		EnvFileKeys:      []string{"GITHUB_PAT", "GITHUB_TOKEN", "GH_TOKEN"},
		ScopeFunc:        scopeGitHubHandler,
	},
	{
		Plugin:           "gh-copilot",
		DisplayName:      "GitHub Copilot",
		Available:        true,
		Endpoint:         "https://api.github.com/",
		NeedsOrgOrEnt:    true,
		SupportsTest:     true,
		RateLimitPerHour: 5000,
		RequiredScopes:   []string{"manage_billing:copilot", "read:org"},
		ScopeHint:        "manage_billing:copilot, read:org (+ read:enterprise for enterprise metrics)",
		TokenPrompt:      "GitHub Copilot PAT",
		OrgPrompt:        "Organization slug (optional if enterprise provided)",
		EnterprisePrompt: "Enterprise slug (optional if org provided)",
		EnvVarNames:      []string{"GITHUB_PAT", "GITHUB_TOKEN", "GH_TOKEN"},
		EnvFileKeys:      []string{"GITHUB_PAT", "GITHUB_TOKEN", "GH_TOKEN"},
		ScopeFunc:        scopeCopilotHandler,
	},
	{
		Plugin:      "gitlab",
		DisplayName: "GitLab",
		Available:   false,
		TokenPrompt: "GitLab Token",
		OrgPrompt:   "GitLab group or project path",
		EnvVarNames: []string{"GITLAB_TOKEN"},
		EnvFileKeys: []string{"GITLAB_TOKEN"},
	},
	{
		Plugin:      "azure-devops",
		DisplayName: "Azure DevOps",
		Available:   false,
		TokenPrompt: "Azure DevOps PAT",
		OrgPrompt:   "Azure DevOps organization",
		EnvVarNames: []string{"AZURE_DEVOPS_PAT"},
		EnvFileKeys: []string{"AZURE_DEVOPS_PAT"},
	},
}

// AvailableConnections returns only available (non-coming-soon) connection defs.
func AvailableConnections() []*ConnectionDef {
	var out []*ConnectionDef
	for _, d := range connectionRegistry {
		if d.Available {
			out = append(out, d)
		}
	}
	return out
}

// FindConnectionDef returns the def for the given plugin slug, or nil.
func FindConnectionDef(plugin string) *ConnectionDef {
	for _, d := range connectionRegistry {
		if d.Plugin == plugin {
			return d
		}
	}
	return nil
}

// ConnSetupResult holds the outcome of setting up one connection.
type ConnSetupResult struct {
	Plugin       string
	ConnectionID int
	Name         string
	Organization string
	Enterprise   string
}

// buildAndCreateConnection creates or reuses an existing connection.
// When interactive is true, prompts for connection name (Enter accepts default).
func buildAndCreateConnection(client *devlake.Client, def *ConnectionDef, params ConnectionParams, org string, interactive bool) (*ConnSetupResult, error) {
	connName := params.Name
	if connName == "" {
		connName = def.defaultConnName(org)
	}

	// Interactive: let user customise connection name
	if interactive {
		custom := prompt.ReadLine(fmt.Sprintf("Connection name [%s]", connName))
		if custom != "" {
			connName = custom
		}
	}

	existing, _ := client.FindConnectionByName(def.Plugin, connName)
	if existing != nil {
		fmt.Printf("   Connection \"%s\" already exists (ID=%d).\n", existing.Name, existing.ID)
		useExisting := true
		if interactive {
			fmt.Println()
			useExisting = prompt.Confirm("   Use existing connection?")
		}
		if useExisting {
			fmt.Printf("   Using existing connection (ID=%d).\n", existing.ID)
			return &ConnSetupResult{
				Plugin:       def.Plugin,
				ConnectionID: existing.ID,
				Name:         existing.Name,
				Organization: org,
				Enterprise:   params.Enterprise,
			}, nil
		}
		fmt.Println()
		newName := prompt.ReadLine("   New connection name")
		if newName == "" {
			return nil, fmt.Errorf("connection name is required when creating a second %s connection", def.DisplayName)
		}
		connName = newName
	}

	if def.SupportsTest {
		fmt.Println("   🔑 Testing connection...")
		testReq := def.BuildTestRequest(connName, params)
		testResult, err := client.TestConnection(def.Plugin, testReq)
		if err != nil {
			return nil, fmt.Errorf("%s connection test failed: %w%s", def.DisplayName, err, def.scopeHintSuffix())
		}
		if !testResult.Success {
			return nil, fmt.Errorf("%s connection test failed: %s%s", def.DisplayName, testResult.Message, def.scopeHintSuffix())
		}
		fmt.Println("   ✅ Connection test passed")
	}

	createReq := def.BuildCreateRequest(connName, params)
	conn, err := client.CreateConnection(def.Plugin, createReq)
	if err != nil {
		// Workaround for older DevLake releases where the GitHub connections table
		// has a NOT NULL token_expires_at column that defaults to an invalid
		// zero-date (0000-00-00) under strict MySQL settings.
		//
		// PATs are effectively non-expiring, so use a far-future sentinel.
		if (def.Plugin == "github" || def.Plugin == "gh-copilot") && looksLikeZeroDateTokenExpiresAt(err) {
			fmt.Println("   ⚠️  DevLake rejected empty token expiry; retrying with a non-expiring sentinel...")
			createReq.TokenExpiresAt = "2099-01-01T00:00:00Z"
			createReq.RefreshTokenExpiresAt = "2099-01-01T00:00:00Z"
			conn, err = client.CreateConnection(def.Plugin, createReq)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to create %s connection: %w", def.DisplayName, err)
		}
	}
	fmt.Printf("   ✅ Created %s connection (ID=%d)\n", def.DisplayName, conn.ID)

	return &ConnSetupResult{
		Plugin:       def.Plugin,
		ConnectionID: conn.ID,
		Name:         conn.Name,
		Organization: org,
		Enterprise:   params.Enterprise,
	}, nil
}

func looksLikeZeroDateTokenExpiresAt(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "token_expires_at") && strings.Contains(msg, "0000-00-00")
}

// aggregateScopeHints merges scope hints from multiple connection defs into one string.
func aggregateScopeHints(defs []*ConnectionDef) string {
	seen := make(map[string]bool)
	var scopes []string
	for _, d := range defs {
		for _, s := range d.RequiredScopes {
			if !seen[s] {
				seen[s] = true
				scopes = append(scopes, s)
			}
		}
	}
	return strings.Join(scopes, ", ")
}
