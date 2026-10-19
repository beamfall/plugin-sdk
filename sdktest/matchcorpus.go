package sdktest

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
)

// corpusRunMu serializes RunMatchCorpus calls: the hermeticity guard swaps the
// process-global http.DefaultTransport, so two corpus runs must not overlap.
var corpusRunMu sync.Mutex

// DefaultMatchThreshold is the fail-closed acceptance floor a corpus case uses
// when it names no MinScore of its own: a candidate is only accepted as the
// match if its score is strictly above this floor. Below it, the file routes to
// the unmatched queue — the fail-closed baseline. It is a
// deliberately low, single-shared constant so a family exercises the parser and
// scorer, not a per-plugin tuning knob.
const DefaultMatchThreshold = 0.5

// MatchCase is one corpus fixture: an unidentified file (path plus the embedded
// tags scan would probe) matched against a frozen candidate set. WantID is the
// ID of the candidate the parser+scorer must resolve to; an empty WantID means
// the case must route to the unmatched queue (no candidate clears the floor).
//
// The regression the corpus guards is a *wrong match*: resolving
// to a candidate whose ID differs from WantID. A WantID case that comes back
// unmatched is a fail-closed miss, still a regression against a case asserted to
// match; but a WantID=="" case that stays unmatched is the intended baseline and
// passes. Fixtures are hermetic: Candidates is the whole world the case sees —
// no live provider call is made, and one attempted mid-run reds the case.
type MatchCase struct {
	Name       string
	Path       string
	Probe      sdk.ParseProbe
	Candidates []sdk.Candidate
	WantID     string
	// MinScore overrides DefaultMatchThreshold for this case; zero uses the
	// default. The winning candidate's score must be strictly greater.
	MinScore float64
	// HintProvider, when set, asserts the case resolves by identification hint:
	// the parser must carry ProviderIDs[HintProvider],
	// that ID must name a candidate in the frozen set, and the pick must come
	// from the hint alone — the scorer is never consulted and no outbound
	// request is attempted ("matched via hint, zero search calls"). WantID
	// still names the expected candidate. Empty keeps the scored pick.
	HintProvider string
}

// MatchFamily is a named group of corpus cases — one hostile-input class such as
// scene naming, anime absolute numbering, or non-Latin titles. The
// plugin owns the families; the SDK owns how they run.
type MatchFamily struct {
	Name  string
	Cases []MatchCase
}

// RunMatchCorpus drives a plugin's parser+scorer through its matching-regression
// corpus and is the corpus sibling of RunConformance /
// RunScoreConformance. For each case it parses the path/probe, scores every
// candidate in the frozen set, applies the fail-closed pick (highest score
// strictly above the case floor; an ambiguous top band routes to unmatched
// unless the top scores are EXACTLY tied and a unique popularity signal breaks
// that tie — popularity never promotes a lower score), and asserts the resolved
// ID equals WantID — a wrong match is the regression, an unmatched WantID==""
// case is the fail-closed baseline.
//
// The run is hermetic: http.DefaultTransport is swapped for a transport that
// both errors on and *records* every request for the duration of the corpus. A
// parser or scorer that reaches a live provider mid-run fails the test even if
// it swallows the transport error, because the harness reds any run in which an
// outbound request was attempted — a network round-trip CI cannot reproduce is
// a hermeticity break, not something the plugin's own error handling can hide.
// The seal covers the default transport only: code that constructs its own
// http.Client/Transport or dials the network directly bypasses it, which is why
// RequireAllowedImports (run first) bans exactly those constructions.
//
// Because the guard mutates the process-global http.DefaultTransport, calls to
// RunMatchCorpus are serialized with each other and must not run inside a
// parallel subtest (do not call t.Parallel() around it).
func RunMatchCorpus(t *testing.T, p sdk.FilenameParser, s sdk.MatchScorer, families []MatchFamily) {
	t.Helper()
	RequireAllowedImports(t)
	if len(families) == 0 {
		t.Fatal("sdktest.RunMatchCorpus: no fixture families supplied")
	}
	corpusRunMu.Lock()
	defer corpusRunMu.Unlock()
	seal := sealNetwork(t)
	defer seal.restore()
	for _, fam := range families {
		fam := fam
		if len(fam.Cases) == 0 {
			t.Fatalf("sdktest.RunMatchCorpus: family %q has no cases", fam.Name)
		}
		t.Run(fam.Name, func(t *testing.T) {
			for _, c := range fam.Cases {
				c := c
				t.Run(c.Name, func(t *testing.T) {
					runMatchCase(t, seal, p, s, c)
				})
			}
		})
	}
}

// runMatchCase resolves one case, asserts it against WantID, then reds the case
// if the parser or scorer attempted any outbound request while resolving it —
// the hermeticity guarantee, enforced by the harness rather than trusting the
// plugin to surface the sealed transport's error.
func runMatchCase(t *testing.T, seal *networkSeal, p sdk.FilenameParser, s sdk.MatchScorer, c MatchCase) {
	t.Helper()
	before := seal.attempts()
	counted := &countingScorer{inner: s}
	gotID, viaHint, err := resolveMatch(p, counted, c)
	if err != nil {
		t.Errorf("case %q: %v", c.Name, err)
		return
	}
	assertMatch(t, c, gotID)
	assertHintPath(t, c, viaHint, counted.calls)
	if n := seal.attempts() - before; n > 0 {
		t.Errorf("case %q attempted %d outbound network request(s) during the run; corpus fixtures must be hermetic", c.Name, n)
	}
}

// resolveMatch runs the parse+score+pick pipeline for one case. A MinScore
// outside [0,1] (or NaN) is a fixture error surfaced loudly: a negative floor
// would let a zero-scoring candidate "win", and a NaN floor would silently
// unmatch every case.
//
// A parse that carries ProviderIDs (a hint) is checked against the hint
// contract, then tried as a lookup-by-ID short circuit: a hinted ID that names
// a candidate in the frozen set wins outright, before any scoring (Core's
// Parse Hint path). A hint naming no candidate falls through to the
// scored pick. The bool result reports whether the hint path resolved the case.
func resolveMatch(p sdk.FilenameParser, s sdk.MatchScorer, c MatchCase) (string, bool, error) {
	parsed, ok := p.Parse(c.Path, c.Probe)
	if !ok {
		return "", false, nil
	}
	if err := sdk.ValidateParseHints(parsed); err != nil {
		return "", false, fmt.Errorf("parser hint contract: %w", err)
	}
	if id := hintedCandidate(parsed, c.Candidates); id != "" {
		return id, true, nil
	}
	floor := c.MinScore
	if floor == 0 {
		floor = DefaultMatchThreshold
	}
	if math.IsNaN(floor) || floor < 0 || floor > 1 {
		return "", false, fmt.Errorf("fixture MinScore %v is outside the [0,1] score contract", c.MinScore)
	}
	id, err := pickMatch(s, parsed, c.Candidates, floor)
	return id, false, err
}

// hintedCandidate returns the ID of the candidate a parsed hint names, or ""
// when the parse carries no hint or no candidate matches a hinted ID. Providers
// are visited in sorted order so a multi-provider hint resolves deterministically.
func hintedCandidate(parsed sdk.ParsedMedia, candidates []sdk.Candidate) string {
	providers := make([]string, 0, len(parsed.ProviderIDs))
	for provider := range parsed.ProviderIDs {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	for _, provider := range providers {
		hinted := parsed.ProviderIDs[provider]
		for _, cand := range candidates {
			if cand.ID == hinted {
				return cand.ID
			}
		}
	}
	return ""
}

// assertHintPath grades a HintProvider case: the hint path must have resolved
// it and the scorer must not have been consulted. Cases without HintProvider
// are unconstrained here.
func assertHintPath(t *testing.T, c MatchCase, viaHint bool, scorerCalls int) {
	t.Helper()
	if c.HintProvider == "" {
		return
	}
	if !viaHint {
		t.Errorf("case %q: expected a %q hint-driven match, but the hint resolved no candidate", c.Name, c.HintProvider)
	}
	if scorerCalls > 0 {
		t.Errorf("case %q: hint-driven match consulted the scorer %d time(s); want zero search calls", c.Name, scorerCalls)
	}
}

// countingScorer wraps a scorer and counts Score invocations so a hint case
// can prove the search/score path was never taken.
type countingScorer struct {
	inner sdk.MatchScorer
	calls int
}

func (c *countingScorer) Score(candidate sdk.Candidate, parsed sdk.ParsedMedia) float64 {
	c.calls++
	return c.inner.Score(candidate, parsed)
}

const ambiguityMargin = 0.05

// pickMatch applies the fail-closed identification the corpus grades against:
// the single candidate scoring strictly above floor wins; nothing above floor,
// an ambiguous top band, or an exact-score tie without a unique popularity
// winner routes to unmatched (empty ID). Popularity only breaks true equal-score
// ties at the top; it never promotes a lower score. A score outside the
// documented [0,1] contract range (or NaN) is a scorer violation the corpus
// surfaces rather than silently picking on, matching RunScoreConformance.
func pickMatch(s sdk.MatchScorer, parsed sdk.ParsedMedia, candidates []sdk.Candidate, floor float64) (string, error) {
	type scoredCandidate struct {
		candidate sdk.Candidate
		score     float64
	}
	var scored []scoredCandidate
	best := scoredCandidate{score: floor}
	for _, cand := range candidates {
		score := s.Score(cand, parsed)
		if math.IsNaN(score) || score < 0 || score > 1 {
			return "", fmt.Errorf("scorer returned %v for candidate %q, out of contract range [0,1]", score, cand.ID)
		}
		entry := scoredCandidate{candidate: cand, score: score}
		scored = append(scored, entry)
		if score > best.score {
			best = entry
		}
	}
	if best.candidate.ID == "" {
		return "", nil
	}
	band := scored[:0]
	for _, entry := range scored {
		if best.score-entry.score < ambiguityMargin {
			band = append(band, entry)
		}
	}
	if len(band) < 2 {
		return best.candidate.ID, nil
	}
	exactTop := band[:0]
	for _, entry := range band {
		if entry.score == best.score {
			exactTop = append(exactTop, entry)
		}
	}
	if len(exactTop) < 2 {
		return "", nil
	}
	popular := exactTop[0]
	popularityTied := false
	popularityEligible := popular.score > floor && popular.candidate.Popularity > 0
	for _, entry := range exactTop[1:] {
		eligible := entry.score > floor && entry.candidate.Popularity > 0
		switch {
		case eligible && (!popularityEligible || entry.candidate.Popularity > popular.candidate.Popularity):
			popular, popularityTied, popularityEligible = entry, false, true
		case eligible && popularityEligible && entry.candidate.Popularity == popular.candidate.Popularity:
			popularityTied = true
		}
	}
	if !popularityEligible || popularityTied {
		return "", nil
	}
	return popular.candidate.ID, nil
}

// assertMatch grades a resolved ID against the case, distinguishing a wrong
// match (the regression) from a fail-closed miss.
func assertMatch(t *testing.T, c MatchCase, gotID string) {
	t.Helper()
	if gotID == c.WantID {
		return
	}
	if c.WantID == "" {
		t.Errorf("case %q matched %q; want unmatched (wrong-match regression)", c.Name, gotID)
		return
	}
	if gotID == "" {
		t.Errorf("case %q routed to unmatched; want match %q (fail-closed miss)", c.Name, c.WantID)
		return
	}
	t.Errorf("case %q matched %q; want %q (wrong-match regression)", c.Name, gotID, c.WantID)
}

// networkSeal is an installed hermetic network guard: it counts every outbound
// round-trip attempted while active and errors each one, so the harness can red
// a case that reached the network even if the plugin swallowed the error.
type networkSeal struct {
	prev      http.RoundTripper
	attempted atomic.Int64
}

// sealNetwork installs the guard as http.DefaultTransport and returns it. Call
// restore() to put the previous transport back.
func sealNetwork(t *testing.T) *networkSeal {
	t.Helper()
	seal := &networkSeal{prev: http.DefaultTransport}
	http.DefaultTransport = seal
	return seal
}

// RoundTrip records the attempt and refuses the request.
func (s *networkSeal) RoundTrip(*http.Request) (*http.Response, error) {
	s.attempted.Add(1)
	return nil, errors.New("sdktest: live network calls are not allowed inside a corpus run (hermetic fixtures)")
}

// attempts is the number of outbound requests seen since the seal was installed.
func (s *networkSeal) attempts() int64 { return s.attempted.Load() }

// restore reinstates the transport the seal replaced.
func (s *networkSeal) restore() { http.DefaultTransport = s.prev }
