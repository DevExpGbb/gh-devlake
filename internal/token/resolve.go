// Package token resolves a GitHub Personal Access Token from multiple sources.
//
// Priority order:
//  1. Explicit flag value (--token / --github-token)
//  2. .devlake.env file (GITHUB_PAT=...)
//  3. Environment variables ($GITHUB_TOKEN / $GH_TOKEN)
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
// scopeHint is displayed in the interactive prompt to guide the user on required scopes.
// Returns an error only if no token can be obtained.
func Resolve(flagValue, envFilePath, scopeHint string) (*ResolveResult, error) {
	// 1. Explicit flag
	if flagValue != "" {
		return &ResolveResult{Token: flagValue, Source: "flag"}, nil
	}

	// 2. .devlake.env file
	if envFilePath == "" {
		envFilePath = ".devlake.env"
	}
	if vals, err := envfile.Load(envFilePath); err == nil {
		for _, key := range []string{"GITHUB_PAT", "GITHUB_TOKEN", "GH_TOKEN"} {
			if v, ok := vals[key]; ok && v != "" {
				return &ResolveResult{Token: v, Source: "envfile", EnvFilePath: envFilePath}, nil
			}
		}
	}

	// 3. Environment variables
	for _, key := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if v := os.Getenv(key); v != "" {
			return &ResolveResult{Token: v, Source: "environment"}, nil
		}
	}

	// 4. Interactive masked prompt
	if !term.IsTerminal(int(syscall.Stdin)) {
		return nil, fmt.Errorf("no GitHub PAT found and stdin is not a terminal.\n" +
			"Provide a token via --token, .devlake.env file, or $GITHUB_TOKEN")
	}

	tok, err := promptMasked(scopeHint)
	if err != nil {
		return nil, err
	}
	return &ResolveResult{Token: tok, Source: "prompt"}, nil
}

func promptMasked(scopeHint string) (string, error) {
	if scopeHint != "" {
		fmt.Fprintf(os.Stderr, "Required PAT scopes: %s\n", scopeHint)
	}
	fmt.Fprint(os.Stderr, "GitHub Personal Access Token: ")
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
