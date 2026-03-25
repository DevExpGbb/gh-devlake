package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
)

func newProjectAddCmd() *cobra.Command {
	var opts ProjectOpts
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a DevLake project and start data collection",
		Long: `Creates a DevLake project that groups data from your connections.

A project ties together existing scopes (repos, orgs) from your connections
into a single view with DORA metrics. It creates a sync schedule (blueprint)
that collects data on a cron schedule (daily by default).

Prerequisites: run 'gh devlake configure scope' first to add scopes.

This command will:
  1. Discover existing scopes on your connections
  2. Let you choose which scopes to include
  3. Create the project with DORA metrics enabled
  4. Configure a sync blueprint
  5. Trigger the first data collection

Flag mode (non-interactive):
  Provide --project-name and --connections to skip all prompts.
  --connections format: "plugin:connID,plugin:connID" e.g. "github:1,gh-copilot:1"
  All scopes on each specified connection are included automatically.

Example (interactive):
  gh devlake configure project add

Example (non-interactive):
  gh devlake configure project add --project-name my-team --connections "github:1,gh-copilot:1"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectAdd(cmd, args, &opts)
		},
	}

	cmd.Flags().StringVar(&opts.ProjectName, "project-name", "", "DevLake project name")
	cmd.Flags().StringVar(&opts.Connections, "connections", "", `Connections to include: "plugin:connID,..." (flag mode)`)
	cmd.Flags().StringVar(&opts.TimeAfter, "time-after", "", "Only collect data after this date (default: 6 months ago)")
	cmd.Flags().StringVar(&opts.Cron, "cron", "0 0 * * *", "Blueprint cron schedule")
	cmd.Flags().BoolVar(&opts.SkipSync, "skip-sync", false, "Skip triggering the first data sync")
	cmd.Flags().BoolVar(&opts.Wait, "wait", true, "Wait for pipeline to complete")
	cmd.Flags().DurationVar(&opts.Timeout, "timeout", 5*time.Minute, "Max time to wait for pipeline")

	return cmd
}

// parseConnectionSpecs parses "github:1,gh-copilot:1" into connChoice entries.
func parseConnectionSpecs(spec string) ([]connChoice, error) {
	if spec == "" {
		return nil, nil
	}
	parts := strings.Split(spec, ",")
	var choices []connChoice
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		colonIdx := strings.LastIndex(p, ":")
		if colonIdx < 0 {
			return nil, fmt.Errorf("invalid connection spec %q — expected plugin:connID", p)
		}
		plugin := p[:colonIdx]
		idStr := p[colonIdx+1:]
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return nil, fmt.Errorf("invalid connection ID %q in spec %q", idStr, p)
		}
		def := FindConnectionDef(plugin)
		if def == nil {
			return nil, fmt.Errorf("unknown plugin %q in connection spec", plugin)
		}
		choices = append(choices, connChoice{
			plugin: def.Plugin,
			id:     id,
			label:  fmt.Sprintf("%s (ID: %d)", def.DisplayName, id),
		})
	}
	return choices, nil
}

func runProjectAdd(cmd *cobra.Command, args []string, opts *ProjectOpts) error {
	// Flag mode: --connections provided → non-interactive path
	if opts.Connections != "" {
		return runProjectAddFlagMode(cmd, args, opts)
	}
	return runConfigureProjects(cmd, args, opts)
}

// runProjectAddFlagMode creates a project non-interactively using --connections.
func runProjectAddFlagMode(cmd *cobra.Command, args []string, opts *ProjectOpts) error {
	printBanner("DevLake — Project Setup")

	if opts.ProjectName == "" {
		return fmt.Errorf("--project-name is required when using --connections")
	}

	specs, err := parseConnectionSpecs(opts.Connections)
	if err != nil {
		return fmt.Errorf("parsing --connections: %w", err)
	}
	if len(specs) == 0 {
		return fmt.Errorf("--connections must specify at least one connection")
	}

	client := opts.Client
	statePath := opts.StatePath
	state := opts.State

	// Discover DevLake if not pre-resolved by an orchestrator
	if client == nil {
		var disc *devlake.DiscoveryResult
		client, disc, err = discoverClient(cfgURL)
		if err != nil {
			return err
		}
		statePath, state = devlake.FindStateFile(disc.URL, disc.GrafanaURL)
	}

	fmt.Printf("\n🔍 Discovering scopes for %d connection(s)...\n", len(specs))

	var added []addedConnection
	for _, spec := range specs {
		ac, err := listConnectionScopes(client, spec)
		if err != nil {
			return fmt.Errorf("connection %s: %w", spec.label, err)
		}
		added = append(added, *ac)
	}

	// Accumulate results
	var connections []devlake.BlueprintConnection
	var allRepos []string
	var pluginNames []string
	for _, a := range added {
		connections = append(connections, a.bpConn)
		allRepos = append(allRepos, a.repos...)
		pluginNames = append(pluginNames, pluginDisplayName(a.plugin))
	}

	return finalizeProject(finalizeProjectOpts{
		Client:      client,
		StatePath:   statePath,
		State:       state,
		ProjectName: opts.ProjectName,
		Connections: connections,
		Repos:       allRepos,
		PluginNames: pluginNames,
		Cron:        opts.Cron,
		TimeAfter:   opts.TimeAfter,
		SkipSync:    opts.SkipSync,
		Wait:        opts.Wait,
		Timeout:     opts.Timeout,
	})
}
