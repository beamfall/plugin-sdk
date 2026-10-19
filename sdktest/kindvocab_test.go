package sdktest_test

import (
	"runtime"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
	"github.com/beamfall/plugin-sdk/sdktest"
)

// podcastConformantBundle is the smallest Bundle carrying the Podcast
// plugin's EntityKind vocabulary of record, standing in for the real
// beamfall-plugin-podcasts Bundle (a separate module the SDK cannot import).
func podcastConformantBundle(kinds []sdk.EntityKind) sdk.Bundle {
	return sdk.Bundle{
		ID:           "podcasts",
		Version:      "0.1.0",
		SDKFloor:     sdk.Version,
		SDKCeiling:   sdk.Version,
		DisplayName:  sdk.LocaleString{"en": "Podcasts"},
		MediaLabel:   sdk.LocaleString{"en": "Episode"},
		MediaClasses: []string{"audio"},
		Layouts:      []sdk.LayoutSpec{{ID: "browse", Role: "browse"}},
		EntityKinds:  kinds,
	}
}

// TestRequireEntityKindVocabPassesOnADR0030Vocab pins the harness's own
// golden vector: a bundle declaring exactly the Podcast vocabulary-of-record Kind names and
// archetypes (show/container, person/person, topic/category) passes.
func TestRequireEntityKindVocabPassesOnADR0030Vocab(t *testing.T) {
	b := podcastConformantBundle([]sdk.EntityKind{
		{Name: "show", Archetype: "container"},
		{Name: "person", Archetype: "person"},
		{Name: "topic", Archetype: "category"},
	})
	sdktest.RequireEntityKindVocab(t, b, sdktest.PodcastADR0030KindVocab)
}

// TestRequireEntityKindVocabCatchesDrift is the negative smoke test: a Kind
// rename, archetype change, reorder, or drop must fail RequireEntityKindVocab
// against the vocabulary of record: a rename fails the gate.
func TestRequireEntityKindVocabCatchesDrift(t *testing.T) {
	cases := []struct {
		name  string
		kinds []sdk.EntityKind
	}{
		{"renamed show kind", []sdk.EntityKind{
			{Name: "podcast", Archetype: "container"},
			{Name: "person", Archetype: "person"},
			{Name: "topic", Archetype: "category"},
		}},
		{"renamed topic kind", []sdk.EntityKind{
			{Name: "show", Archetype: "container"},
			{Name: "person", Archetype: "person"},
			{Name: "category", Archetype: "category"},
		}},
		{"changed archetype", []sdk.EntityKind{
			{Name: "show", Archetype: "category"},
			{Name: "person", Archetype: "person"},
			{Name: "topic", Archetype: "category"},
		}},
		{"reordered kinds", []sdk.EntityKind{
			{Name: "person", Archetype: "person"},
			{Name: "show", Archetype: "container"},
			{Name: "topic", Archetype: "category"},
		}},
		{"dropped kind", []sdk.EntityKind{
			{Name: "show", Archetype: "container"},
			{Name: "person", Archetype: "person"},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ft := &fakeT{}
			b := podcastConformantBundle(c.kinds)
			done := make(chan struct{})
			go func() {
				defer close(done)
				sdktest.RequireEntityKindVocab(ft, b, sdktest.PodcastADR0030KindVocab)
			}()
			<-done
			if !ft.failed {
				t.Fatalf("RequireEntityKindVocab did not fail for %+v", c.kinds)
			}
		})
	}
}

// fakeT is the minimal testing.TB fake needed to observe a Fatal call. Like
// the real testing.T, Fatalf halts the calling goroutine via runtime.Goexit
// after recording the failure, so RequireEntityKindVocab's post-Fatalf code
// (if any) never runs — the same guarantee production callers rely on.
type fakeT struct {
	testing.TB
	failed bool
}

func (f *fakeT) Helper() {}
func (f *fakeT) Fatalf(format string, args ...any) {
	f.failed = true
	runtime.Goexit()
}
