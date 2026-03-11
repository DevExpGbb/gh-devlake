package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	azurepkg "github.com/DevExpGBB/gh-devlake/internal/azure"
	dockerpkg "github.com/DevExpGBB/gh-devlake/internal/docker"
	"github.com/spf13/cobra"
)

var (
	stopService string
	stopAzure   bool
	stopLocal   bool
	stopState   string
)

func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop running DevLake services (preserves containers and data)",
		Long: `Gracefully stops running DevLake services without removing containers, volumes, or state.

For local deployments (Docker Compose), runs 'docker compose stop', which preserves
containers and volumes so they can be quickly restarted with 'gh devlake start'.

For Azure deployments, stops Container Instances and the MySQL server using the Azure CLI.

Auto-detects deployment type from state files in the current directory.

This is the non-destructive counterpart to 'gh devlake start'.
Use 'gh devlake cleanup' to permanently tear down resources.`,
		RunE: runStop,
	}

	cmd.Flags().StringVar(&stopService, "service", "", "Stop only a specific service (e.g., grafana)")
	cmd.Flags().BoolVar(&stopAzure, "azure", false, "Force Azure stop mode")
	cmd.Flags().BoolVar(&stopLocal, "local", false, "Force local (Docker Compose) stop mode")
	cmd.Flags().StringVar(&stopState, "state-file", "", "Path to state file (auto-detected if omitted)")

	return cmd
}

func runStop(cmd *cobra.Command, args []string) error {
	mode := detectStopMode()
	switch mode {
	case "local":
		return runLocalStop()
	case "azure":
		return runAzureStop()
	default:
		return fmt.Errorf("no deployment found — no state file or docker-compose.yml in current directory\nRun 'gh devlake deploy' to create a new deployment")
	}
}

// detectStopMode determines whether to stop local (Docker Compose) or Azure resources.
// Priority: explicit flags → explicit state file (inspected for method) → auto-detect files.
func detectStopMode() string {
	if stopAzure {
		return "azure"
	}
	if stopLocal {
		return "local"
	}

	// Check explicit state file — inspect its content rather than guessing from the filename.
	if stopState != "" {
		if data, err := os.ReadFile(stopState); err == nil {
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

func runLocalStop() error {
	// In JSON mode, all progress goes to stderr to keep stdout clean for JSON.
	var prog io.Writer = os.Stdout
	if outputJSON {
		prog = os.Stderr
	}

	fmt.Fprintln(prog)
	fmt.Fprintln(prog, "════════════════════════════════════════")
	fmt.Fprintln(prog, "  DevLake — Stop Services")
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
	if stopState != "" {
		absState, err := filepath.Abs(stopState)
		if err != nil {
			fmt.Fprintf(prog, "   ⚠️  Could not resolve --state-file path: %v — using current directory\n", err)
		} else {
			dir = filepath.Dir(absState)
		}
	}

	// ── Determine services to stop ──
	var services []string
	if stopService != "" {
		services = []string{stopService}
	}

	// ── Run docker compose stop ──
	if len(services) > 0 {
		fmt.Fprintf(prog, "\n🐳 Stopping service %q in %s...\n", stopService, dir)
	} else {
		fmt.Fprintf(prog, "\n🐳 Stopping containers in %s...\n", dir)
	}
	if err := dockerpkg.ComposeStop(dir, services...); err != nil {
		return fmt.Errorf("failed to stop containers: %w", err)
	}
	fmt.Fprintln(prog, "   ✅ Containers stopped (data preserved)")

	if outputJSON {
		return printJSON(map[string]string{"status": "stopped", "mode": "local"})
	}

	fmt.Fprintln(prog)
	fmt.Fprintln(prog, "════════════════════════════════════════")
	fmt.Fprintln(prog, "  ✅ Services Stopped!")
	fmt.Fprintln(prog, "════════════════════════════════════════")
	fmt.Fprintln(prog)
	fmt.Fprintln(prog, "  Containers and volumes are preserved.")
	fmt.Fprintln(prog, "  Run 'gh devlake start' to bring them back up.")
	fmt.Fprintln(prog)
	return nil
}

func runAzureStop() error {
	// In JSON mode, all progress goes to stderr to keep stdout clean for JSON.
	var prog io.Writer = os.Stdout
	if outputJSON {
		prog = os.Stderr
	}

	fmt.Fprintln(prog)
	fmt.Fprintln(prog, "════════════════════════════════════════")
	fmt.Fprintln(prog, "  DevLake Azure — Stop Services")
	fmt.Fprintln(prog, "════════════════════════════════════════")

	stateFile := stopState
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
		return fmt.Errorf("state file %s has no resource group — cannot stop Azure resources", stateFile)
	}

	// ── Check Azure CLI login ──
	fmt.Fprintln(prog, "\n🔑 Checking Azure login...")
	if _, err := azurepkg.CheckLogin(); err != nil {
		return fmt.Errorf("not logged in to Azure CLI — run 'az login' first")
	}
	fmt.Fprintln(prog, "   ✅ Logged in")

	// ── Stop containers ──
	containers := state.Resources.Containers
	if stopService != "" {
		var filtered []string
		for _, c := range containers {
			if strings.Contains(c, stopService) {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("no container matching %q found in state file", stopService)
		}
		containers = filtered
	}

	for _, container := range containers {
		fmt.Fprintf(prog, "\n📦 Stopping container %q...\n", container)
		if err := azurepkg.ContainerStop(container, state.ResourceGroup); err != nil {
			fmt.Fprintf(prog, "   ⚠️  Could not stop %s: %v\n", container, err)
		} else {
			fmt.Fprintln(prog, "   ✅ Stop initiated")
		}
	}

	// ── Stop MySQL (only when stopping all services) ──
	if stopService == "" && state.Resources.MySQL != "" {
		fmt.Fprintf(prog, "\n🐳 Stopping MySQL server %q...\n", state.Resources.MySQL)
		if err := azurepkg.MySQLStop(state.Resources.MySQL, state.ResourceGroup); err != nil {
			fmt.Fprintf(prog, "   ⚠️  Could not stop MySQL: %v\n", err)
		} else {
			fmt.Fprintln(prog, "   ✅ MySQL stop initiated")
		}
	}

	if outputJSON {
		return printJSON(map[string]string{"status": "stopped", "mode": "azure"})
	}

	fmt.Fprintln(prog)
	fmt.Fprintln(prog, "════════════════════════════════════════")
	fmt.Fprintln(prog, "  ✅ Services Stopped!")
	fmt.Fprintln(prog, "════════════════════════════════════════")
	fmt.Fprintln(prog)
	fmt.Fprintln(prog, "  Run 'gh devlake start --azure' to bring them back up.")
	fmt.Fprintln(prog)
	return nil
}
