package gh

import (
	"testing"
)

func TestParseListOutput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single repo",
			input: "org/repo1\n",
			want:  []string{"org/repo1"},
		},
		{
			name:  "multiple repos",
			input: "org/repo1\norg/repo2\norg/repo3\n",
			want:  []string{"org/repo1", "org/repo2", "org/repo3"},
		},
		{
			name:  "trims whitespace around names",
			input: "  org/repo1  \n  org/repo2  \n",
			want:  []string{"org/repo1", "org/repo2"},
		},
		{
			name:  "skips blank lines",
			input: "org/repo1\n\norg/repo2\n",
			want:  []string{"org/repo1", "org/repo2"},
		},
		{
			name:  "empty output",
			input: "",
			want:  nil,
		},
		{
			name:  "only whitespace",
			input: "   \n   \n",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseListOutput([]byte(tt.input))
			if len(got) != len(tt.want) {
				t.Fatalf("got %d repos %v, want %d %v", len(got), got, len(tt.want), tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("repos[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGetRepoDetailsJSONParsing(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantID    int
		wantName  string
		wantFull  string
		wantHTML  string
		wantClone string
		wantErr   bool
	}{
		{
			name:      "valid JSON",
			input:     `{"id":12345,"name":"myrepo","full_name":"org/myrepo","html_url":"https://github.com/org/myrepo","clone_url":"https://github.com/org/myrepo.git"}`,
			wantID:    12345,
			wantName:  "myrepo",
			wantFull:  "org/myrepo",
			wantHTML:  "https://github.com/org/myrepo",
			wantClone: "https://github.com/org/myrepo.git",
		},
		{
			name:    "invalid JSON",
			input:   `not json`,
			wantErr: true,
		},
		{
			name:  "empty object",
			input: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			details, err := parseRepoDetails([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if details.ID != tt.wantID {
				t.Errorf("ID = %d, want %d", details.ID, tt.wantID)
			}
			if details.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", details.Name, tt.wantName)
			}
			if details.FullName != tt.wantFull {
				t.Errorf("FullName = %q, want %q", details.FullName, tt.wantFull)
			}
			if details.HTMLURL != tt.wantHTML {
				t.Errorf("HTMLURL = %q, want %q", details.HTMLURL, tt.wantHTML)
			}
			if details.CloneURL != tt.wantClone {
				t.Errorf("CloneURL = %q, want %q", details.CloneURL, tt.wantClone)
			}
		})
	}
}

func TestIsAvailable_NotInPath(t *testing.T) {
	t.Setenv("PATH", "")
	if IsAvailable() {
		t.Error("IsAvailable() = true with empty PATH, want false")
	}
}
