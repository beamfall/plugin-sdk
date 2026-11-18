package sdk

import (
	"maps"
	"strings"
	"testing"
)

// containerExtrasBundle is containerScraperBundle with a series → genre
// relation and a genre map the series map can link through it.
func containerExtrasBundle() Bundle {
	bundle := containerScraperBundle()
	series := &bundle.EntityKinds[len(bundle.EntityKinds)-1]
	series.Relations = append(series.Relations, RelationSpec{Name: "in-genre", ToKind: "genre"})
	return bundle
}

// TestValidateBundleContainerMapExtras: a container map with an artwork map,
// a title path and a sibling relation validates, as does one that only sets
// the title; unknown slots, blank paths, a duplicated poster, undeclared or
// Media-targeted container relations, a relation no sibling map declares, one
// a map of another kind shares and
// the fields on a non-container map are rejected.
func TestValidateBundleContainerMapExtras(t *testing.T) {
	genreMap := EntityMap{Kind: "genre", Relation: "genres", Path: "genres", NamePath: "name"}
	base := EntityMap{
		Kind: "series", Container: true, ArtworkPath: "poster_path",
		ArtworkMap:         map[string]string{"backdrop": "backdrop_path"},
		TitlePath:          "name",
		ContainerRelations: map[string]string{"in-genre": "genres"},
	}
	valid := containerExtrasBundle()
	valid.Scrapers[0].EntityMaps = []EntityMap{genreMap, base}
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected a valid container map: %v", err)
	}
	titleOnly := containerExtrasBundle()
	titleOnly.Scrapers[0].EntityMaps = []EntityMap{{Kind: "series", Container: true, TitlePath: "name"}}
	if err := ValidateBundle(titleOnly); err != nil {
		t.Fatalf("ValidateBundle rejected a title-only container map: %v", err)
	}

	cases := map[string]struct {
		mutate func(*EntityMap)
		want   string
	}{
		"unknown slot":        {func(m *EntityMap) { m.ArtworkMap["nope"] = "x" }, "unknown artwork slot"},
		"blank slot path":     {func(m *EntityMap) { m.ArtworkMap["backdrop"] = " " }, "blank path"},
		"duplicated poster":   {func(m *EntityMap) { m.ArtworkMap["poster"] = "poster_path" }, "both artwork path and artwork map"},
		"blank title path":    {func(m *EntityMap) { m.TitlePath = " " }, "blank title path"},
		"undeclared relation": {func(m *EntityMap) { m.ContainerRelations["nope"] = "genres" }, "does not declare"},
		"media relation":      {func(m *EntityMap) { m.ContainerRelations["holds"] = "genres" }, "does not declare"},
		"no sibling map":      {func(m *EntityMap) { m.ContainerRelations["in-genre"] = "other" }, "no \"genre\" entity map"},
		"not a container": {func(m *EntityMap) {
			*m = EntityMap{Kind: "genre", Relation: "genres", Path: "genres", TitlePath: "name"}
		}, "without being a container map"},
	}
	ambiguous := containerExtrasBundle()
	ambiguous.EntityKinds = append(ambiguous.EntityKinds, EntityKind{
		Name: "tag", Archetype: "category", Relations: []RelationSpec{{Name: "genres", ToKind: MediaKindName}},
	})
	ambiguous.Scrapers[0].EntityMaps = []EntityMap{genreMap, {Kind: "tag", Relation: "genres", Path: "tags"}, base}
	if err := ValidateBundle(ambiguous); err == nil || !strings.Contains(err.Error(), "of a kind other than") {
		t.Fatalf("ValidateBundle = %v, want the shared sibling relation rejected", err)
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			bundle := containerExtrasBundle()
			entityMap := base
			entityMap.ArtworkMap = maps.Clone(base.ArtworkMap)
			entityMap.ContainerRelations = maps.Clone(base.ContainerRelations)
			tc.mutate(&entityMap)
			bundle.Scrapers[0].EntityMaps = []EntityMap{genreMap, entityMap}
			err := ValidateBundle(bundle)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateBundle = %v, want an error containing %q", err, tc.want)
			}
		})
	}
}
