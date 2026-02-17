package cmd

import (
	"fmt"
	"strings"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check DevLake health and connection status",
	Long: `Checks the DevLake backend health endpoint and lists all configured
connections, showing whether each is reachable.`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Discover
	disc, err := devlake.Discover(cfgURL)
	if err != nil {
		return err
	}

	client := devlake.NewClient(disc.URL)

	// Health check
	health, err := client.Health()
	if err != nil {
		fmt.Printf("❌ DevLake at %s is unreachable: %v\n", disc.URL, err)
		return nil
	}
	fmt.Printf("✅ DevLake is healthy (%s) — %s (via %s)\n", health.Status, disc.URL, disc.Source)

	if disc.GrafanaURL != "" {
		fmt.Printf("   Grafana: %s\n", disc.GrafanaURL)
	}

	// List connections
	fmt.Println("\n" + strings.Repeat("─", 50))
	plugins := []string{"github", "gh-copilot"}
	for _, p := range plugins {
		conns, err := client.ListConnections(p)
		if err != nil {
			fmt.Printf("   %s: ⚠️  %v\n", p, err)
			continue
		}
		if len(conns) == 0 {
			fmt.Printf("   %s: (no connections)\n", p)
			continue
		}
		for _, c := range conns {
			fmt.Printf("   %s: ID=%d %q\n", p, c.ID, c.Name)
		}
	}
	fmt.Println(strings.Repeat("─", 50))

	return nil
}
