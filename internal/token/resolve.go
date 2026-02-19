// Package token resolves a Personal Access Token from multiple sources.
//
// Priority order:
//  1. Explicit flag value (--token)
//  2. .devlake.env file (plugin-specific key, e.g. GITLAB_TOKEN=...)
//  3. Plugin-specific environment variable (e.g. $GITLAB_TOKEN)
//  4. Interactive masked prompt (terminal)
package token

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/DevExpGBB/gh-devlake/internal/envfile"
	"golang.org/x/term"
)

// ResolveResult contains the resolved token and its source.
type ResolveResult struct {
	Token         string
	Source        string // "flag", "envfile", "environment", "prompt"
	EnvFilePath   string // non-empty if loaded from envfile (for cleanup)
}

// Resolve attempts to find a PAT using the priority chain.
// plugin determines which env vars and env file keys to check (e.g. "github", "gitlab", "azure-devops").
// scopeHint is displayed in the interactive prompt to guide the user on required scopes.
// Returns an error only if no token can be obtained.
func Resolve(plugin, flagValue, envFilePath, scopeHint string) (*ResolveResult, error) {
	// 1. Explicit flag
	if flagValue != "" {
		return &ResolveResult{Token: flagValue, Source: "flag"}, nil
	}

	// 2. .devlake.env file
	if envFilePath == "" {
		envFilePath = ".devlake.env"
	}
	if vals, err := envfile.Load(envFilePath); err == nil {
		for _, key := range pluginEnvFileKeys(plugin) {
			if v, ok := vals[key]; ok && v != "" {
				return &ResolveResult{Token: v, Source: "envfile", EnvFilePath: envFilePath}, nil
			}
		}
	}

	// 3. Environment variables
	for _, key := range pluginEnvVarKeys(plugin) {
		if v := os.Getenv(key); v != "" {
			return &ResolveResult{Token: v, Source: "environment"}, nil
		}
	}

	// 4. Interactive masked prompt
	displayName := pluginDisplayName(plugin)
	if !term.IsTerminal(int(syscall.Stdin)) {
		envVarExample := pluginEnvVarKeys(plugin)[0]
		return nil, fmt.Errorf("no %s PAT found and stdin is not a terminal.\n"+
			"Provide a token via --token, .devlake.env file, or $%s", displayName, envVarExample)
	}

	tok, err := promptMasked(displayName, scopeHint)
	if err != nil {
		return nil, err
	}
	return &ResolveResult{Token: tok, Source: "prompt"}, nil
}

// pluginEnvFileKeys returns the ordered .devlake.env key names to check for the given plugin.
func pluginEnvFileKeys(plugin string) []string {
	switch plugin {
	case "gitlab":
		return []string{"GITLAB_TOKEN"}
	case "azure-devops":
		return []string{"AZURE_DEVOPS_PAT"}
	default: // "github", "gh-copilot", or unknown
		return []string{"GITHUB_PAT", "GITHUB_TOKEN", "GH_TOKEN"}
	}
}

// pluginEnvVarKeys returns the ordered environment variable names to check for the given plugin.
func pluginEnvVarKeys(plugin string) []string {
	switch plugin {
	case "gitlab":
		return []string{"GITLAB_TOKEN"}
	case "azure-devops":
		return []string{"AZURE_DEVOPS_PAT"}
	default: // "github", "gh-copilot", or unknown
		return []string{"GITHUB_TOKEN", "GH_TOKEN"}
	}
}

// pluginDisplayName returns a human-readable label for prompt messages.
func pluginDisplayName(plugin string) string {
	switch plugin {
	case "gh-copilot":
		return "GitHub Copilot"
	case "gitlab":
		return "GitLab"
	case "azure-devops":
		return "Azure DevOps"
	default:
		return "GitHub"
	}
}

func promptMasked(displayName, scopeHint string) (string, error) {
	if scopeHint != "" {
		fmt.Fprintf(os.Stderr, "Required PAT scopes: %s\n", scopeHint)
	}
	fmt.Fprintf(os.Stderr, "%s Personal Access Token: ", displayName)
	raw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr) // newline after masked input
	if err != nil {
		return "", fmt.Errorf("failed to read token: %w", err)
	}

	tok := strings.TrimSpace(string(raw))
	if tok == "" {
		return "", fmt.Errorf("no token provided")
	}
	return tok, nil
}
