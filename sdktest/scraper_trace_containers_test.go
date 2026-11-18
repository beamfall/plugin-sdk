package sdktest

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
)

// containerTraceSpec is a TMDB-TV-like scraper whose show Container map
// hydrates fields (an index and a predicate path among them), poster and
// backdrop, the matched title and links to the genres and cast its sibling
// maps hydrate; the season map's ordinal path has no offline value.
func containerTraceSpec() sdk.ScraperSpec {
	return sdk.ScraperSpec{
		ID: "example-tv", Kind: "http-json", Endpoint: "https://api.example.test",
		LookupTemplate: "/tv/{id}",
		ArtworkBaseURL: "https://img.example.test",
		EntityMaps: []sdk.EntityMap{
			{Kind: "genre", Relation: "genre-of", Path: "genres", NamePath: "name"},
			{Kind: "person", Relation: "credited-in", Path: "credits.cast", NamePath: "name",
				LookupTemplate: "/person/{id}", LookupIDPath: "id", MaxLookups: 2},
			{Kind: "person", Relation: "crew-of", Path: "credits.crew", NamePath: "name",
				LookupTemplate: "/person/{id}", LookupIDPath: "id", MaxLookups: 1,
				ElementFilter: map[string][]string{"job": {"Director"}}},
			{
				Kind: "show", Container: true, ArtworkPath: "poster_path",
				ArtworkMap: map[string]string{"backdrop": "backdrop_path", "logo": "logo_path"},
				TitlePath:  "name",
				FieldMap: map[string]string{
					"network":               "networks.0.name",
					"certification-display": "content_ratings.results[iso_3166_1=US].rating",
					"community-rating":      "vote_average",
					"tagline":               "tagline",
				},
				ContainerRelations: map[string]string{"in-genre": "genre-of", "credit": "credited-in", "directed": "crew-of"},
			},
			{Kind: "season", Container: true, Path: "seasons[season_number={sequence}]", ArtworkPath: "poster_path"},
		},
	}
}

func containerTraceDoc(t *testing.T) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(`{"name":" 90 Day Fiancé ","poster_path":"/p.jpg","backdrop_path":"/b.jpg","logo_path":"","tagline":"",
		"vote_average":8.1,"networks":[{"name":"Netflix"},{"name":"Other"}],
		"content_ratings":{"results":[{"iso_3166_1":"GB","rating":"15"},{"iso_3166_1":"US","rating":"TV-MA"}]},
		"genres":[{"name":"Drama"},{"name":"Mystery"}],
		"credits":{"cast":[{"id":1,"name":"Lead"},{"name":"No ID"},{"id":2,"name":"Second"},{"id":3,"name":"Third"}],
			"crew":[{"id":5,"name":"Grip","job":"Grip"},{"id":6,"name":"Dir","job":" Director "}]},
		"seasons":[{"season_number":1,"poster_path":"/s1.jpg"}]}`), &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

// containerTraceEntityDocs are the credits' referenced /person/{id}
// documents: the name comes from the referenced document, not the element;
// person 2 has no document, so it hydrates nothing; person 3 is past
// MaxLookups, so it hydrates from the cast element. The crew filter drops the
// grip (person 5) before it uses the crew map's single lookup, so the director
// (person 6) is looked up.
func containerTraceEntityDocs() map[string]map[string]any {
	return map[string]map[string]any{
		"/person/1": {"name": "Lead Actor"},
		"/person/3": {"name": "Third Actor"},
		"/person/5": {"name": "Grip Person"},
		"/person/6": {"name": "Director Person"},
	}
}

func TestScraperTraceContainers(t *testing.T) {
	want := []TraceContainer{{
		Kind: "show",
		Fields: map[string]string{
			"network":               "Netflix",
			"certification-display": "TV-MA",
			"community-rating":      "8.1",
		},
		Artwork: map[string]string{"poster": "https://img.example.test/p.jpg", "backdrop": "https://img.example.test/b.jpg"},
		Title:   "90 Day Fiancé",
		Links: []TraceLink{
			{Relation: "credit", Kind: "person", Name: "Lead Actor"},
			{Relation: "credit", Kind: "person", Name: "Third"},
			{Relation: "directed", Kind: "person", Name: "Director Person"},
			{Relation: "in-genre", Kind: "genre", Name: "Drama"},
			{Relation: "in-genre", Kind: "genre", Name: "Mystery"},
		},
	}}
	RunScraperTraceConformance(t, []ScraperTraceVector{{
		Name:  "show container",
		Spec:  containerTraceSpec(),
		Input: TraceInput{ID: "42", LookupDoc: containerTraceDoc(t), EntityDocs: containerTraceEntityDocs()},
		Want: ScraperTrace{
			Requests:   []TraceRequest{{Role: "lookup", Method: "GET", URL: "https://api.example.test/tv/42"}},
			Containers: want,
		},
	}})
}

func TestScraperTraceContainerDiagnostics(t *testing.T) {
	got := CompileScraperTrace(containerTraceSpec(), TraceInput{ID: "42", LookupDoc: containerTraceDoc(t)})
	wrong := got
	wrong.Containers = []TraceContainer{{Kind: "show", Title: "90 Day Fiance"}}
	reasons := strings.Join(scraperTraceViolations(wrong, got), "\n")
	for _, fragment := range []string{"container[0] title", `container[0] field "network": unexpected`, "container[0] artwork", "container[0] links"} {
		if !strings.Contains(reasons, fragment) {
			t.Errorf("violations missing %q:\n%s", fragment, reasons)
		}
	}
	// Links compare structurally: one oddly named link never equals two.
	merged := got
	merged.Containers = []TraceContainer{got.Containers[0]}
	merged.Containers[0].Links = []TraceLink{{Relation: "in-genre", Kind: "genre", Name: "Drama} {in-genre genre Mystery"}}
	if reasons := scraperTraceViolations(merged, got); len(reasons) == 0 {
		t.Error("a merged link name passed as two links")
	}
	// A vector that sets no Containers does not grade them.
	unset := got
	unset.Containers = nil
	if reasons := scraperTraceViolations(unset, got); len(reasons) != 0 {
		t.Errorf("containers graded without a want: %v", reasons)
	}
}

func TestResolvePathGrammar(t *testing.T) {
	doc := containerTraceDoc(t)
	for path, want := range map[string]string{
		"networks.1.name": "Other",
		"content_ratings.results[iso_3166_1=GB].rating": "15",
		"credits.cast[id=2,name=Second].name":           "Second",
	} {
		if got, ok := resolvePath(doc, path); !ok || got != want {
			t.Errorf("resolvePath(%q) = %q, %v; want %q", path, got, ok, want)
		}
	}
	for _, path := range []string{"networks.9.name", "networks.x.name", "content_ratings.results[iso_3166_1=FR].rating", "genres"} {
		if got, ok := resolvePath(doc, path); ok {
			t.Errorf("resolvePath(%q) = %q, want unresolved", path, fmt.Sprint(got))
		}
	}
}
