package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// captureStdout runs fn and returns everything written to os.Stdout.
func captureStdout(fn func()) string {
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	os.Stdout = w

	// Ensure os.Stdout is always restored, even if fn panics.
	defer func() {
		os.Stdout = orig
	}()
	defer func() {
		_ = r.Close()
	}()

	fn()

	// Close the writer before copying so io.Copy sees EOF and does not block.
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

// --- collectFlagDefs ---

func TestCollectAllScopeFlagDefs_IncludesGitHubFlags(t *testing.T) {
	defs := collectAllScopeFlagDefs()
	names := make(map[string]bool)
	for _, fd := range defs {
		names[fd.Name] = true
	}
	expected := []string{"repos", "repos-file", "deployment-pattern", "production-pattern", "incident-label"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected scope flag %q to be present", name)
		}
	}
}

func TestCollectAllScopeFlagDefs_IncludesCopilotEnterprise(t *testing.T) {
	defs := collectAllScopeFlagDefs()
	for _, fd := range defs {
		if fd.Name == "enterprise" {
			if len(fd.Plugins) == 0 {
				t.Error("enterprise scope flag should have at least one plugin")
			}
			found := false
			for _, p := range fd.Plugins {
				if p == "gh-copilot" {
					found = true
				}
			}
			if !found {
				t.Errorf("enterprise scope flag should list gh-copilot, got %v", fd.Plugins)
			}
			return
		}
	}
	t.Error("enterprise scope flag not found in collected defs")
}

func TestCollectAllConnectionFlagDefs_IncludesCopilotEnterprise(t *testing.T) {
	defs := collectAllConnectionFlagDefs()
	for _, fd := range defs {
		if fd.Name == "enterprise" {
			found := false
			for _, p := range fd.Plugins {
				if p == "gh-copilot" {
					found = true
				}
			}
			if !found {
				t.Errorf("enterprise connection flag should list gh-copilot, got %v", fd.Plugins)
			}
			return
		}
	}
	t.Error("enterprise connection flag not found in collected defs")
}

func TestCollectAllScopeFlagDefs_GitHubDoesNotHaveEnterprise(t *testing.T) {
	defs := collectAllScopeFlagDefs()
	for _, fd := range defs {
		if fd.Name == "enterprise" {
			for _, p := range fd.Plugins {
				if p == "github" {
					t.Error("enterprise scope flag should not list github")
				}
			}
		}
	}
}

// --- warnIrrelevantFlags ---

func makeTestCmd(flags ...string) *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	for _, f := range flags {
		var v string
		cmd.Flags().StringVar(&v, f, "", "")
	}
	return cmd
}

func TestWarnIrrelevantFlags_NoWarningForRelevantFlag(t *testing.T) {
	allDefs := collectAllScopeFlagDefs()
	githubDef := FindConnectionDef("github")
	cmd := makeTestCmd("repos", "deployment-pattern")
	_ = cmd.Flags().Set("repos", "org/repo1")

	out := captureStdout(func() {
		warnIrrelevantFlags(cmd, githubDef, allDefs)
	})
	if strings.Contains(out, "⚠️") {
		t.Errorf("expected no warning for --repos with github plugin, got: %q", out)
	}
}

func TestWarnIrrelevantFlags_WarnsForEnterpriseWithGitHub(t *testing.T) {
	allDefs := collectAllScopeFlagDefs()
	githubDef := FindConnectionDef("github")
	cmd := makeTestCmd("enterprise")
	_ = cmd.Flags().Set("enterprise", "my-ent")

	out := captureStdout(func() {
		warnIrrelevantFlags(cmd, githubDef, allDefs)
	})
	if !strings.Contains(out, "⚠️") {
		t.Errorf("expected warning for --enterprise with github plugin, got: %q", out)
	}
	if !strings.Contains(out, "--enterprise") {
		t.Errorf("warning should mention --enterprise, got: %q", out)
	}
	if !strings.Contains(out, "GitHub") {
		t.Errorf("warning should mention the plugin display name, got: %q", out)
	}
}

func TestWarnIrrelevantFlags_WarnsForReposWithCopilot(t *testing.T) {
	allDefs := collectAllScopeFlagDefs()
	copilotDef := FindConnectionDef("gh-copilot")
	cmd := makeTestCmd("repos", "repos-file", "deployment-pattern")
	_ = cmd.Flags().Set("repos", "org/repo1")

	out := captureStdout(func() {
		warnIrrelevantFlags(cmd, copilotDef, allDefs)
	})
	if !strings.Contains(out, "⚠️") {
		t.Errorf("expected warning for --repos with gh-copilot plugin, got: %q", out)
	}
}

func TestWarnIrrelevantFlags_NoWarningForSharedFlag(t *testing.T) {
	allDefs := collectAllScopeFlagDefs()
	githubDef := FindConnectionDef("github")
	cmd := makeTestCmd("org", "plugin")
	_ = cmd.Flags().Set("org", "my-org")

	out := captureStdout(func() {
		warnIrrelevantFlags(cmd, githubDef, allDefs)
	})
	if strings.Contains(out, "⚠️") {
		t.Errorf("expected no warning for --org (shared flag), got: %q", out)
	}
}

func TestWarnIrrelevantFlags_AppliesTo_InMessage(t *testing.T) {
	allDefs := collectAllScopeFlagDefs()
	githubDef := FindConnectionDef("github")
	cmd := makeTestCmd("enterprise")
	_ = cmd.Flags().Set("enterprise", "my-ent")

	out := captureStdout(func() {
		warnIrrelevantFlags(cmd, githubDef, allDefs)
	})
	if !strings.Contains(out, "applies to:") {
		t.Errorf("warning should include 'applies to:' message, got: %q", out)
	}
	if !strings.Contains(out, "GitHub Copilot") {
		t.Errorf("warning should mention GitHub Copilot, got: %q", out)
	}
}

func TestWarnIrrelevantFlags_NoWarningWhenNoFlagsSet(t *testing.T) {
	allDefs := collectAllScopeFlagDefs()
	githubDef := FindConnectionDef("github")
	cmd := makeTestCmd("repos", "enterprise")
	// Don't set any flags

	out := captureStdout(func() {
		warnIrrelevantFlags(cmd, githubDef, allDefs)
	})
	if strings.Contains(out, "⚠️") {
		t.Errorf("expected no warning when no flags explicitly set, got: %q", out)
	}
}

func TestWarnIrrelevantFlags_MultipleIrrelevantFlags(t *testing.T) {
	allDefs := collectAllScopeFlagDefs()
	copilotDef := FindConnectionDef("gh-copilot")
	cmd := makeTestCmd("repos", "repos-file", "deployment-pattern", "production-pattern", "incident-label")
	_ = cmd.Flags().Set("repos", "org/repo1")
	_ = cmd.Flags().Set("deployment-pattern", "(?i)deploy")

	out := captureStdout(func() {
		warnIrrelevantFlags(cmd, copilotDef, allDefs)
	})
	// Should warn about both --repos and --deployment-pattern
	count := strings.Count(out, "⚠️")
	if count < 2 {
		t.Errorf("expected at least 2 warnings, got %d; output: %q", count, out)
	}
}

func TestWarnIrrelevantFlags_CopilotEnterprise_NoWarning(t *testing.T) {
	allDefs := collectAllScopeFlagDefs()
	copilotDef := FindConnectionDef("gh-copilot")
	cmd := makeTestCmd("enterprise")
	_ = cmd.Flags().Set("enterprise", "my-ent")

	out := captureStdout(func() {
		warnIrrelevantFlags(cmd, copilotDef, allDefs)
	})
	if strings.Contains(out, "⚠️") {
		t.Errorf("expected no warning for --enterprise with gh-copilot, got: %q", out)
	}
}

// --- printContextualFlagHelp ---

func TestPrintContextualFlagHelp_PrintsFlags(t *testing.T) {
	githubDef := FindConnectionDef("github")
	out := captureStdout(func() {
		printContextualFlagHelp(githubDef, githubDef.ScopeFlags, "Scope")
	})
	if !strings.Contains(out, "📚") {
		t.Errorf("expected 📚 in output, got: %q", out)
	}
	if !strings.Contains(out, "Scope flags for GitHub") {
		t.Errorf("expected header, got: %q", out)
	}
	for _, name := range []string{"repos", "repos-file", "deployment-pattern", "production-pattern", "incident-label"} {
		if !strings.Contains(out, "--"+name) {
			t.Errorf("expected --%s in output, got: %q", name, out)
		}
	}
}

func TestPrintContextualFlagHelp_EmptyFlagDefs_NoOutput(t *testing.T) {
	githubDef := FindConnectionDef("github")
	out := captureStdout(func() {
		printContextualFlagHelp(githubDef, []FlagDef{}, "Connection")
	})
	if out != "" {
		t.Errorf("expected no output for empty flag defs, got: %q", out)
	}
}

func TestPrintContextualFlagHelp_CopilotConnectionFlags(t *testing.T) {
	copilotDef := FindConnectionDef("gh-copilot")
	out := captureStdout(func() {
		printContextualFlagHelp(copilotDef, copilotDef.ConnectionFlags, "Connection")
	})
	if !strings.Contains(out, "--enterprise") {
		t.Errorf("expected --enterprise in output, got: %q", out)
	}
	if !strings.Contains(out, "Connection flags for GitHub Copilot") {
		t.Errorf("expected header, got: %q", out)
	}
}

func TestPrintContextualFlagHelp_AlignedOutput(t *testing.T) {
	// Verify columns are aligned: all descriptions should start at the same absolute position.
	githubDef := FindConnectionDef("github")
	out := captureStdout(func() {
		printContextualFlagHelp(githubDef, githubDef.ScopeFlags, "Scope")
	})
	lines := strings.Split(out, "\n")
	// Find lines starting with "   --" and record the column where the description begins.
	// Format: "   --<name><spaces><description>"
	// Description starts at the first non-space after the flag name.
	var descCols []int
	for _, line := range lines {
		if !strings.HasPrefix(line, "   --") {
			continue
		}
		// Skip "   --", then scan past name, then find start of description.
		rest := line[5:] // skip "   --"
		i := 0
		for i < len(rest) && rest[i] != ' ' {
			i++ // skip flag name
		}
		for i < len(rest) && rest[i] == ' ' {
			i++ // skip padding
		}
		descCols = append(descCols, 5+i) // 5 = len("   --")
	}
	if len(descCols) < 2 {
		return // not enough lines to check alignment
	}
	for i := 1; i < len(descCols); i++ {
		if descCols[i] != descCols[0] {
			t.Errorf("description columns not aligned: line 0 at col %d, line %d at col %d\n%s",
				descCols[0], i, descCols[i], out)
		}
	}
}

// --- Registry FlagDef entries ---

func TestConnectionDef_ScopeFlags_GitHub(t *testing.T) {
	def := FindConnectionDef("github")
	if def == nil {
		t.Fatal("github def not found")
	}
	names := make(map[string]bool)
	for _, fd := range def.ScopeFlags {
		names[fd.Name] = true
		if fd.Description == "" {
			t.Errorf("ScopeFlag %q has empty description", fd.Name)
		}
	}
	for _, want := range []string{"repos", "repos-file", "deployment-pattern", "production-pattern", "incident-label"} {
		if !names[want] {
			t.Errorf("github ScopeFlags should include %q", want)
		}
	}
}

func TestConnectionDef_ScopeFlags_Copilot(t *testing.T) {
	def := FindConnectionDef("gh-copilot")
	if def == nil {
		t.Fatal("gh-copilot def not found")
	}
	names := make(map[string]bool)
	for _, fd := range def.ScopeFlags {
		names[fd.Name] = true
	}
	if !names["enterprise"] {
		t.Error("gh-copilot ScopeFlags should include enterprise")
	}
}

func TestConnectionDef_ConnectionFlags_Copilot(t *testing.T) {
	def := FindConnectionDef("gh-copilot")
	if def == nil {
		t.Fatal("gh-copilot def not found")
	}
	names := make(map[string]bool)
	for _, fd := range def.ConnectionFlags {
		names[fd.Name] = true
	}
	if !names["enterprise"] {
		t.Error("gh-copilot ConnectionFlags should include enterprise")
	}
}

// --- collectFlagDefs deduplication ---

func TestCollectFlagDefs_MergesDuplicateNames(t *testing.T) {
	// Temporarily inject a duplicate flag name across two defs to test merging.
	orig := connectionRegistry
	defer func() { connectionRegistry = orig }()

	connectionRegistry = []*ConnectionDef{
		{Plugin: "pluginA", Available: true, ScopeFlags: []FlagDef{{Name: "shared-flag", Description: "desc A"}}},
		{Plugin: "pluginB", Available: true, ScopeFlags: []FlagDef{{Name: "shared-flag", Description: "desc A"}}},
	}

	defs := collectAllScopeFlagDefs()
	if len(defs) != 1 {
		t.Fatalf("expected 1 merged FlagDef, got %d", len(defs))
	}
	if len(defs[0].Plugins) != 2 {
		t.Errorf("expected 2 plugins for merged flag, got %d: %v", len(defs[0].Plugins), defs[0].Plugins)
	}
}

// Verify that the --help string for scope add contains plugin-specific flag names.
func TestScopeAddCmd_LongHelp_ContainsPluginFlags(t *testing.T) {
	cmd := newScopeAddCmd()
	long := cmd.Long
	for _, name := range []string{"repos", "enterprise", "deployment-pattern"} {
		if !strings.Contains(long, name) {
			t.Errorf("scope add Long help should mention %q", name)
		}
	}
}

// Ensure warnIrrelevantFlags handles a nil/missing ConnectionDef gracefully.
func TestWarnIrrelevantFlags_UnknownFlagInDefs(t *testing.T) {
	// Custom flag defs with no matching plugin
	customDefs := []FlagDef{
		{Name: "custom-flag", Description: "custom", Plugins: []string{"some-plugin"}},
	}
	githubDef := FindConnectionDef("github")
	cmd := makeTestCmd("custom-flag")
	_ = cmd.Flags().Set("custom-flag", "value")

	out := captureStdout(func() {
		warnIrrelevantFlags(cmd, githubDef, customDefs)
	})
	if !strings.Contains(out, "⚠️") {
		t.Errorf("expected warning for custom-flag not used by github, got: %q", out)
	}
	// "applies to" should include the raw plugin slug since FindConnectionDef returns nil
	if !strings.Contains(out, "some-plugin") {
		t.Errorf("expected plugin slug in warning, got: %q", out)
	}
}

// verify that the example in the issue (--enterprise with --plugin github) triggers a warning.
func TestWarnIrrelevantFlags_IssueExampleFlow(t *testing.T) {
	allDefs := collectAllScopeFlagDefs()
	githubDef := FindConnectionDef("github")

	cmd := makeTestCmd("enterprise", "repos")
	_ = cmd.Flags().Set("enterprise", "my-ent")
	_ = cmd.Flags().Set("repos", "org/repo1")

	var output string
	output = captureStdout(func() {
		warnIrrelevantFlags(cmd, githubDef, allDefs)
	})

	// Should warn about --enterprise
	if !strings.Contains(output, "--enterprise") {
		t.Errorf("expected warning about --enterprise, got: %q", output)
	}
	// Should NOT warn about --repos (it's valid for github)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "⚠️") && strings.Contains(line, "--repos") {
			t.Errorf("should not warn about --repos for github plugin, got: %q", line)
		}
	}
	// Warning message should indicate it applies to GitHub Copilot
	if !strings.Contains(output, "GitHub Copilot") {
		t.Errorf("warning should mention GitHub Copilot, got: %q", output)
	}
}
