package cmd

import (
	"testing"
)

func TestProjectAddCmd_Registered(t *testing.T) {
	cmd := newConfigureProjectsCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Use == "add" {
			found = true
			break
		}
	}
	if !found {
		t.Error("'add' subcommand not registered under project command")
	}
}

func TestProjectListCmd_Registered(t *testing.T) {
	cmd := newConfigureProjectsCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Use == "list" {
			found = true
			break
		}
	}
	if !found {
		t.Error("'list' subcommand not registered under project command")
	}
}

func TestProjectDeleteCmd_Registered(t *testing.T) {
	cmd := newConfigureProjectsCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Use == "delete" {
			found = true
			break
		}
	}
	if !found {
		t.Error("'delete' subcommand not registered under project command")
	}
}

func TestProjectCmd_NoRunE(t *testing.T) {
	cmd := newConfigureProjectsCmd()
	if cmd.RunE != nil {
		t.Error("project parent command should not have RunE — it should only print help")
	}
	if cmd.Run != nil {
		t.Error("project parent command should not have Run — it should only print help")
	}
}

func TestNewProjectAddCmd_Flags(t *testing.T) {
	cmd := newProjectAddCmd()
	if cmd.Use != "add" {
		t.Errorf("expected Use %q, got %q", "add", cmd.Use)
	}
	flags := []string{"project-name", "connections", "time-after", "cron", "skip-sync", "wait", "timeout"}
	for _, f := range flags {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("expected flag --%s to be registered on project add cmd", f)
		}
	}
}

func TestParseConnectionSpecs_Valid(t *testing.T) {
	specs, err := parseConnectionSpecs("github:1,gh-copilot:2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(specs))
	}
	if specs[0].plugin != "github" || specs[0].id != 1 {
		t.Errorf("spec[0]: got plugin=%q id=%d, want github:1", specs[0].plugin, specs[0].id)
	}
	if specs[1].plugin != "gh-copilot" || specs[1].id != 2 {
		t.Errorf("spec[1]: got plugin=%q id=%d, want gh-copilot:2", specs[1].plugin, specs[1].id)
	}
}

func TestParseConnectionSpecs_Empty(t *testing.T) {
	specs, err := parseConnectionSpecs("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if specs != nil {
		t.Errorf("expected nil for empty string, got %v", specs)
	}
}

func TestParseConnectionSpecs_InvalidFormat(t *testing.T) {
	_, err := parseConnectionSpecs("github-without-id")
	if err == nil {
		t.Fatal("expected error for missing colon separator")
	}
}

func TestParseConnectionSpecs_InvalidID(t *testing.T) {
	_, err := parseConnectionSpecs("github:abc")
	if err == nil {
		t.Fatal("expected error for non-numeric ID")
	}
}

func TestParseConnectionSpecs_UnknownPlugin(t *testing.T) {
	_, err := parseConnectionSpecs("nonexistent:1")
	if err == nil {
		t.Fatal("expected error for unknown plugin")
	}
}

func TestParseConnectionSpecs_SingleConnection(t *testing.T) {
	specs, err := parseConnectionSpecs("jenkins:5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	if specs[0].plugin != "jenkins" || specs[0].id != 5 {
		t.Errorf("spec[0]: got plugin=%q id=%d, want jenkins:5", specs[0].plugin, specs[0].id)
	}
}

func TestParseConnectionSpecs_PluginAlias(t *testing.T) {
	specs, err := parseConnectionSpecs("azure-devops:3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	// Should resolve the alias to the real plugin slug
	if specs[0].plugin != "azuredevops_go" {
		t.Errorf("expected plugin %q after alias resolution, got %q", "azuredevops_go", specs[0].plugin)
	}
}

func TestParseConnectionSpecs_WhitespaceHandling(t *testing.T) {
	specs, err := parseConnectionSpecs(" github:1 , gh-copilot:2 ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(specs))
	}
}

func TestNewProjectDeleteCmd_Flags(t *testing.T) {
	cmd := newProjectDeleteCmd()
	if cmd.Use != "delete" {
		t.Errorf("expected Use %q, got %q", "delete", cmd.Use)
	}
	if cmd.Flags().Lookup("name") == nil {
		t.Error("expected flag --name to be registered on project delete cmd")
	}
}

func TestNewProjectListCmd_NoFlags(t *testing.T) {
	cmd := newProjectListCmd()
	if cmd.Use != "list" {
		t.Errorf("expected Use %q, got %q", "list", cmd.Use)
	}
}
