package sdk

import (
	"math"
	"strings"
	"testing"
)

// wantBundleError asserts ValidateBundle rejects b with an error whose message
// contains fragment (copied verbatim from bundle_validate.go).
func wantBundleError(t *testing.T, b Bundle, fragment string) {
	t.Helper()
	err := ValidateBundle(b)
	if err == nil {
		t.Fatalf("ValidateBundle accepted an invalid bundle, want error containing %q", fragment)
	}
	if !strings.Contains(err.Error(), fragment) {
		t.Fatalf("ValidateBundle error %q does not contain %q", err, fragment)
	}
}

// validBundle is the minimal bundle ValidateBundle accepts, the base the
// error-path tests below mutate one invariant at a time.
func validBundle() Bundle {
	return Bundle{
		ID:           "smoke",
		Version:      "0.1.0",
		SDKFloor:     Version,
		SDKCeiling:   Version,
		DisplayName:  L("Smoke"),
		MediaLabel:   L("Item"),
		MediaClasses: []string{"video"},
		Layouts:      []LayoutSpec{{ID: "browse", Role: "browse"}},
	}
}

// sourceSectionBundle wraps one SourceSpec in a single browse-layout shelf
// section, the shape the validateSource error-path table exercises.
func sourceSectionBundle(source SourceSpec) Bundle {
	b := validBundle()
	b.Layouts = []LayoutSpec{{
		ID:   "browse",
		Role: "browse",
		Sections: []SectionSpec{{
			ID:     "s",
			Kind:   "shelf",
			Source: source,
		}},
	}}
	return b
}

// sourceFixtureBundle declares the dependencies used by canonical Source
// fixtures so validation failures stay inside the Source parameter contract.
func sourceFixtureBundle(source SourceSpec) Bundle {
	b := sourceSectionBundle(source)
	b.EntityKinds = []EntityKind{{Name: sourceFixtureEntityKind, Archetype: "category"}}
	b.MediaFields = []FieldSpec{{Name: sourceFixtureMediaField, Type: "text", Label: L("Fixture Field")}}
	return b
}

func canonicalPermissionSpec(t *testing.T, kind string) PermissionSpec {
	t.Helper()
	policy, ok := PermissionPolicyFor(kind)
	if !ok {
		t.Fatalf("missing permission policy for %q", kind)
	}
	return PermissionSpec{
		Kind:        kind,
		Label:       policy.Label,
		Description: policy.Description,
		HighRisk:    policy.HighRisk,
	}
}

func TestHostEnforcesPermissionRiskFloor(t *testing.T) {
	for _, kind := range PermissionKinds() {
		policy, ok := PermissionPolicyFor(kind)
		if !ok {
			t.Fatalf("missing permission policy for %q", kind)
		}
		t.Run(kind+"/canonical", func(t *testing.T) {
			b := validBundle()
			permission := canonicalPermissionSpec(t, kind)
			permission.Context = LocaleString{"en": "Needed to synchronize this plugin.", "fr": "Nécessaire pour synchroniser ce module."}
			b.Permissions = []PermissionSpec{permission}
			if err := ValidateBundle(b); err != nil {
				t.Fatalf("ValidateBundle rejected canonical permission policy: %v", err)
			}
		})

		t.Run(kind+"/label-replaced", func(t *testing.T) {
			b := validBundle()
			permission := canonicalPermissionSpec(t, kind)
			permission.Label = L("Routine maintenance")
			b.Permissions = []PermissionSpec{permission}
			wantBundleError(t, b, `permission "`+kind+`" label must equal the host-owned policy label`)
		})

		t.Run(kind+"/warning-replaced", func(t *testing.T) {
			b := validBundle()
			permission := canonicalPermissionSpec(t, kind)
			permission.Description = L("Makes harmless housekeeping changes.")
			b.Permissions = []PermissionSpec{permission}
			wantBundleError(t, b, `permission "`+kind+`" description must equal the host-owned policy warning`)
		})

		t.Run(kind+"/risk-floor", func(t *testing.T) {
			b := validBundle()
			permission := canonicalPermissionSpec(t, kind)
			permission.HighRisk = !policy.HighRisk
			b.Permissions = []PermissionSpec{permission}
			if policy.HighRisk {
				wantBundleError(t, b, `permission "`+kind+`" high-risk flag cannot be lower than the host-owned policy`)
				return
			}
			if err := ValidateBundle(b); err != nil {
				t.Fatalf("ValidateBundle rejected conservative high-risk elevation: %v", err)
			}
		})
	}

	b := validBundle()
	permission := canonicalPermissionSpec(t, "plugin-secret")
	permission.Context = LocaleString{"fr": "Contexte sans valeur anglaise."}
	b.Permissions = []PermissionSpec{permission}
	wantBundleError(t, b, `permission "plugin-secret" context: locale map must carry an "en" entry`)
}

// TestValidateBundleFamilyScopes: the FamilyScopes error family — non-slug
// value, duplicate value, missing display name, and the exactly-one-default
// invariant on both sides.
func TestValidateBundleFamilyScopes(t *testing.T) {
	valid := validBundle()
	valid.FamilyScopes = []FamilyScopeSpec{
		{Value: "films", DisplayName: L("Films"), Default: true},
		{Value: "tvshows", DisplayName: L("TV Shows")},
	}
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected valid family scopes: %v", err)
	}

	badSlug := validBundle()
	badSlug.FamilyScopes = []FamilyScopeSpec{{Value: "Not-Slug", DisplayName: L("Bad"), Default: true}}
	wantBundleError(t, badSlug, `family scope value "Not-Slug" must be a lowercase slug`)

	duplicate := validBundle()
	duplicate.FamilyScopes = []FamilyScopeSpec{
		{Value: "films", DisplayName: L("Films"), Default: true},
		{Value: "films", DisplayName: L("Films Again")},
	}
	wantBundleError(t, duplicate, `duplicate family scope value "films"`)

	noName := validBundle()
	noName.FamilyScopes = []FamilyScopeSpec{{Value: "films", Default: true}}
	wantBundleError(t, noName, `family scope "films" display name is required`)

	zeroDefaults := validBundle()
	zeroDefaults.FamilyScopes = []FamilyScopeSpec{{Value: "films", DisplayName: L("Films")}}
	wantBundleError(t, zeroDefaults, "marks exactly one default, got 0")

	twoDefaults := validBundle()
	twoDefaults.FamilyScopes = []FamilyScopeSpec{
		{Value: "films", DisplayName: L("Films"), Default: true},
		{Value: "tvshows", DisplayName: L("TV Shows"), Default: true},
	}
	wantBundleError(t, twoDefaults, "marks exactly one default, got 2")
}

// TestValidateBundleSourceKindFixtures proves every independently cataloged
// Source kind has a canonical fixture accepted through the public validator.
func TestValidateBundleSourceKindFixtures(t *testing.T) {
	for _, fixture := range sourceKindFixtures {
		t.Run(fixture.kind, func(t *testing.T) {
			if err := ValidateBundle(sourceFixtureBundle(fixture.canonicalSource())); err != nil {
				t.Fatalf("ValidateBundle rejected canonical %q source fixture: %v", fixture.kind, err)
			}
		})
	}
}

// TestValidateBundleSourceParameterCombinations walks the complete presence
// matrix for Value, FieldFilters, EntityFilter, EntityKind, and Seed on every
// Source kind. Optional allowed parameters may be present or absent; required
// parameters must be present; every forbidden presence must reject.
func TestValidateBundleSourceParameterCombinations(t *testing.T) {
	const (
		withValue = 1 << iota
		withFieldFilters
		withEntityFilter
		withEntityKind
		withSeed
	)
	parameters := []struct {
		bit  int
		name string
	}{
		{withValue, "value"},
		{withFieldFilters, "field-filters"},
		{withEntityFilter, "entity-filter"},
		{withEntityKind, "entity-kind"},
		{withSeed, "seed"},
	}

	for _, fixture := range sourceKindFixtures {
		t.Run(fixture.kind, func(t *testing.T) {
			for combination := 0; combination < 1<<len(parameters); combination++ {
				names := make([]string, 0, len(parameters))
				for _, parameter := range parameters {
					if combination&parameter.bit != 0 {
						names = append(names, parameter.name)
					}
				}
				if len(names) == 0 {
					names = append(names, "none")
				}

				t.Run(strings.Join(names, "+"), func(t *testing.T) {
					source := SourceSpec{Kind: fixture.kind}
					valuePresent := combination&withValue != 0
					fieldFiltersPresent := combination&withFieldFilters != 0
					entityFilterPresent := combination&withEntityFilter != 0
					entityKindPresent := combination&withEntityKind != 0
					seedPresent := combination&withSeed != 0

					if valuePresent {
						source.Value = "fixture-value"
						if len(fixture.values) > 0 {
							source.Value = fixture.values[0]
						}
					}
					if fieldFiltersPresent {
						source.FieldFilters = map[string]any{sourceFixtureMediaField: "fixture-value"}
					}
					if entityFilterPresent {
						source.EntityFilter = &EntityFilter{Kind: sourceFixtureEntityKind, Name: "Fixture Entity"}
					}
					if entityKindPresent {
						source.EntityKind = sourceFixtureEntityKind
					}
					if seedPresent {
						source.Seed = "fixture-seed"
					}

					wantValid := valuePresent == (len(fixture.values) > 0)
					wantValid = wantValid && (!fieldFiltersPresent || fixture.filterable)
					wantValid = wantValid && (!entityFilterPresent || fixture.filterable)
					wantValid = wantValid && entityKindPresent == fixture.entityKind
					wantValid = wantValid && (!seedPresent || fixture.seedable)

					err := ValidateBundle(sourceFixtureBundle(source))
					if wantValid && err != nil {
						t.Fatalf("ValidateBundle rejected allowed parameter combination: %v", err)
					}
					if !wantValid && err == nil {
						t.Fatal("ValidateBundle accepted forbidden parameter combination")
					}
				})
			}
		})
	}
}

// TestValidateBundleSourceParameterErrors pins each required/forbidden error
// class independently from a valid fixture baseline.
func TestValidateBundleSourceParameterErrors(t *testing.T) {
	for _, fixture := range sourceKindFixtures {
		t.Run(fixture.kind, func(t *testing.T) {
			if len(fixture.values) > 0 {
				for _, value := range fixture.values {
					t.Run("value/"+value, func(t *testing.T) {
						source := fixture.canonicalSource()
						source.Value = value
						if err := ValidateBundle(sourceFixtureBundle(source)); err != nil {
							t.Fatalf("ValidateBundle rejected allowed value %q: %v", value, err)
						}
					})
				}
				for _, value := range []string{"", "not-a-source-value"} {
					t.Run("value/reject/"+value, func(t *testing.T) {
						source := fixture.canonicalSource()
						source.Value = value
						wantBundleError(t, sourceFixtureBundle(source), `source kind "`+fixture.kind+`" requires a value`)
					})
				}
			} else {
				source := fixture.canonicalSource()
				source.Value = "fixture-value"
				wantBundleError(t, sourceFixtureBundle(source), `source kind "`+fixture.kind+`" does not take a value`)
			}

			if fixture.filterable {
				withoutFieldFilters := fixture.canonicalSource()
				withoutFieldFilters.FieldFilters = nil
				if err := ValidateBundle(sourceFixtureBundle(withoutFieldFilters)); err != nil {
					t.Fatalf("ValidateBundle rejected optional field filters being absent: %v", err)
				}
				withoutEntityFilter := fixture.canonicalSource()
				withoutEntityFilter.EntityFilter = nil
				if err := ValidateBundle(sourceFixtureBundle(withoutEntityFilter)); err != nil {
					t.Fatalf("ValidateBundle rejected optional entity filter being absent: %v", err)
				}
			} else {
				withFieldFilters := fixture.canonicalSource()
				withFieldFilters.FieldFilters = map[string]any{sourceFixtureMediaField: "fixture-value"}
				wantBundleError(t, sourceFixtureBundle(withFieldFilters), `source kind "`+fixture.kind+`" does not take field filters`)

				withEntityFilter := fixture.canonicalSource()
				withEntityFilter.EntityFilter = &EntityFilter{Kind: sourceFixtureEntityKind, Name: "Fixture Entity"}
				wantBundleError(t, sourceFixtureBundle(withEntityFilter), `source kind "`+fixture.kind+`" does not take an entity filter`)
			}

			if fixture.entityKind {
				withoutEntityKind := fixture.canonicalSource()
				withoutEntityKind.EntityKind = ""
				wantBundleError(t, sourceFixtureBundle(withoutEntityKind), "entity-items source requires an entity kind")
			} else {
				withEntityKind := fixture.canonicalSource()
				withEntityKind.EntityKind = sourceFixtureEntityKind
				wantBundleError(t, sourceFixtureBundle(withEntityKind), `source kind "`+fixture.kind+`" does not take an entity kind`)
			}

			if fixture.seedable {
				withoutSeed := fixture.canonicalSource()
				withoutSeed.Seed = ""
				if err := ValidateBundle(sourceFixtureBundle(withoutSeed)); err != nil {
					t.Fatalf("ValidateBundle rejected optional seed being absent: %v", err)
				}
			} else {
				withSeed := fixture.canonicalSource()
				withSeed.Seed = "fixture-seed"
				wantBundleError(t, sourceFixtureBundle(withSeed), `source kind "`+fixture.kind+`" does not take a seed`)
			}
		})
	}
}

// TestValidateBundleSourceParameterShapes covers malformed parameter payloads
// after the fixture-driven capability matrix has selected an allowed kind.
func TestValidateBundleSourceParameterShapes(t *testing.T) {
	cases := []struct {
		name     string
		source   SourceSpec
		fragment string
	}{
		{
			"unknown kind",
			SourceSpec{Kind: "nope"},
			`unknown source kind "nope"`,
		},
		{
			"field filter key must be a slug",
			SourceSpec{Kind: "all-items", FieldFilters: map[string]any{"Not-Slug": "x"}},
			`field filter key "Not-Slug" must be a lowercase slug`,
		},
		{
			"field filter requires a value",
			SourceSpec{Kind: "all-items", FieldFilters: map[string]any{"genre": nil}},
			`field filter "genre" requires a value`,
		},
		{
			"entity filter kind must be a slug",
			SourceSpec{Kind: "all-items", EntityFilter: &EntityFilter{Kind: "Nope", Name: "x"}},
			`entity filter kind "Nope" must be a lowercase slug`,
		},
		{
			"entity filter requires a name",
			SourceSpec{Kind: "all-items", EntityFilter: &EntityFilter{Kind: "genre", Name: "  "}},
			`entity filter on kind "genre" requires a name`,
		},
		{
			"entity filter names an undeclared kind",
			SourceSpec{Kind: "all-items", EntityFilter: &EntityFilter{Kind: "genre", Name: "Comedy"}},
			`entity filter names undeclared entity kind "genre"`,
		},
		{
			"entity-items lists an undeclared kind",
			SourceSpec{Kind: "entity-items", EntityKind: "genre"},
			`lists undeclared entity kind "genre"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wantBundleError(t, sourceSectionBundle(tc.source), tc.fragment)
		})
	}
}

// TestValidateBundleFieldFilterNumberValues: NaN and ±Inf are rejected as
// number filter operands — both as equality values and as range bounds
// (isNumberValue's explicit rejection) — while finite values pass.
func TestValidateBundleFieldFilterNumberValues(t *testing.T) {
	base := func(value any) Bundle {
		b := sourceSectionBundle(SourceSpec{Kind: "all-items", FieldFilters: map[string]any{"runtime": value}})
		b.MediaFields = []FieldSpec{{Name: "runtime", Type: "number", Label: L("Runtime")}}
		return b
	}

	if err := ValidateBundle(base(90)); err != nil {
		t.Fatalf("ValidateBundle rejected a finite number equality filter: %v", err)
	}
	if err := ValidateBundle(base(map[string]any{"gte": 10.5, "lte": 200})); err != nil {
		t.Fatalf("ValidateBundle rejected a finite number range filter: %v", err)
	}

	wantBundleError(t, base(math.NaN()), "number filters require a number")
	wantBundleError(t, base(math.Inf(1)), "number filters require a number")
	wantBundleError(t, base(math.Inf(-1)), "number filters require a number")
	wantBundleError(t, base(map[string]any{"gte": math.NaN()}), `range bound "gte" must be a number`)
	wantBundleError(t, base(map[string]any{"lte": math.Inf(-1)}), `range bound "lte" must be a number`)
	wantBundleError(t, base(map[string]any{"gte": math.Inf(1)}), `range bound "gte" must be a number`)
}

// TestValidateBundleEntityMapBranches walks every EntityMap reject branch in
// bundle_validate.go against the scraperBundle fixture, plus the passing
// reference-lookup and parent-link shapes.
func TestValidateBundleEntityMapBranches(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(b *Bundle)
		fragment string // empty means the bundle must validate
	}{
		{
			"rss scrapers cannot declare entity maps",
			func(b *Bundle) {
				b.Scrapers[0].Kind = "rss"
				b.Scrapers[0].EntityMaps = []EntityMap{{Kind: "genre", Relation: "genres", Path: "genres"}}
			},
			"rss scrapers cannot declare entity maps",
		},
		{
			"blank path",
			func(b *Bundle) {
				b.Scrapers[0].EntityMaps = []EntityMap{{Kind: "genre", Relation: "genres", Path: "  "}}
			},
			`entity map for kind "genre" requires a path`,
		},
		{
			"blank artwork path",
			func(b *Bundle) {
				b.Scrapers[0].EntityMaps = []EntityMap{{Kind: "genre", Relation: "genres", Path: "genres", ArtworkPath: "  "}}
			},
			`entity map for kind "genre" has a blank artwork path`,
		},
		{
			"lookup template without an ID path",
			func(b *Bundle) {
				b.Scrapers[0].EntityMaps = []EntityMap{{Kind: "genre", Relation: "genres", Path: "genres", LookupTemplate: "/genre/{id}"}}
			},
			"reference lookup requires both lookup template and ID path",
		},
		{
			"reference lookup requires http-json",
			func(b *Bundle) {
				b.Scrapers[0].Kind = "http-graphql"
				b.Scrapers[0].EntityMaps = []EntityMap{{
					Kind: "genre", Relation: "genres", Path: "genres",
					LookupTemplate: "/genre/{id}", LookupIDPath: "id", MaxLookups: 3,
				}}
			},
			"reference lookup requires http-json",
		},
		{
			"reference lookup requires positive max lookups",
			func(b *Bundle) {
				b.Scrapers[0].EntityMaps = []EntityMap{{
					Kind: "genre", Relation: "genres", Path: "genres",
					LookupTemplate: "/genre/{id}", LookupIDPath: "id",
				}}
			},
			"reference lookup requires positive max lookups",
		},
		{
			"reference lookup template must contain {id}",
			func(b *Bundle) {
				b.Scrapers[0].EntityMaps = []EntityMap{{
					Kind: "genre", Relation: "genres", Path: "genres",
					LookupTemplate: "/genre/static", LookupIDPath: "id", MaxLookups: 3,
				}}
			},
			"reference lookup template must contain {id}",
		},
		{
			"reference lookup ID path is blank",
			func(b *Bundle) {
				b.Scrapers[0].EntityMaps = []EntityMap{{
					Kind: "genre", Relation: "genres", Path: "genres",
					LookupTemplate: "/genre/{id}", LookupIDPath: "  ", MaxLookups: 3,
				}}
			},
			"reference lookup ID path is blank",
		},
		{
			"negative max lookups without a lookup template",
			func(b *Bundle) {
				b.Scrapers[0].EntityMaps = []EntityMap{{Kind: "genre", Relation: "genres", Path: "genres", MaxLookups: -1}}
			},
			"max lookups must not be negative",
		},
		{
			"artwork URL template must be https and contain {value}",
			func(b *Bundle) {
				b.Scrapers[0].EntityMaps = []EntityMap{{
					Kind: "genre", Relation: "genres", Path: "genres",
					ArtworkURLTemplate: "http://img.example.test/{value}",
				}}
			},
			`entity map for kind "genre" artwork URL template must be https and contain {value}`,
		},
		{
			"field map names an undeclared field",
			func(b *Bundle) {
				b.Scrapers[0].EntityMaps = []EntityMap{{
					Kind: "genre", Relation: "genres", Path: "genres",
					FieldMap: map[string]string{"origin": "origin"},
				}}
			},
			`entity map for kind "genre" maps undeclared field "origin"`,
		},
		{
			"field map has a blank path",
			func(b *Bundle) {
				b.EntityKinds[0].Fields = []FieldSpec{{Name: "origin", Type: "text"}}
				b.Scrapers[0].EntityMaps = []EntityMap{{
					Kind: "genre", Relation: "genres", Path: "genres",
					FieldMap: map[string]string{"origin": "  "},
				}}
			},
			`entity map field "origin" for kind "genre" has a blank path`,
		},
		{
			"parent name path without a parent relation",
			func(b *Bundle) {
				b.Scrapers[0].EntityMaps = []EntityMap{{
					Kind: "genre", Relation: "genres", Path: "genres",
					ParentNamePath: "parent",
				}}
			},
			"declares a parent name path without a parent relation",
		},
		{
			"parent relation is not a same-kind relation on a hierarchical kind",
			func(b *Bundle) {
				b.Scrapers[0].EntityMaps = []EntityMap{{
					Kind: "genre", Relation: "genres", Path: "genres",
					ParentNamePath: "parent", ParentRelation: "up",
				}}
			},
			`entity map parent relation "up" is not a same-kind relation declared on hierarchical kind "genre"`,
		},
		{
			"parent relation without a parent name path",
			func(b *Bundle) {
				b.Scrapers[0].EntityMaps = []EntityMap{{
					Kind: "genre", Relation: "genres", Path: "genres",
					ParentRelation: "up",
				}}
			},
			"declares a parent relation without a parent name path",
		},
		{
			"ordinal field must be in the field map",
			func(b *Bundle) {
				b.Scrapers[0].EntityMaps = []EntityMap{{
					Kind: "genre", Relation: "genres", Path: "genres",
					OrdinalField: "ordinal",
				}}
			},
			`declares ordinal field "ordinal" that is not in its field map`,
		},
		{
			"valid reference lookup",
			func(b *Bundle) {
				b.Scrapers[0].EntityMaps = []EntityMap{{
					Kind: "genre", Relation: "genres", Path: "genres",
					LookupTemplate: "/genre/{id}", LookupIDPath: "id", MaxLookups: 3,
				}}
			},
			"",
		},
		{
			"valid parent link on a hierarchical kind",
			func(b *Bundle) {
				b.EntityKinds[0].Hierarchical = true
				b.EntityKinds[0].Relations = append(b.EntityKinds[0].Relations,
					RelationSpec{Name: "parent-of", ToKind: "genre"})
				b.Scrapers[0].EntityMaps = []EntityMap{{
					Kind: "genre", Relation: "genres", Path: "genres",
					ParentNamePath: "parent", ParentRelation: "parent-of",
				}}
			},
			"",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := scraperBundle()
			tc.mutate(&b)
			if tc.fragment == "" {
				if err := ValidateBundle(b); err != nil {
					t.Fatalf("ValidateBundle rejected a valid entity map: %v", err)
				}
				return
			}
			wantBundleError(t, b, tc.fragment)
		})
	}
}

// TestValidateBundleRSSScraperRestrictions: the rss transport carve-outs —
// no search template, and archive backfill only on rss with a guid source.
func TestValidateBundleRSSScraperRestrictions(t *testing.T) {
	searchOnRSS := scraperBundle()
	searchOnRSS.Scrapers[0].Kind = "rss"
	searchOnRSS.Scrapers[0].SearchTemplate = "/search?q={query}"
	searchOnRSS.Scrapers[0].CandidatesPath = "results"
	searchOnRSS.Scrapers[0].CandidateIDPath = "id"
	wantBundleError(t, searchOnRSS, "rss scrapers cannot declare a search template")

	backfillOffRSS := scraperBundle()
	backfillOffRSS.Scrapers[0].ArchiveBackfill = true
	backfillOffRSS.Scrapers[0].ArchiveBackfillGUIDPath = "guid"
	wantBundleError(t, backfillOffRSS, "archive backfill is only valid on rss scrapers")

	backfillNoGUID := scraperBundle()
	backfillNoGUID.Scrapers[0].Kind = "rss"
	backfillNoGUID.Scrapers[0].ArchiveBackfill = true
	wantBundleError(t, backfillNoGUID, "declares archive backfill without a guid source")

	valid := scraperBundle()
	valid.Scrapers[0].Kind = "rss"
	valid.Scrapers[0].ArchiveBackfill = true
	valid.Scrapers[0].ArchiveBackfillGUIDPath = "guid"
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected a valid rss archive-backfill scraper: %v", err)
	}
}

// TestValidateBundleScraperThrottleAndArtwork: MinRequestIntervalMS must not
// be negative, ArtworkBaseURL must be https, and ArtworkURLTemplates must name
// known slots with https-{value} templates.
func TestValidateBundleScraperThrottleAndArtwork(t *testing.T) {
	valid := scraperBundle()
	valid.Scrapers[0].MinRequestIntervalMS = 1000
	valid.Scrapers[0].ArtworkBaseURL = "https://img.example.test"
	valid.Scrapers[0].ArtworkURLTemplates = map[string]string{"poster": "https://img.example.test/{value}"}
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected valid throttle and artwork declarations: %v", err)
	}

	negativeInterval := scraperBundle()
	negativeInterval.Scrapers[0].MinRequestIntervalMS = -1
	wantBundleError(t, negativeInterval, "min request interval must not be negative")

	insecureBase := scraperBundle()
	insecureBase.Scrapers[0].ArtworkBaseURL = "http://img.example.test"
	wantBundleError(t, insecureBase, "artwork base URL must be https")

	unknownSlot := scraperBundle()
	unknownSlot.Scrapers[0].ArtworkURLTemplates = map[string]string{"not-a-slot": "https://img.example.test/{value}"}
	wantBundleError(t, unknownSlot, `maps unknown artwork slot "not-a-slot"`)

	badTemplate := scraperBundle()
	badTemplate.Scrapers[0].ArtworkURLTemplates = map[string]string{"poster": "https://img.example.test/static"}
	wantBundleError(t, badTemplate, `artwork URL template for slot "poster" must be https and contain {value}`)
}

// TestValidateBundleArtworkSlotVocabulary proves the widened artwork-slot
// vocabulary admits season posters without admitting near-match typos.
func TestValidateBundleArtworkSlotVocabulary(t *testing.T) {
	seasonPoster := scraperBundle()
	seasonPoster.Scrapers[0].ArtworkMap = map[string]string{"season-poster": "season.poster_path"}
	if err := ValidateBundle(seasonPoster); err != nil {
		t.Fatalf("ValidateBundle rejected season-poster artwork: %v", err)
	}

	unknown := scraperBundle()
	unknown.Scrapers[0].ArtworkMap = map[string]string{"bannerx": "banner_path"}
	wantBundleError(t, unknown, `maps unknown artwork slot "bannerx"`)
}

// TestValidateBundleScraperTransforms: a transform must name a TransformKinds
// entry and apply to a mapped field.
func TestValidateBundleScraperTransforms(t *testing.T) {
	valid := scraperBundle()
	valid.Scrapers[0].FieldMap = map[string]string{"title": "name", "year": "released"}
	valid.Scrapers[0].Transforms = map[string]string{"year": "year-from-date"}
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected a valid transform: %v", err)
	}

	unknownKind := scraperBundle()
	unknownKind.Scrapers[0].Transforms = map[string]string{"title": "uppercase"}
	wantBundleError(t, unknownKind, `uses unknown transform "uppercase" on field "title"`)

	unmapped := scraperBundle()
	unmapped.Scrapers[0].Transforms = map[string]string{"year": "year-from-date"}
	wantBundleError(t, unmapped, `transforms unmapped field "year"`)
}

// TestValidateBundleScraperImpliedMediaFields: an implied media field must be
// a declared plugin text MediaField ("media.<name>") with a non-blank value —
// built-ins, undeclared names, non-text fields, and blank values all reject.
func TestValidateBundleScraperImpliedMediaFields(t *testing.T) {
	base := func() Bundle {
		b := scraperBundle()
		b.MediaFields = []FieldSpec{
			{Name: "class", Type: "text", Label: L("Class")},
			{Name: "runtime", Type: "number", Label: L("Runtime")},
		}
		return b
	}

	valid := base()
	valid.Scrapers[0].ImpliedMediaFields = map[string]string{MediaFieldKeyPrefix + "class": "movie"}
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected a valid implied media field: %v", err)
	}

	undeclared := base()
	undeclared.Scrapers[0].ImpliedMediaFields = map[string]string{MediaFieldKeyPrefix + "nope": "x"}
	wantBundleError(t, undeclared, `names media field "nope", which the bundle does not declare`)

	builtin := base()
	builtin.Scrapers[0].ImpliedMediaFields = map[string]string{"title": "x"}
	wantBundleError(t, builtin, `implies built-in field "title"; only plugin media fields (media.<name>) are implied`)

	nonText := base()
	nonText.Scrapers[0].ImpliedMediaFields = map[string]string{MediaFieldKeyPrefix + "runtime": "90"}
	wantBundleError(t, nonText, `implies media field "runtime" of type "number", only text fields can be implied`)

	blankValue := base()
	blankValue.Scrapers[0].ImpliedMediaFields = map[string]string{MediaFieldKeyPrefix + "class": "  "}
	wantBundleError(t, blankValue, `implies media field "class" without a value`)
}

// TestValidateBundleInteractiveSidecars: the sidecarExtPattern contract — a
// lowercase extension with no dot, non-empty, no duplicates.
func TestValidateBundleInteractiveSidecars(t *testing.T) {
	valid := validBundle()
	valid.InteractiveSidecars = []string{"funscript"}
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected a valid interactive sidecar extension: %v", err)
	}

	uppercase := validBundle()
	uppercase.InteractiveSidecars = []string{"FUNSCRIPT"}
	wantBundleError(t, uppercase, `interactive sidecar extension "FUNSCRIPT" must be a lowercase extension with no dot`)

	dotted := validBundle()
	dotted.InteractiveSidecars = []string{"tar.gz"}
	wantBundleError(t, dotted, `interactive sidecar extension "tar.gz" must be a lowercase extension with no dot`)

	empty := validBundle()
	empty.InteractiveSidecars = []string{""}
	wantBundleError(t, empty, `interactive sidecar extension "" must be a lowercase extension with no dot`)

	duplicate := validBundle()
	duplicate.InteractiveSidecars = []string{"funscript", "funscript"}
	wantBundleError(t, duplicate, `duplicate interactive sidecar extension "funscript"`)
}

// TestValidateBundleScrobbleMatch: every ScrobbleMatch entry must be a bare
// slug naming a declared Media-targeted relation, without duplicates.
func TestValidateBundleScrobbleMatch(t *testing.T) {
	valid := scraperBundle()
	valid.ScrobbleMatch = &ScrobbleMatchSpec{ArtistConfirm: []string{"genres"}, AlbumTieBreak: []string{"genres"}}
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected a valid scrobble match spec: %v", err)
	}

	undeclared := scraperBundle()
	undeclared.ScrobbleMatch = &ScrobbleMatchSpec{ArtistConfirm: []string{"nope"}}
	wantBundleError(t, undeclared, `scrobble match artistConfirm relation "nope" is not a declared Media-targeted relation`)

	nonMedia := scraperBundle()
	nonMedia.EntityKinds[0].Relations = append(nonMedia.EntityKinds[0].Relations,
		RelationSpec{Name: "sub", ToKind: "genre"})
	nonMedia.ScrobbleMatch = &ScrobbleMatchSpec{ArtistConfirm: []string{"sub"}}
	wantBundleError(t, nonMedia, `scrobble match artistConfirm relation "sub" is not a declared Media-targeted relation`)

	duplicate := scraperBundle()
	duplicate.ScrobbleMatch = &ScrobbleMatchSpec{AlbumTieBreak: []string{"genres", "genres"}}
	wantBundleError(t, duplicate, `scrobble match albumTieBreak duplicates relation "genres"`)

	badSlug := scraperBundle()
	badSlug.ScrobbleMatch = &ScrobbleMatchSpec{ArtistConfirm: []string{"Not-Slug"}}
	wantBundleError(t, badSlug, `scrobble match artistConfirm relation "Not-Slug" must be a lowercase slug`)
}

// TestValidateBundleDuplicateMediaClasses: MediaClasses reject unknown and
// duplicate entries.
func TestValidateBundleDuplicateMediaClasses(t *testing.T) {
	unknown := validBundle()
	unknown.MediaClasses = []string{"podcast"}
	wantBundleError(t, unknown, `unknown media class "podcast"`)

	duplicate := validBundle()
	duplicate.MediaClasses = []string{"video", "video"}
	wantBundleError(t, duplicate, `duplicate media class "video"`)
}

// TestValidateBundleEngagementEventKinds: declared engagement event kinds must
// be non-blank and unique.
func TestValidateBundleEngagementEventKinds(t *testing.T) {
	valid := validBundle()
	valid.Engagement.EventKinds = []string{"o-counter", "replay"}
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected valid engagement event kinds: %v", err)
	}

	blank := validBundle()
	blank.Engagement.EventKinds = []string{"  "}
	wantBundleError(t, blank, "engagement event kinds must be non-blank")

	duplicate := validBundle()
	duplicate.Engagement.EventKinds = []string{"o-counter", "o-counter"}
	wantBundleError(t, duplicate, `duplicate engagement event kind "o-counter"`)
}

// TestValidateBundleRejectsMalformedLookupByIDTemplate: the LookupByIDTemplate
// shape check is wired into ValidateBundle, so a malformed declaration fails
// bundle validation, not only the standalone validator.
func TestValidateBundleRejectsMalformedLookupByIDTemplate(t *testing.T) {
	missingID := scraperBundle()
	missingID.Scrapers[0].LookupByIDTemplate = map[string]string{"tmdb": "/movie/"}
	wantBundleError(t, missingID, `LookupByIDTemplate["tmdb"] must contain {id}`)

	unknownProvider := scraperBundle()
	unknownProvider.Scrapers[0].LookupByIDTemplate = map[string]string{"plex": "/x/{id}"}
	wantBundleError(t, unknownProvider, `LookupByIDTemplate key "plex" is outside the vocabulary`)
}

// TestValidateBundleScraperConfigPlaceholders proves request templates may
// reference declared ConfigFields and fail closed on undeclared names.
func TestValidateBundleScraperConfigPlaceholders(t *testing.T) {
	cases := map[string]func(*Bundle){
		"lookup": func(b *Bundle) {
			b.Scrapers[0].LookupTemplate = "/lookup/{id}?region={config.region}"
		},
		"lookup by provider ID": func(b *Bundle) {
			b.Scrapers[0].LookupByIDTemplate = map[string]string{
				"tmdb": "/movie/{id}?region={config.region}",
			}
		},
		"search": func(b *Bundle) {
			b.Scrapers[0].SearchTemplate = "/search?q={query}&region={config.region}"
			b.Scrapers[0].CandidatesPath = "results"
			b.Scrapers[0].CandidateIDPath = "id"
		},
	}

	for name, setTemplate := range cases {
		t.Run(name, func(t *testing.T) {
			declared := scraperBundle()
			declared.ConfigFields = []ConfigFieldSpec{{Name: "region", Type: "text", Label: L("Region")}}
			setTemplate(&declared)
			if err := ValidateBundle(declared); err != nil {
				t.Fatalf("ValidateBundle rejected declared config placeholder: %v", err)
			}

			undeclared := scraperBundle()
			setTemplate(&undeclared)
			wantBundleError(t, undeclared, `template references undeclared config field "region"`)
		})
	}
}
