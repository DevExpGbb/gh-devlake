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

func runStatus(cmd *cobra.Command, args []string) error {
	fmt.Println()
	sep := "  " + strings.Repeat("─", 42)

	fmt.Println("════════════════════════════════════════")
	fmt.Println("  DevLake Status")
	fmt.Println("════════════════════════════════════════")

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

	if state == nil {
		disc, err := devlake.Discover(cfgURL)
		if err != nil {
			fmt.Println("\n  No state file found. Run 'gh devlake deploy' to get started.")
			return nil
		}
		client := devlake.NewClient(disc.URL)
		if _, herr := client.Health(); herr == nil {
			fmt.Printf("\n  ✅ DevLake reachable at %s\n", disc.URL)
		} else {
			fmt.Printf("\n  ❌ DevLake unreachable at %s: %v\n", disc.URL, herr)
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
	type svcEntry struct {
		label     string
		url       string
		isGrafana bool
	}
	svcs := []svcEntry{
		{"Backend  ", state.Endpoints.Backend, false},
		{"Grafana  ", state.Endpoints.Grafana, true},
		{"Config UI", state.Endpoints.ConfigUI, false},
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
			icon := pingEndpoint(svc.url, svc.isGrafana)
			fmt.Printf("  %s  %s  %s\n", svc.label, icon, svc.url)
		}
	}

	// ── Connections section ──
	fmt.Println("\n  Connections")
	fmt.Println(sep)
	if len(state.Connections) == 0 {
		fmt.Println("  (none — run 'gh devlake configure connection')")
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

// pingEndpoint returns ✅, ❌, or a warning icon for the given URL.
func pingEndpoint(url string, isGrafana bool) string {
	checkURL := strings.TrimRight(url, "/")
	if isGrafana {
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

// friendlyTime parses RFC3339 and returns a more readable format.
func friendlyTime(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Format("2006-01-02 15:04 UTC")
}
