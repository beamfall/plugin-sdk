package sdk

import (
	"slices"
	"testing"
)

const (
	sourceFixtureEntityKind = "fixture-entity"
	sourceFixtureMediaField = "fixture-field"
)

// sourceKindFixtures is an independent contract catalog: adding a Source kind
// or changing one of its parameter capabilities requires an explicit fixture.
type sourceKindFixture struct {
	kind       string
	values     []string
	filterable bool
	entityKind bool
	seedable   bool
}

var sourceKindFixtures = []sourceKindFixture{
	{kind: "recently-added", filterable: true},
	{kind: "continue-watching"},
	{kind: "next-up"},
	{kind: "recently-played"},
	{kind: "most-played"},
	{kind: "all-items", filterable: true},
	{kind: "on-this-day"},
	{kind: "entity-items", entityKind: true},
	{kind: "entity-media"},
	{kind: "random", seedable: true},
	{kind: "resume-progress", values: []string{"in-progress"}},
	{kind: "availability", values: []string{"archived", "streamed-provisional", "not-downloaded"}},
	{kind: "watched-by-any-profile", values: []string{"any", "none"}},
	{kind: "directory", values: []string{"trending"}},
}

func (fixture sourceKindFixture) canonicalSource() SourceSpec {
	source := SourceSpec{Kind: fixture.kind}
	if len(fixture.values) > 0 {
		source.Value = fixture.values[0]
	}
	if fixture.filterable {
		source.FieldFilters = map[string]any{sourceFixtureMediaField: "fixture-value"}
		source.EntityFilter = &EntityFilter{Kind: sourceFixtureEntityKind, Name: "Fixture Entity"}
	}
	if fixture.entityKind {
		source.EntityKind = sourceFixtureEntityKind
	}
	if fixture.seedable {
		source.Seed = "fixture-seed"
	}
	return source
}

func sourceKindFixtureIndex(t *testing.T) map[string]sourceKindFixture {
	t.Helper()
	index := make(map[string]sourceKindFixture, len(sourceKindFixtures))
	for _, fixture := range sourceKindFixtures {
		if _, duplicate := index[fixture.kind]; duplicate {
			t.Fatalf("sourceKindFixtures duplicates %q", fixture.kind)
		}
		index[fixture.kind] = fixture
	}
	return index
}

func TestSourceKindFixturesCoverVocabulary(t *testing.T) {
	fixtures := sourceKindFixtureIndex(t)
	known := make(map[string]bool, len(SourceKinds()))
	for _, kind := range SourceKinds() {
		if known[kind] {
			t.Errorf("SourceKinds() duplicates %q", kind)
		}
		known[kind] = true
		if _, covered := fixtures[kind]; !covered {
			t.Errorf("SourceKinds() entry %q has no canonical fixture", kind)
		}
	}
	for kind := range fixtures {
		if !known[kind] {
			t.Errorf("sourceKindFixtures contains orphan %q", kind)
		}
	}
}

func TestSourceKindFixtureSubsets(t *testing.T) {
	fixtures := sourceKindFixtureIndex(t)
	known := make(map[string]bool, len(SourceKinds()))
	for _, kind := range SourceKinds() {
		known[kind] = true
	}

	filterable := make(map[string]bool, len(FilterableSourceKinds()))
	for _, kind := range FilterableSourceKinds() {
		if filterable[kind] {
			t.Errorf("FilterableSourceKinds() duplicates %q", kind)
		}
		filterable[kind] = true
		fixture, covered := fixtures[kind]
		if !known[kind] {
			t.Errorf("FilterableSourceKinds() contains orphan %q", kind)
		} else if !covered || !fixture.filterable {
			t.Errorf("FilterableSourceKinds() unexpectedly contains %q", kind)
		}
	}
	for _, fixture := range sourceKindFixtures {
		if fixture.filterable && !filterable[fixture.kind] {
			t.Errorf("FilterableSourceKinds() is missing fixture kind %q", fixture.kind)
		}
	}

	valued := ValuedSourceKinds()
	for kind, values := range valued {
		fixture, covered := fixtures[kind]
		if !known[kind] {
			t.Errorf("ValuedSourceKinds() contains orphan %q", kind)
		} else if !covered || len(fixture.values) == 0 {
			t.Errorf("ValuedSourceKinds() unexpectedly contains %q", kind)
		}
		seenValues := make(map[string]bool, len(values))
		for _, value := range values {
			if value == "" {
				t.Errorf("ValuedSourceKinds()[%q] contains an empty value", kind)
			}
			if seenValues[value] {
				t.Errorf("ValuedSourceKinds()[%q] duplicates %q", kind, value)
			}
			seenValues[value] = true
		}
		if covered && !slices.Equal(values, fixture.values) {
			t.Errorf("ValuedSourceKinds()[%q] = %v, want fixture values %v", kind, values, fixture.values)
		}
	}
	for _, fixture := range sourceKindFixtures {
		if len(fixture.values) == 0 {
			continue
		}
		if _, present := valued[fixture.kind]; !present {
			t.Errorf("ValuedSourceKinds() is missing fixture kind %q", fixture.kind)
		}
	}
}
