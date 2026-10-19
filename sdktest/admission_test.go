package sdktest_test

import (
	"strings"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
	"github.com/beamfall/plugin-sdk/sdktest"
)

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

func admissibleListing() sdktest.Listing {
	return sdktest.Listing{Bundle: admissibleBundle(), ConformancePassed: true}
}

// TestAdmitAcceptsConformantListing is the positive control: every case below
// changes exactly one thing away from this passing baseline.
func TestAdmitAcceptsConformantListing(t *testing.T) {
	result := sdktest.Admit(admissibleListing(), sdk.Version)
	if !result.Admitted {
		t.Fatalf("Admit(conformant listing) rejected: gate=%s reason=%s", result.FailedGate, result.Reason)
	}
	want := []sdktest.GateClass{sdktest.GateManifest, sdktest.GateLifecycle, sdktest.GateCompatRange, sdktest.GateConformance}
	if len(result.GatesPassed) != len(want) {
		t.Fatalf("GatesPassed = %v, want all four gates recorded", result.GatesPassed)
	}
}

// TestAdmitRejectsPerGateClass proves the mechanical admission pipeline
// rejects a fixture plugin failing each of the four gate classes:
// manifest, lifecycle, compat range, and conformance.
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
				l := admissibleListing()
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
				l := admissibleListing()
				l.Bundle.SDKFloor = "0.5.0"
				l.Bundle.SDKCeiling = "0.6.0"
				return l
			},
			sdkVer:   sdk.Version,
			wantGate: sdktest.GateCompatRange,
		},
		{
			name: "conformance: sdktest suite did not pass",
			listing: func() sdktest.Listing {
				l := admissibleListing()
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
	l := admissibleListing()
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
			listing := admissibleListing()
			test.mutate(&listing)
			result := sdktest.Admit(listing, test.sdkVersion)
			if result.Admitted || result.FailedGate != sdktest.GateCompatRange {
				t.Fatalf("Admit = %+v, want compatibility rejection despite ConformancePassed", result)
			}
		})
	}
}

func TestAdmitAcceptsThirdPartyWithoutProducerMatrixRow(t *testing.T) {
	listing := admissibleListing()
	listing.Bundle.ID = "independent"
	listing.Bundle.Version = "3.2.1"

	result := sdktest.Admit(listing, sdk.Version)
	if !result.Admitted {
		t.Fatalf("Admit(third-party listing) rejected: gate=%s reason=%s", result.FailedGate, result.Reason)
	}

	listing.Bundle.SDKFloor = "0.3.0"
	listing.Bundle.SDKCeiling = "0.3.0"
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
