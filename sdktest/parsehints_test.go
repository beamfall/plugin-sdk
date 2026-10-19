package sdktest_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
	"github.com/beamfall/plugin-sdk/sdktest"
)

// hintParser is a conformant parser that carries a provider-ID hint: a path
// tagged `{tmdb-<id>}` yields ProviderIDs{"tmdb": id} from the filename; any
// other path parses title-only, the legacy shape.
type hintParser struct{}

func (hintParser) Parse(path string, _ sdk.ParseProbe) (sdk.ParsedMedia, bool) {
	if path == "" {
		return sdk.ParsedMedia{}, false
	}
	stem := strings.TrimSuffix(path, ".mkv")
	open := strings.Index(stem, "{tmdb-")
	if open < 0 {
		return sdk.ParsedMedia{Title: stem}, true
	}
	id := strings.TrimSuffix(stem[open+len("{tmdb-"):], "}")
	return sdk.ParsedMedia{
		Title:       strings.TrimSpace(stem[:open]),
		ProviderIDs: map[string]string{"tmdb": id},
		HintSource:  sdk.ParseHintSourceFilename,
	}, true
}

// TestRunConformanceGreenWithProviderIDs: the SDK conformance suite is green
// for a parser that sets ProviderIDs, and a vector can assert the hint.
func TestRunConformanceGreenWithProviderIDs(t *testing.T) {
	sdktest.RunConformance(t, hintParser{}, []sdktest.ParseVector{
		{Name: "tagged", Path: "Blade Runner {tmdb-78}.mkv", WantOK: true, Want: sdk.ParsedMedia{
			Title:       "Blade Runner",
			ProviderIDs: map[string]string{"tmdb": "78"},
			HintSource:  sdk.ParseHintSourceFilename,
		}},
		{Name: "untagged", Path: "Heat.mkv", WantOK: true, Want: sdk.ParsedMedia{Title: "Heat"}},
		{Name: "refused", Path: "", WantOK: false},
	})
}

// brokenHintParser carries ProviderIDs without a HintSource — the pairing the
// hint contract rejects — so the conformance runner can be seen to fail on it.
type brokenHintParser struct{}

func (brokenHintParser) Parse(path string, _ sdk.ParseProbe) (sdk.ParsedMedia, bool) {
	if path == "" {
		return sdk.ParsedMedia{}, false
	}
	return sdk.ParsedMedia{Title: path, Year: 1982, ProviderIDs: map[string]string{"tmdb": "78"}}, true
}

// TestRunConformanceRejectsHintContractBreak proves the hint-contract check
// inside RunConformance can fail: a parser that carries ProviderIDs without a
// HintSource reds the vector even though it matches its golden exactly.
func TestRunConformanceRejectsHintContractBreak(t *testing.T) {
	const helperEnv = "BEAMFALL_TEST_HINT_CONTRACT_BREAK"
	if os.Getenv(helperEnv) == "1" {
		sdktest.RunConformance(t, brokenHintParser{}, []sdktest.ParseVector{
			{Name: "sourceless", Path: "a", WantOK: true, Want: sdk.ParsedMedia{Title: "a", Year: 1982, ProviderIDs: map[string]string{"tmdb": "78"}}},
			{Name: "refused", Path: "", WantOK: false},
		})
		return
	}
	output := rerunExpectingFailure(t, "^TestRunConformanceRejectsHintContractBreak$", helperEnv)
	if !strings.Contains(output, "hint contract: ProviderIDs carried with HintSource \"\"") {
		t.Fatalf("failure output did not name the hint contract:\n%s", output)
	}
}

// rerunExpectingFailure re-executes the named test in a child process with
// env set, and returns its combined output; it fails the caller when the child
// exits green.
func rerunExpectingFailure(t *testing.T, pattern, env string) string {
	t.Helper()
	args := []string{"-test.run=" + pattern}
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-test.gocoverdir=") {
			args = append(args, arg)
		}
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0], args...)
	cmd.Env = append(os.Environ(), env+"=1")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("%s passed in the child process; want a failure. output:\n%s", pattern, output)
	}
	return string(output)
}

// TestRunMatchCorpusHintDrivenMatch: a fixture asserting "matched via hint,
// zero search calls" passes when the hinted ID names a frozen candidate, and a
// hint naming no candidate falls through to the scored pick.
func TestRunMatchCorpusHintDrivenMatch(t *testing.T) {
	candidates := []sdk.Candidate{
		{ID: "78", Title: "Blade Runner"},
		{ID: "949", Title: "Heat"},
	}
	sdktest.RunMatchCorpus(t, hintParser{}, titleScorer{}, []sdktest.MatchFamily{{
		Name: "provider-id-hints",
		Cases: []sdktest.MatchCase{
			// The title is deliberately wrong: only the hint can resolve it.
			{Name: "tmdb-tag-short-circuit", Path: "Wrong Title {tmdb-78}.mkv", Candidates: candidates, WantID: "78", HintProvider: "tmdb"},
			{Name: "hint-absent-scored-pick", Path: "Heat.mkv", Candidates: candidates, WantID: "949"},
			{Name: "hint-unknown-falls-through", Path: "Heat {tmdb-1}.mkv", Candidates: candidates, WantID: "949"},
		},
	}})
}

// TestRunMatchCorpusHintCaseFailsWithoutHintPath proves the HintProvider
// assertion can fail: a case claiming a hint-driven match reds when the parser
// carried no hint and the scorer had to resolve it.
func TestRunMatchCorpusHintCaseFailsWithoutHintPath(t *testing.T) {
	const helperEnv = "BEAMFALL_TEST_HINT_CASE_NO_HINT"
	if os.Getenv(helperEnv) == "1" {
		candidates := []sdk.Candidate{{ID: "949", Title: "Heat"}}
		sdktest.RunMatchCorpus(t, hintParser{}, titleScorer{}, []sdktest.MatchFamily{{
			Name:  "provider-id-hints",
			Cases: []sdktest.MatchCase{{Name: "claims-hint", Path: "Heat.mkv", Candidates: candidates, WantID: "949", HintProvider: "tmdb"}},
		}})
		return
	}
	output := rerunExpectingFailure(t, "^TestRunMatchCorpusHintCaseFailsWithoutHintPath$", helperEnv)
	for _, want := range []string{"the hint resolved no candidate", "consulted the scorer 1 time(s); want zero search calls"} {
		if !strings.Contains(output, want) {
			t.Errorf("failure output lacks %q:\n%s", want, output)
		}
	}
}
