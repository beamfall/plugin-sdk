package sdk

import (
	"os"
	"strings"
	"testing"
)

// TestGoModSingleDependency pins the single-dependency invariant (README
// "Single-dependency invariant"): go.mod must carry exactly one require
// directive, and it must be golang.org/x/text. Any added third-party
// dependency propagates across the AGPL boundary to Core and every plugin, so
// the contract is enforced here in stdlib-only code.
func TestGoModSingleDependency(t *testing.T) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	requires := parseRequires(string(data))
	if len(requires) != 1 {
		t.Fatalf("go.mod declares %d require paths %v, want exactly 1 (golang.org/x/text)", len(requires), requires)
	}
	if requires[0] != "golang.org/x/text" {
		t.Fatalf("go.mod's sole dependency is %q, want golang.org/x/text", requires[0])
	}
}

// parseRequires returns the module paths of every require directive in go.mod,
// handling both the single-line `require path v...` form and the block
// `require ( ... )` form.
func parseRequires(mod string) []string {
	var paths []string
	inBlock := false
	for _, line := range strings.Split(mod, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case inBlock && line == ")":
			inBlock = false
		case inBlock:
			if path := requirePath(line); path != "" {
				paths = append(paths, path)
			}
		case line == "require (":
			inBlock = true
		case strings.HasPrefix(line, "require "):
			if path := requirePath(strings.TrimPrefix(line, "require ")); path != "" {
				paths = append(paths, path)
			}
		}
	}
	return paths
}

// requirePath extracts the module path from a require entry line (path + version,
// optionally with a "// indirect" comment), or "" for a blank/comment line.
func requirePath(entry string) string {
	entry = strings.TrimSpace(entry)
	if entry == "" || strings.HasPrefix(entry, "//") {
		return ""
	}
	return strings.Fields(entry)[0]
}
