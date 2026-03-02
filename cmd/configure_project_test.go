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
	flags := []string{"project-name", "time-after", "cron", "skip-sync", "wait", "timeout"}
	for _, f := range flags {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("expected flag --%s to be registered on project add cmd", f)
		}
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
