package sdk

import (
	"maps"
	"strings"
	"testing"
)

// containerScraperBundle adds a container kind (with a text field and a
// Media-targeted relation) to scraperBundle.
func containerScraperBundle() Bundle {
	bundle := scraperBundle()
	bundle.EntityKinds = append(bundle.EntityKinds, EntityKind{
		Name:      "series",
		Archetype: "container",
		Fields:    []FieldSpec{{Name: "overview", Type: "text"}},
		Relations: []RelationSpec{{Name: "holds", ToKind: MediaKindName, Cardinality: "one"}},
	})
	return bundle
}

// TestValidateBundleItemLookup: a per-item lookup with item maps validates; item
// maps without a template, a template without {id}, a non-http-json scraper,
// unknown fields or slots and blank paths are rejected.
func TestValidateBundleItemLookup(t *testing.T) {
	valid := scraperBundle()
	valid.Scrapers[0].ItemLookupTemplate = "/lookup/{id}/part/{sequence}/item/{index}"
	valid.Scrapers[0].ItemFieldMap = map[string]string{"summary": "overview", "year": "air_date"}
	valid.Scrapers[0].ItemArtworkMap = map[string]string{"thumb": "still_path"}
	valid.Scrapers[0].Transforms = map[string]string{"year": "year-from-date"}
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected a valid item lookup: %v", err)
	}

	cases := map[string]func(*ScraperSpec){
		"item maps without template": func(s *ScraperSpec) { s.ItemLookupTemplate = "" },
		"template without id":        func(s *ScraperSpec) { s.ItemLookupTemplate = "/item/{sequence}/{index}" },
		"template without maps": func(s *ScraperSpec) {
			s.ItemFieldMap, s.ItemArtworkMap, s.Transforms = nil, nil, nil
		},
		"unknown field":     func(s *ScraperSpec) { s.ItemFieldMap["media.nope"] = "x" },
		"blank field path":  func(s *ScraperSpec) { s.ItemFieldMap["summary"] = " " },
		"unknown slot":      func(s *ScraperSpec) { s.ItemArtworkMap = map[string]string{"nope": "x"} },
		"blank slot path":   func(s *ScraperSpec) { s.ItemArtworkMap = map[string]string{"thumb": ""} },
		"graphql transport": func(s *ScraperSpec) { s.Kind = "http-graphql" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			bundle := valid
			scraper := valid.Scrapers[0]
			scraper.ItemFieldMap = maps.Clone(scraper.ItemFieldMap)
			scraper.ItemArtworkMap = maps.Clone(scraper.ItemArtworkMap)
			mutate(&scraper)
			bundle.Scrapers = []ScraperSpec{scraper}
			err := ValidateBundle(bundle)
			if err == nil || !strings.Contains(err.Error(), "item") {
				t.Fatalf("ValidateBundle = %v, want an item lookup error", err)
			}
		})
	}
}

// TestValidateBundleContainerEntityMap: a container map with an empty path, no
// relation and an artwork path or field map validates; a non-container kind, a
// relation, a name path, a reference lookup, a parent, an ordinal field or an
// empty hydration are rejected.
func TestValidateBundleContainerEntityMap(t *testing.T) {
	base := EntityMap{Kind: "series", Container: true, ArtworkPath: "poster_path", FieldMap: map[string]string{"overview": "overview"}}
	valid := containerScraperBundle()
	valid.Scrapers[0].EntityMaps = []EntityMap{base, {Kind: "series", Container: true, Path: "parts[number={sequence}]", ArtworkPath: "poster_path"}}
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected a valid container map: %v", err)
	}

	cases := map[string]func(*EntityMap){
		"category kind":    func(m *EntityMap) { m.Kind, m.FieldMap = "genre", nil },
		"relation":         func(m *EntityMap) { m.Relation = "holds" },
		"name path":        func(m *EntityMap) { m.NamePath = "name" },
		"reference lookup": func(m *EntityMap) { m.LookupTemplate, m.LookupIDPath, m.MaxLookups = "/x/{id}", "id", 1 },
		"parent":           func(m *EntityMap) { m.ParentNamePath = "parent" },
		"ordinal":          func(m *EntityMap) { m.OrdinalField = "overview" },
		"nothing hydrated": func(m *EntityMap) { m.ArtworkPath, m.FieldMap = "", nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			bundle := containerScraperBundle()
			entityMap := base
			mutate(&entityMap)
			bundle.Scrapers[0].EntityMaps = []EntityMap{entityMap}
			err := ValidateBundle(bundle)
			if err == nil || !strings.Contains(err.Error(), "container entity map") {
				t.Fatalf("ValidateBundle = %v, want a container entity map error", err)
			}
		})
	}

	undeclaredField := containerScraperBundle()
	undeclaredField.Scrapers[0].EntityMaps = []EntityMap{{Kind: "series", Container: true, FieldMap: map[string]string{"nope": "x"}}}
	if err := ValidateBundle(undeclaredField); err == nil {
		t.Fatal("ValidateBundle accepted a container map field the kind does not declare")
	}
}
