package sdktest_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	sdk "github.com/beamfall/plugin-sdk"
	"github.com/beamfall/plugin-sdk/catalogindex"
	"github.com/beamfall/plugin-sdk/sdktest"
)

// The legal evidence below is a TEST FIXTURE: keys come from public seeds,
// the policy is not a licence decision and the reviewer is not a person.

var (
	fixturePolicyIssued = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	fixtureReviewedAt   = time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	fixtureEvaluatedAt  = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
)

func fixtureKey(label string) ed25519.PrivateKey {
	seed := sha256.Sum256([]byte("beamfall sdktest TEST FIXTURE key: " + label))
	return ed25519.NewKeyFromSeed(seed[:])
}

func hexDigest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func fixtureSignedPolicy(t testing.TB) catalogindex.SignedPolicy {
	t.Helper()
	policy, err := catalogindex.SignPolicy(catalogindex.Policy{
		PolicyID:              "TEST-FIXTURE-POLICY-not-for-production",
		IssuedAt:              fixturePolicyIssued,
		Licenses:              []catalogindex.PolicyLicense{{Expression: "LicenseRef-Test-Fixture", SourceOfferRequired: true}},
		Reviewers:             []catalogindex.PolicyReviewer{{KeyID: "reviewer-fixture", Identity: "Test fixture reviewer (not a real person)", PublicKey: base64.RawURLEncoding.EncodeToString(fixtureKey("reviewer").Public().(ed25519.PublicKey))}},
		AttestationMaxAgeDays: 365,
	}, "registry-fixture", fixtureKey("registry"))
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

// legalEvidence returns attested TEST FIXTURE legal metadata for bundle.
func legalEvidence(t testing.TB, bundle sdk.Bundle) *sdktest.LegalEvidence {
	t.Helper()
	tag := string(bundle.ID) + " " + bundle.Version
	data := catalogindex.EntryBytes{
		Artifact:   []byte("TEST FIXTURE artifact " + tag),
		Source:     []byte("TEST FIXTURE source " + tag),
		Provenance: []byte("TEST FIXTURE provenance " + tag),
		Notices:    map[string][]byte{"LICENSE": []byte("TEST FIXTURE notice " + tag)},
	}
	policy := fixtureSignedPolicy(t)
	digest, err := catalogindex.PolicyDigest(policy.Policy)
	if err != nil {
		t.Fatal(err)
	}
	entry := catalogindex.Entry{
		PluginID:       string(bundle.ID),
		Version:        bundle.Version,
		SDKFloor:       bundle.SDKFloor,
		SDKCeiling:     bundle.SDKCeiling,
		ArtifactSHA256: hexDigest(data.Artifact),
		DeliveryRef:    "delivery-" + string(bundle.ID),
		Legal: catalogindex.Legal{
			LicenseExpression: "LicenseRef-Test-Fixture",
			Notices:           []catalogindex.Notice{{Path: "LICENSE", SHA256: hexDigest(data.Notices["LICENSE"])}},
			SourceRepository:  "https://example.invalid/fixture/" + string(bundle.ID),
			SourceCommit:      hexDigest([]byte(tag))[:40],
			SourceSHA256:      hexDigest(data.Source),
			SourceOffer:       "https://example.invalid/fixture/source-offer",
			ProvenanceSHA256:  hexDigest(data.Provenance),
			RightsAttestation: catalogindex.RightsAttestation{
				ReviewerKeyID:    "reviewer-fixture",
				ReviewerIdentity: "Test fixture reviewer (not a real person)",
				ReviewedAt:       fixtureReviewedAt,
				Disposition:      catalogindex.DispositionApproved,
			},
		},
	}
	att, err := catalogindex.SignRightsAttestation(entry, digest, fixtureKey("reviewer"))
	if err != nil {
		t.Fatal(err)
	}
	entry.Legal.RightsAttestation = att
	return &sdktest.LegalEvidence{
		Entry:  entry,
		Policy: policy,
		Roots:  map[string]ed25519.PublicKey{"registry-fixture": fixtureKey("registry").Public().(ed25519.PublicKey)},
		At:     fixtureEvaluatedAt,
		Bytes:  data,
	}
}

// admissibleBundle is the smallest Bundle that clears every
// mechanical admission gate — the fixture every rejection case in this file
// starts from and breaks exactly one way.
func admissibleBundle() sdk.Bundle {
	return sdk.Bundle{
		ID:           "independent",
		Version:      "0.1.0",
		SDKFloor:     sdk.Version,
		SDKCeiling:   sdk.Version,
		DisplayName:  sdk.LocaleString{"en": "Market Fixture"},
		MediaLabel:   sdk.LocaleString{"en": "Item"},
		MediaClasses: []string{"video"},
		Layouts:      []sdk.LayoutSpec{{ID: "browse", Role: "browse"}},
	}
}

func admissibleListing(t testing.TB) sdktest.Listing {
	bundle := admissibleBundle()
	return sdktest.Listing{Bundle: bundle, ConformancePassed: true, Legal: legalEvidence(t, bundle)}
}

// TestAdmitAcceptsConformantListing is the positive control: every case below
// changes exactly one thing away from this passing baseline.
func TestAdmitAcceptsConformantListing(t *testing.T) {
	result := sdktest.Admit(admissibleListing(t), sdk.Version)
	if !result.Admitted {
		t.Fatalf("Admit(conformant listing) rejected: gate=%s reason=%s", result.FailedGate, result.Reason)
	}
	want := []sdktest.GateClass{sdktest.GateManifest, sdktest.GateLifecycle, sdktest.GateCompatRange, sdktest.GateLegal, sdktest.GateConformance}
	if len(result.GatesPassed) != len(want) {
		t.Fatalf("GatesPassed = %v, want all five gates recorded", result.GatesPassed)
	}
	for i := range want {
		if result.GatesPassed[i] != want[i] {
			t.Fatalf("GatesPassed = %v, want %v", result.GatesPassed, want)
		}
	}
}

// TestAdmitRejectsPerGateClass proves the mechanical admission pipeline
// rejects a fixture plugin failing each of the five gate classes:
// manifest, lifecycle, compat range, legal, and conformance.
func TestAdmitRejectsPerGateClass(t *testing.T) {
	cases := []struct {
		name     string
		listing  func() sdktest.Listing
		sdkVer   string
		wantGate sdktest.GateClass
	}{
		{
			name: "manifest: invalid bundle (missing display name)",
			listing: func() sdktest.Listing {
				l := admissibleListing(t)
				l.Bundle.DisplayName = sdk.LocaleString{}
				return l
			},
			sdkVer:   sdk.Version,
			wantGate: sdktest.GateManifest,
		},
		{
			name: "lifecycle: Entity Kind collides with an already-admitted plugin",
			listing: func() sdktest.Listing {
				colliding := admissibleBundle()
				colliding.ID = "market"
				colliding.EntityKinds = []sdk.EntityKind{{Name: "shared", Archetype: "category"}}
				candidate := admissibleBundle()
				candidate.ID = "market-two"
				candidate.EntityKinds = []sdk.EntityKind{{Name: "shared", Archetype: "category"}}
				return sdktest.Listing{
					Bundle:            candidate,
					ConformancePassed: true,
					Registry:          []sdk.Bundle{colliding},
				}
			},
			sdkVer:   sdk.Version,
			wantGate: sdktest.GateLifecycle,
		},
		{
			name: "compat-range: registry SDK version is below the declared floor",
			listing: func() sdktest.Listing {
				l := admissibleListing(t)
				l.Bundle.SDKFloor = "0.5.0"
				l.Bundle.SDKCeiling = "0.6.0"
				return l
			},
			sdkVer:   sdk.Version,
			wantGate: sdktest.GateCompatRange,
		},
		{
			name: "legal: no signed legal metadata",
			listing: func() sdktest.Listing {
				l := admissibleListing(t)
				l.Legal = nil
				return l
			},
			sdkVer:   sdk.Version,
			wantGate: sdktest.GateLegal,
		},
		{
			name: "conformance: sdktest suite did not pass",
			listing: func() sdktest.Listing {
				l := admissibleListing(t)
				l.ConformancePassed = false
				return l
			},
			sdkVer:   sdk.Version,
			wantGate: sdktest.GateConformance,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result := sdktest.Admit(tc.listing(), tc.sdkVer)
			if result.Admitted {
				t.Fatalf("Admit(%s) accepted; want rejection at gate %s", tc.name, tc.wantGate)
			}
			if result.FailedGate != tc.wantGate {
				t.Fatalf("Admit(%s) failed gate = %s, want %s (reason: %s)", tc.name, result.FailedGate, tc.wantGate, result.Reason)
			}
			if result.Reason == "" {
				t.Fatalf("Admit(%s) rejected with no reason", tc.name)
			}
		})
	}
}

// TestAdmitStopsAtFirstFailure asserts Admit is fail-fast: a listing broken on
// two gates reports the earlier one in evaluation order and never reaches the
// later gate.
func TestAdmitStopsAtFirstFailure(t *testing.T) {
	l := admissibleListing(t)
	l.Bundle.DisplayName = sdk.LocaleString{} // manifest gate fails
	l.ConformancePassed = false               // conformance gate would also fail
	result := sdktest.Admit(l, sdk.Version)
	if result.FailedGate != sdktest.GateManifest {
		t.Fatalf("FailedGate = %s, want %s (manifest evaluates before conformance)", result.FailedGate, sdktest.GateManifest)
	}
}

func TestAdmitRejectsUnprovedCanonicalCompatibility(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*sdktest.Listing)
		sdkVersion string
	}{
		{"consumer version", func(listing *sdktest.Listing) {
			listing.Bundle.ID = "adult"
			listing.Bundle.Version = "0.1.1"
		}, sdk.Version},
		{"SDK version", func(listing *sdktest.Listing) {
			listing.Bundle.ID = "adult"
		}, sdk.Version},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			listing := admissibleListing(t)
			test.mutate(&listing)
			result := sdktest.Admit(listing, test.sdkVersion)
			if result.Admitted || result.FailedGate != sdktest.GateCompatRange {
				t.Fatalf("Admit = %+v, want compatibility rejection despite ConformancePassed", result)
			}
		})
	}
}

func TestAdmitAcceptsThirdPartyWithoutProducerMatrixRow(t *testing.T) {
	listing := admissibleListing(t)
	listing.Bundle.ID = "independent"
	listing.Bundle.Version = "3.2.1"
	listing.Legal = legalEvidence(t, listing.Bundle)

	result := sdktest.Admit(listing, sdk.Version)
	if !result.Admitted {
		t.Fatalf("Admit(third-party listing) rejected: gate=%s reason=%s", result.FailedGate, result.Reason)
	}

	listing.Bundle.SDKFloor = "0.3.0"
	listing.Bundle.SDKCeiling = "0.3.0"
	listing.Legal = legalEvidence(t, listing.Bundle)
	result = sdktest.Admit(listing, sdk.Version)
	if result.Admitted || result.FailedGate != sdktest.GateCompatRange {
		t.Fatalf("Admit(incompatible third-party listing) = %+v, want compat-range rejection", result)
	}
}

// TestAdmitReportsBrokenRegistryIndependentOfCandidate: when the ALREADY-
// admitted registry set is itself invalid (two admitted bundles colliding on
// an Entity Kind name), the lifecycle gate still rejects fail-closed but
// attributes the failure to registry state — not to the innocent candidate.
func TestAdmitReportsBrokenRegistryIndependentOfCandidate(t *testing.T) {
	one := admissibleBundle()
	one.ID = "registry-one"
	one.EntityKinds = []sdk.EntityKind{{Name: "shared", Archetype: "category"}}
	two := admissibleBundle()
	two.ID = "registry-two"
	two.EntityKinds = []sdk.EntityKind{{Name: "shared", Archetype: "category"}}

	candidate := admissibleBundle()
	candidate.ID = "innocent"
	listing := sdktest.Listing{
		Bundle:            candidate,
		ConformancePassed: true,
		Registry:          []sdk.Bundle{one, two},
	}

	result := sdktest.Admit(listing, sdk.Version)
	if result.Admitted {
		t.Fatal("Admit accepted a listing whose registry set is invalid; want a fail-closed lifecycle rejection")
	}
	if result.FailedGate != sdktest.GateLifecycle {
		t.Fatalf("FailedGate = %s, want %s (reason: %s)", result.FailedGate, sdktest.GateLifecycle, result.Reason)
	}
	if !strings.Contains(result.Reason, "registry admitted set is invalid independent of candidate innocent") {
		t.Fatalf("Reason = %q, want it attributed to the registry admitted set, not the candidate", result.Reason)
	}
}

// TestAdmitRejectsIncompleteOrInconsistentLegalMetadata breaks the legal
// evidence one way at a time; each must fail the legal gate, never reach
// conformance, and never admit.
func TestAdmitRejectsIncompleteOrInconsistentLegalMetadata(t *testing.T) {
	cases := map[string]func(*sdktest.LegalEvidence){
		"entry names another plugin":    func(e *sdktest.LegalEvidence) { e.Entry.PluginID = "someone-else" },
		"entry names another version":   func(e *sdktest.LegalEvidence) { e.Entry.Version = "0.1.1" },
		"entry names another SDK range": func(e *sdktest.LegalEvidence) { e.Entry.SDKCeiling = "9.9.9" },
		"missing licence":               func(e *sdktest.LegalEvidence) { e.Entry.Legal.LicenseExpression = "" },
		"ambiguous licence":             func(e *sdktest.LegalEvidence) { e.Entry.Legal.LicenseExpression = "NOASSERTION" },
		"licence outside policy":        func(e *sdktest.LegalEvidence) { e.Entry.Legal.LicenseExpression = "MIT" },
		"missing notices":               func(e *sdktest.LegalEvidence) { e.Entry.Legal.Notices = nil },
		"missing source offer":          func(e *sdktest.LegalEvidence) { e.Entry.Legal.SourceOffer = "" },
		"missing provenance":            func(e *sdktest.LegalEvidence) { e.Entry.Legal.ProvenanceSHA256 = "" },
		"unattested":                    func(e *sdktest.LegalEvidence) { e.Entry.Legal.RightsAttestation.Signature = "" },
		"reviewer outside the policy":   func(e *sdktest.LegalEvidence) { e.Entry.Legal.RightsAttestation.ReviewerKeyID = "nobody" },
		"policy from an unknown root":   func(e *sdktest.LegalEvidence) { e.Roots = nil },
		"policy altered after signing":  func(e *sdktest.LegalEvidence) { e.Policy.Policy.AttestationMaxAgeDays = 9999 },
		"no evaluation time":            func(e *sdktest.LegalEvidence) { e.At = time.Time{} },
		"artifact bytes differ":         func(e *sdktest.LegalEvidence) { e.Bytes.Artifact = []byte("other") },
		"notice bytes missing":          func(e *sdktest.LegalEvidence) { e.Bytes.Notices = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			listing := admissibleListing(t)
			mutate(listing.Legal)
			result := sdktest.Admit(listing, sdk.Version)
			if result.Admitted || result.FailedGate != sdktest.GateLegal || result.Reason == "" {
				t.Fatalf("Admit = %+v, want a legal-gate rejection", result)
			}
		})
	}
}
