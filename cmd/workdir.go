package cmd

import (
	"fmt"
	"os"
	"path/filepath"
)

func homeDirTip(exampleSubdir string) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		fmt.Println("\n💡 Tip: It's recommended to run this command from your home directory (~/) or a dedicated folder.")
		return
	}
	example := filepath.Join(home, exampleSubdir)
	fmt.Printf("\n💡 Tip: It's recommended to run this command from your home directory (~/) or a dedicated folder (e.g. %s).\n", example)
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

func warnIfWritingIntoGitRepo(targetDir string, files string) {
	abs, err := filepath.Abs(targetDir)
	if err != nil {
		abs = targetDir
	}
	if root, ok := findGitRepoRoot(abs); ok {
		fmt.Printf("\n⚠️  You're running inside a git repository: %s\n", root)
		fmt.Printf("   %s will be written to: %s\n", files, abs)
		fmt.Println("   Consider running from ~/ (home) or a dedicated folder, or pass --dir.")
	}
}
