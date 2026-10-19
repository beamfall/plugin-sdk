package sdktest

import (
	"math"
	"reflect"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
)

// conformantInternalBundle is the smallest Bundle that satisfies every
// ValidateBundle check, including the exactly-one-browse-Layout
// guarantee — the fixture every negative case mutates one dimension away from.
func conformantInternalBundle() sdk.Bundle {
	return sdk.Bundle{
		ID:           "independent",
		Version:      "0.1.0",
		SDKFloor:     sdk.Version,
		SDKCeiling:   sdk.Version,
		DisplayName:  sdk.LocaleString{"en": "Smoke"},
		MediaLabel:   sdk.LocaleString{"en": "Item"},
		MediaClasses: []string{"video"},
		Layouts:      []sdk.LayoutSpec{{ID: "browse", Role: "browse"}},
	}
}

func TestBundleCompatibilityAcceptsCompatibleThirdParty(t *testing.T) {
	bundle := conformantInternalBundle()
	bundle.SDKFloor = "0.1.6"

	if err := bundleCompatibilityError(bundle); err != nil {
		t.Fatalf("compatible third-party bundle rejected: %v", err)
	}
}

func TestBundleCompatibilityRejectsIncompatibleThirdParty(t *testing.T) {
	bundle := conformantInternalBundle()
	bundle.SDKFloor = "0.3.0"
	bundle.SDKCeiling = "0.3.0"

	if err := bundleCompatibilityError(bundle); err == nil {
		t.Fatal("incompatible third-party bundle accepted")
	}
}

func TestBundleCompatibilityRejectsCanonicalDrift(t *testing.T) {
	bundle := conformantInternalBundle()
	bundle.ID = "adult"
	bundle.SDKFloor = "0.1.7"

	if err := bundleCompatibilityError(bundle); err == nil {
		t.Fatal("canonical bundle without exact matrix evidence accepted")
	}
}

// TestRunBundleConformanceCatchesNonConformantBundle is the harness's own
// negative smoke test: a Bundle violating each newly-covered contract
// dimension must fail the shared SDK validator, not just compile.
func TestRunBundleConformanceCatchesNonConformantBundle(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*sdk.Bundle)
	}{
		{"bad archetype", func(b *sdk.Bundle) {
			b.EntityKinds = []sdk.EntityKind{{Name: "k", Archetype: "not-a-real-archetype"}}
		}},
		{"bad artwork slot", func(b *sdk.Bundle) {
			b.EntityKinds = []sdk.EntityKind{{Name: "k", Archetype: "person", ArtworkSlots: []string{"nope"}}}
		}},
		{"bad field type", func(b *sdk.Bundle) {
			b.MediaFields = []sdk.FieldSpec{{Name: "f", Type: "not-a-type"}}
		}},
		{"duplicate media field name", func(b *sdk.Bundle) {
			b.MediaFields = []sdk.FieldSpec{{Name: "f", Type: "text"}, {Name: "f", Type: "flag"}}
		}},
		{"media class label bad slug key", func(b *sdk.Bundle) {
			b.MediaFields = []sdk.FieldSpec{{Name: "class", Type: "text"}}
			b.MediaClassLabels = map[string]sdk.LocaleString{"Not A Slug": {"en": "X"}}
		}},
		{"media class label missing english fallback", func(b *sdk.Bundle) {
			b.MediaFields = []sdk.FieldSpec{{Name: "class", Type: "text"}}
			b.MediaClassLabels = map[string]sdk.LocaleString{"x": {"fr": "X"}}
		}},
		{"missing locale en anchor", func(b *sdk.Bundle) {
			b.EntityKinds = []sdk.EntityKind{{Name: "k", Archetype: "person", DisplayName: sdk.LocaleString{"fr": "x"}}}
		}},
		{"no browse layout", func(b *sdk.Bundle) {
			b.Layouts = nil
		}},
		{"two browse layouts", func(b *sdk.Bundle) {
			b.Layouts = []sdk.LayoutSpec{{ID: "a", Role: "browse"}, {ID: "b", Role: "browse"}}
		}},
		{"unknown layout archetype", func(b *sdk.Bundle) {
			b.Layouts = append(b.Layouts, sdk.LayoutSpec{ID: "movie", Role: "detail", Archetype: "not-a-real-archetype"})
		}},
		{"duplicate layout archetype", func(b *sdk.Bundle) {
			b.Layouts = append(b.Layouts,
				sdk.LayoutSpec{ID: "movie", Role: "detail", Archetype: "item"},
				sdk.LayoutSpec{ID: "short", Role: "detail", Archetype: "item"},
			)
		}},
		{"layout archetype on browse layout", func(b *sdk.Bundle) {
			b.Layouts[0].Archetype = "item"
		}},
		{"unknown title-join rule", func(b *sdk.Bundle) {
			b.EntityKinds = []sdk.EntityKind{{
				Name: "season", Archetype: "container", TitleJoinRule: "not-a-real-rule",
			}}
		}},
		{"unknown rating-scale archetype", func(b *sdk.Bundle) {
			b.EntityKinds = []sdk.EntityKind{{
				Name: "season", Archetype: "container", RatingScaleArchetype: "not-a-real-scale",
			}}
		}},
		{"bad section kind", func(b *sdk.Bundle) {
			b.Layouts[0].Sections = []sdk.SectionSpec{{ID: "s", Kind: "not-a-kind"}}
		}},
		{"bad playback affordance", func(b *sdk.Bundle) {
			b.Layouts[0].Sections = []sdk.SectionSpec{{
				ID:                  "s",
				Kind:                "playback-affordances",
				Source:              sdk.SourceSpec{Kind: "all-items"},
				PlaybackAffordances: []string{"cast"},
			}}
		}},
		{"filter on non-filterable source", func(b *sdk.Bundle) {
			b.Layouts[0].Sections = []sdk.SectionSpec{{
				ID:     "s",
				Kind:   "shelf",
				Source: sdk.SourceSpec{Kind: "next-up", FieldFilters: map[string]any{"x": "y"}},
			}}
		}},
		{"filter names undeclared media field", func(b *sdk.Bundle) {
			b.Layouts[0].Sections = []sdk.SectionSpec{{
				ID:     "s",
				Kind:   "shelf",
				Source: sdk.SourceSpec{Kind: "all-items", FieldFilters: map[string]any{"not-a-field": "x"}},
			}}
		}},
		{"filter value wrong type for declared field", func(b *sdk.Bundle) {
			b.MediaFields = []sdk.FieldSpec{{Name: "is-hd", Type: "flag"}}
			b.Layouts[0].Sections = []sdk.SectionSpec{{
				ID:     "s",
				Kind:   "shelf",
				Source: sdk.SourceSpec{Kind: "all-items", FieldFilters: map[string]any{"is-hd": "not-a-bool"}},
			}}
		}},
		{"valued source with bad value", func(b *sdk.Bundle) {
			b.Layouts[0].Sections = []sdk.SectionSpec{{
				ID:     "s",
				Kind:   "shelf",
				Source: sdk.SourceSpec{Kind: "availability", Value: "not-a-band"},
			}}
		}},
		{"scraper non-https endpoint", func(b *sdk.Bundle) {
			b.Scrapers = []sdk.ScraperSpec{{ID: "s", Kind: "http-json", Endpoint: "http://example.com", LookupTemplate: "/x"}}
		}},
		{"scraper bad auth style", func(b *sdk.Bundle) {
			b.Scrapers = []sdk.ScraperSpec{{ID: "s", Kind: "http-json", Endpoint: "https://example.com", LookupTemplate: "/x", AuthRef: "k", AuthStyle: "header:"}}
		}},
		{"scraper bad field map key", func(b *sdk.Bundle) {
			b.Scrapers = []sdk.ScraperSpec{{ID: "s", Kind: "http-json", Endpoint: "https://example.com", LookupTemplate: "/x", FieldMap: map[string]string{"subtitle": "name"}}}
		}},
		{"scraper login-bearer without Login", func(b *sdk.Bundle) {
			b.Scrapers = []sdk.ScraperSpec{{ID: "s", Kind: "http-json", Endpoint: "https://example.com", LookupTemplate: "/x", AuthRef: "k", AuthStyle: "login-bearer"}}
		}},
		{"scraper signature style without Signature", func(b *sdk.Bundle) {
			b.Scrapers = []sdk.ScraperSpec{{ID: "s", Kind: "http-json", Endpoint: "https://example.com", LookupTemplate: "/x", AuthRef: "k", AuthStyle: "signature:podcast-index-sha1"}}
		}},
		{"scraper unknown signature archetype", func(b *sdk.Bundle) {
			b.Scrapers = []sdk.ScraperSpec{{
				ID: "s", Kind: "http-json", Endpoint: "https://example.com", LookupTemplate: "/x",
				AuthRef: "k", AuthStyle: "signature:not-a-real-archetype",
				Signature: &sdk.SignatureSpec{Archetype: "not-a-real-archetype", HeaderName: "Authorization"},
			}}
		}},
		{"scraper signature archetype mismatched with auth style name", func(b *sdk.Bundle) {
			b.Scrapers = []sdk.ScraperSpec{{
				ID: "s", Kind: "http-json", Endpoint: "https://example.com", LookupTemplate: "/x",
				AuthRef: "k", AuthStyle: "signature:other-name",
				Signature: &sdk.SignatureSpec{Archetype: "podcast-index-sha1", HeaderName: "Authorization"},
			}}
		}},
		{"scraper signature declaration missing header name", func(b *sdk.Bundle) {
			b.Scrapers = []sdk.ScraperSpec{{
				ID: "s", Kind: "http-json", Endpoint: "https://example.com", LookupTemplate: "/x",
				AuthRef: "k", AuthStyle: "signature:podcast-index-sha1",
				Signature: &sdk.SignatureSpec{Archetype: "podcast-index-sha1"},
			}}
		}},
		{"scraper archive backfill on non-rss kind", func(b *sdk.Bundle) {
			b.Scrapers = []sdk.ScraperSpec{{
				ID: "s", Kind: "http-json", Endpoint: "https://example.com", LookupTemplate: "/x",
				ArchiveBackfill: true, ArchiveBackfillGUIDPath: "guid",
			}}
		}},
		{"scraper archive backfill without guid source", func(b *sdk.Bundle) {
			b.Scrapers = []sdk.ScraperSpec{{
				ID: "s", Kind: "rss", Endpoint: "https://example.com", LookupTemplate: "{id}",
				ArchiveBackfill: true,
			}}
		}},
		{"scraper no templates", func(b *sdk.Bundle) {
			b.Scrapers = []sdk.ScraperSpec{{ID: "s", Kind: "http-json", Endpoint: "https://example.com"}}
		}},
		{"scraper entity lookup without max lookups", func(b *sdk.Bundle) {
			b.Scrapers = []sdk.ScraperSpec{{
				ID:             "s",
				Kind:           "http-json",
				Endpoint:       "https://example.com",
				LookupTemplate: "/x",
				EntityMaps: []sdk.EntityMap{{
					Kind:           "person",
					Relation:       "credited-on",
					Path:           "authors",
					LookupIDPath:   "author.key",
					LookupTemplate: "{id}.json",
				}},
			}}
		}},
		{"bad identification policy", func(b *sdk.Bundle) {
			b.IdentificationPolicy = "manual"
		}},
		{"importer no categories", func(b *sdk.Bundle) {
			b.Importers = []sdk.ImporterSpec{{ID: "i", DisplayName: sdk.L("I"), Input: "export-file", Conflict: "additive"}}
		}},
		{"importer credentials without ref", func(b *sdk.Bundle) {
			b.Importers = []sdk.ImporterSpec{{ID: "i", DisplayName: sdk.L("I"), Categories: []string{"ratings"}, Input: "credentials", Conflict: "additive"}}
		}},
		{"permission bad kind", func(b *sdk.Bundle) {
			b.Permissions = []sdk.PermissionSpec{{Kind: "not-a-permission", Label: sdk.L("L")}}
		}},
		{"permission replaces file mutation warning", func(b *sdk.Bundle) {
			b.Permissions = []sdk.PermissionSpec{{
				Kind: "request-file-mutation", Label: sdk.L("Routine maintenance"),
				Description: sdk.L("Makes harmless housekeeping changes."),
			}}
		}},
		{"permission replaces secret warning", func(b *sdk.Bundle) {
			b.Permissions = []sdk.PermissionSpec{{
				Kind: "plugin-secret", Label: sdk.L("Plugin preferences"),
				Description: sdk.L("Reads ordinary plugin settings."),
			}}
		}},
		{"action bad risk", func(b *sdk.Bundle) {
			b.Actions = []sdk.ActionSpec{{ID: "a", Label: sdk.L("A"), Risk: "extreme"}}
		}},
		{"engagement empty event kind", func(b *sdk.Bundle) {
			b.Engagement = sdk.Engagement{EventKinds: []string{""}}
		}},
		{"label map empty label", func(b *sdk.Bundle) {
			b.LabelMaps = []sdk.LabelMapSpec{{Label: " \t ", Tag: "mood"}}
		}},
		{"label map bad confidence", func(b *sdk.Bundle) {
			b.LabelMaps = []sdk.LabelMapSpec{{Label: "mood", Tag: "mood", MinConfidence: math.NaN()}}
		}},
		{"label map no target", func(b *sdk.Bundle) {
			b.LabelMaps = []sdk.LabelMapSpec{{Label: "mood"}}
		}},
		{"label map tag and entity targets", func(b *sdk.Bundle) {
			b.EntityKinds = []sdk.EntityKind{{Name: "person", Archetype: "person", Relations: []sdk.RelationSpec{{Name: "appears-in", ToKind: sdk.MediaKindName}}}}
			b.LabelMaps = []sdk.LabelMapSpec{{Label: "mood", Tag: "mood", Kind: "person", Relation: "appears-in"}}
		}},
		{"label map blank tag", func(b *sdk.Bundle) {
			b.LabelMaps = []sdk.LabelMapSpec{{Label: "mood", Tag: " "}}
		}},
		{"label map undeclared kind", func(b *sdk.Bundle) {
			b.LabelMaps = []sdk.LabelMapSpec{{Label: "person", Kind: "person", Relation: "appears-in"}}
		}},
		{"label map non-media relation", func(b *sdk.Bundle) {
			b.EntityKinds = []sdk.EntityKind{{Name: "person", Archetype: "person", Relations: []sdk.RelationSpec{{Name: "parent", ToKind: "person"}}}}
			b.LabelMaps = []sdk.LabelMapSpec{{Label: "person", Kind: "person", Relation: "parent"}}
		}},
		{"label map duplicate normalized label and target", func(b *sdk.Bundle) {
			b.LabelMaps = []sdk.LabelMapSpec{
				{Label: "Action  Movie", Tag: "genre"},
				{Label: " action movie ", Tag: "genre"},
			}
		}},
		{"label map duplicate after NFC", func(b *sdk.Bundle) {
			b.LabelMaps = []sdk.LabelMapSpec{
				{Label: "Cafe\u0301", Tag: "genre"},
				{Label: "Café", Tag: "genre"},
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// A fresh conformantInternalBundle() per case, not a shared base
			// shallow-copied per iteration: Bundle's slice-typed fields
			// (Layouts, Scrapers, ...) share their backing array across a
			// shallow copy, so a case that mutates a nested element in place
			// (e.g. Layouts[0].Sections = ...) would otherwise corrupt every
			// later case's fixture too, masking which check actually fired.
			bundle := conformantInternalBundle()
			c.mutate(&bundle)
			if err := sdk.ValidateBundle(bundle); err == nil {
				t.Errorf("ValidateBundle accepted non-conformant bundle case %q", c.name)
			}
		})
	}
}

func TestPermissionPolicyNegativeCorpus(t *testing.T) {
	if err := permissionPolicyNegativeCorpusError(conformantInternalBundle()); err != nil {
		t.Fatal(err)
	}
}

// TestRunBundleConformanceAcceptsPodcastIndexSignatureAndArchiveBackfill
// exercises the accept path for the two RSS/ingest seams: a well-formed "signature:<name>" scraper (the
// Podcast Index per-request-signature triple) and a well-formed rss archive
// backfill declaration keyed on a guid source.
func TestRunBundleConformanceAcceptsPodcastIndexSignatureAndArchiveBackfill(t *testing.T) {
	bundle := conformantInternalBundle()
	bundle.Scrapers = []sdk.ScraperSpec{
		{
			ID: "podcast-index", Kind: "http-json", Endpoint: "https://api.podcastindex.org",
			LookupTemplate: "/api/1.0/podcasts/byfeedid?id={id}",
			AuthRef:        "podcast-index-key", AuthStyle: "signature:podcast-index-sha1",
			Signature: &sdk.SignatureSpec{Archetype: "podcast-index-sha1", HeaderName: "Authorization"},
		},
		{
			ID: "feed", Kind: "rss", Endpoint: "https://example.com", LookupTemplate: "{id}",
			ArchiveBackfill: true, ArchiveBackfillGUIDPath: "guid",
		},
	}

	RequireBundleValid(t, bundle)
}

func TestRunBundleConformanceAcceptsAdditiveLabelMaps(t *testing.T) {
	bundle := conformantInternalBundle()
	bundle.EntityKinds = []sdk.EntityKind{{
		Name:      "person",
		Archetype: "person",
		Relations: []sdk.RelationSpec{{
			Name:   "appears-in",
			ToKind: sdk.MediaKindName,
		}},
	}}
	bundle.LabelMaps = []sdk.LabelMapSpec{
		{Label: "  PERSON ", Tag: "person", MinConfidence: 0.25},
		{Label: "person", Kind: "person", Relation: "appears-in", EntityName: "Known Person", MinConfidence: 1},
	}

	RequireBundleValid(t, bundle)
}

func TestLabelMapSpecHasNoPatternFields(t *testing.T) {
	typ := reflect.TypeOf(sdk.LabelMapSpec{})
	for _, name := range []string{"Pattern", "Regex", "Expression", "MatchExpression"} {
		if _, ok := typ.FieldByName(name); ok {
			t.Fatalf("LabelMapSpec exposes %s; label maps must be equality-only by construction", name)
		}
	}
}

// TestBundleConformanceCatchesEntityStorageResiduals exercises the reject paths
// the entity-storage residual seams add to the shared SDK validator: the
// ordering-source field must name a declared number field on an ordered
// container, the orphan policy must be recognized vocabulary, typed relation
// attributes must carry a known type and not collide with a bare string
// attribute, and the relation cardinality must be recognized. Each mutant of
// the conformant fixture must red the validator.
func TestBundleConformanceCatchesEntityStorageResiduals(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*sdk.Bundle)
	}{
		{"ordering source not a declared field", func(b *sdk.Bundle) {
			b.EntityKinds = []sdk.EntityKind{{
				Name: "season", Archetype: "container", OrderedChildren: true,
				OrderingSourceField: "nope",
			}}
		}},
		{"ordering source non-number field", func(b *sdk.Bundle) {
			b.EntityKinds = []sdk.EntityKind{{
				Name: "season", Archetype: "container", OrderedChildren: true,
				Fields:              []sdk.FieldSpec{{Name: "special", Type: "flag"}},
				OrderingSourceField: "special",
			}}
		}},
		{"ordering source on non-ordered container", func(b *sdk.Bundle) {
			b.EntityKinds = []sdk.EntityKind{{
				Name: "season", Archetype: "container",
				Fields:              []sdk.FieldSpec{{Name: "episode_no", Type: "number"}},
				OrderingSourceField: "episode_no",
			}}
		}},
		{"unknown orphan policy", func(b *sdk.Bundle) {
			b.EntityKinds = []sdk.EntityKind{{Name: "k", Archetype: "person", OrphanPolicy: "delete"}}
		}},
		{"typed attribute unknown type", func(b *sdk.Bundle) {
			b.EntityKinds = []sdk.EntityKind{{
				Name: "person", Archetype: "person",
				Relations: []sdk.RelationSpec{{
					Name: "credit", ToKind: sdk.MediaKindName,
					TypedAttributes: []sdk.RelationAttributeSpec{{Name: "billing", Type: "bogus"}},
				}},
			}}
		}},
		{"typed attribute collides with string attribute", func(b *sdk.Bundle) {
			b.EntityKinds = []sdk.EntityKind{{
				Name: "person", Archetype: "person",
				Relations: []sdk.RelationSpec{{
					Name: "credit", ToKind: sdk.MediaKindName,
					Attributes:      []string{"billing"},
					TypedAttributes: []sdk.RelationAttributeSpec{{Name: "billing", Type: "number"}},
				}},
			}}
		}},
		{"unknown relation cardinality", func(b *sdk.Bundle) {
			b.EntityKinds = []sdk.EntityKind{{
				Name: "person", Archetype: "person",
				Relations: []sdk.RelationSpec{{
					Name: "credit", ToKind: sdk.MediaKindName, Cardinality: "some",
				}},
			}}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bundle := conformantInternalBundle()
			c.mutate(&bundle)
			if err := sdk.ValidateBundle(bundle); err == nil {
				t.Errorf("ValidateBundle accepted non-conformant bundle case %q", c.name)
			}
		})
	}
}

// TestParseVectorFloorRejectsUnderCoveredTables is the harness's own
// self-test for the 2026-07-05 test-quality review floor: a vector table
// missing a WantOK:false case, or missing a WantOK:true case with 2+
// non-zero ParsedMedia fields, must fail requireParseVectorFloor, while a
// table clearing both bars passes.
func TestParseVectorFloorRejectsUnderCoveredTables(t *testing.T) {
	singleField := ParseVector{Name: "single field", Path: "a", Want: sdk.ParsedMedia{Title: "a"}, WantOK: true}
	multiField := ParseVector{Name: "multi field", Path: "b", Want: sdk.ParsedMedia{Title: "b", Year: 2000}, WantOK: true}
	failure := ParseVector{Name: "failure", Path: "", Want: sdk.ParsedMedia{}, WantOK: false}

	cases := []struct {
		name      string
		vectors   []ParseVector
		wantFails bool
	}{
		{"happy-path only, single field: fails floor", []ParseVector{singleField}, true},
		{"failure vector but no multi-field vector: fails floor", []ParseVector{singleField, failure}, true},
		{"multi-field vector but no failure vector: fails floor", []ParseVector{multiField}, true},
		{"both dimensions covered: passes floor", []ParseVector{multiField, failure}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			violations := parseVectorFloorViolations(c.vectors)
			if gotFails := len(violations) > 0; gotFails != c.wantFails {
				t.Errorf("parseVectorFloorViolations(%v) = %v, want failing=%v", c.vectors, violations, c.wantFails)
			}
		})
	}
}

func TestContextParseVectorFloorRequiresEveryBoundedShape(t *testing.T) {
	base := []ContextParseVector{
		{Name: "unknown multi-field", Path: "a", Want: sdk.ParsedMedia{Title: "a", Year: 2000}, WantOK: true},
		{Name: "single", Path: "b", Context: sdk.ParseContext{DirectoryMediaFileCount: sdk.DirectoryMediaFileCountSingle}, Want: sdk.ParsedMedia{Title: "b"}, WantOK: true},
		{Name: "multiple", Path: "c", Context: sdk.ParseContext{DirectoryMediaFileCount: sdk.DirectoryMediaFileCountMultiple}, Want: sdk.ParsedMedia{Title: "c"}, WantOK: true},
		{Name: "failure", Path: "", Want: sdk.ParsedMedia{}, WantOK: false},
	}
	if violations := contextParseVectorFloorViolations(base); len(violations) != 0 {
		t.Fatalf("complete context vector table violations = %v", violations)
	}
	for i, name := range []string{"unknown", "single", "multiple"} {
		without := append([]ContextParseVector(nil), base[:i]...)
		without = append(without, base[i+1:]...)
		if violations := contextParseVectorFloorViolations(without); len(violations) == 0 {
			t.Errorf("table without %s context unexpectedly passed", name)
		}
	}
}

func TestValidateScoreResultRejectsOutOfRange(t *testing.T) {
	for _, got := range []float64{math.NaN(), math.Inf(-1), -0.1, 1.1, math.Inf(1)} {
		if outOfRange, _ := validateScoreResult(got, 0.5, 1e-9); !outOfRange {
			t.Errorf("validateScoreResult(%v) accepted out-of-contract score", got)
		}
	}
}

// TestBundlesConformanceRejectsCrossPluginKindCollision: two conformant plugins
// that each declare the same Entity Kind name must be rejected by the
// cross-plugin validator (a global kind namespace), while distinct kinds pass.
func TestBundlesConformanceRejectsCrossPluginKindCollision(t *testing.T) {
	a := conformantInternalBundle()
	a.ID = "plugin-a"
	a.EntityKinds = []sdk.EntityKind{{Name: "show", Archetype: "container"}}

	b := conformantInternalBundle()
	b.ID = "plugin-b"
	b.EntityKinds = []sdk.EntityKind{{Name: "album", Archetype: "container"}}
	RequireBundlesValid(t, a, b)

	collide := b
	collide.EntityKinds = []sdk.EntityKind{{Name: "show", Archetype: "container"}}
	RequireBundlesReject(t, a, collide)
}
