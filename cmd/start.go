package cmd

import (
	"encoding/json"
	"fmt"
	"os"
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

func detectStartMode() string {
	if startAzure {
		return "azure"
	}
	if startLocal {
		return "local"
	}

	// Check for explicit state file
	if startState != "" {
		if _, err := os.Stat(startState); err == nil {
			if strings.Contains(startState, "azure") {
				return "azure"
			}
			return "local"
		}
	}

	// Auto-detect from state files
	if _, err := os.Stat(".devlake-azure.json"); err == nil {
		return "azure"
	}
	if _, err := os.Stat(".devlake-local.json"); err == nil {
		return "local"
	}
	// Fall back to docker-compose.yml in cwd
	if _, err := os.Stat("docker-compose.yml"); err == nil {
		return "local"
	}
	return ""
}

func runLocalStart() error {
	printBanner("DevLake — Start Services")

	// ── Check Docker ──
	fmt.Println("\n🐳 Checking Docker...")
	if err := dockerpkg.CheckAvailable(); err != nil {
		return fmt.Errorf("Docker is not available: %w\nMake sure Docker Desktop or the Docker daemon is running", err)
	}
	fmt.Println("   ✅ Docker is running")

	// ── Find deployment directory ──
	cwd, _ := os.Getwd()
	dir := cwd

	// ── Determine services to start ──
	var services []string
	if startService != "" {
		services = []string{startService}
	}

	// ── Run docker compose up -d ──
	if len(services) > 0 {
		fmt.Printf("\n🐳 Starting service %q in %s...\n", startService, dir)
	} else {
		fmt.Printf("\n🐳 Starting containers in %s...\n", dir)
	}
	if err := dockerpkg.ComposeUp(dir, false, services...); err != nil {
		return fmt.Errorf("failed to start containers: %w", err)
	}
	fmt.Println("   ✅ Containers starting")

	// ── Health polling ──
	if !startNoWait && startService == "" {
		fmt.Println("\n⏳ Waiting for DevLake to be ready...")
		backendURLCandidates := []string{"http://localhost:8080", "http://localhost:8085"}
		_, err := waitForReadyAny(backendURLCandidates, startHealthAttempts, 10*time.Second)
		if err != nil {
			fmt.Println("   ⚠️  DevLake not ready after 60s — services may still be initializing")
			fmt.Println("   Run 'gh devlake status' to check.")
		}
	}

	if outputJSON {
		return printJSON(map[string]string{"status": "started", "mode": "local"})
	}

	printBanner("✅ Services Started!")
	fmt.Println("\n  Backend API: http://localhost:8080")
	fmt.Println("  Config UI:   http://localhost:4000")
	fmt.Println("  Grafana:     http://localhost:3002 (admin/admin)")
	fmt.Println()
	return nil
}

func runAzureStart() error {
	printBanner("DevLake Azure — Start Services")

	stateFile := startState
	if stateFile == "" {
		stateFile = ".devlake-azure.json"
	}

	var state azureStateData
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return fmt.Errorf("state file not found: %s\nUse --state-file to specify the path", stateFile)
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("invalid state file: %w", err)
	}
	if state.ResourceGroup == "" {
		return fmt.Errorf("state file %s has no resource group — cannot start Azure resources", stateFile)
	}

	// ── Check Azure CLI login ──
	fmt.Println("\n🔑 Checking Azure login...")
	if _, err := azurepkg.CheckLogin(); err != nil {
		return fmt.Errorf("not logged in to Azure CLI — run 'az login' first")
	}
	fmt.Println("   ✅ Logged in")

	// ── Start MySQL ──
	if state.Resources.MySQL != "" {
		fmt.Printf("\n🐳 Starting MySQL server %q...\n", state.Resources.MySQL)
		if err := azurepkg.MySQLStart(state.Resources.MySQL, state.ResourceGroup); err != nil {
			fmt.Printf("   ⚠️  Could not start MySQL: %v\n", err)
		} else {
			fmt.Println("   ✅ MySQL start initiated")
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
		fmt.Printf("\n📦 Starting container %q...\n", container)
		if err := azurepkg.ContainerStart(container, state.ResourceGroup); err != nil {
			fmt.Printf("   ⚠️  Could not start %s: %v\n", container, err)
		} else {
			fmt.Println("   ✅ Start initiated")
		}
	}

	// ── Health polling ──
	if !startNoWait && state.Endpoints.Backend != "" {
		fmt.Println("\n⏳ Waiting for DevLake to be ready...")
		if err := waitForReady(state.Endpoints.Backend, startHealthAttempts, 10*time.Second); err != nil {
			fmt.Println("   ⚠️  Backend not ready after 60s — Azure containers may still be starting")
			fmt.Println("   Run 'gh devlake status' to check.")
		}
	}

	if outputJSON {
		return printJSON(map[string]string{"status": "started", "mode": "azure"})
	}

	printBanner("✅ Services Started!")
	if state.Endpoints.Backend != "" {
		fmt.Printf("\n  Backend API: %s\n", state.Endpoints.Backend)
	}
	if state.Endpoints.ConfigUI != "" {
		fmt.Printf("  Config UI:   %s\n", state.Endpoints.ConfigUI)
	}
	if state.Endpoints.Grafana != "" {
		fmt.Printf("  Grafana:     %s\n", state.Endpoints.Grafana)
	}
	fmt.Println()
	return nil
}
