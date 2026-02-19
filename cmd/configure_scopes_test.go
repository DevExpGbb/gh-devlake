package cmd

import "testing"

func TestCopilotScopeID(t *testing.T) {
	tests := []struct {
		name       string
		org        string
		enterprise string
		want       string
	}{
		{
			name: "org only",
			org:  "my-org",
			want: "my-org",
		},
		{
			name:       "enterprise and org",
			org:        "my-org",
			enterprise: "my-enterprise",
			want:       "my-enterprise/my-org",
		},
		{
			name:       "enterprise only",
			enterprise: "my-enterprise",
			want:       "my-enterprise",
		},
		{
			name:       "enterprise with whitespace-only org",
			org:        "   ",
			enterprise: "my-enterprise",
			want:       "my-enterprise",
		},
		{
			name:       "whitespace-only enterprise falls back to org",
			org:        "my-org",
			enterprise: "   ",
			want:       "my-org",
		},
		{
			name:       "both with leading/trailing spaces",
			org:        "  my-org  ",
			enterprise: "  my-ent  ",
			want:       "my-ent/my-org",
		},
		{
			name: "both empty",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := copilotScopeID(tt.org, tt.enterprise)
			if got != tt.want {
				t.Errorf("copilotScopeID(%q, %q) = %q, want %q", tt.org, tt.enterprise, got, tt.want)
			}
		})
	}
}
