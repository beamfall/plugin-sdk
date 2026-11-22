package sdk

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/beamfall/plugin-sdk/internal/jsonpath"
)

func childListingBundle(children ...ChildListing) Bundle {
	bundle := scraperBundle()
	bundle.ConfigFields = []ConfigFieldSpec{{Name: "language", Type: "text", Label: L("Language")}}
	scraper := &bundle.Scrapers[0]
	scraper.SearchTemplate = "/search?query={query}&language={config.language}"
	scraper.CandidatesPath = "results"
	scraper.CandidateIDPath = "id"
	scraper.Children = children
	return bundle
}

func TestValidateBundleChildListings(t *testing.T) {
	cases := []struct {
		name     string
		children []ChildListing
	}{
		{"no children", nil},
		{"show seasons and episodes", []ChildListing{
			{Level: "season", ListTemplate: "/tv/{id}?language={config.language}", ItemsPath: "seasons", Key: "season_number", Label: "name", Sequence: "season_number", Year: "air_date", Artwork: "poster_path"},
			{Level: "episode", ListTemplate: "/tv/{id}/season/{parent}", ItemsPath: "episodes", Key: "id", Label: "name", Index: "episode_number", Artwork: "still_path", MaxItems: 500},
		}},
		{"artist release groups, releases and tracks", []ChildListing{
			{Level: "release-group", ListTemplate: "/release-group?artist={id}", ItemsPath: "release-groups", Key: "id", Label: "title", Year: "first-release-date"},
			{Level: "release", ListTemplate: "/release?release-group={parent}", ItemsPath: "releases", Key: "id", Label: "title", Year: "date"},
			{Level: "track", ListTemplate: "/release/{parent}?inc=recordings", ItemsPath: "media.0.tracks", Key: "recording.id", Label: "recording.title", Index: "position"},
		}},
		{"ordinal selection and predicate paths", []ChildListing{
			{Level: "outer", ListTemplate: "/list", ItemsPath: "groups[kind=outer,version=1.0].items", Key: "id", Label: "name", Sequence: "sequence"},
			{Level: "inner", ListTemplate: "/list/{sequence}", ItemsPath: "items", Key: "id", Label: "name", Index: "position"},
			{Level: "leaf", ListTemplate: "/list?position={index}&language={config.language}", ItemsPath: "items", Key: "id", Label: "names[language=en].text"},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateBundle(childListingBundle(tc.children...)); err != nil {
				t.Fatalf("ValidateBundle rejected valid child listings: %v", err)
			}
		})
	}
}

func TestValidateBundleChildPathParity(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"items", true},
		{"data.0.items", true},
		{"groups[tag=edition[1]].items", true},
		{"groups[tag=edition,version=1.0].items", true},
		{"groups[tag=].items", true},
		{"data..items", false},
		{"groups[tag=edition[1].items", false},
		{"groups[tag].items", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%q", tc.path), func(t *testing.T) {
			_, parsed := jsonpath.Parse(tc.path)
			if parsed != tc.want {
				t.Fatalf("CandidatesPath parser accepted %q = %t, want %t", tc.path, parsed, tc.want)
			}
			child := ChildListing{Level: "item", ListTemplate: "/list/{id}", ItemsPath: tc.path, Key: "id", Label: "name"}
			err := ValidateBundle(childListingBundle(child))
			if (err == nil) != parsed {
				t.Errorf("ValidateBundle accepted child path %q = %t, parser accepted = %t (error: %v)", tc.path, err == nil, parsed, err)
			}
		})
	}
}

func TestValidateBundleRejectsInvalidChildListings(t *testing.T) {
	cases := []struct {
		name   string
		level  int
		mutate func(*ScraperSpec)
		want   string
	}{
		{"too many levels", 4, func(s *ScraperSpec) { s.Children = append(s.Children, s.Children...) }, "exceeds the limit"},
		{"empty level", 1, func(s *ScraperSpec) { s.Children[0].Level = "" }, "must be lower-kebab"},
		{"uppercase level", 1, func(s *ScraperSpec) { s.Children[0].Level = "Part" }, "must be lower-kebab"},
		{"underscore level", 1, func(s *ScraperSpec) { s.Children[0].Level = "some_part" }, "must be lower-kebab"},
		{"repeated hyphen", 1, func(s *ScraperSpec) { s.Children[0].Level = "some--part" }, "must be lower-kebab"},
		{"duplicate level", 2, func(s *ScraperSpec) { s.Children[1].Level = s.Children[0].Level }, "duplicate Level"},
		{"GraphQL", 1, func(s *ScraperSpec) { s.Kind = "http-graphql" }, "requires an http-json scraper"},
		{"RSS", 1, func(s *ScraperSpec) { s.Kind = "rss" }, "requires an http-json scraper"},
		{"local kind", 1, func(s *ScraperSpec) { s.Kind = "exif" }, "requires an http-json scraper"},
		{"no search template", 1, func(s *ScraperSpec) { s.SearchTemplate = "" }, "with a SearchTemplate"},
		{"empty list template", 1, func(s *ScraperSpec) { s.Children[0].ListTemplate = "" }, "endpoint-relative"},
		{"relative list path", 1, func(s *ScraperSpec) { s.Children[0].ListTemplate = "list/{id}" }, "endpoint-relative"},
		{"scheme", 1, func(s *ScraperSpec) { s.Children[0].ListTemplate = "https://other.test/list" }, "endpoint-relative"},
		{"authority", 1, func(s *ScraperSpec) { s.Children[0].ListTemplate = "//other.test/list" }, "endpoint-relative"},
		{"encoded authority", 1, func(s *ScraperSpec) { s.Children[0].ListTemplate = "/%2fother.test/list" }, "endpoint-relative"},
		{"parent path segment", 1, func(s *ScraperSpec) { s.Children[0].ListTemplate = "/lookup/../list" }, "endpoint-relative"},
		{"encoded parent path segment", 1, func(s *ScraperSpec) { s.Children[0].ListTemplate = "/lookup/.%2e/list" }, "endpoint-relative"},
		{"fragment", 1, func(s *ScraperSpec) { s.Children[0].ListTemplate = "/list#fragment" }, "endpoint-relative"},
		{"control character", 1, func(s *ScraperSpec) { s.Children[0].ListTemplate = "/list\n" }, "endpoint-relative"},
		{"first level parent", 1, func(s *ScraperSpec) { s.Children[0].ListTemplate = "/list/{parent}" }, "cannot use {parent}"},
		{"first level sequence", 1, func(s *ScraperSpec) { s.Children[0].ListTemplate = "/list/{sequence}" }, "cannot use {sequence}"},
		{"first level index", 1, func(s *ScraperSpec) { s.Children[0].ListTemplate = "/list?i={index}" }, "cannot use {index}"},
		{"next level without previous row", 2, func(s *ScraperSpec) { s.Children[1].ListTemplate = "/list/{id}" }, "from the previous level"},
		{"unknown placeholder", 1, func(s *ScraperSpec) { s.Children[0].ListTemplate = "/list?q={query}" }, "unsupported placeholder {query}"},
		{"malformed placeholder", 1, func(s *ScraperSpec) { s.Children[0].ListTemplate = "/list/{id" }, "malformed placeholder"},
		{"hint placeholder", 2, func(s *ScraperSpec) { s.Children[1].ListTemplate = "/list/{parent}?h={hint.tag}" }, "hint placeholder outside SearchTemplate"},
		{"undeclared config", 1, func(s *ScraperSpec) { s.Children[0].ListTemplate = "/list?lang={config.unknown}" }, "undeclared config field"},
		{"empty items path", 1, func(s *ScraperSpec) { s.Children[0].ItemsPath = "" }, "ItemsPath must be a non-empty JSON path"},
		{"malformed items path", 1, func(s *ScraperSpec) { s.Children[0].ItemsPath = "data..items" }, "ItemsPath must be a non-empty JSON path"},
		{"empty key path", 1, func(s *ScraperSpec) { s.Children[0].Key = "" }, "Key must be a non-empty JSON path"},
		{"malformed key path", 1, func(s *ScraperSpec) { s.Children[0].Key = "data[id" }, "Key must be a non-empty JSON path"},
		{"empty label path", 1, func(s *ScraperSpec) { s.Children[0].Label = " " }, "Label must be a non-empty JSON path"},
		{"malformed label path", 2, func(s *ScraperSpec) { s.Children[1].Label = "names[language].text" }, "Label must be a non-empty JSON path"},
		{"malformed sequence path", 1, func(s *ScraperSpec) { s.Children[0].Sequence = "data..sequence" }, "Sequence must be a non-empty JSON path"},
		{"negative cap", 1, func(s *ScraperSpec) { s.Children[0].MaxItems = -1 }, "MaxItems must be between 0 and 500"},
		{"excessive cap", 1, func(s *ScraperSpec) { s.Children[0].MaxItems = 501 }, "MaxItems must be between 0 and 500"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bundle := childListingBundle(
				ChildListing{Level: "part", ListTemplate: "/list/{id}", ItemsPath: "parts", Key: "id", Label: "name"},
				ChildListing{Level: "item", ListTemplate: "/list/{parent}", ItemsPath: "items", Key: "id", Label: "name"},
			)
			tc.mutate(&bundle.Scrapers[0])
			wantBundleError(t, bundle, fmt.Sprintf(`plugin smoke: scraper "example" child level %d`, tc.level))
			wantBundleError(t, bundle, tc.want)
		})
	}
}

func TestScraperChildListingsJSON(t *testing.T) {
	child := ChildListing{Level: "item", ListTemplate: "/list/{id}", ItemsPath: "items", Key: "id", Label: "name"}
	cases := []struct {
		name     string
		children []ChildListing
		want     string
	}{
		{"omitted", nil, ""},
		{"empty", []ChildListing{}, ""},
		{"declared", []ChildListing{child}, `[{"Level":"item","ListTemplate":"/list/{id}","ItemsPath":"items","Key":"id","Label":"name"}]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := json.Marshal(ScraperSpec{Children: tc.children})
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			if got := string(fields["Children"]); got != tc.want {
				t.Errorf("Children JSON = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidChildKeySegment(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"3", true}, {"abc-123", true}, {"a b", true},
		{"", false}, {".", false}, {"..", false}, {"a/b", false}, {`a\b`, false},
		{"a%2Fb", false}, {"a%2fb", false}, {"a?b", false}, {"a#b", false},
	}
	for _, tc := range cases {
		if got := ValidChildKeySegment(tc.key); got != tc.want {
			t.Errorf("ValidChildKeySegment(%q) = %t, want %t", tc.key, got, tc.want)
		}
	}
}
