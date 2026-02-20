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
	Token       string
	Source      string // "flag", "envfile", "environment", "prompt"
	EnvFilePath string // non-empty if loaded from envfile (for cleanup)
}

// ResolveOpts holds the plugin-specific lookup data for token resolution.
type ResolveOpts struct {
	FlagValue   string   // explicit --token value
	EnvFilePath string   // path to .devlake.env
	EnvFileKeys []string // keys to check in .devlake.env (e.g. ["GITHUB_PAT", "GITHUB_TOKEN"])
	EnvVarNames []string // environment variable names (e.g. ["GITHUB_TOKEN", "GH_TOKEN"])
	DisplayName string   // plugin display name for prompts (e.g. "GitHub Copilot")
	ScopeHint   string   // required PAT scopes hint
}

// Resolve attempts to find a PAT using the priority chain.
// All lookup keys and display names come from ResolveOpts, making this
// fully data-driven with no hardcoded plugin assumptions.
func Resolve(opts ResolveOpts) (*ResolveResult, error) {
	// 1. Explicit flag
	if opts.FlagValue != "" {
		return &ResolveResult{Token: opts.FlagValue, Source: "flag"}, nil
	}

	// 2. .devlake.env file
	envFilePath := opts.EnvFilePath
	if envFilePath == "" {
		envFilePath = ".devlake.env"
	}
	if vals, err := envfile.Load(envFilePath); err == nil {
		for _, key := range opts.EnvFileKeys {
			if v, ok := vals[key]; ok && v != "" {
				return &ResolveResult{Token: v, Source: "envfile", EnvFilePath: envFilePath}, nil
			}
		}
	}

	// 3. Environment variables
	for _, key := range opts.EnvVarNames {
		if v := os.Getenv(key); v != "" {
			return &ResolveResult{Token: v, Source: "environment"}, nil
		}
	}

	// 4. Interactive masked prompt
	displayName := opts.DisplayName
	if displayName == "" {
		displayName = "PAT"
	}
	if !term.IsTerminal(int(syscall.Stdin)) {
		envVarExample := ""
		if len(opts.EnvVarNames) > 0 {
			envVarExample = opts.EnvVarNames[0]
		}
		return nil, fmt.Errorf("no %s token found and stdin is not a terminal.\n"+
			"Provide a token via --token, .devlake.env file, or $%s", displayName, envVarExample)
	}

	tok, err := promptMasked(displayName, opts.ScopeHint)
	if err != nil {
		return nil, err
	}
	return &ResolveResult{Token: tok, Source: "prompt"}, nil
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
