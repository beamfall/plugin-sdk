package sdktest_test

import (
	"net/http"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
	"github.com/beamfall/plugin-sdk/sdktest"
)

// titleParser echoes the filename stem (sans the trailing extension token) as
// the parsed title — enough to exercise the corpus pick without a real parser.
type titleParser struct{}

func (titleParser) Parse(path string, _ sdk.ParseProbe) (sdk.ParsedMedia, bool) {
	title := path
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			title = path[:i]
			break
		}
	}
	if title == "" {
		return sdk.ParsedMedia{}, false
	}
	return sdk.ParsedMedia{Title: title}, true
}

// titleScorer scores 1.0 on an exact title match, else 0 — trivially conformant.
type titleScorer struct{}

func (titleScorer) Score(candidate sdk.Candidate, parsed sdk.ParsedMedia) float64 {
	if candidate.Title == parsed.Title {
		return 1
	}
	return 0
}

func sampleFamily() sdktest.MatchFamily {
	candidates := []sdk.Candidate{
		{ID: "tt-blade", Title: "Blade Runner"},
		{ID: "tt-heat", Title: "Heat"},
	}
	return sdktest.MatchFamily{
		Name: "scene-naming",
		Cases: []sdktest.MatchCase{
			{Name: "exact-title", Path: "Blade Runner.mkv", Candidates: candidates, WantID: "tt-blade"},
			{Name: "other-title", Path: "Heat.mkv", Candidates: candidates, WantID: "tt-heat"},
			{Name: "no-candidate-matches", Path: "Unknown Film.mkv", Candidates: candidates, WantID: ""},
			{Name: "unparseable-routes-unmatched", Path: ".mkv", Candidates: candidates, WantID: ""},
		},
	}
}

// TestRunMatchCorpusGreenOnSampleFamily is the SDK's own smoke check that the
// corpus runner grades a conformant parser+scorer green across a sample fixture
// family, including the fail-closed unmatched baseline.
func TestRunMatchCorpusGreenOnSampleFamily(t *testing.T) {
	sdktest.RunMatchCorpus(t, titleParser{}, titleScorer{}, []sdktest.MatchFamily{sampleFamily()})
}

// TestRunMatchCorpusRestoresTransport confirms the seal is scoped to the run:
// after RunMatchCorpus returns, the default transport is the one it found.
func TestRunMatchCorpusRestoresTransport(t *testing.T) {
	before := http.DefaultTransport
	sdktest.RunMatchCorpus(t, titleParser{}, titleScorer{}, []sdktest.MatchFamily{sampleFamily()})
	if http.DefaultTransport != before {
		t.Fatal("RunMatchCorpus did not restore http.DefaultTransport after the run")
	}
}
