// Package sdktest is the cross-plugin conformance harness.
//
// The SDK ships types and interfaces only; every first-party plugin is its own
// module with its own CI. These runners let each plugin pin golden vectors and
// bundle invariants against the SDK contract, so a scan or locale contract
// break reds that plugin's CI and its tagged release (semver-gated)
// rather than drifting silently.
//
// Four entry points, composed by each plugin's conformance_test.go:
//
//   - RunConformance        — golden filename-parse vectors (FilenameParser).
//   - RunContextConformance — bounded scan-context filename-parse vectors
//     (ContextFilenameParser), including unknown/future-value fallback.
//   - RunScoreConformance   — golden match-score vectors (Scorer).
//   - RunBundleConformance — declarative Bundle + locale invariants; every
//     plugin has a Bundle, so this is the one runner all plugins call.
package sdktest

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
	"github.com/beamfall/plugin-sdk/compatibility"
)

var recognizedLibraryDefaults = map[string]bool{
	"Restricted": true,
}

// ParseVector is one golden filename-parse case: a path + probe in, the
// expected ParsedMedia and ok out.
type ParseVector struct {
	Name   string
	Path   string
	Probe  sdk.ParseProbe
	Want   sdk.ParsedMedia
	WantOK bool
}

// ContextParseVector is one golden contextual filename-parse case. It is
// separate from ParseVector so adding the optional context seam does not alter
// existing plugin fixture literals.
type ContextParseVector struct {
	Name    string
	Path    string
	Probe   sdk.ParseProbe
	Context sdk.ParseContext
	Want    sdk.ParsedMedia
	WantOK  bool
}

// RunConformance asserts a FilenameParser matches its golden vectors and obeys
// the scan contract: each Parse is deterministic (same input → same output).
//
// The vector table must also clear a coverage floor (2026-07-05 test-quality
// review): at least one WantOK:false vector (a hostile input the parser
// correctly refuses) and at least one WantOK:true vector whose Want sets two
// or more ParsedMedia fields (a multi-field parse, not just a bare title). A
// happy-path-only or single-field-only table passes every per-vector
// assertion below while leaving the parser's failure and multi-field paths
// completely unexercised, so the floor fails fast here instead.
func RunConformance(t *testing.T, p sdk.FilenameParser, vectors []ParseVector) {
	t.Helper()
	RequireAllowedImports(t)
	if len(vectors) == 0 {
		t.Fatal("sdktest.RunConformance: no parse vectors supplied")
	}
	requireParseVectorFloor(t, vectors)
	for _, v := range vectors {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			got, ok := p.Parse(v.Path, v.Probe)
			if ok != v.WantOK {
				t.Fatalf("Parse(%q) ok = %v, want %v", v.Path, ok, v.WantOK)
			}
			if !reflect.DeepEqual(got, v.Want) {
				t.Errorf("Parse(%q) =\n  %+v\nwant\n  %+v", v.Path, got, v.Want)
			}
			got2, ok2 := p.Parse(v.Path, v.Probe)
			if !reflect.DeepEqual(got2, got) || ok2 != ok {
				t.Errorf("Parse(%q) not deterministic: %+v/%v then %+v/%v",
					v.Path, got, ok, got2, ok2)
			}
			if err := sdk.ValidateParseHints(got); err != nil {
				t.Errorf("Parse(%q) hint contract: %v", v.Path, err)
			}
		})
	}
}

// RunContextConformance asserts a ContextFilenameParser matches its golden
// vectors, is deterministic, and preserves its legacy Parse result when the
// context is absent or unrecognized. A vector table must cover the bounded
// unknown/single/multiple directory-media-file shapes in addition to the same
// refusal and multi-field coverage floor as RunConformance.
func RunContextConformance(t *testing.T, p sdk.ContextFilenameParser, vectors []ContextParseVector) {
	t.Helper()
	RequireAllowedImports(t)
	if len(vectors) == 0 {
		t.Fatal("sdktest.RunContextConformance: no parse vectors supplied")
	}
	for _, reason := range contextParseVectorFloorViolations(vectors) {
		t.Fatal(reason)
	}
	for _, v := range vectors {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			got, ok := p.ParseWithContext(v.Path, v.Probe, v.Context)
			if ok != v.WantOK {
				t.Fatalf("ParseWithContext(%q) ok = %v, want %v", v.Path, ok, v.WantOK)
			}
			if !reflect.DeepEqual(got, v.Want) {
				t.Errorf("ParseWithContext(%q) =\n  %+v\nwant\n  %+v", v.Path, got, v.Want)
			}
			got2, ok2 := p.ParseWithContext(v.Path, v.Probe, v.Context)
			if !reflect.DeepEqual(got2, got) || ok2 != ok {
				t.Errorf("ParseWithContext(%q) not deterministic: %+v/%v then %+v/%v",
					v.Path, got, ok, got2, ok2)
			}
			if err := sdk.ValidateParseHints(got); err != nil {
				t.Errorf("ParseWithContext(%q) hint contract: %v", v.Path, err)
			}
			count := v.Context.DirectoryMediaFileCount
			if count != sdk.DirectoryMediaFileCountSingle && count != sdk.DirectoryMediaFileCountMultiple {
				legacy, legacyOK := p.Parse(v.Path, v.Probe)
				if !reflect.DeepEqual(got, legacy) || ok != legacyOK {
					t.Errorf("ParseWithContext(%q) with absent/unrecognized context = %+v/%v; legacy Parse = %+v/%v",
						v.Path, got, ok, legacy, legacyOK)
				}
			}
		})
	}
}

func contextParseVectorFloorViolations(vectors []ContextParseVector) []string {
	legacy := make([]ParseVector, 0, len(vectors))
	seen := map[sdk.DirectoryMediaFileCount]bool{}
	for _, v := range vectors {
		legacy = append(legacy, ParseVector{
			Name: v.Name, Path: v.Path, Probe: v.Probe, Want: v.Want, WantOK: v.WantOK,
		})
		seen[v.Context.DirectoryMediaFileCount] = true
	}
	violations := parseVectorFloorViolations(legacy)
	for _, required := range []struct {
		count sdk.DirectoryMediaFileCount
		name  string
	}{
		{sdk.DirectoryMediaFileCountUnknown, "unknown"},
		{sdk.DirectoryMediaFileCountSingle, "single"},
		{sdk.DirectoryMediaFileCountMultiple, "multiple"},
	} {
		if !seen[required.count] {
			violations = append(violations, "sdktest.RunContextConformance: vector table has no "+required.name+" directory-media-file context")
		}
	}
	return violations
}

// requireParseVectorFloor fails the test if vectors lacks either required
// dimension: a WantOK:false vector, or a WantOK:true vector whose Want sets
// two or more non-zero ParsedMedia fields.
func requireParseVectorFloor(t *testing.T, vectors []ParseVector) {
	t.Helper()
	for _, reason := range parseVectorFloorViolations(vectors) {
		t.Fatal(reason)
	}
}

// parseVectorFloorViolations is the pure predicate behind requireParseVectorFloor:
// it reports which coverage-floor dimensions vectors fails to clear, with no
// *testing.T dependency, so the floor logic itself is directly unit-testable.
func parseVectorFloorViolations(vectors []ParseVector) []string {
	var hasFailureVector, hasMultiFieldVector bool
	for _, v := range vectors {
		if !v.WantOK {
			hasFailureVector = true
			continue
		}
		if nonZeroFieldCount(v.Want) >= 2 {
			hasMultiFieldVector = true
		}
	}
	var violations []string
	if !hasFailureVector {
		violations = append(violations, "sdktest.RunConformance: vector table has no WantOK:false vector (a hostile-input/failure case); the parser's refusal path is untested")
	}
	if !hasMultiFieldVector {
		violations = append(violations, "sdktest.RunConformance: vector table has no WantOK:true vector with 2+ non-zero ParsedMedia fields; only single-field parses are tested")
	}
	return violations
}

// nonZeroFieldCount counts how many of ParsedMedia's fields hold a non-zero
// value — the coverage-floor's definition of "multi-field".
func nonZeroFieldCount(m sdk.ParsedMedia) int {
	v := reflect.ValueOf(m)
	n := 0
	for i := 0; i < v.NumField(); i++ {
		if !v.Field(i).IsZero() {
			n++
		}
	}
	return n
}

// Scorer is the match-scoring seam: higher = a stronger match.
type Scorer = sdk.MatchScorer

// ScoreVector is one golden match-score case. Tol is the absolute tolerance for
// the float comparison; zero defaults to 1e-9.
type ScoreVector struct {
	Name         string
	CandTitle    string
	CandYear     int
	CandSequence int
	CandIndex    int
	// CandSequencePresent and CandIndexPresent are the vector-table counterparts
	// of Candidate.SequencePresent/IndexPresent (scan.go): set true to assert a
	// scorer's behavior on a genuine ordinal 0, distinct from the zero-value
	// "not carried" a vector leaves false.
	CandSequencePresent bool
	CandIndexPresent    bool
	CandPopularity      float64
	Parsed              sdk.ParsedMedia
	Want                float64
	Tol                 float64
}

// RunScoreConformance asserts a Scorer matches its golden vectors, stays within
// the documented [0,1] confidence range, and is deterministic.
func RunScoreConformance(t *testing.T, s Scorer, vectors []ScoreVector) {
	t.Helper()
	RequireAllowedImports(t)
	if len(vectors) == 0 {
		t.Fatal("sdktest.RunScoreConformance: no score vectors supplied")
	}
	for _, v := range vectors {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			tol := v.Tol
			if tol == 0 {
				tol = 1e-9
			}
			got := s.Score(scoreCandidate(v), v.Parsed)
			outOfRange, mismatch := validateScoreResult(got, v.Want, tol)
			if outOfRange {
				t.Errorf("Score(%q, %d, %+v) = %v, out of contract range [0,1]",
					v.CandTitle, v.CandYear, v.Parsed, got)
			}
			if mismatch {
				t.Errorf("Score(%q, %d, %+v) = %v, want %v (±%v)",
					v.CandTitle, v.CandYear, v.Parsed, got, v.Want, tol)
			}
			if got2 := s.Score(scoreCandidate(v), v.Parsed); got2 != got && !(math.IsNaN(got2) && math.IsNaN(got)) {
				t.Errorf("Score(%q, ...) not deterministic: %v then %v", v.CandTitle, got, got2)
			}
		})
	}
}

func validateScoreResult(got, want, tol float64) (outOfRange, mismatch bool) {
	return math.IsNaN(got) || got < 0 || got > 1, math.Abs(got-want) > tol
}

func scoreCandidate(v ScoreVector) sdk.Candidate {
	return sdk.Candidate{
		Title:           v.CandTitle,
		Year:            v.CandYear,
		Sequence:        v.CandSequence,
		Index:           v.CandIndex,
		SequencePresent: v.CandSequencePresent,
		IndexPresent:    v.CandIndexPresent,
		Popularity:      v.CandPopularity,
	}
}

// RunBundleConformance asserts a plugin's declarative Bundle obeys the SDK
// contract every plugin shares.
func RunBundleConformance(t *testing.T, b sdk.Bundle) {
	t.Helper()
	RequireBundleValid(t, b)
	RequirePermissionPolicyNegativeCorpus(t, b)
	RequireBundleCompatibility(t, b)
	RequireBundleDefaultsRecognized(t, b)
	RequireAllowedImports(t)
}

// RequireBundleValid fails the test if b violates sdk.ValidateBundle.
func RequireBundleValid(t testing.TB, b sdk.Bundle) {
	t.Helper()
	if err := sdk.ValidateBundle(b); err != nil {
		t.Fatal(err)
	}
}

// RequirePermissionPolicyNegativeCorpus proves the SDK rejects the shared
// malicious permission declarations: unknown capabilities, replaced host
// labels or warnings, and cleared minimum high-risk flags.
func RequirePermissionPolicyNegativeCorpus(t testing.TB, b sdk.Bundle) {
	t.Helper()
	if err := permissionPolicyNegativeCorpusError(b); err != nil {
		t.Fatal(err)
	}
}

func permissionPolicyNegativeCorpusError(b sdk.Bundle) error {
	type negativeCase struct {
		name       string
		permission sdk.PermissionSpec
	}
	cases := []negativeCase{{
		name: "unknown capability",
		permission: sdk.PermissionSpec{
			Kind:        "not-a-permission",
			Label:       sdk.L("Unknown"),
			Description: sdk.L("Unknown capability."),
			HighRisk:    true,
		},
	}}
	for _, kind := range sdk.PermissionKinds() {
		policy, ok := sdk.PermissionPolicyFor(kind)
		if !ok {
			return fmt.Errorf("sdktest: permission %q has no host-owned policy", kind)
		}
		canonical := sdk.PermissionSpec{
			Kind:        kind,
			Label:       policy.Label,
			Description: policy.Description,
			HighRisk:    policy.HighRisk,
		}
		relabelled := canonical
		relabelled.Label = sdk.L("Routine maintenance")
		cases = append(cases, negativeCase{name: kind + " relabelled", permission: relabelled})

		warningReplaced := canonical
		warningReplaced.Description = sdk.L("This capability is harmless.")
		cases = append(cases, negativeCase{name: kind + " warning replaced", permission: warningReplaced})

		if policy.HighRisk {
			riskDowngraded := canonical
			riskDowngraded.HighRisk = false
			cases = append(cases, negativeCase{name: kind + " risk downgraded", permission: riskDowngraded})
		}
	}
	for _, testCase := range cases {
		candidate := b
		candidate.Permissions = []sdk.PermissionSpec{testCase.permission}
		if err := sdk.ValidateBundle(candidate); err == nil {
			return fmt.Errorf("sdktest: sdk.ValidateBundle accepted malicious permission case %q", testCase.name)
		}
	}
	return nil
}

// RequireBundleCompatibility fails unless the producer-owned matrix contains
// exact conformance evidence for a canonical first-party bundle against the
// current SDK. Other bundles must declare a range compatible with the current
// SDK but do not require a first-party matrix row.
func RequireBundleCompatibility(t testing.TB, b sdk.Bundle) {
	t.Helper()
	if err := bundleCompatibilityError(b); err != nil {
		t.Fatal(err)
	}
}

func bundleCompatibilityError(b sdk.Bundle) error {
	for _, entry := range compatibility.Entries() {
		if entry.ConsumerID == b.ID {
			return compatibility.Verify(b, sdk.Version)
		}
	}
	if !sdk.CompatibleWithSDK(b, sdk.Version) {
		return fmt.Errorf("plugin %s: SDK range [%s, %s] does not support SDK %s", b.ID, b.SDKFloor, b.SDKCeiling, sdk.Version)
	}
	return nil
}

// RequireBundlesValid fails the test if the bundles violate the cross-plugin
// contract (sdk.ValidateBundles) — most notably two plugins declaring the same
// Entity Kind name, which the store's global kind namespace forbids. Use this
// when a test activates more than one plugin's bundle together.
func RequireBundlesValid(t testing.TB, bundles ...sdk.Bundle) {
	t.Helper()
	if err := sdk.ValidateBundles(bundles); err != nil {
		t.Fatal(err)
	}
}

// RequireBundlesReject fails the test if sdk.ValidateBundles accepts the given
// bundles — the cross-plugin reject-path assertion (e.g. a colliding kind
// name).
func RequireBundlesReject(t testing.TB, bundles ...sdk.Bundle) {
	t.Helper()
	if err := sdk.ValidateBundles(bundles); err == nil {
		t.Fatal("sdk.ValidateBundles accepted bundles that violate the cross-plugin contract")
	}
}

// RequireBundleDefaultsRecognized fails when the SDK adds a LibraryDefaults
// field before sdktest names it as governed conformance vocabulary.
func RequireBundleDefaultsRecognized(t testing.TB, b sdk.Bundle) {
	t.Helper()
	defaults := reflect.TypeOf(b.LibraryDefaults)
	seen := map[string]bool{}
	for i := 0; i < defaults.NumField(); i++ {
		field := defaults.Field(i)
		if field.PkgPath != "" {
			continue
		}
		seen[field.Name] = true
		if !recognizedLibraryDefaults[field.Name] {
			t.Fatalf("sdktest: LibraryDefaults.%s is not recognized default vocabulary", field.Name)
		}
	}
	for name := range recognizedLibraryDefaults {
		if !seen[name] {
			t.Fatalf("sdktest: recognized LibraryDefaults.%s no longer exists in the SDK", name)
		}
	}
}
