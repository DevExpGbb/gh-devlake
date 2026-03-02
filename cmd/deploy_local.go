package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	dockerpkg "github.com/DevExpGBB/gh-devlake/internal/docker"
	"github.com/DevExpGBB/gh-devlake/internal/download"
	"github.com/DevExpGBB/gh-devlake/internal/gitclone"
	"github.com/DevExpGBB/gh-devlake/internal/prompt"
	"github.com/DevExpGBB/gh-devlake/internal/secrets"
	"github.com/spf13/cobra"
)

const (
	poetryWorkaroundVersion = "2.2.1"
)

var (
	deployLocalDir     string
	deployLocalVersion string
	deployLocalRepoURL string // fork/clone URL for "fork" source mode
	deployLocalStart   bool   // start containers after setup
	deployLocalQuiet   bool   // suppress summary when called from init wizard
	// deployLocalSource is set by flag or interactive prompt:
	//   "official" — download Apache release (default)
	//   "fork"     — clone a repo and build from source
	//   "custom"   — user provides their own docker-compose.yml
	deployLocalSource string
)

func newDeployLocalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "local",
		Short: "Deploy DevLake locally via Docker Compose",
		Long: `Sets up and starts Apache DevLake locally using Docker Compose.

Image source (interactive prompt or flags):
  official  Download the official Apache release (default)
  fork      Clone a DevLake repo and build images from source
  custom    Use your own docker-compose.yml already in the target directory

Example:
  gh devlake deploy local
  gh devlake deploy local --version v1.0.2 --dir ./devlake
  gh devlake deploy local --source fork --repo-url https://github.com/DevExpGBB/incubator-devlake`,
		RunE: runDeployLocal,
	}

	cmd.Flags().StringVar(&deployLocalDir, "dir", ".", "Target directory for Docker Compose files")
	cmd.Flags().StringVar(&deployLocalVersion, "version", "latest", "DevLake version to deploy (e.g. v1.0.2)")
	cmd.Flags().StringVar(&deployLocalSource, "source", "", "Image source: official, fork, or custom")
	cmd.Flags().StringVar(&deployLocalRepoURL, "repo-url", "", "Repository URL to clone (for fork source)")
	cmd.Flags().BoolVar(&deployLocalStart, "start", true, "Start containers after setup")

	return cmd
}

func runDeployLocal(cmd *cobra.Command, args []string) error {
	printBanner("Apache DevLake — Local Docker Setup")

	// Suggest a dedicated directory unless already in the right place or called from init
	if !deployLocalQuiet {
		if suggestDedicatedDir("local", "gh devlake deploy local") {
			return nil
		}
	}

	// ── Interactive image-source prompt (when no explicit flag set) ──
	if deployLocalSource == "" {
		imageChoices := []string{
			"official - Apache DevLake images from GitHub releases (recommended)",
			"fork    - Clone a DevLake repo and build from source",
			"custom  - Use your own docker-compose.yml in the target directory",
		}
		fmt.Println()
		imgChoice := prompt.Select("Which DevLake images to use?", imageChoices)
		if imgChoice == "" {
			return fmt.Errorf("image choice is required")
		}
		switch {
		case strings.HasPrefix(imgChoice, "official"):
			deployLocalSource = "official"
		case strings.HasPrefix(imgChoice, "fork"):
			deployLocalSource = "fork"
		default:
			deployLocalSource = "custom"
		}
	}

	// Ensure target directory exists
	if err := os.MkdirAll(deployLocalDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", deployLocalDir, err)
	}
	absDir, _ := filepath.Abs(deployLocalDir)
	fmt.Printf("\nTarget directory: %s\n", absDir)

	envPath := filepath.Join(absDir, ".env")

	switch deployLocalSource {
	case "official":
		if err := deployLocalOfficial_download(absDir, envPath); err != nil {
			return err
		}

	case "fork":
		if err := deployLocalFork_clone(absDir); err != nil {
			return err
		}
		// The cloned repo has its own .env template
		envPath = filepath.Join(absDir, ".env")

	case "custom":
		fmt.Println("\n📂 Using existing docker-compose.yml in target directory")
		// Verify docker-compose exists
		composePath := filepath.Join(absDir, "docker-compose.yml")
		devComposePath := filepath.Join(absDir, "docker-compose-dev.yml")
		if _, err := os.Stat(composePath); os.IsNotExist(err) {
			if _, err := os.Stat(devComposePath); os.IsNotExist(err) {
				return fmt.Errorf("no docker-compose.yml or docker-compose-dev.yml found in %s", absDir)
			}
		}
	}

	// ── Generate + inject ENCRYPTION_SECRET ──
	// If the .env already has a non-empty ENCRYPTION_SECRET, keep it —
	// replacing it would break any existing database that was encrypted
	// with the old key.
	fmt.Println("\n🔐 Checking ENCRYPTION_SECRET...")
	var envBytes []byte
	if data, readErr := os.ReadFile(envPath); readErr == nil {
		envBytes = data
	} else if !os.IsNotExist(readErr) {
		return readErr
	}
	content := string(envBytes)

	existingSecret := ""
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "ENCRYPTION_SECRET=") {
			existingSecret = strings.TrimPrefix(trimmed, "ENCRYPTION_SECRET=")
			break
		}
	}

	if existingSecret != "" {
		fmt.Println("   ✅ ENCRYPTION_SECRET already set (keeping existing)")
	} else {
		secret, err := secrets.EncryptionSecret(128)
		if err != nil {
			return fmt.Errorf("failed to generate secret: %w", err)
		}
		if strings.Contains(content, "ENCRYPTION_SECRET=") {
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
	}

	// ── Check Docker ──
	fmt.Println("\n🐳 Checking Docker...")
	if err := dockerpkg.CheckAvailable(); err != nil {
		fmt.Println("   ❌ Docker not found or not running")
		fmt.Println("   Install Docker Desktop: https://docs.docker.com/get-docker")
		fmt.Println("   Start Docker Desktop, then re-run: gh devlake deploy local")
		return fmt.Errorf("Docker is not available — start Docker Desktop and retry")
	}
	fmt.Println("   ✅ Docker found")

	// ── Start containers (unless --start=false) ──
	if deployLocalStart {
		buildImages := deployLocalSource == "fork"
		var services []string
		if deployLocalSource == "fork" {
			services = []string{"mysql", "devlake", "grafana", "config-ui"}
		}
		backendURL, err := startLocalContainers(absDir, buildImages, services...)
		if err != nil {
			return err
		}
		cfgURL = backendURL

		fmt.Println("\n🔄 Triggering database migration...")
		migClient := devlake.NewClient(backendURL)
		if err := migClient.TriggerMigration(); err != nil {
			fmt.Printf("   ⚠️  Migration may need manual trigger: %v\n", err)
		} else {
			fmt.Println("   ✅ Migration triggered")
			fmt.Println("\n⏳ Waiting for migration to complete...")
			if err := waitForMigration(backendURL, 60, 5*time.Second); err != nil {
				fmt.Printf("   ⚠️  %v\n", err)
				fmt.Println("   Migration may still be running — proceeding anyway")
			}
		}

		if !deployLocalQuiet {
			printBanner("✅ DevLake is running!")
			fmt.Printf("\n  Backend API: %s\n", backendURL)
			fmt.Println("  Config UI:   http://localhost:4000")
			fmt.Println("  Grafana:     http://localhost:3002 (admin/admin)")
			fmt.Println("\nTo stop/remove DevLake:")
			fmt.Printf("  cd \"%s\" && gh devlake cleanup\n", absDir)
		}
	} else {
		// Print manual instructions
		if !deployLocalQuiet {
			printBanner("✅ Setup Complete!")
			fmt.Printf("\nFiles prepared in: %s\n", absDir)
			fmt.Println("  • .env (with ENCRYPTION_SECRET)")
			fmt.Println("\nNext steps:")
			fmt.Printf("  1. cd %s\n", absDir)
			fmt.Println("  2. docker compose up -d")
			fmt.Println("  3. Wait 2-3 minutes for services to start")
			fmt.Println("  4. Backend API:    http://localhost:8080")
			fmt.Println("  5. Open Config UI: http://localhost:4000")
			fmt.Println("  6. Open Grafana:   http://localhost:3002 (admin/admin)")
			fmt.Println("\nTo stop/remove DevLake later:")
			fmt.Printf("  cd \"%s\" && gh devlake cleanup\n", absDir)
		}
	}

	return nil
}

// deployLocalOfficial_download downloads the official Apache release files.
func deployLocalOfficial_download(absDir, envPath string) error {
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

	// Rename env.example → .env
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
	return nil
}

// deployLocalFork_clone clones a DevLake repo into the target directory for building from source.
func deployLocalFork_clone(absDir string) error {
	if deployLocalRepoURL == "" {
		deployLocalRepoURL = prompt.ReadLine(fmt.Sprintf("Repository URL [%s]", gitclone.DefaultForkURL))
		if deployLocalRepoURL == "" {
			deployLocalRepoURL = gitclone.DefaultForkURL
		}
	}

	fmt.Printf("\n🏗️  Cloning %s...\n", deployLocalRepoURL)
	// Clone into a temp dir, then move contents into absDir
	tmpDir, err := os.MkdirTemp("", "devlake-clone-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if err := gitclone.Clone(deployLocalRepoURL, tmpDir); err != nil {
		return err
	}

	// Copy the dev compose file and .env to absDir
	devComposeSrc := filepath.Join(tmpDir, "docker-compose-dev.yml")
	devComposeDst := filepath.Join(absDir, "docker-compose.yml")
	if data, err := os.ReadFile(devComposeSrc); err == nil {
		if err := os.WriteFile(devComposeDst, data, 0644); err != nil {
			return fmt.Errorf("failed to write docker-compose.yml: %w", err)
		}
		fmt.Println("   ✅ docker-compose.yml (dev) copied")
	} else {
		return fmt.Errorf("cloned repo does not contain docker-compose-dev.yml: %w", err)
	}

	// Copy .env template if present
	envSrc := filepath.Join(tmpDir, "config-ui", "env.example")
	envDst := filepath.Join(absDir, ".env")
	if data, err := os.ReadFile(envSrc); err == nil {
		if err := os.WriteFile(envDst, data, 0644); err != nil {
			return fmt.Errorf("failed to write .env: %w", err)
		}
	}

	// The dev compose file expects DB_URL in .env (unlike the official release
	// compose which has MySQL credentials inline). Inject it so DevLake can
	// connect to the MySQL service defined in docker-compose-dev.yml.
	envData, _ := os.ReadFile(envDst)
	envContent := string(envData)
	if !strings.Contains(envContent, "DB_URL=") {
		envContent += "\nDB_URL=mysql://merico:merico@mysql:3306/lake?charset=utf8mb4&parseTime=True&loc=UTC\n"
		if err := os.WriteFile(envDst, []byte(envContent), 0644); err != nil {
			return fmt.Errorf("failed to write DB_URL to .env: %w", err)
		}
	}

	// Copy build context directories so docker compose build works
	for _, dir := range []string{"backend", "config-ui", "grafana"} {
		src := filepath.Join(tmpDir, dir)
		dst := filepath.Join(absDir, dir)
		if _, err := os.Stat(src); err == nil {
			if _, err := os.Stat(dst); os.IsNotExist(err) {
				if err := copyDir(src, dst); err != nil {
					fmt.Printf("   ⚠️  Could not copy %s: %v\n", dir, err)
				}
			}
		}
	}

	if err := applyPoetryPinWorkaround(absDir); err != nil {
		fmt.Printf("   ⚠️  Could not apply temporary Poetry pin workaround: %v\n", err)
	} else {
		fmt.Printf("   ⚠️  Applied temporary Poetry pin workaround (poetry==%s) for fork builds\n", poetryWorkaroundVersion)
	}
	fmt.Println("   ✅ Build contexts ready")

	return nil
}

// applyPoetryPinWorkaround pins Poetry only for fork/source builds until
// apache/incubator-devlake#8734 is fixed.
// Tracking removal: DevExpGbb/gh-devlake#79.
func applyPoetryPinWorkaround(absDir string) error {
	dockerfilePath := filepath.Join(absDir, "backend", "Dockerfile")
	data, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return fmt.Errorf("reading backend Dockerfile: %w", err)
	}

	rewritten, changed := rewritePoetryInstallLine(string(data), poetryWorkaroundVersion)
	if !changed {
		return nil
	}

	if err := os.WriteFile(dockerfilePath, []byte(rewritten), 0644); err != nil {
		return fmt.Errorf("writing backend Dockerfile: %w", err)
	}
	return nil
}

func rewritePoetryInstallLine(content, version string) (string, bool) {
	original := "RUN curl -sSL https://install.python-poetry.org | python3 -"
	pinned := fmt.Sprintf("RUN curl -sSL https://install.python-poetry.org | python3 - --version %s", version)

	if strings.Contains(content, pinned) {
		return content, false
	}

	if !strings.Contains(content, original) {
		return content, false
	}

	return strings.Replace(content, original, pinned, 1), true
}

// copyDir recursively copies a directory tree.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

// startLocalContainers runs docker compose up -d and polls until DevLake is healthy.
// If build is true, images are rebuilt from local Dockerfiles (fork mode).
// If services are specified, only those services are started (used by fork mode
// to avoid starting unnecessary services like postgres/authproxy).
// Returns the backend URL on success.
func startLocalContainers(dir string, build bool, services ...string) (string, error) {
	absDir, _ := filepath.Abs(dir)
	if build {
		fmt.Printf("\n🐳 Building and starting containers in %s...\n", absDir)
		fmt.Println("   (Building from source — this may take a few minutes on first run)")
	} else {
		fmt.Printf("\n🐳 Starting containers in %s...\n", absDir)
	}
	if err := dockerpkg.ComposeUp(absDir, build, services...); err != nil {
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
						composePath := filepath.Join(workDir, "docker-compose.yml")
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
			fmt.Println("\n💡 To clean up partial artifacts:")
			fmt.Println("   gh devlake cleanup --local --force")
			return "", fmt.Errorf("port conflict — stop the conflicting container and retry")
		}
		fmt.Println("\n💡 To clean up partial artifacts:")
		fmt.Println("   gh devlake cleanup --local --force")
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
