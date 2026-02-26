// Package gitclone provides a shared git-clone helper used by both local
// and Azure deployment commands to clone DevLake forks.
package gitclone

import (
	"fmt"
	"os/exec"
)

// DefaultForkURL is the default DevLake fork URL offered during interactive prompts.
const DefaultForkURL = "https://github.com/DevExpGBB/incubator-devlake"

// Clone performs a shallow clone (depth 1) of repoURL into targetDir.
func Clone(repoURL, targetDir string) error {
	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, targetDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %w\n%s", err, string(out))
	}
	return nil
}
