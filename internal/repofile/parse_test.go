package repofile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParse_Basic(t *testing.T) {
	content := `# My repos
repo
org/repo1
org/repo2
# comment
org/repo3
`
	path := filepath.Join(t.TempDir(), "repos.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	repos, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"org/repo1", "org/repo2", "org/repo3"}
	if len(repos) != len(want) {
		t.Fatalf("got %d repos, want %d", len(repos), len(want))
	}
	for i, r := range repos {
		if r != want[i] {
			t.Errorf("repos[%d] = %q, want %q", i, r, want[i])
		}
	}
}

func TestParse_CSV(t *testing.T) {
	content := `repo,description
org/repo1,My first repo
org/repo2,My second repo
`
	path := filepath.Join(t.TempDir(), "repos.csv")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	repos, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"org/repo1", "org/repo2"}
	if len(repos) != len(want) {
		t.Fatalf("got %d repos, want %d", len(repos), len(want))
	}
	for i, r := range repos {
		if r != want[i] {
			t.Errorf("repos[%d] = %q, want %q", i, r, want[i])
		}
	}
}

func TestParse_Empty(t *testing.T) {
	content := `# only comments
# nothing here
`
	path := filepath.Join(t.TempDir(), "repos.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	repos, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 0 {
		t.Errorf("got %d repos, want 0", len(repos))
	}
}

func TestParse_FileNotFound(t *testing.T) {
	_, err := Parse("/nonexistent/repos.txt")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
