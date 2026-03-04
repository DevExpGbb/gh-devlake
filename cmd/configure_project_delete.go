package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DevExpGBB/gh-devlake/internal/prompt"
)

func newProjectDeleteCmd() *cobra.Command {
	var projectDeleteName string
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a DevLake project",
		Long: `Deletes a DevLake project by name.

If --name is not specified, prompts interactively.

⚠️  Deleting a project also removes its blueprint and sync schedule.

Examples:
  gh devlake configure project delete
  gh devlake configure project delete --name my-project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectDelete(cmd, args, projectDeleteName)
		},
	}
	cmd.Flags().StringVar(&projectDeleteName, "name", "", "Name of the project to delete")
	return cmd
}

func runProjectDelete(cmd *cobra.Command, args []string, projectDeleteName string) error {
	printBanner("DevLake — Delete Project")

	// ── Discover DevLake ──
	client, _, err := discoverClient(cfgURL)
	if err != nil {
		return err
	}

	// ── Resolve project name ──
	name := projectDeleteName
	if name == "" {
		// Interactive: list projects and let the user pick one
		projects, err := client.ListProjects()
		if err != nil {
			return fmt.Errorf("listing projects: %w", err)
		}
		if len(projects) == 0 {
			fmt.Println("\n  No projects found.")
			fmt.Println()
			return nil
		}

		labels := make([]string, len(projects))
		for i, p := range projects {
			labels[i] = p.Name
		}

		fmt.Println()
		chosen := prompt.Select("Select a project to delete", labels)
		if chosen == "" {
			fmt.Println("\n  Deletion cancelled.")
			fmt.Println()
			return nil
		}
		name = chosen
	}

	// ── Confirm deletion ──
	fmt.Printf("\n⚠️  This will delete project %q.\n", name)
	fmt.Println("   The associated blueprint and sync schedule will also be removed.")
	fmt.Println()
	if !prompt.Confirm("Are you sure you want to delete this project?") {
		fmt.Println("\n  Deletion cancelled.")
		fmt.Println()
		return nil
	}

	// ── Delete project ──
	fmt.Printf("\n🗑️  Deleting project %q...\n", name)
	if err := client.DeleteProject(name); err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}
	fmt.Println("   ✅ Project deleted")

	fmt.Println("\n" + strings.Repeat("─", 40))
	fmt.Printf("✅ Project %q deleted\n", name)
	fmt.Println(strings.Repeat("─", 40))
	fmt.Println()

	return nil
}
