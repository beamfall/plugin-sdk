package sdktest_test

import (
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
	"github.com/beamfall/plugin-sdk/sdktest"
)

// fakeParser is a trivial conformant FilenameParser for the smoke test: it
// echoes the path as the title and always succeeds.
type fakeParser struct{}

func (fakeParser) Parse(path string, _ sdk.ParseProbe) (sdk.ParsedMedia, bool) {
	if path == "" {
		return sdk.ParsedMedia{}, false
	}
	if path == "series.m4b" {
		return sdk.ParsedMedia{Title: path, Sequence: "4.5"}, true
	}
	return sdk.ParsedMedia{Title: path}, true
}

// fakeContextParser opts into the additive context seam. Unknown and future
// values preserve fakeParser's legacy result; only known bounded shapes vary.
type fakeContextParser struct{ fakeParser }

func (p fakeContextParser) ParseWithContext(path string, probe sdk.ParseProbe, context sdk.ParseContext) (sdk.ParsedMedia, bool) {
	legacy, ok := p.Parse(path, probe)
	if !ok {
		return legacy, ok
	}
	switch context.DirectoryMediaFileCount {
	case sdk.DirectoryMediaFileCountSingle:
		legacy.Class = "single"
	case sdk.DirectoryMediaFileCountMultiple:
		legacy.Class = "multiple"
	}
	return legacy, true
}

var _ sdk.ContextFilenameParser = fakeContextParser{}

// fakeScorer is a trivial conformant Scorer: 1.0 on an exact title match, else 0.
type fakeScorer struct{}

func (fakeScorer) Score(candidate sdk.Candidate, parsed sdk.ParsedMedia) float64 {
	if candidate.Title == parsed.Title {
		return 1
	}
	if candidate.Sequence == 2 && candidate.Index == 5 && candidate.Popularity == 10 &&
		parsed.Sequence == "2" && parsed.Index == 5 {
		return 0.5
	}
	// "ordinal-probe" exercises Candidate.IndexPresent: a scorer that only
	// looked at Index would score an absent ordinal the same as a genuine
	// zero, which is exactly what SequencePresent/IndexPresent exist to
	// prevent. The three distinct scores below give absent/zero/nonzero each
	// a separately assertable outcome.
	if candidate.Title == "ordinal-probe" {
		switch {
		case !candidate.IndexPresent:
			return 0.1
		case candidate.Index == 0:
			return 0.6
		default:
			return 0.9
		}
	}
	return 0
}

// conformantBundle is the smallest Bundle that satisfies every
// RunBundleConformance check, including the exactly-one-browse-Layout
// guarantee — the fixture both the positive and negative cases start from.
func conformantBundle() sdk.Bundle {
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

// TestHarnessRunsGreenOnConformantInputs is the SDK's own smoke check: the three
// runners must pass for trivially-conformant inputs, so a plugin that fails them
// is failing the contract, not the harness.
func TestHarnessRunsGreenOnConformantInputs(t *testing.T) {
	sdktest.RunConformance(t, fakeParser{}, []sdktest.ParseVector{
		{Name: "echo", Path: "a.mkv", Want: sdk.ParsedMedia{Title: "a.mkv"}, WantOK: true},
		{Name: "fractional sequence", Path: "series.m4b", Want: sdk.ParsedMedia{Title: "series.m4b", Sequence: "4.5"}, WantOK: true},
		{Name: "empty path refused", Path: "", Want: sdk.ParsedMedia{}, WantOK: false},
	})
	sdktest.RunContextConformance(t, fakeContextParser{}, []sdktest.ContextParseVector{
		{Name: "unknown preserves legacy", Path: "series.m4b",
			Want: sdk.ParsedMedia{Title: "series.m4b", Sequence: "4.5"}, WantOK: true},
		{Name: "single directory media file", Path: "a.m4b",
			Context: sdk.ParseContext{DirectoryMediaFileCount: sdk.DirectoryMediaFileCountSingle},
			Want:    sdk.ParsedMedia{Title: "a.m4b", Class: "single"}, WantOK: true},
		{Name: "multiple directory media files", Path: "a.mp3",
			Context: sdk.ParseContext{DirectoryMediaFileCount: sdk.DirectoryMediaFileCountMultiple},
			Want:    sdk.ParsedMedia{Title: "a.mp3", Class: "multiple"}, WantOK: true},
		{Name: "future context preserves legacy", Path: "a.mp3",
			Context: sdk.ParseContext{DirectoryMediaFileCount: sdk.DirectoryMediaFileCount("future")},
			Want:    sdk.ParsedMedia{Title: "a.mp3"}, WantOK: true},
		{Name: "empty path refused", Path: "", Want: sdk.ParsedMedia{}, WantOK: false},
	})
	sdktest.RunScoreConformance(t, fakeScorer{}, []sdktest.ScoreVector{
		{Name: "exact", CandTitle: "x", Parsed: sdk.ParsedMedia{Title: "x"}, Want: 1},
		{Name: "miss", CandTitle: "x", Parsed: sdk.ParsedMedia{Title: "y"}, Want: 0},
		{Name: "candidate signals", CandTitle: "x", CandSequence: 2, CandIndex: 5, CandPopularity: 10,
			Parsed: sdk.ParsedMedia{Title: "y", Sequence: "2", Index: 5}, Want: 0.5},
		// Ordinal presence fixtures (absent / zero / nonzero): the zero-value
		// vector below leaves CandIndexPresent false, so it must NOT be
		// confused with a genuine ordinal 0.
		{Name: "ordinal absent", CandTitle: "ordinal-probe",
			Parsed: sdk.ParsedMedia{Title: "irrelevant"}, Want: 0.1},
		{Name: "ordinal zero present", CandTitle: "ordinal-probe", CandIndex: 0, CandIndexPresent: true,
			Parsed: sdk.ParsedMedia{Title: "irrelevant"}, Want: 0.6},
		{Name: "ordinal nonzero present", CandTitle: "ordinal-probe", CandIndex: 7, CandIndexPresent: true,
			Parsed: sdk.ParsedMedia{Title: "irrelevant"}, Want: 0.9},
	})
	sdktest.RunBundleConformance(t, conformantBundle())
}
