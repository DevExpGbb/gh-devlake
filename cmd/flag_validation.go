package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// collectAllScopeFlagDefs returns all ScopeFlags from the registry, with the
// Plugins field populated to indicate which plugins define each flag.
func collectAllScopeFlagDefs() []FlagDef {
	return collectFlagDefs(func(d *ConnectionDef) []FlagDef { return d.ScopeFlags })
}

// collectAllConnectionFlagDefs returns all ConnectionFlags from the registry,
// with the Plugins field populated to indicate which plugins define each flag.
func collectAllConnectionFlagDefs() []FlagDef {
	return collectFlagDefs(func(d *ConnectionDef) []FlagDef { return d.ConnectionFlags })
}

// collectFlagDefs aggregates FlagDefs from all ConnectionDefs in the registry,
// merging entries that share the same flag name and populating the Plugins field.
func collectFlagDefs(extract func(*ConnectionDef) []FlagDef) []FlagDef {
	type entry struct {
		flagDef FlagDef
		plugins []string
	}
	byName := make(map[string]*entry)
	var order []string

	for _, d := range connectionRegistry {
		for _, fd := range extract(d) {
			if e, ok := byName[fd.Name]; ok {
				e.plugins = append(e.plugins, d.Plugin)
			} else {
				byName[fd.Name] = &entry{flagDef: fd, plugins: []string{d.Plugin}}
				order = append(order, fd.Name)
			}
		}
	}

	result := make([]FlagDef, 0, len(order))
	for _, name := range order {
		e := byName[name]
		result = append(result, FlagDef{
			Name:        e.flagDef.Name,
			Description: e.flagDef.Description,
			Plugins:     e.plugins,
		})
	}
	return result
}

// warnIrrelevantFlags prints a ⚠️ warning for each explicitly-set flag that
// is plugin-specific but does not apply to the given plugin. Warnings are
// non-fatal — the command continues after printing them.
func warnIrrelevantFlags(cmd *cobra.Command, def *ConnectionDef, allFlagDefs []FlagDef) {
	// Build map: flagName → list of plugin slugs that use the flag.
	flagPlugins := make(map[string][]string, len(allFlagDefs))
	for _, fd := range allFlagDefs {
		flagPlugins[fd.Name] = fd.Plugins
	}

	cmd.Flags().Visit(func(f *pflag.Flag) {
		plugins, isPluginSpecific := flagPlugins[f.Name]
		if !isPluginSpecific {
			return // shared flag, no warning
		}
		for _, p := range plugins {
			if p == def.Plugin {
				return // flag is relevant for this plugin
			}
		}

		// Flag is plugin-specific but not used by the selected plugin.
		var applicableTo []string
		for _, p := range plugins {
			if d := FindConnectionDef(p); d != nil {
				applicableTo = append(applicableTo, d.DisplayName)
			} else {
				applicableTo = append(applicableTo, p)
			}
		}
		msg := fmt.Sprintf("\n⚠️  --%s is not used by the %s plugin", f.Name, def.DisplayName)
		if len(applicableTo) > 0 {
			msg += fmt.Sprintf(" (applies to: %s)", strings.Join(applicableTo, ", "))
		}
		fmt.Println(msg)
	})
}

// printContextualFlagHelp prints the plugin-specific flags to the terminal
// after a plugin is selected interactively. commandType is a short label like
// "Scope" or "Connection".
func printContextualFlagHelp(def *ConnectionDef, flagDefs []FlagDef, commandType string) {
	if len(flagDefs) == 0 {
		return
	}
	// Find the longest flag name for alignment.
	maxLen := 0
	for _, fd := range flagDefs {
		if len(fd.Name) > maxLen {
			maxLen = len(fd.Name)
		}
	}
	fmt.Printf("\n📚 %s flags for %s:\n", commandType, def.DisplayName)
	for _, fd := range flagDefs {
		padding := strings.Repeat(" ", maxLen-len(fd.Name)+3)
		fmt.Printf("   --%s%s%s\n", fd.Name, padding, fd.Description)
	}
}
