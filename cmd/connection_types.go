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
}

// MenuLabel returns the label for interactive menus.
func (d *ConnectionDef) MenuLabel() string {
	if !d.Available {
		return fmt.Sprintf("%s (coming soon)", d.DisplayName)
	}
	return d.DisplayName
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
	req := &devlake.ConnectionCreateRequest{
		Name:             name,
		Endpoint:         endpoint,
		Proxy:            params.Proxy,
		AuthMethod:       "AccessToken",
		Token:            params.Token,
		RateLimitPerHour: 4500,
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
	req := &devlake.ConnectionTestRequest{
		Endpoint:         endpoint,
		AuthMethod:       "AccessToken",
		Token:            params.Token,
		RateLimitPerHour: 4500,
		Proxy:            params.Proxy,
	}
	if d.Plugin == "github" {
		req.EnableGraphql = true
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
	},
	{
		Plugin:          "gh-copilot",
		DisplayName:     "GitHub Copilot",
		Available:       true,
		Endpoint:        "https://api.github.com/",
		NeedsOrg:        true,
		NeedsEnterprise: true,
		RequiredScopes:  []string{"manage_billing:copilot", "read:org"},
		ScopeHint:       "manage_billing:copilot, read:org (+ read:enterprise for enterprise metrics)",
	},
	{
		Plugin:      "gitlab",
		DisplayName: "GitLab",
		Available:   false,
	},
	{
		Plugin:      "azure-devops",
		DisplayName: "Azure DevOps",
		Available:   false,
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
// When interactive is true, prompts for connection name and optional proxy.
func buildAndCreateConnection(client *devlake.Client, def *ConnectionDef, params ConnectionParams, org string, interactive bool) (*ConnSetupResult, error) {
	connName := params.Name
	if connName == "" {
		connName = def.defaultConnName(org)
	}

	// Interactive: let user customise name, proxy, endpoint
	if interactive {
		custom := prompt.ReadLine(fmt.Sprintf("Connection name [%s]", connName))
		if custom != "" {
			connName = custom
		}

		if def.Plugin == "github" && params.Endpoint == "" {
			endpointChoices := []string{
				"cloud  - GitHub.com (https://api.github.com/)",
				"server - GitHub Enterprise Server (custom URL)",
			}
			picked := prompt.Select("GitHub environment", endpointChoices)
			if strings.HasPrefix(picked, "server") {
				params.Endpoint = prompt.ReadLine("GitHub Enterprise Server API URL (e.g. https://github.example.com/api/v3/)")
			}
		}

		if params.Proxy == "" {
			params.Proxy = prompt.ReadLine("HTTP proxy [none]")
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
		testReq := def.BuildTestRequest(params)
		testResult, err := client.TestConnection(def.Plugin, testReq)
		if err != nil {
			return nil, fmt.Errorf("%s connection test failed: %w", def.DisplayName, err)
		}
		if !testResult.Success {
			return nil, fmt.Errorf("%s connection test failed: %s", def.DisplayName, testResult.Message)
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
