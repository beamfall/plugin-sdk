package sdktest

import (
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
)

// KindVocab is one declared EntityKind's name + archetype pair. Plugins pin
// their bundle's Kind vocabulary against a design of record with
// this shape, so a future rename or archetype change fails the plugin's own
// gate instead of drifting silently out of sync with the doc.
type KindVocab struct {
	Name      string
	Archetype string
}

// PodcastADR0030KindVocab is the Podcast plugin's EntityKind vocabulary of
// record: one container Kind for the
// show, one person-archetype Kind carrying both hosts and guests, and one
// flat category Kind seeded from the Apple genre taxonomy. Order matches the
// bundle's declared EntityKinds.
var PodcastADR0030KindVocab = []KindVocab{
	{Name: "show", Archetype: "container"},
	{Name: "person", Archetype: "person"},
	{Name: "topic", Archetype: "category"},
}

// RequireEntityKindVocab fails the test if b's declared EntityKinds (name +
// archetype, in order) don't match want exactly. Use this to pin a bundle's
// Kind vocabulary against a design-of-record so a rename or archetype change
// reds the plugin's own gate instead of drifting silently out of sync with
// the ADR that names it.
func RequireEntityKindVocab(t testing.TB, b sdk.Bundle, want []KindVocab) {
	t.Helper()
	got := make([]KindVocab, 0, len(b.EntityKinds))
	for _, kind := range b.EntityKinds {
		got = append(got, KindVocab{Name: kind.Name, Archetype: kind.Archetype})
	}
	if len(got) != len(want) {
		t.Fatalf("Bundle EntityKinds = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Bundle EntityKinds = %+v, want %+v", got, want)
		}
	}
}
