// Package azure wraps Azure CLI (az) commands.
package azure

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Account represents the current Azure account.
type Account struct {
	Name         string `json:"name"`
	ID           string `json:"id"`
	UserName     string `json:"user_name"`
	TenantID     string `json:"tenantId"`
	HomeTenantID string `json:"homeTenantId"`
	User         struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"user"`
}

// CheckLogin checks if az CLI is logged in and returns the account info.
func CheckLogin() (*Account, error) {
	out, err := exec.Command("az", "account", "show", "-o", "json").Output()
	if err != nil {
		return nil, fmt.Errorf("not logged in to Azure CLI")
	}
	var acct Account
	if err := json.Unmarshal(out, &acct); err != nil {
		return nil, err
	}
	return &acct, nil
}

// Login runs az login interactively.
func Login() error {
	cmd := exec.Command("az", "login")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// GetSubscription returns the current subscription name.
func GetSubscription() (string, error) {
	out, err := exec.Command("az", "account", "show", "--query", "name", "-o", "tsv").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// CreateResourceGroup creates or updates a resource group.
func CreateResourceGroup(name, location string) error {
	return runAz("group", "create", "--name", name, "--location", location, "--output", "none")
}

// DeleteResourceGroup deletes a resource group (no-wait).
func DeleteResourceGroup(name string) error {
	return runAz("group", "delete", "--name", name, "--yes", "--no-wait")
}

// DeploymentOutput represents selected outputs from a Bicep deployment.
type DeploymentOutput struct {
	BackendEndpoint  string
	GrafanaEndpoint  string
	ConfigUIEndpoint string
	ACRLoginServer   string
	ACRName          string
	KeyVaultName     string
	MySQLServerName  string
}

// DeployBicep deploys a Bicep template and returns the outputs.
func DeployBicep(resourceGroup, templatePath string, params map[string]string) (*DeploymentOutput, error) {
	args := []string{"deployment", "group", "create",
		"--resource-group", resourceGroup,
		"--template-file", templatePath,
		"--query", "properties.outputs",
		"-o", "json",
	}
	var paramParts []string
	for k, v := range params {
		paramParts = append(paramParts, fmt.Sprintf("%s=%s", k, v))
	}
	if len(paramParts) > 0 {
		args = append(args, "--parameters")
		args = append(args, paramParts...)
	}

	out, err := exec.Command("az", args...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("bicep deployment failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("bicep deployment failed: %w", err)
	}

	var outputs map[string]struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(out, &outputs); err != nil {
		return nil, fmt.Errorf("failed to parse deployment outputs: %w", err)
	}

	return &DeploymentOutput{
		BackendEndpoint:  outputs["backendEndpoint"].Value,
		GrafanaEndpoint:  outputs["grafanaEndpoint"].Value,
		ConfigUIEndpoint: outputs["configUiEndpoint"].Value,
		ACRLoginServer:   outputs["acrLoginServer"].Value,
		ACRName:          outputs["acrName"].Value,
		KeyVaultName:     outputs["keyVaultName"].Value,
		MySQLServerName:  outputs["mysqlServerName"].Value,
	}, nil
}

// ACRLogin logs into an Azure Container Registry.
func ACRLogin(acrName string) error {
	return runAz("acr", "login", "--name", acrName)
}

// MySQLStart starts a stopped MySQL flexible server.
func MySQLStart(name, resourceGroup string) error {
	return runAz("mysql", "flexible-server", "start", "--name", name, "--resource-group", resourceGroup, "--output", "none")
}

// MySQLState returns the current state of a MySQL flexible server.
func MySQLState(name, resourceGroup string) (string, error) {
	out, err := exec.Command("az", "mysql", "flexible-server", "show",
		"--name", name,
		"--resource-group", resourceGroup,
		"--query", "state",
		"-o", "tsv",
	).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// DeleteResource deletes a specific Azure resource by type.
func DeleteResource(resourceType, name, resourceGroup string) error {
	switch resourceType {
	case "container":
		return runAz("container", "delete", "--name", name, "--resource-group", resourceGroup, "--yes")
	case "mysql":
		return runAz("mysql", "flexible-server", "delete", "--name", name, "--resource-group", resourceGroup, "--yes")
	case "acr":
		return runAz("acr", "delete", "--name", name, "--resource-group", resourceGroup, "--yes")
	case "keyvault":
		return runAz("keyvault", "delete", "--name", name, "--resource-group", resourceGroup)
	default:
		return fmt.Errorf("unknown resource type: %s", resourceType)
	}
}

func runAz(args ...string) error {
	cmd := exec.Command("az", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("az %s failed: %s", strings.Join(args[:2], " "), string(out))
	}
	return nil
}
