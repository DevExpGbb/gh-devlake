package cmd

import (
	"fmt"
	"net/http"
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
	deployLocalDir     string
	deployLocalVersion string
	deployLocalQuiet   bool // suppress summary when called from init wizard
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

	return cmd
}

func runDeployLocal(cmd *cobra.Command, args []string) error {
	fmt.Println("\n========================================")
	fmt.Println("  Apache DevLake — Local Docker Setup")
	fmt.Println("========================================")

	// Ensure target directory exists
	if err := os.MkdirAll(deployLocalDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", deployLocalDir, err)
	}
	absDir, _ := filepath.Abs(deployLocalDir)
	fmt.Printf("\nTarget directory: %s\n", absDir)

	// ── Step 1: Resolve version ──
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

	// ── Step 2: Download files ──
	baseURL := fmt.Sprintf("https://raw.githubusercontent.com/apache/incubator-devlake/%s", version)
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

	// ── Step 3: Rename env.example → .env ──
	envExamplePath := filepath.Join(absDir, "env.example")
	envPath := filepath.Join(absDir, ".env")

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

	// ── Step 4: Generate + inject ENCRYPTION_SECRET ──
	fmt.Println("\n🔐 Generating ENCRYPTION_SECRET...")
	secret, err := secrets.EncryptionSecret(128)
	if err != nil {
		return fmt.Errorf("failed to generate secret: %w", err)
	}

	envContent, err := os.ReadFile(envPath)
	if err != nil {
		return err
	}
	content := string(envContent)
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
		fmt.Println("\n========================================")
		fmt.Println("  ✅ Setup Complete!")
		fmt.Println("========================================")
		fmt.Printf("\nFiles created in: %s\n", absDir)
		fmt.Println("  • docker-compose.yml")
		fmt.Println("  • .env (with ENCRYPTION_SECRET)")
		fmt.Println("\nNext steps:")
		fmt.Printf("  1. cd %s\n", absDir)
		fmt.Println("  2. docker compose up -d")
		fmt.Println("  3. Wait 2-3 minutes for services to start")
		fmt.Println("  4. Open Config UI: http://localhost:4000")
		fmt.Println("  5. Open Grafana:   http://localhost:3002 (admin/admin)")
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
		return "", err
	}
	fmt.Println("   ✅ Containers starting")

	backendURL := "http://localhost:8080"
	fmt.Println("\n⏳ Waiting for DevLake to be ready...")
	client := &http.Client{Timeout: 5 * time.Second}
	for attempt := 1; attempt <= 30; attempt++ {
		resp, err := client.Get(backendURL + "/ping")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				fmt.Println("   ✅ DevLake is responding!")
				return backendURL, nil
			}
		}
		fmt.Printf("   Attempt %d/30 — waiting...\n", attempt)
		time.Sleep(10 * time.Second)
	}
	return backendURL, fmt.Errorf("DevLake not ready after 5 minutes — check docker compose logs")
}
