package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	azurepkg "github.com/DevExpGBB/gh-devlake/internal/azure"
	dockerpkg "github.com/DevExpGBB/gh-devlake/internal/docker"
	"github.com/spf13/cobra"
)

var (
	startService string
	startNoWait  bool
	startAzure   bool
	startLocal   bool
	startState   string
)

// startHealthAttempts is the number of 10-second polling intervals used when waiting
// for DevLake to become healthy after start. 6 × 10s = 60s total — much shorter than
// the 36 × 10s = 6-minute timeout used during deploy, because databases and volumes
// already exist when starting an existing deployment.
const startHealthAttempts = 6

// Well-known local port pairs for DevLake services (matching discovery.go).
const (
	localBackendPort8080  = "http://localhost:8080"
	localGrafanaPort8080  = "http://localhost:3002"
	localConfigUIPort8080 = "http://localhost:4000"
	localBackendPort8085  = "http://localhost:8085"
	localGrafanaPort8085  = "http://localhost:3004"
	localConfigUIPort8085 = "http://localhost:4004"
)

func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start stopped or exited DevLake services",
		Long: `Brings up stopped or exited DevLake services for an existing deployment.

For local deployments (Docker Compose), runs 'docker compose up -d' from the
deployment directory. This is idempotent — running containers are unaffected,
and exited or crashed containers are restarted.

For Azure deployments, starts any stopped Container Instances and MySQL server.

Auto-detects deployment type from state files in the current directory.`,
		RunE: runStart,
	}

	cmd.Flags().StringVar(&startService, "service", "", "Start only a specific service (e.g., config-ui)")
	cmd.Flags().BoolVar(&startNoWait, "no-wait", false, "Skip health polling after start")
	cmd.Flags().BoolVar(&startAzure, "azure", false, "Force Azure start mode")
	cmd.Flags().BoolVar(&startLocal, "local", false, "Force local (Docker Compose) start mode")
	cmd.Flags().StringVar(&startState, "state-file", "", "Path to state file (auto-detected if omitted)")

	return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
	mode := detectStartMode()
	switch mode {
	case "local":
		return runLocalStart()
	case "azure":
		return runAzureStart()
	default:
		return fmt.Errorf("no deployment found — no state file or docker-compose.yml in current directory\nRun 'gh devlake deploy' to create a new deployment")
	}
}

// detectStartMode determines whether to start local (Docker Compose) or Azure resources.
// Priority: explicit flags → explicit state file (inspected for method) → auto-detect files.
func detectStartMode() string {
	if startAzure {
		return "azure"
	}
	if startLocal {
		return "local"
	}

	// Check explicit state file — inspect its content rather than guessing from the filename.
	if startState != "" {
		if data, err := os.ReadFile(startState); err == nil {
			var meta struct {
				Method        string `json:"method"`
				ResourceGroup string `json:"resourceGroup"`
			}
			if json.Unmarshal(data, &meta) == nil {
				switch strings.ToLower(meta.Method) {
				case "azure":
					return "azure"
				case "local", "docker-compose":
					return "local"
				}
				// If method is absent but resourceGroup is set, it's an Azure state file.
				if meta.ResourceGroup != "" {
					return "azure"
				}
			}
		}
		// File exists but could not be parsed, or method is unknown — fall through.
		return "local"
	}

	// Auto-detect from well-known state file names.
	if _, err := os.Stat(".devlake-azure.json"); err == nil {
		return "azure"
	}
	if _, err := os.Stat(".devlake-local.json"); err == nil {
		return "local"
	}
	// Fall back to docker-compose.yml in cwd.
	if _, err := os.Stat("docker-compose.yml"); err == nil {
		return "local"
	}
	return ""
}

func runLocalStart() error {
	// In JSON mode, all progress goes to stderr to keep stdout clean for JSON.
	var prog io.Writer = os.Stdout
	if outputJSON {
		prog = os.Stderr
	}

	fmt.Fprintln(prog)
	fmt.Fprintln(prog, "════════════════════════════════════════")
	fmt.Fprintln(prog, "  DevLake — Start Services")
	fmt.Fprintln(prog, "════════════════════════════════════════")

	// ── Check Docker ──
	fmt.Fprintln(prog, "\n🐳 Checking Docker...")
	if err := dockerpkg.CheckAvailable(); err != nil {
		return fmt.Errorf("Docker is not available: %w\nMake sure Docker Desktop or the Docker daemon is running", err)
	}
	fmt.Fprintln(prog, "   ✅ Docker is running")

	// ── Find deployment directory ──
	// When --state-file is provided, run docker compose from that file's directory
	// so it finds the correct docker-compose.yml.
	cwd, _ := os.Getwd()
	dir := cwd
	if startState != "" {
		absState, err := filepath.Abs(startState)
		if err != nil {
			fmt.Fprintf(prog, "   ⚠️  Could not resolve --state-file path: %v — using current directory\n", err)
		} else {
			dir = filepath.Dir(absState)
		}
	}

	// ── Determine services to start ──
	var services []string
	if startService != "" {
		services = []string{startService}
	}

	// ── Run docker compose up -d ──
	if len(services) > 0 {
		fmt.Fprintf(prog, "\n🐳 Starting service %q in %s...\n", startService, dir)
	} else {
		fmt.Fprintf(prog, "\n🐳 Starting containers in %s...\n", dir)
	}
	if err := dockerpkg.ComposeUp(dir, false, services...); err != nil {
		return fmt.Errorf("failed to start containers: %w", err)
	}
	fmt.Fprintln(prog, "   ✅ Containers starting")

	// ── Health polling ──
	backendURL := ""
	if !startNoWait && startService == "" {
		fmt.Fprintln(prog, "\n⏳ Waiting for DevLake to be ready...")
		backendURLCandidates := []string{localBackendPort8080, localBackendPort8085}
		var err error
		backendURL, err = waitForReadyAny(backendURLCandidates, startHealthAttempts, 10*time.Second)
		if err != nil {
			fmt.Fprintln(prog, "   ⚠️  DevLake not ready after 60s — services may still be initializing")
			fmt.Fprintln(prog, "   Run 'gh devlake status' to check.")
		}
	}

	if outputJSON {
		return printJSON(map[string]string{"status": "started", "mode": "local"})
	}

	fmt.Fprintln(prog)
	fmt.Fprintln(prog, "════════════════════════════════════════")
	fmt.Fprintln(prog, "  ✅ Services Started!")
	fmt.Fprintln(prog, "════════════════════════════════════════")

	// Print accurate URLs based on the healthy backend that responded.
	if backendURL == "" {
		backendURL = localBackendPort8080
	}
	grafanaURL, configUIURL := localCompanionURLs(backendURL)
	fmt.Fprintf(prog, "\n  Backend API: %s\n", backendURL)
	if configUIURL != "" {
		fmt.Fprintf(prog, "  Config UI:   %s\n", configUIURL)
	}
	if grafanaURL != "" {
		fmt.Fprintf(prog, "  Grafana:     %s (admin/admin)\n", grafanaURL)
	}
	fmt.Fprintln(prog)
	return nil
}

// localCompanionURLs returns the Grafana and Config UI URLs that correspond to
// a given DevLake backend URL, matching the well-known local port pairs.
func localCompanionURLs(backendURL string) (grafanaURL, configUIURL string) {
	if strings.HasPrefix(backendURL, localBackendPort8085) {
		return localGrafanaPort8085, localConfigUIPort8085
	}
	// Default port mapping (8080).
	return localGrafanaPort8080, localConfigUIPort8080
}

func runAzureStart() error {
	// In JSON mode, all progress goes to stderr to keep stdout clean for JSON.
	var prog io.Writer = os.Stdout
	if outputJSON {
		prog = os.Stderr
	}

	fmt.Fprintln(prog)
	fmt.Fprintln(prog, "════════════════════════════════════════")
	fmt.Fprintln(prog, "  DevLake Azure — Start Services")
	fmt.Fprintln(prog, "════════════════════════════════════════")

	stateFile := startState
	if stateFile == "" {
		stateFile = ".devlake-azure.json"
	}

	var state azureStateData
	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("state file not found: %s\nUse --state-file to specify the path", stateFile)
		}
		return fmt.Errorf("failed to read state file %s: %w", stateFile, err)
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("invalid state file: %w", err)
	}
	if state.ResourceGroup == "" {
		return fmt.Errorf("state file %s has no resource group — cannot start Azure resources", stateFile)
	}

	// ── Check Azure CLI login ──
	fmt.Fprintln(prog, "\n🔑 Checking Azure login...")
	if _, err := azurepkg.CheckLogin(); err != nil {
		return fmt.Errorf("not logged in to Azure CLI — run 'az login' first")
	}
	fmt.Fprintln(prog, "   ✅ Logged in")

	// ── Start MySQL ──
	if state.Resources.MySQL != "" {
		fmt.Fprintf(prog, "\n🐳 Starting MySQL server %q...\n", state.Resources.MySQL)
		if err := azurepkg.MySQLStart(state.Resources.MySQL, state.ResourceGroup); err != nil {
			fmt.Fprintf(prog, "   ⚠️  Could not start MySQL: %v\n", err)
		} else {
			fmt.Fprintln(prog, "   ✅ MySQL start initiated")
		}
	}

	// ── Start containers ──
	containers := state.Resources.Containers
	if startService != "" {
		var filtered []string
		for _, c := range containers {
			if strings.Contains(c, startService) {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("no container matching %q found in state file", startService)
		}
		containers = filtered
	}

	for _, container := range containers {
		fmt.Fprintf(prog, "\n📦 Starting container %q...\n", container)
		if err := azurepkg.ContainerStart(container, state.ResourceGroup); err != nil {
			fmt.Fprintf(prog, "   ⚠️  Could not start %s: %v\n", container, err)
		} else {
			fmt.Fprintln(prog, "   ✅ Start initiated")
		}
	}

	// ── Health polling ──
	if !startNoWait && state.Endpoints.Backend != "" {
		fmt.Fprintln(prog, "\n⏳ Waiting for DevLake to be ready...")
		if err := waitForReady(state.Endpoints.Backend, startHealthAttempts, 10*time.Second); err != nil {
			fmt.Fprintln(prog, "   ⚠️  Backend not ready after 60s — Azure containers may still be starting")
			fmt.Fprintln(prog, "   Run 'gh devlake status' to check.")
		}
	}

	if outputJSON {
		return printJSON(map[string]string{"status": "started", "mode": "azure"})
	}

	fmt.Fprintln(prog)
	fmt.Fprintln(prog, "════════════════════════════════════════")
	fmt.Fprintln(prog, "  ✅ Services Started!")
	fmt.Fprintln(prog, "════════════════════════════════════════")
	if state.Endpoints.Backend != "" {
		fmt.Fprintf(prog, "\n  Backend API: %s\n", state.Endpoints.Backend)
	}
	if state.Endpoints.ConfigUI != "" {
		fmt.Fprintf(prog, "  Config UI:   %s\n", state.Endpoints.ConfigUI)
	}
	if state.Endpoints.Grafana != "" {
		fmt.Fprintf(prog, "  Grafana:     %s\n", state.Endpoints.Grafana)
	}
	fmt.Fprintln(prog)
	return nil
}
