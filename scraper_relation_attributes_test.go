package sdk

import (
	"strings"
	"testing"
)

// creditBundle is scraperBundle with a credit relation that declares plain and
// typed attributes, the shape a cast/crew EntityMap writes.
func creditBundle() Bundle {
	bundle := scraperBundle()
	bundle.EntityKinds[0].Relations = []RelationSpec{{
		Name: "genres", ToKind: MediaKindName,
		Attributes:      []string{"character", "job", "department", "billing"},
		TypedAttributes: []RelationAttributeSpec{{Name: "weight", Type: "number"}},
	}}
	return bundle
}

// TestValidateBundleEntityMapRelationValues is the validation vector set for
// RelationAttributeMap, ElementFilter, OrdinalOffset and an OrdinalField naming
// a relation attribute (CORE-2503): the cast and crew maps a credit list needs
// validate; undeclared or typed attributes, blank paths, empty filters,
// negative offsets and ordinals naming neither map are rejected.
func TestValidateBundleEntityMapRelationValues(t *testing.T) {
	cast := EntityMap{
		Kind: "genre", Relation: "genres", Path: "credits.cast", NamePath: "name",
		LookupTemplate: "/person/{id}", LookupIDPath: "id", MaxLookups: 10,
		RelationAttributeMap: map[string]string{"character": "character", "billing": "order"},
		OrdinalField:         "billing",
	}
	crew := EntityMap{
		Kind: "genre", Relation: "genres", Path: "credits.crew", NamePath: "name",
		RelationAttributeMap: map[string]string{"job": "job", "department": "department"},
		ElementFilter:        map[string][]string{"job": {"Director", "Writer"}},
		OrdinalOffset:        1000,
	}
	valid := creditBundle()
	valid.Scrapers[0].EntityMaps = []EntityMap{cast, crew}
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected valid cast and crew maps: %v", err)
	}

	cases := []struct {
		name     string
		mutate   func(*EntityMap)
		fragment string
	}{
		{"undeclared attribute", func(m *EntityMap) { m.RelationAttributeMap = map[string]string{"role": "role"} },
			`maps relation attribute "role" that relation "genres" does not declare`},
		{"typed attribute", func(m *EntityMap) { m.RelationAttributeMap = map[string]string{"weight": "weight"} },
			`maps relation attribute "weight" that relation "genres" does not declare`},
		{"blank attribute path", func(m *EntityMap) { m.RelationAttributeMap = map[string]string{"job": " "} },
			`relation attribute "job" has a blank path`},
		{"blank filter path", func(m *EntityMap) { m.ElementFilter = map[string][]string{" ": {"Director"}} },
			"blank element filter path"},
		{"filter without values", func(m *EntityMap) { m.ElementFilter = map[string][]string{"job": nil} },
			`element filter "job" accepts no values`},
		{"negative offset", func(m *EntityMap) { m.OrdinalOffset = -1 },
			"ordinal offset must not be negative"},
		{"ordinal in neither map", func(m *EntityMap) { m.OrdinalField = "order" },
			`declares ordinal field "order" that is not in its field map or relation attribute map`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bundle := creditBundle()
			entityMap := crew
			tc.mutate(&entityMap)
			bundle.Scrapers[0].EntityMaps = []EntityMap{cast, entityMap}
			err := ValidateBundle(bundle)
			if err == nil || !strings.Contains(err.Error(), tc.fragment) {
				t.Fatalf("ValidateBundle = %v, want error containing %q", err, tc.fragment)
			}
		})
	}
}

// TestValidateBundleContainerEntityMapRejectsRelationValues: a container map
// names no relation and creates no rows, so relation attributes, an element
// filter and an ordinal offset are container map errors.
func TestValidateBundleContainerEntityMapRejectsRelationValues(t *testing.T) {
	cases := map[string]func(*EntityMap){
		"relation attributes": func(m *EntityMap) { m.RelationAttributeMap = map[string]string{"character": "character"} },
		"element filter":      func(m *EntityMap) { m.ElementFilter = map[string][]string{"type": {"main"}} },
		"ordinal offset":      func(m *EntityMap) { m.OrdinalOffset = 5 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			bundle := containerScraperBundle()
			entityMap := EntityMap{Kind: "series", Container: true, ArtworkPath: "poster_path"}
			mutate(&entityMap)
			bundle.Scrapers[0].EntityMaps = []EntityMap{entityMap}
			err := ValidateBundle(bundle)
			if err == nil || !strings.Contains(err.Error(), "container entity map") {
				t.Fatalf("ValidateBundle = %v, want a container entity map error", err)
			}
		})
	}
}
