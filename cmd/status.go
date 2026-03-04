package cmd

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show DevLake deployment summary and health",
	Long: `Displays a summary of the current DevLake deployment:
  • Endpoint health for each service
  • Configured plugin connections with display names
  • Project and scope configuration`,
	RunE: runStatus,
}

func init() {
	statusCmd.GroupID = "operate"
	rootCmd.AddCommand(statusCmd)
}

// statusOutput is the JSON representation of the status command output.
type statusOutput struct {
	Deployment  *statusDeployment  `json:"deployment"`
	Endpoints   []statusEndpoint   `json:"endpoints"`
	Connections []statusConnection `json:"connections"`
	Project     *statusProject     `json:"project"`
}

type statusDeployment struct {
	Method    string `json:"method"`
	StateFile string `json:"stateFile"`
}

type statusEndpoint struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Healthy bool   `json:"healthy"`
}

type statusConnection struct {
	Plugin       string `json:"plugin"`
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Organization string `json:"organization,omitempty"`
	Enterprise   string `json:"enterprise,omitempty"`
}

type statusProject struct {
	Name        string `json:"name"`
	BlueprintID int    `json:"blueprintId"`
}

func runStatus(cmd *cobra.Command, args []string) error {
	// ── Load state file ──
	var state *devlake.State
	var stateFile string
	cwd, _ := os.Getwd()
	for _, name := range []string{".devlake-azure.json", ".devlake-local.json"} {
		path := filepath.Join(cwd, name)
		s, err := devlake.LoadState(path)
		if err == nil && s != nil {
			state = s
			stateFile = name
			break
		}
	}

	// ── JSON output path ──
	if outputJSON {
		return runStatusJSON(state, stateFile)
	}

	printBanner("DevLake Status")
	sep := "  " + strings.Repeat("─", 38)

	if state == nil {
		disc, err := devlake.Discover(cfgURL)
		if err != nil {
			fmt.Println("\n  No state file found. Run 'gh devlake deploy' to get started.")
			return nil
		}
		client := devlake.NewClient(disc.URL)
		if _, herr := client.Health(); herr == nil {
			fmt.Printf("\n  ✅ Backend API: %s\n", disc.URL)
		} else {
			fmt.Printf("\n  ❌ Backend API unreachable at %s: %v\n", disc.URL, herr)
		}
		if disc.ConfigUIURL != "" {
			fmt.Printf("  Config UI:  %s\n", disc.ConfigUIURL)
		}
		if disc.GrafanaURL != "" {
			fmt.Printf("  Grafana:    %s\n", disc.GrafanaURL)
		}
		fmt.Println("  Run 'gh devlake configure full' to set up connections.")
		return nil
	}

	// ── Deployment section ──
	method := state.Method
	if method == "" {
		method = "unknown"
	}
	fmt.Printf("\n  Deployment  [%s]\n", stateFile)
	fmt.Println(sep)
	fmt.Printf("  Method:    %s\n", method)
	if state.DeployedAt != "" {
		fmt.Printf("  Deployed:  %s\n", friendlyTime(state.DeployedAt))
	}

	// ── Services section ──
	backendURL := state.Endpoints.Backend
	grafanaURL := state.Endpoints.Grafana
	configUIURL := state.Endpoints.ConfigUI

	// If the state file doesn't contain all endpoints (common for local deployments),
	// infer companion URLs from the backend URL.
	if backendURL != "" && (grafanaURL == "" || configUIURL == "") {
		if disc, err := devlake.Discover(backendURL); err == nil {
			if grafanaURL == "" {
				grafanaURL = disc.GrafanaURL
			}
			if configUIURL == "" {
				configUIURL = disc.ConfigUIURL
			}
		}
	}

	type svcEntry struct {
		label string
		url   string
		kind  string
	}
	svcs := []svcEntry{
		{"Backend  ", backendURL, "backend"},
		{"Grafana  ", grafanaURL, "grafana"},
		{"Config UI", configUIURL, "config-ui"},
	}
	hasServices := false
	for _, svc := range svcs {
		if svc.url != "" {
			hasServices = true
			break
		}
	}
	if hasServices {
		fmt.Println("\n  Services")
		fmt.Println(sep)
		for _, svc := range svcs {
			if svc.url == "" {
				continue
			}
			icon := pingEndpoint(svc.url, svc.kind)
			fmt.Printf("  %s  %s  %s\n", svc.label, icon, svc.url)
		}
	}

	// ── Connections section ──
	fmt.Println("\n  Connections")
	fmt.Println(sep)
	if len(state.Connections) == 0 {
		fmt.Println("  (none — run 'gh devlake configure connection add')")
	} else {
		for _, c := range state.Connections {
			displayName := c.Plugin
			if def := FindConnectionDef(c.Plugin); def != nil {
				displayName = def.DisplayName
			}
			extra := ""
			if c.Organization != "" {
				extra = fmt.Sprintf("  [org: %s]", c.Organization)
			}
			fmt.Printf("  %-18s  ID=%-3d  %q%s\n", displayName, c.ConnectionID, c.Name, extra)
		}
		if state.ConnectionsConfiguredAt != "" {
			fmt.Printf("  Configured: %s\n", friendlyTime(state.ConnectionsConfiguredAt))
		}
	}

	// ── Project section ──
	fmt.Println("\n  Project")
	fmt.Println(sep)
	if state.Project == nil {
		fmt.Println("  (none — run 'gh devlake configure project')")
	} else {
		p := state.Project
		fmt.Printf("  Name:       %s\n", p.Name)
		fmt.Printf("  Blueprint:  %d\n", p.BlueprintID)
		if len(p.Repos) > 0 {
			fmt.Printf("  Repos:      %s\n", strings.Join(p.Repos, ", "))
		}
		if state.ScopesConfiguredAt != "" {
			fmt.Printf("  Configured: %s\n", friendlyTime(state.ScopesConfiguredAt))
		}
	}

	fmt.Println("\n" + strings.Repeat("═", 40))
	return nil
}

// runStatusJSON outputs the status in JSON format.
func runStatusJSON(state *devlake.State, stateFile string) error {
	out := statusOutput{
		Endpoints:   []statusEndpoint{},
		Connections: []statusConnection{},
	}

	if state != nil {
		method := state.Method
		if method == "" {
			method = "unknown"
		}
		out.Deployment = &statusDeployment{
			Method:    method,
			StateFile: stateFile,
		}

		// Resolve endpoints (same logic as human path)
		backendURL := state.Endpoints.Backend
		grafanaURL := state.Endpoints.Grafana
		configUIURL := state.Endpoints.ConfigUI
		if backendURL != "" && (grafanaURL == "" || configUIURL == "") {
			if disc, err := devlake.Discover(backendURL); err == nil {
				if grafanaURL == "" {
					grafanaURL = disc.GrafanaURL
				}
				if configUIURL == "" {
					configUIURL = disc.ConfigUIURL
				}
			}
		}

		type endpointEntry struct {
			name string
			url  string
			kind string
		}
		for _, svc := range []endpointEntry{
			{"backend", backendURL, "backend"},
			{"grafana", grafanaURL, "grafana"},
			{"config-ui", configUIURL, "config-ui"},
		} {
			if svc.url == "" {
				continue
			}
			out.Endpoints = append(out.Endpoints, statusEndpoint{
				Name:    svc.name,
				URL:     svc.url,
				Healthy: checkEndpointHealth(svc.url, svc.kind),
			})
		}

		for _, c := range state.Connections {
			out.Connections = append(out.Connections, statusConnection{
				Plugin:       c.Plugin,
				ID:           c.ConnectionID,
				Name:         c.Name,
				Organization: c.Organization,
				Enterprise:   c.Enterprise,
			})
		}

		if state.Project != nil {
			out.Project = &statusProject{
				Name:        state.Project.Name,
				BlueprintID: state.Project.BlueprintID,
			}
		}
	} else {
		// No state file — try discovery; fail with an error in JSON mode if unreachable
		disc, err := devlake.Discover(cfgURL)
		if err != nil {
			return fmt.Errorf("discovering DevLake: %w", err)
		}
		client := devlake.NewClient(disc.URL)
		_, healthy := client.Health()
		out.Endpoints = append(out.Endpoints, statusEndpoint{
			Name:    "backend",
			URL:     disc.URL,
			Healthy: healthy == nil,
		})
		if disc.GrafanaURL != "" {
			out.Endpoints = append(out.Endpoints, statusEndpoint{
				Name:    "grafana",
				URL:     disc.GrafanaURL,
				Healthy: checkEndpointHealth(disc.GrafanaURL, "grafana"),
			})
		}
		if disc.ConfigUIURL != "" {
			out.Endpoints = append(out.Endpoints, statusEndpoint{
				Name:    "config-ui",
				URL:     disc.ConfigUIURL,
				Healthy: checkEndpointHealth(disc.ConfigUIURL, "config-ui"),
			})
		}
	}

	return printJSON(out)
}

// pingEndpoint returns ✅, ❌, or a warning icon for the given URL.
func pingEndpoint(url string, kind string) string {
	checkURL := strings.TrimRight(url, "/")
	if kind == "backend" {
		checkURL += "/ping"
	}
	if kind == "grafana" {
		checkURL += "/api/health"
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(checkURL)
	if err != nil {
		return "❌"
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return "✅"
	}
	return fmt.Sprintf("⚠️  (%d)", resp.StatusCode)
}

// checkEndpointHealth returns true if the endpoint responds successfully.
func checkEndpointHealth(url string, kind string) bool {
	checkURL := strings.TrimRight(url, "/")
	if kind == "backend" {
		checkURL += "/ping"
	}
	if kind == "grafana" {
		checkURL += "/api/health"
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(checkURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

// friendlyTime parses RFC3339 and returns a more readable format.
func friendlyTime(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Format("2006-01-02 15:04 UTC")
}
