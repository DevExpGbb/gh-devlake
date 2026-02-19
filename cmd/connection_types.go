package cmd

import (
	"fmt"
	"strings"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/DevExpGBB/gh-devlake/internal/prompt"
)

// ConnectionDef describes a plugin connection type and how to create it.
type ConnectionDef struct {
	Plugin          string
	DisplayName     string
	Available       bool // false = coming soon
	Endpoint        string
	NeedsOrg        bool
	NeedsEnterprise bool
	SupportsTest    bool
	RequiredScopes  []string // PAT scopes needed for this plugin
	ScopeHint       string   // short hint for error messages
	EnvVarNames     []string // environment variable names (informational; resolution logic lives in token.Resolve)
	EnvFileKeys     []string // .devlake.env keys (informational; resolution logic lives in token.Resolve)
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

// BuildCreateRequest constructs the API payload for creating this connection.
func (d *ConnectionDef) BuildCreateRequest(name string, params ConnectionParams) *devlake.ConnectionCreateRequest {
	endpoint := d.Endpoint
	if params.Endpoint != "" {
		endpoint = params.Endpoint
	}
	rateLimitPerHour := 4500
	if d.Plugin == "gh-copilot" {
		rateLimitPerHour = 5000
	}
	req := &devlake.ConnectionCreateRequest{
		Name:             name,
		Endpoint:         endpoint,
		Proxy:            params.Proxy,
		AuthMethod:       "AccessToken",
		Token:            params.Token,
		RateLimitPerHour: rateLimitPerHour,
	}
	if d.Plugin == "github" {
		req.EnableGraphql = true
	}
	if d.NeedsOrg && params.Org != "" {
		req.Organization = params.Org
	}
	if d.NeedsEnterprise && params.Enterprise != "" {
		req.Enterprise = params.Enterprise
	}
	return req
}

// BuildTestRequest constructs the API payload for testing this connection.
func (d *ConnectionDef) BuildTestRequest(params ConnectionParams) *devlake.ConnectionTestRequest {
	endpoint := d.Endpoint
	if params.Endpoint != "" {
		endpoint = params.Endpoint
	}
	rateLimitPerHour := 4500
	if d.Plugin == "gh-copilot" {
		rateLimitPerHour = 5000
	}
	req := &devlake.ConnectionTestRequest{
		Endpoint:         endpoint,
		AuthMethod:       "AccessToken",
		Token:            params.Token,
		RateLimitPerHour: rateLimitPerHour,
		Proxy:            params.Proxy,
	}
	if d.Plugin == "github" {
		req.EnableGraphql = true
	}
	if d.NeedsOrg && params.Org != "" {
		req.Organization = params.Org
	}
	if d.NeedsEnterprise && params.Enterprise != "" {
		req.Enterprise = params.Enterprise
	}
	return req
}

// connectionRegistry is the ordered list of all known plugin connection types.
var connectionRegistry = []*ConnectionDef{
	{
		Plugin:         "github",
		DisplayName:    "GitHub",
		Available:      true,
		Endpoint:       "https://api.github.com/",
		SupportsTest:   true,
		RequiredScopes: []string{"repo", "read:org", "read:user"},
		ScopeHint:      "repo, read:org, read:user",
		EnvVarNames:    []string{"GITHUB_TOKEN", "GH_TOKEN"},
		EnvFileKeys:    []string{"GITHUB_PAT", "GITHUB_TOKEN", "GH_TOKEN"},
	},
	{
		Plugin:          "gh-copilot",
		DisplayName:     "GitHub Copilot",
		Available:       true,
		Endpoint:        "https://api.github.com/",
		NeedsOrg:        true,
		NeedsEnterprise: true,
		SupportsTest:    true,
		RequiredScopes:  []string{"manage_billing:copilot", "read:org"},
		ScopeHint:       "manage_billing:copilot, read:org (+ read:enterprise for enterprise metrics)",
		EnvVarNames:     []string{"GITHUB_TOKEN", "GH_TOKEN"},
		EnvFileKeys:     []string{"GITHUB_PAT", "GITHUB_TOKEN", "GH_TOKEN"},
	},
	{
		Plugin:      "gitlab",
		DisplayName: "GitLab",
		Available:   false,
		EnvVarNames: []string{"GITLAB_TOKEN"},
		EnvFileKeys: []string{"GITLAB_TOKEN"},
	},
	{
		Plugin:      "azure-devops",
		DisplayName: "Azure DevOps",
		Available:   false,
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
		fmt.Printf("   Connection already exists (ID=%d), skipping.\n", existing.ID)
		return &ConnSetupResult{
			Plugin:       def.Plugin,
			ConnectionID: existing.ID,
			Name:         existing.Name,
			Organization: org,
			Enterprise:   params.Enterprise,
		}, nil
	}

	if def.SupportsTest {
		fmt.Println("   🔑 Testing connection...")
		testReq := def.BuildTestRequest(params)
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
		return nil, fmt.Errorf("failed to create %s connection: %w", def.DisplayName, err)
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
