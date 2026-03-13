package query

import "fmt"

// registry holds all registered queries.
var registry = make(map[string]*QueryDef)

// Register adds a query definition to the registry.
func Register(def *QueryDef) {
	if def == nil || def.Name == "" {
		panic("cannot register nil or unnamed query")
	}
	registry[def.Name] = def
}

// Get retrieves a query definition by name.
func Get(name string) (*QueryDef, error) {
	def, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("query %q not found", name)
	}
	return def, nil
}

// List returns all registered query names.
func List() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
