package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DevExpGBB/gh-devlake/internal/prompt"
)

// expectedDirName returns the recommended directory name for a deployment target.
func expectedDirName(target string) string {
	if target == "azure" {
		return "devlake-azure"
	}
	return "devlake-local"
}

// suggestDedicatedDir checks whether the user is running from the expected
// dedicated directory (e.g. devlake-local or devlake-azure). If not, it
// prints cross-platform commands to create and cd into the right directory,
// then offers "exit" vs "continue". Returns true if the user chose to exit.
func suggestDedicatedDir(target string, rerunCmd string) bool {
	dirName := expectedDirName(target)

	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	base := filepath.Base(cwd)
	if strings.EqualFold(base, dirName) {
		return false // already in the right directory
	}

	fmt.Printf("\n💡 We recommend running from a dedicated directory (%s).\n", dirName)
	fmt.Println("   This keeps DevLake files isolated from your other projects.")
	fmt.Println()
	fmt.Println("  PowerShell:")
	fmt.Printf("    $dir = \"$HOME\\%s\"\n", dirName)
	fmt.Println("    New-Item -ItemType Directory -Force $dir | Out-Null")
	fmt.Println("    Set-Location $dir")
	fmt.Printf("    %s\n", rerunCmd)
	fmt.Println()
	fmt.Println("  Bash / Zsh:")
	fmt.Printf("    dir=\"$HOME/%s\"\n", dirName)
	fmt.Println("    mkdir -p \"$dir\"")
	fmt.Println("    cd \"$dir\"")
	fmt.Printf("    %s\n", rerunCmd)
	fmt.Println()

	choices := []string{
		"exit     - run the commands above first (recommended)",
		"continue - keep going in the current directory",
	}
	picked := prompt.Select("How do you want to proceed?", choices)
	if strings.HasPrefix(picked, "exit") {
		fmt.Println("\n✅ Exiting. Re-run after changing directory.")
		fmt.Println()
		return true
	}
	return false
}

func findGitRepoRoot(start string) (string, bool) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	current := abs
	for {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", false
}
