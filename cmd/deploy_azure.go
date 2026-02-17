package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/DevExpGBB/gh-devlake/internal/azure"
	dockerpkg "github.com/DevExpGBB/gh-devlake/internal/docker"
	"github.com/DevExpGBB/gh-devlake/internal/prompt"
	"github.com/DevExpGBB/gh-devlake/internal/secrets"
	"github.com/spf13/cobra"
)

var (
	azureRG             string
	azureLocation       string
	azureBaseName       string
	azureSkipImageBuild bool
	azureRepoURL        string
	azureOfficial       bool
)

func newDeployAzureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "azure",
		Short: "Deploy DevLake to Azure Container Apps",
		Long: `Provisions DevLake on Azure using Container Instances, Azure Database for MySQL,
and (optionally) Azure Container Registry.

Example:
  gh devlake deploy azure --resource-group devlake-rg --location eastus
  gh devlake deploy azure --resource-group devlake-rg --location eastus --official`,
		RunE: runDeployAzure,
	}

	cmd.Flags().StringVar(&azureRG, "resource-group", "", "Azure Resource Group name")
	cmd.Flags().StringVar(&azureLocation, "location", "", "Azure region")
	cmd.Flags().StringVar(&azureBaseName, "base-name", "devlake", "Base name for Azure resources")
	cmd.Flags().BoolVar(&azureSkipImageBuild, "skip-image-build", false, "Skip Docker image building")
	cmd.Flags().StringVar(&azureRepoURL, "repo-url", "", "Clone a remote DevLake repository for building")
	cmd.Flags().BoolVar(&azureOfficial, "official", false, "Use official Apache images from Docker Hub (no ACR)")

	return cmd
}

// Common Azure regions for interactive selection.
var azureRegions = []string{
	"eastus", "eastus2", "westus2", "westus3",
	"centralus", "northeurope", "westeurope",
	"southeastasia", "australiaeast", "uksouth",
}

func runDeployAzure(cmd *cobra.Command, args []string) error {
	// ── Interactive prompts for missing required flags ──
	if azureLocation == "" {
		azureLocation = prompt.Select("Select Azure region", azureRegions)
		if azureLocation == "" {
			return fmt.Errorf("--location is required")
		}
	}
	if azureRG == "" {
		azureRG = prompt.ReadLine("Resource group name (e.g. devlake-rg)")
		if azureRG == "" {
			return fmt.Errorf("--resource-group is required")
		}
	}

	suffix := azure.Suffix(azureRG)
	acrName := "devlakeacr" + suffix

	if azureOfficial {
		fmt.Println("\n========================================")
		fmt.Println("  DevLake Azure Deployment (Official)")
		fmt.Println("========================================")
		fmt.Println("\nUsing official Apache DevLake images from Docker Hub")
		azureSkipImageBuild = true
	} else {
		fmt.Println("\n========================================")
		fmt.Println("  DevLake Azure Deployment")
		fmt.Println("========================================")
	}

	fmt.Printf("\nConfiguration:\n")
	fmt.Printf("  Resource Group: %s\n", azureRG)
	fmt.Printf("  Location:       %s\n", azureLocation)
	fmt.Printf("  Base Name:      %s\n", azureBaseName)
	if !azureOfficial {
		fmt.Printf("  ACR Name:       %s\n", acrName)
	} else {
		fmt.Println("  Images:         Official (Docker Hub)")
	}

	// ── Check Azure login ──
	fmt.Println("\n🔑 Checking Azure CLI login...")
	acct, err := azure.CheckLogin()
	if err != nil {
		fmt.Println("   Not logged in. Running az login...")
		if loginErr := azure.Login(); loginErr != nil {
			return fmt.Errorf("az login failed: %w", loginErr)
		}
		acct, err = azure.CheckLogin()
		if err != nil {
			return fmt.Errorf("still not logged in after az login: %w", err)
		}
	}
	fmt.Printf("   Logged in as: %s\n", acct.User.Name)

	// ── Create Resource Group ──
	fmt.Println("\n📦 Creating Resource Group...")
	if err := azure.CreateResourceGroup(azureRG, azureLocation); err != nil {
		return err
	}
	fmt.Println("   ✅ Resource Group created")

	// ── Generate secrets ──
	fmt.Println("\n🔐 Generating secrets...")
	mysqlPwd, err := secrets.MySQLPassword()
	if err != nil {
		return err
	}
	encSecret, err := secrets.EncryptionSecret(32)
	if err != nil {
		return err
	}
	fmt.Println("   ✅ Secrets generated")

	// ── Build and push images (if needed) ──
	if !azureSkipImageBuild {
		repoRoot, err := findRepoRoot()
		if err != nil {
			return err
		}
		fmt.Printf("\n🏗️  Building Docker images from %s...\n", repoRoot)

		// Deploy ACR first
		templateName := "main.bicep"
		templatePath, cleanup, err := azure.WriteTemplate(templateName)
		if err != nil {
			return err
		}
		defer cleanup()

		params := map[string]string{
			"baseName":           azureBaseName,
			"uniqueSuffix":       suffix,
			"mysqlAdminPassword": mysqlPwd,
			"encryptionSecret":   encSecret,
		}

		deployOut, err := azure.DeployBicep(azureRG, templatePath, params)
		if err != nil {
			fmt.Println("   ⚠️  Bicep pre-deploy for ACR failed, will retry after image build.")
		}

		acrServer := acrName + ".azurecr.io"
		if deployOut != nil && deployOut.ACRLoginServer != "" {
			acrServer = deployOut.ACRLoginServer
		}

		fmt.Println("\n   Logging into ACR...")
		if err := azure.ACRLogin(acrName); err != nil {
			return err
		}

		images := []struct {
			name       string
			dockerfile string
			context    string
		}{
			{"devlake-backend", "backend/Dockerfile", filepath.Join(repoRoot, "backend")},
			{"devlake-config-ui", "config-ui/Dockerfile", filepath.Join(repoRoot, "config-ui")},
			{"devlake-grafana", "grafana/Dockerfile", filepath.Join(repoRoot, "grafana")},
		}

		for _, img := range images {
			fmt.Printf("\n   Building %s...\n", img.name)
			localTag := img.name + ":latest"
			if err := dockerpkg.Build(localTag, filepath.Join(repoRoot, img.dockerfile), img.context); err != nil {
				fmt.Fprintf(os.Stderr, "\n   ❌ Docker build failed for %s.\n", img.name)
				fmt.Fprintf(os.Stderr, "   Tip: re-run with --official to skip building and use\n")
				fmt.Fprintf(os.Stderr, "   official Apache DevLake images from Docker Hub instead.\n")
				return fmt.Errorf("docker build failed for %s: %w", img.name, err)
			}
			remoteTag := acrServer + "/" + localTag
			fmt.Printf("   Pushing %s...\n", img.name)
			if err := dockerpkg.TagAndPush(localTag, remoteTag); err != nil {
				return err
			}
		}
		fmt.Println("\n   ✅ All images pushed")
	}

	// ── Check MySQL state ──
	mysqlName := fmt.Sprintf("%smysql%s", azureBaseName, suffix)
	fmt.Println("\n🗄️  Checking MySQL state...")
	state, err := azure.MySQLState(mysqlName, azureRG)
	if err == nil && state == "Stopped" {
		fmt.Println("   MySQL is stopped. Starting...")
		if err := azure.MySQLStart(mysqlName, azureRG); err != nil {
			fmt.Printf("   ⚠️  Could not start MySQL: %v\n", err)
		} else {
			fmt.Println("   Waiting 30s for MySQL...")
			time.Sleep(30 * time.Second)
			fmt.Println("   ✅ MySQL started")
		}
	} else if state != "" {
		fmt.Printf("   MySQL state: %s\n", state)
	} else {
		fmt.Println("   MySQL not yet created (will be created by Bicep)")
	}

	// ── Deploy infrastructure ──
	fmt.Println("\n🚀 Deploying infrastructure with Bicep...")
	templateName := "main.bicep"
	if azureOfficial {
		templateName = "main-official.bicep"
	}
	templatePath, cleanup, err := azure.WriteTemplate(templateName)
	if err != nil {
		return err
	}
	defer cleanup()

	params := map[string]string{
		"baseName":           azureBaseName,
		"uniqueSuffix":       suffix,
		"mysqlAdminPassword": mysqlPwd,
		"encryptionSecret":   encSecret,
	}
	if !azureOfficial {
		params["acrName"] = acrName
	}

	deployment, err := azure.DeployBicep(azureRG, templatePath, params)
	if err != nil {
		return fmt.Errorf("Bicep deployment failed: %w", err)
	}

	fmt.Println("\n========================================")
	fmt.Println("  ✅ Deployment Complete!")
	fmt.Println("========================================")
	fmt.Printf("\nEndpoints:\n")
	fmt.Printf("  Backend API: %s\n", deployment.BackendEndpoint)
	fmt.Printf("  Config UI:   %s\n", deployment.ConfigUIEndpoint)
	fmt.Printf("  Grafana:     %s\n", deployment.GrafanaEndpoint)

	// ── Wait for backend and trigger migration ──
	fmt.Println("\n⏳ Waiting for backend to start...")
	backendReady := false
	httpClient := &http.Client{Timeout: 5 * time.Second}
	for attempt := 1; attempt <= 30; attempt++ {
		resp, err := httpClient.Get(deployment.BackendEndpoint + "/ping")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				backendReady = true
				fmt.Println("   ✅ Backend is responding!")
				break
			}
		}
		fmt.Printf("   Attempt %d/30 — waiting...\n", attempt)
		time.Sleep(10 * time.Second)
	}

	if backendReady {
		fmt.Println("\n🔄 Triggering database migration...")
		resp, err := httpClient.Get(deployment.BackendEndpoint + "/proceed-db-migration")
		if err == nil {
			resp.Body.Close()
			fmt.Println("   ✅ Migration triggered")
		} else {
			fmt.Printf("   ⚠️  Migration may need manual trigger: %v\n", err)
		}
	} else {
		fmt.Println("   Backend not ready after 30 attempts.")
		fmt.Printf("   Trigger migration manually: GET %s/proceed-db-migration\n", deployment.BackendEndpoint)
	}

	// ── Save state file ──
	stateFile := filepath.Join(".", ".devlake-azure.json")
	containers := []string{
		fmt.Sprintf("%s-backend-%s", azureBaseName, suffix),
		fmt.Sprintf("%s-grafana-%s", azureBaseName, suffix),
		fmt.Sprintf("%s-ui-%s", azureBaseName, suffix),
	}

	kvName := deployment.KeyVaultName
	if kvName == "" {
		kvName = fmt.Sprintf("%skv%s", azureBaseName, suffix)
	}

	// Write a combined state file: Azure-specific metadata + DevLake discovery fields
	combinedState := map[string]any{
		"deployedAt":        time.Now().Format(time.RFC3339),
		"method":            methodName(),
		"subscription":      acct.Name,
		"subscriptionId":    acct.ID,
		"resourceGroup":     azureRG,
		"region":            azureLocation,
		"suffix":            suffix,
		"useOfficialImages": azureOfficial,
		"resources": map[string]any{
			"acr":        conditionalACR(),
			"keyVault":   kvName,
			"mysql":      mysqlName,
			"database":   "lake",
			"containers": containers,
		},
		"endpoints": map[string]string{
			"backend":  deployment.BackendEndpoint,
			"grafana":  deployment.GrafanaEndpoint,
			"configUi": deployment.ConfigUIEndpoint,
		},
	}

	data, _ := json.MarshalIndent(combinedState, "", "  ")
	if err := os.WriteFile(stateFile, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Could not save state file: %v\n", err)
	} else {
		fmt.Printf("\n💾 State saved to %s\n", stateFile)
	}

	fmt.Println("\nNext Steps:")
	fmt.Println("  1. Wait 2-3 minutes for containers to start")
	fmt.Printf("  2. Open Config UI: %s\n", deployment.ConfigUIEndpoint)
	fmt.Println("  3. Configure your data sources")
	fmt.Printf("\nTo cleanup: gh devlake cleanup --azure\n")

	return nil
}

func findRepoRoot() (string, error) {
	if azureRepoURL != "" {
		// Clone to temp dir
		tmpDir, err := os.MkdirTemp("", "devlake-clone-*")
		if err != nil {
			return "", err
		}
		fmt.Printf("   Cloning %s...\n", azureRepoURL)
		cmd := newGitClone(azureRepoURL, tmpDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git clone failed: %s\n%s", err, string(out))
		}
		return tmpDir, nil
	}

	// Walk up looking for backend/Dockerfile
	dir, _ := os.Getwd()
	for dir != "" && dir != filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "backend", "Dockerfile")); err == nil {
			return dir, nil
		}
		dir = filepath.Dir(dir)
	}
	return "", fmt.Errorf("could not find DevLake repo root.\n" +
		"Options:\n" +
		"  --repo-url <url>  Clone a fork with the custom Dockerfile\n" +
		"  --official        Use official Apache images (no build needed)")
}

func newGitClone(url, dir string) *exec.Cmd {
	return exec.Command("git", "clone", "--depth", "1", url, dir)
}

func methodName() string {
	if azureOfficial {
		return "bicep-official"
	}
	return "bicep"
}

func conditionalACR() any {
	if azureOfficial {
		return nil
	}
	return "devlakeacr" + azure.Suffix(azureRG)
}
