package sdktest

import (
	"math"
	"net/http"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
)

// constScorer scores each candidate ID by a fixed table, so pickMatch's
// fail-closed logic can be exercised without a real scorer.
type constScorer map[string]float64

func (s constScorer) Score(c sdk.Candidate, _ sdk.ParsedMedia) float64 { return s[c.ID] }

// TestPickMatchFailClosed pins the fail-closed identification the corpus grades
// against: strictly-above-floor wins, nothing-above-floor and ambiguous top
// bands route to unmatched — so a wrong match can never be produced by a tie.
func TestPickMatchFailClosed(t *testing.T) {
	cands := []sdk.Candidate{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	cases := []struct {
		name   string
		scores constScorer
		floor  float64
		want   string
	}{
		{"clear winner", constScorer{"a": 0.9, "b": 0.2, "c": 0.1}, 0.5, "a"},
		{"nothing above floor", constScorer{"a": 0.4, "b": 0.3}, 0.5, ""},
		{"exactly at floor is not above", constScorer{"a": 0.5}, 0.5, ""},
		{"tie at top routes unmatched", constScorer{"a": 0.9, "b": 0.9}, 0.5, ""},
		{"in-band scores without popularity route unmatched", constScorer{"a": 0.9, "b": 0.87}, 0.5, ""},
		{"winner over an out-of-band field", constScorer{"a": 0.6, "b": 0.51}, 0.5, "a"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got, err := pickMatch(c.scores, sdk.ParsedMedia{}, cands, c.floor)
			if err != nil {
				t.Fatalf("pickMatch error: %v", err)
			}
			if got != c.want {
				t.Errorf("pickMatch = %q, want %q", got, c.want)
			}
		})
	}
}

func TestPickMatchPopularityBreaksEligibleAmbiguityOnly(t *testing.T) {
	cases := []struct {
		name   string
		cands  []sdk.Candidate
		scores constScorer
		want   string
	}{
		{
			name:   "exact tie",
			cands:  []sdk.Candidate{{ID: "a", Popularity: 10}, {ID: "b", Popularity: 100}},
			scores: constScorer{"a": 0.9, "b": 0.9},
			want:   "b",
		},
		{
			name:   "near tie",
			cands:  []sdk.Candidate{{ID: "a", Popularity: 10}, {ID: "b", Popularity: 100}},
			scores: constScorer{"a": 0.9, "b": 0.87},
			want:   "",
		},
		{
			name:   "exact top tie only",
			cands:  []sdk.Candidate{{ID: "a", Popularity: 10}, {ID: "b", Popularity: 100}, {ID: "c", Popularity: 1000}},
			scores: constScorer{"a": 0.9, "b": 0.9, "c": 0.87},
			want:   "b",
		},
		{
			name:   "below floor ignored",
			cands:  []sdk.Candidate{{ID: "a", Popularity: 100}},
			scores: constScorer{"a": 0.5},
			want:   "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := pickMatch(c.scores, sdk.ParsedMedia{}, c.cands, 0.5)
			if err != nil {
				t.Fatalf("pickMatch error: %v", err)
			}
			if got != c.want {
				t.Fatalf("pickMatch = %q, want %q", got, c.want)
			}
		})
	}
}

// TestPickMatchRejectsOutOfRangeScore proves the corpus enforces the same [0,1]
// scorer contract RunScoreConformance does, rather than picking on a bad score.
func TestPickMatchRejectsOutOfRangeScore(t *testing.T) {
	cands := []sdk.Candidate{{ID: "a"}}
	for _, bad := range []float64{-0.1, 1.1, math.NaN()} {
		if _, err := pickMatch(constScorer{"a": bad}, sdk.ParsedMedia{}, cands, 0.5); err == nil {
			t.Errorf("pickMatch accepted out-of-contract score %v; want error", bad)
		}
	}
}

// swallowingNetworkScorer makes a live HTTP call inside Score and ignores the
// error, returning a passing score anyway — the exact evasion the harness must
// catch: the run cannot depend on the plugin surfacing the transport error.
type swallowingNetworkScorer struct{}

func (swallowingNetworkScorer) Score(c sdk.Candidate, p sdk.ParsedMedia) float64 {
	_, _ = http.DefaultClient.Get("http://provider.invalid/lookup")
	if c.Title == p.Title {
		return 1
	}
	return 0
}

// TestRunMatchCaseRedsOnNetworkAttempt proves the hermeticity guarantee holds
// even against a scorer that swallows the sealed transport's error: the harness
// reds the case because a request was attempted, independent of the score.
func TestRunMatchCaseRedsOnNetworkAttempt(t *testing.T) {
	seal := sealNetwork(t)
	defer seal.restore()
	spy := &testing.T{}
	runMatchCase(spy, seal, titleParserInternal{}, swallowingNetworkScorer{}, MatchCase{
		Name:       "swallows-network-error",
		Path:       "Blade Runner.mkv",
		Candidates: []sdk.Candidate{{ID: "tt-blade", Title: "Blade Runner"}},
		WantID:     "tt-blade",
	})
	if !spy.Failed() {
		t.Fatal("runMatchCase passed a case whose scorer hit the network; want it red on the attempt")
	}
}

// titleParserInternal echoes the filename stem as the title (internal copy so the
// internal test package does not depend on the external test package).
type titleParserInternal struct{}

func (titleParserInternal) Parse(path string, _ sdk.ParseProbe) (sdk.ParsedMedia, bool) {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return sdk.ParsedMedia{Title: path[:i]}, path[:i] != ""
		}
	}
	return sdk.ParsedMedia{Title: path}, path != ""
}

// TestResolveMatchRejectsBadMinScoreFixture: a fixture MinScore outside the
// [0,1] score contract — negative, NaN, or above 1 — is a fixture error
// surfaced loudly, never a floor the pick silently runs against: a negative
// floor would let a zero-scoring candidate "win", and a NaN floor would
// silently unmatch every case.
func TestResolveMatchRejectsBadMinScoreFixture(t *testing.T) {
	badCase := func(floor float64) MatchCase {
		return MatchCase{
			Name:       "bad-floor",
			Path:       "Blade Runner.mkv",
			Candidates: []sdk.Candidate{{ID: "tt-blade", Title: "Blade Runner"}},
			WantID:     "tt-blade",
			MinScore:   floor,
		}
	}
	for _, bad := range []float64{-0.1, math.NaN(), 1.1} {
		if _, _, err := resolveMatch(titleParserInternal{}, constScorer{"tt-blade": 0.9}, badCase(bad)); err == nil {
			t.Errorf("resolveMatch accepted fixture MinScore %v; want a fixture error", bad)
		}
	}

	gotID, _, err := resolveMatch(titleParserInternal{}, constScorer{"tt-blade": 0.9}, badCase(0.5))
	if err != nil {
		t.Fatalf("resolveMatch rejected a legal MinScore: %v", err)
	}
	if gotID != "tt-blade" {
		t.Errorf("resolveMatch = %q, want %q", gotID, "tt-blade")
	}
}

// TestAssertMatchGradesWrongMatch is the harness's own negative check: the three
// disposition classes (correct, wrong match, fail-closed miss, unexpected match)
// must be graded distinctly, so a wrong match reds and a fail-closed miss reds
// while the fail-closed unmatched baseline passes.
func TestAssertMatchGradesWrongMatch(t *testing.T) {
	cases := []struct {
		name       string
		wantID     string
		gotID      string
		wantFailed bool
	}{
		{"correct match", "a", "a", false},
		{"unmatched baseline", "", "", false},
		{"wrong match", "a", "b", true},
		{"fail-closed miss", "a", "", true},
		{"unexpected match", "", "b", true},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			spy := &testing.T{}
			assertMatch(spy, MatchCase{Name: c.name, WantID: c.wantID}, c.gotID)
			if spy.Failed() != c.wantFailed {
				t.Errorf("assertMatch failed=%v, want %v", spy.Failed(), c.wantFailed)
			}
		})
	}
}
