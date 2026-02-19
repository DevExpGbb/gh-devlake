package cmd

import "testing"

func TestBuildCreateRequest_RateLimit(t *testing.T) {
	tests := []struct {
		name     string
		plugin   string
		wantRate int
	}{
		{"github uses 4500", "github", 4500},
		{"gh-copilot uses 5000", "gh-copilot", 5000},
		{"unknown uses 4500", "gitlab", 4500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := &ConnectionDef{Plugin: tt.plugin, Endpoint: "https://api.github.com/"}
			req := def.BuildCreateRequest("test", ConnectionParams{Token: "tok"})
			if req.RateLimitPerHour != tt.wantRate {
				t.Errorf("got rate limit %d, want %d", req.RateLimitPerHour, tt.wantRate)
			}
		})
	}
}

func TestBuildCreateRequest_EnterpriseOrg(t *testing.T) {
	def := &ConnectionDef{
		Plugin:          "gh-copilot",
		Endpoint:        "https://api.github.com/",
		NeedsOrg:        true,
		NeedsEnterprise: true,
	}

	t.Run("org and enterprise", func(t *testing.T) {
		req := def.BuildCreateRequest("test", ConnectionParams{
			Token:      "tok",
			Org:        "my-org",
			Enterprise: "my-ent",
		})
		if req.Organization != "my-org" {
			t.Errorf("got Organization %q, want %q", req.Organization, "my-org")
		}
		if req.Enterprise != "my-ent" {
			t.Errorf("got Enterprise %q, want %q", req.Enterprise, "my-ent")
		}
	})

	t.Run("org only", func(t *testing.T) {
		req := def.BuildCreateRequest("test", ConnectionParams{
			Token: "tok",
			Org:   "my-org",
		})
		if req.Organization != "my-org" {
			t.Errorf("got Organization %q, want %q", req.Organization, "my-org")
		}
		if req.Enterprise != "" {
			t.Errorf("got Enterprise %q, want empty", req.Enterprise)
		}
	})

	t.Run("enterprise only", func(t *testing.T) {
		req := def.BuildCreateRequest("test", ConnectionParams{
			Token:      "tok",
			Enterprise: "my-ent",
		})
		if req.Organization != "" {
			t.Errorf("got Organization %q, want empty", req.Organization)
		}
		if req.Enterprise != "my-ent" {
			t.Errorf("got Enterprise %q, want %q", req.Enterprise, "my-ent")
		}
	})
}

func TestBuildTestRequest_CopilotFields(t *testing.T) {
	def := &ConnectionDef{
		Plugin:          "gh-copilot",
		Endpoint:        "https://api.github.com/",
		NeedsOrg:        true,
		NeedsEnterprise: true,
	}

	t.Run("includes org and enterprise", func(t *testing.T) {
		req := def.BuildTestRequest(ConnectionParams{
			Token:      "tok",
			Org:        "my-org",
			Enterprise: "my-ent",
		})
		if req.Organization != "my-org" {
			t.Errorf("got Organization %q, want %q", req.Organization, "my-org")
		}
		if req.Enterprise != "my-ent" {
			t.Errorf("got Enterprise %q, want %q", req.Enterprise, "my-ent")
		}
		if req.RateLimitPerHour != 5000 {
			t.Errorf("got rate limit %d, want 5000", req.RateLimitPerHour)
		}
	})

	t.Run("github does not include org/enterprise", func(t *testing.T) {
		ghDef := &ConnectionDef{
			Plugin:       "github",
			Endpoint:     "https://api.github.com/",
			SupportsTest: true,
		}
		req := ghDef.BuildTestRequest(ConnectionParams{
			Token:      "tok",
			Org:        "ignored",
			Enterprise: "ignored",
		})
		if req.Organization != "" {
			t.Errorf("github test request should not have Organization, got %q", req.Organization)
		}
		if req.Enterprise != "" {
			t.Errorf("github test request should not have Enterprise, got %q", req.Enterprise)
		}
		if req.RateLimitPerHour != 4500 {
			t.Errorf("got rate limit %d, want 4500", req.RateLimitPerHour)
		}
		if !req.EnableGraphql {
			t.Error("github test request should have EnableGraphql=true")
		}
	})
}
