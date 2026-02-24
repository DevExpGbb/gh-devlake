package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	dockerpkg "github.com/DevExpGBB/gh-devlake/internal/docker"
	"github.com/DevExpGBB/gh-devlake/internal/download"
	"github.com/DevExpGBB/gh-devlake/internal/secrets"
	"github.com/spf13/cobra"
)

var (
	deployLocalDir      string
	deployLocalVersion  string
	deployLocalOfficial bool // download official Apache release assets; false = skip download
	deployLocalQuiet    bool // suppress summary when called from init wizard
)

func newDeployLocalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "local",
		Short: "Deploy DevLake locally via Docker Compose",
		Long: `Downloads the official Apache DevLake Docker Compose files, generates
an encryption secret, and prepares for local deployment.

Example:
  gh devlake deploy local
  gh devlake deploy local --version v1.0.2 --dir ./devlake`,
		RunE: runDeployLocal,
	}

	cmd.Flags().StringVar(&deployLocalDir, "dir", ".", "Target directory for Docker Compose files")
	cmd.Flags().StringVar(&deployLocalVersion, "version", "latest", "DevLake version to deploy (e.g. v1.0.2)")
	cmd.Flags().BoolVar(&deployLocalOfficial, "official", true, "Download official Apache DevLake release assets (set false to use your own docker-compose.yml)")

	return cmd
}

func runDeployLocal(cmd *cobra.Command, args []string) error {
	printBanner("Apache DevLake — Local Docker Setup")

	homeDirTip("devlake")
	warnIfWritingIntoGitRepo(deployLocalDir, "docker-compose.yml and .env")

	// Ensure target directory exists
	if err := os.MkdirAll(deployLocalDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", deployLocalDir, err)
	}
	absDir, _ := filepath.Abs(deployLocalDir)
	fmt.Printf("\nTarget directory: %s\n", absDir)

	envPath := filepath.Join(absDir, ".env")

	// ── Steps 1–3: Official release (download files) ──
	if deployLocalOfficial {
		// Step 1: Resolve version
		version := deployLocalVersion
		if version == "latest" {
			fmt.Println("\n🔍 Fetching latest release version...")
			tag, err := download.GitHubLatestTag("apache", "incubator-devlake")
			if err != nil {
				return fmt.Errorf("failed to fetch latest release: %w", err)
			}
			version = tag
			fmt.Printf("   Latest version: %s\n", version)
		}

		// Step 2: Download files
		baseURL := fmt.Sprintf("https://github.com/apache/incubator-devlake/releases/download/%s", version)
		files := []struct {
			name string
			url  string
		}{
			{"docker-compose.yml", baseURL + "/docker-compose.yml"},
			{"env.example", baseURL + "/env.example"},
		}

		fmt.Printf("\n📥 Downloading files for %s...\n", version)
		for _, f := range files {
			dest := filepath.Join(absDir, f.name)
			fmt.Printf("   Downloading %s...", f.name)
			if err := download.File(f.url, dest); err != nil {
				return fmt.Errorf("\n   failed to download %s: %w", f.name, err)
			}
			fmt.Println(" ✅")
		}

		// Step 3: Rename env.example → .env
		envExamplePath := filepath.Join(absDir, "env.example")
		if _, err := os.Stat(envPath); err == nil {
			backupPath := envPath + ".bak"
			fmt.Printf("\n   .env already exists. Backing up to %s\n", filepath.Base(backupPath))
			data, err := os.ReadFile(envPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(backupPath, data, 0644); err != nil {
				return err
			}
		}
		if err := os.Rename(envExamplePath, envPath); err != nil {
			return fmt.Errorf("failed to rename env.example to .env: %w", err)
		}
		fmt.Println("   ✅ Renamed env.example → .env")
	}

	// ── Step 4: Generate + inject ENCRYPTION_SECRET ──
	fmt.Println("\n🔐 Generating ENCRYPTION_SECRET...")
	secret, err := secrets.EncryptionSecret(128)
	if err != nil {
		return fmt.Errorf("failed to generate secret: %w", err)
	}

	var envBytes []byte
	if data, readErr := os.ReadFile(envPath); readErr == nil {
		envBytes = data
	} else if !os.IsNotExist(readErr) {
		return readErr
	}
	content := string(envBytes)
	if strings.Contains(content, "ENCRYPTION_SECRET=") {
		// Replace existing placeholder
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "ENCRYPTION_SECRET=") {
				lines[i] = "ENCRYPTION_SECRET=" + secret
			}
		}
		content = strings.Join(lines, "\n")
	} else {
		content += "\nENCRYPTION_SECRET=" + secret + "\n"
	}
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		return err
	}
	fmt.Println("   ✅ ENCRYPTION_SECRET generated and saved")

	// ── Step 5: Check Docker ──
	fmt.Println("\n🐳 Checking Docker...")
	dockerOut, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Output()
	if err != nil {
		fmt.Println("   ⚠️  Docker not found or not running")
		fmt.Println("   Install Docker Desktop: https://docs.docker.com/get-docker")
	} else {
		fmt.Printf("   ✅ Docker %s found\n", strings.TrimSpace(string(dockerOut)))
	}

	// ── Summary ──
	if !deployLocalQuiet {
		printBanner("✅ Setup Complete!")
		fmt.Printf("\nFiles prepared in: %s\n", absDir)
		if deployLocalOfficial {
			fmt.Println("  • docker-compose.yml")
		}
		fmt.Println("  • .env (with ENCRYPTION_SECRET)")
		fmt.Println("\nNext steps:")
		fmt.Printf("  1. cd %s\n", absDir)
		fmt.Println("  2. docker compose up -d")
		fmt.Println("  3. Wait 2-3 minutes for services to start")
		fmt.Println("  4. Backend API:    http://localhost:8080")
		fmt.Println("  5. Open Config UI: http://localhost:4000")
		fmt.Println("  6. Open Grafana:   http://localhost:3002 (admin/admin)")
		fmt.Println("\nTo stop DevLake:")
		fmt.Println("  docker compose down")
	}

	return nil
}

// startLocalContainers runs docker compose up -d and polls until DevLake is healthy.
// Returns the backend URL on success.
func startLocalContainers(dir string) (string, error) {
	absDir, _ := filepath.Abs(dir)
	fmt.Printf("\n🐳 Starting containers in %s...\n", absDir)
	if err := dockerpkg.ComposeUp(absDir); err != nil {
		// Give a friendlier error for port conflicts
		errStr := err.Error()
		if strings.Contains(errStr, "port is already allocated") || strings.Contains(errStr, "Bind for") {
			// Extract the port number from the error
			port := ""
			if idx := strings.Index(errStr, "Bind for 0.0.0.0:"); idx != -1 {
				rest := errStr[idx+len("Bind for 0.0.0.0:"):]
				if end := strings.IndexAny(rest, " \n"); end > 0 {
					port = rest[:end]
				}
			}

			fmt.Println()
			if port != "" {
				fmt.Printf("❌ Port conflict: %s is already in use.\n", port)
			} else {
				fmt.Println("❌ Port conflict: a required port is already in use.")
			}

			// Ask Docker which container owns the port
			conflictCmd := ""
			if port != "" {
				out, dockerErr := exec.Command(
					"docker",
					"ps",
					"--filter",
					"publish="+port,
					"--format",
					"{{.Names}}\t{{.Label \"com.docker.compose.project.config_files\"}}\t{{.Label \"com.docker.compose.project.working_dir\"}}",
				).Output()
				if dockerErr == nil && len(strings.TrimSpace(string(out))) > 0 {
					lines := strings.Split(strings.TrimSpace(string(out)), "\n")
					// Use the first match
					parts := strings.SplitN(lines[0], "\t", 3)
					containerName := parts[0]
					configFiles := ""
					workDir := ""
					if len(parts) >= 2 {
						configFiles = strings.TrimSpace(parts[1])
					}
					if len(parts) == 3 {
						workDir = strings.TrimSpace(parts[2])
					}
					fmt.Printf("   Container holding the port: %s\n", containerName)
					// Prefer the exact compose file path Docker recorded (most reliable).
					if configFiles != "" {
						configFile := strings.Split(configFiles, ";")[0]
						configFile = strings.TrimSpace(configFile)
						if configFile != "" {
							if _, statErr := os.Stat(configFile); statErr == nil {
								fmt.Println("\n   Stop it with:")
								fmt.Printf("   docker compose -f \"%s\" down\n", configFile)
								conflictCmd = fmt.Sprintf("docker compose -f \"%s\" down", configFile)
							} else {
								fmt.Println("\n   Stop it with:")
								fmt.Printf("   docker stop %s\n", containerName)
								fmt.Printf("\n   ⚠️  Compose file not found at: %s\n", configFile)
								fmt.Println("      (It may have been moved/deleted since the container was created.)")
								conflictCmd = "docker stop " + containerName
							}
						}
					} else if workDir != "" {
						// Fallback for older Docker versions: assume docker-compose.yml under working_dir.
						composePath := fmt.Sprintf("%s\\docker-compose.yml", workDir)
						if _, statErr := os.Stat(composePath); statErr == nil {
							fmt.Println("\n   Stop it with:")
							fmt.Printf("   docker compose -f \"%s\" down\n", composePath)
							conflictCmd = fmt.Sprintf("docker compose -f \"%s\" down", composePath)
						}
					}
					if conflictCmd == "" {
						fmt.Println("\n   Stop it with:")
						fmt.Printf("   docker stop %s\n", containerName)
						conflictCmd = "docker stop " + containerName
					}
				}
			}
			if conflictCmd == "" {
				fmt.Println("\n   Find what's using it:")
				fmt.Println("   docker ps --format \"table {{.Names}}\\t{{.Ports}}\"")
			}
			fmt.Println("\n   Then re-run:")
			fmt.Println("   gh devlake init")
			return "", fmt.Errorf("port conflict — stop the conflicting container and retry")
		}
		return "", err
	}
	fmt.Println("   ✅ Containers starting")

	backendURL := "http://localhost:8080"
	fmt.Println("\n⏳ Waiting for DevLake to be ready...")
	fmt.Println("   Giving MySQL time to initialize (this takes ~30s on first run)...")
	time.Sleep(30 * time.Second)

	if err := waitForReady(backendURL, 36, 10*time.Second); err != nil {
		return backendURL, fmt.Errorf("DevLake not ready after 6 minutes — check: docker compose logs devlake")
	}
	return backendURL, nil
}
