package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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

For official and fork deployments, if the default local port bundle
(8080/3002/4000) is already in use, the CLI automatically retries once with
alternate ports (8085/3004/4004). Custom deployments require manual port
conflict resolution.

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

		// Allow alternate port bundle for official/fork (not custom)
		allowPortFallback := deployLocalSource != "custom"

		backendURL, err := startLocalContainers(absDir, buildImages, allowPortFallback, services...)
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
			// Infer companion URLs based on compose file ports
			composePath := findComposeFile(absDir)
			grafanaURL, configUIURL := inferCompanionURLs(backendURL, composePath)
			fmt.Printf("  Config UI:   %s\n", configUIURL)
			fmt.Printf("  Grafana:     %s (admin/admin)\n", grafanaURL)
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
// If allowPortFallback is true, the function will retry once with alternate ports (8085/3004/4004)
// when a port conflict is detected on the default bundle (8080/3002/4000).
// If services are specified, only those services are started (used by fork mode
// to avoid starting unnecessary services like postgres/authproxy).
// Returns the backend URL on success.
func startLocalContainers(dir string, build, allowPortFallback bool, services ...string) (string, error) {
	absDir, _ := filepath.Abs(dir)
	if build {
		fmt.Printf("\n🐳 Building and starting containers in %s...\n", absDir)
		fmt.Println("   (Building from source — this may take a few minutes on first run)")
	} else {
		fmt.Printf("\n🐳 Starting containers in %s...\n", absDir)
	}

	// Attempt 1: Default ports
	err := dockerpkg.ComposeUp(absDir, build, services...)
	if err == nil {
		fmt.Println("   ✅ Containers starting")
		return waitAndDetectBackendURL(absDir)
	}

	// Classify the error
	deployErr := classifyDockerComposeError(err)
	if deployErr == nil || deployErr.Class != ErrorClassDockerPortConflict {
		// Not a port conflict or unknown error - print general cleanup and fail
		fmt.Println("\n💡 To clean up partial artifacts:")
		fmt.Println("   gh devlake cleanup --local --force")
		return "", fmt.Errorf("docker compose up failed: %w", err)
	}

	// Port conflict detected
	if !allowPortFallback {
		// Custom deployments don't get auto-fallback - print friendly error
		printDockerPortConflictError(deployErr, "", "")
		return "", fmt.Errorf("port conflict — stop the conflicting container and retry: %w", err)
	}

	// Bounded recovery: Try alternate port bundle once
	// Find compose file
	composePath := findComposeFile(absDir)

	// Detect which port bundle the compose file is using
	bundle := detectPortBundle(composePath)

	switch bundle {
	case portBundleAlternate:
		// Compose file is already on alternate ports - can't fallback further
		// Build custom header with port/container info
		header := "\n❌ Port conflict detected on alternate ports (8085/3004/4004)"
		if deployErr.Port != "" {
			header += fmt.Sprintf("\n   Port %s is in use", deployErr.Port)
			if deployErr.Container != "" {
				header += fmt.Sprintf(" by container: %s", deployErr.Container)
			}
		}
		nextSteps := "   The alternate port bundle is already in use.\n   Free ports 8085/3004/4004, then retry deployment."
		printDockerPortConflictError(deployErr, header, nextSteps)
		return "", fmt.Errorf("port conflict on alternate ports: %w", err)

	case portBundleCustom:
		// Custom ports - don't attempt automatic rewrite
		// Build custom header with port/container info
		header := "\n❌ Port conflict detected on custom ports"
		if deployErr.Port != "" {
			header += fmt.Sprintf("\n   Port %s is in use", deployErr.Port)
			if deployErr.Container != "" {
				header += fmt.Sprintf(" by container: %s", deployErr.Container)
			}
		}
		nextSteps := "   Edit your compose file to use different host ports, or stop the conflicting container."
		printDockerPortConflictError(deployErr, header, nextSteps)
		return "", fmt.Errorf("port conflict on custom ports: %w", err)

	case portBundleDefault:
		// Compose file has default ports - try rewriting to alternate bundle
		fmt.Println("\n🔧 Port conflict detected on default ports (8080/3002/4000)")
		if deployErr.Port != "" {
			fmt.Printf("   Port %s is in use", deployErr.Port)
			if deployErr.Container != "" {
				fmt.Printf(" by container: %s", deployErr.Container)
			}
			fmt.Println()
		}
		fmt.Println("\n🔄 Retrying with alternate ports (8085/3004/4004)...")

		if err := rewriteComposePorts(composePath); err != nil {
			fmt.Printf("   ⚠️  Could not rewrite ports: %v\n", err)
			printDockerPortConflictError(deployErr, "", "")
			return "", fmt.Errorf("port conflict and failed to apply alternate ports: %w", err)
		}

		fmt.Println("   ✅ Ports updated in compose file")

		// Attempt 2: Retry with alternate ports
		fmt.Println("   Starting containers with alternate ports...")
		err = dockerpkg.ComposeUp(absDir, build, services...)
		if err != nil {
			// Second attempt failed - classify again
			retryErr := classifyDockerComposeError(err)
			if retryErr != nil && retryErr.Class == ErrorClassDockerPortConflict {
				// Build header that indicates both bundles failed
				header := "\n❌ Alternate ports are also in use."
				nextSteps := "   Both default (8080/3002/4000) and alternate (8085/3004/4004) port bundles are occupied.\n   Free at least one bundle, then retry deployment."
				printDockerPortConflictError(retryErr, header, nextSteps)
			} else {
				fmt.Println("\n💡 To clean up partial artifacts:")
				fmt.Println("   gh devlake cleanup --local --force")
			}
			return "", fmt.Errorf("deployment failed after port fallback: %w", err)
		}

		fmt.Println("   ✅ Containers starting on alternate ports")
		return waitAndDetectBackendURL(absDir)
	}

	return "", fmt.Errorf("unexpected port bundle detection result")
}

// findComposeFile returns the path to the active docker compose file in the given directory.
// Checks for docker-compose.yml first, then docker-compose-dev.yml.
func findComposeFile(dir string) string {
	composePath := filepath.Join(dir, "docker-compose.yml")
	if _, err := os.Stat(composePath); err == nil {
		return composePath
	}
	return filepath.Join(dir, "docker-compose-dev.yml")
}

// waitAndDetectBackendURL polls the backend URL extracted from the compose file.
// Falls back to probing both 8080 and 8085 if extraction fails.
func waitAndDetectBackendURL(dir string) (string, error) {
	composePath := findComposeFile(dir)

	// Try to extract the actual backend port from the compose file
	ports := extractServicePorts(composePath, "devlake")
	var backendURLCandidates []string

	if backendPort, ok := ports["devlake"]; ok {
		// Use the extracted port
		backendURLCandidates = []string{fmt.Sprintf("http://localhost:%d", backendPort)}
	} else {
		// Fall back to probing both default and alternate ports
		backendURLCandidates = []string{"http://localhost:8080", "http://localhost:8085"}
	}

	fmt.Println("\n⏳ Waiting for DevLake to be ready...")
	fmt.Println("   Giving MySQL time to initialize (this takes ~30s on first run)...")
	time.Sleep(30 * time.Second)

	backendURL, err := waitForReadyAny(backendURLCandidates, 36, 10*time.Second)
	if err != nil {
		return "", fmt.Errorf("DevLake not ready after 6 minutes — check: docker compose logs devlake: %w", err)
	}
	return backendURL, nil
}

// portBundle represents the detected port configuration in a compose file
type portBundle int

const (
	portBundleDefault   portBundle = iota // 8080/3002/4000
	portBundleAlternate                   // 8085/3004/4004
	portBundleCustom                      // Other custom ports
)

// detectPortBundle analyzes a compose file to determine which port bundle it uses.
// Returns:
// - portBundleDefault if compose file contains any of 8080:8080, 3002:3002, or 4000:4000
// - portBundleAlternate if compose file contains any of 8085:8080, 3004:3002, or 4004:4000
// - portBundleCustom if compose file has other custom host ports
func detectPortBundle(composePath string) portBundle {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return portBundleDefault // Assume default if we can't read - let rewriteComposePorts surface the I/O error
	}

	content := string(data)

	// Use regex to match port mappings as list items (avoiding substring matches like 18080:8080)
	// Pattern: start of line, optional whitespace, dash, optional whitespace, optional quotes, port mapping
	defaultPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^\s*-\s*["']?8080:8080["']?`),
		regexp.MustCompile(`(?m)^\s*-\s*["']?3002:3002["']?`),
		regexp.MustCompile(`(?m)^\s*-\s*["']?4000:4000["']?`),
	}
	for _, re := range defaultPatterns {
		if re.MatchString(content) {
			return portBundleDefault
		}
	}

	// Check for alternate port bundle
	alternatePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^\s*-\s*["']?8085:8080["']?`),
		regexp.MustCompile(`(?m)^\s*-\s*["']?3004:3002["']?`),
		regexp.MustCompile(`(?m)^\s*-\s*["']?4004:4000["']?`),
	}
	for _, re := range alternatePatterns {
		if re.MatchString(content) {
			return portBundleAlternate
		}
	}

	// If neither default nor alternate, it's custom
	return portBundleCustom
}

// rewriteComposePorts rewrites the port mappings in a docker-compose.yml file
// from the default bundle (8080/3002/4000) to the alternate bundle (8085/3004/4004).
// Uses regex with proper boundaries to avoid rewriting custom ports like 18080:8080.
func rewriteComposePorts(composePath string) error {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return fmt.Errorf("reading compose file: %w", err)
	}

	content := string(data)
	modified := content

	// Port mapping patterns with regex boundaries
	// Match: "- 8080:8080" or "- "8080:8080" or "- '8080:8080'" at start of list item
	// Avoid: "- 18080:8080" (custom host port that contains 8080)
	portReplacements := []struct {
		pattern     string
		replacement string
	}{
		// Backend: 8080:8080 -> 8085:8080
		{`(?m)(^\s*-\s*)["']?8080:8080["']?`, `${1}8085:8080`},
		// Grafana: 3002:3002 -> 3004:3002
		{`(?m)(^\s*-\s*)["']?3002:3002["']?`, `${1}3004:3002`},
		// Config UI: 4000:4000 -> 4004:4000
		{`(?m)(^\s*-\s*)["']?4000:4000["']?`, `${1}4004:4000`},
	}

	for _, repl := range portReplacements {
		re := regexp.MustCompile(repl.pattern)
		modified = re.ReplaceAllString(modified, repl.replacement)
	}

	if modified == content {
		return fmt.Errorf("no port mappings found to rewrite (expected 8080/3002/4000)")
	}

	if err := os.WriteFile(composePath, []byte(modified), 0644); err != nil {
		return fmt.Errorf("writing compose file: %w", err)
	}

	return nil
}

// extractServicePorts parses the docker-compose.yml file and extracts host ports for the specified services.
// Returns a map of service name to host port (e.g., "devlake" -> 8080).
// Returns empty map if parsing fails or service not found.
func extractServicePorts(composePath string, serviceNames ...string) map[string]int {
	result := make(map[string]int)

	// Use docker compose config to parse the compose file reliably
	cmd := exec.Command("docker", "compose", "-f", composePath, "config", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		// Silently return empty map - caller will fall back to default behavior
		return result
	}

	var config struct {
		Services map[string]struct {
			Ports []struct {
				Published string `json:"published,omitempty"`
				Target    int    `json:"target,omitempty"`
			} `json:"ports,omitempty"`
		} `json:"services"`
	}

	if err := json.Unmarshal(output, &config); err != nil {
		return result
	}

	// Extract host ports for requested services
	for _, serviceName := range serviceNames {
		service, exists := config.Services[serviceName]
		if !exists {
			continue
		}

		// Find the first published port mapping
		for _, port := range service.Ports {
			if port.Published != "" {
				// Published can be a port number as string
				if hostPort, err := strconv.Atoi(port.Published); err == nil {
					result[serviceName] = hostPort
					break
				}
			}
		}
	}

	return result
}

// inferCompanionURLs extracts and returns the actual Grafana and Config UI URLs from the compose file.
// Falls back to inferring from backend URL if extraction fails.
func inferCompanionURLs(backendURL string, composePath string) (grafanaURL, configUIURL string) {
	// Try to extract actual ports from compose file
	ports := extractServicePorts(composePath, "grafana", "config-ui")

	if grafanaPort, ok := ports["grafana"]; ok {
		grafanaURL = fmt.Sprintf("http://localhost:%d", grafanaPort)
	}
	if configUIPort, ok := ports["config-ui"]; ok {
		configUIURL = fmt.Sprintf("http://localhost:%d", configUIPort)
	}

	// If extraction succeeded for both, return
	if grafanaURL != "" && configUIURL != "" {
		return grafanaURL, configUIURL
	}

	// Fall back to inference from backend URL
	if strings.Contains(backendURL, ":8085") {
		if grafanaURL == "" {
			grafanaURL = "http://localhost:3004"
		}
		if configUIURL == "" {
			configUIURL = "http://localhost:4004"
		}
	} else {
		if grafanaURL == "" {
			grafanaURL = "http://localhost:3002"
		}
		if configUIURL == "" {
			configUIURL = "http://localhost:4000"
		}
	}

	return grafanaURL, configUIURL
}
