package sdktest

import (
	"fmt"

	sdk "github.com/beamfall/plugin-sdk"
	"github.com/beamfall/plugin-sdk/compatibility"
)

// GateClass names one of the four mechanical admission gates required
// before a plugin listing is admitted to the registry: pass the SDK
// conformance suite, declare a valid bundle manifest, respect the
// fail-closed lifecycle contract, and declare an SDK compat
// range that covers the registry's SDK version. Admission beyond these gates
// (legality/malware) is a manual review function, not mechanical — see
// docs/registry-admission-runbook.md.
type GateClass string

const (
	GateConformance GateClass = "conformance"
	GateManifest    GateClass = "manifest"
	GateLifecycle   GateClass = "lifecycle"
	GateCompatRange GateClass = "compat-range"
)

// gateClasses is the fixed, ordered admission vocabulary. Order
// is evaluation order: cheapest/most-diagnostic failures surface first.
var gateClasses = []GateClass{GateManifest, GateLifecycle, GateCompatRange, GateConformance}

// Listing is the registry-facing manifest for one plugin admission attempt:
// the plugin's Bundle plus the out-of-band conformance signal (did
// `go test ./...` against sdktest's runners pass for this artifact). The
// registry computes ConformancePassed by running the artifact's own module
// tests before calling Admit; sdktest cannot execute another module's tests
// from within this package.
type Listing struct {
	Bundle sdk.Bundle
	// ConformancePassed reports whether the artifact's sdktest-based
	// conformance suite (RunConformance / RunScoreConformance /
	// RunBundleConformance) passed. Required because conformance runs as
	// that plugin's own `go test`, outside this process.
	ConformancePassed bool
	// Registry holds the Bundles of plugins already admitted to the target
	// registry listing set. The lifecycle gate checks the candidate against
	// this set (fails-closed, all-or-nothing load: a new revision
	// must not silently collide with another plugin's ID or Entity Kind
	// namespace once both are loaded together).
	Registry []sdk.Bundle
}

// AdmissionResult is the outcome of Admit: which gate classes passed, and the
// first failure with a human-readable reason (mechanical, not
// taste: every rejection traces to exactly one named gate class).
type AdmissionResult struct {
	Admitted    bool
	FailedGate  GateClass
	Reason      string
	GatesPassed []GateClass
}

// Admit runs the four mechanical admission gates against a
// listing, in order, stopping at the first failure. sdkVersion is the
// registry's reference plugin-sdk release (e.g. sdk.Version of the SDK
// build the registry curates against).
func Admit(listing Listing, sdkVersion string) AdmissionResult {
	var passed []GateClass
	for _, gate := range gateClasses {
		if reason, ok := evaluateGate(gate, listing, sdkVersion); !ok {
			return AdmissionResult{Admitted: false, FailedGate: gate, Reason: reason, GatesPassed: passed}
		}
		passed = append(passed, gate)
	}
	return AdmissionResult{Admitted: true, GatesPassed: passed}
}

// evaluateGate runs a single gate class and reports (reason, ok). ok is
// false and reason is non-empty on rejection.
func evaluateGate(gate GateClass, listing Listing, sdkVersion string) (string, bool) {
	switch gate {
	case GateManifest:
		// The bundle's own declarative manifest must be self-consistent
		// (fail-closed load validation): valid ID, semver,
		// locale maps, vocabularies, sources, layouts, etc.
		if err := sdk.ValidateBundle(listing.Bundle); err != nil {
			return err.Error(), false
		}
		return "", true
	case GateLifecycle:
		// The lifecycle contract's admission-time obligation is
		// fail-closed, all-or-nothing load: a listing must not
		// silently collide with a plugin ID or Entity Kind namespace already
		// held by the registry's admitted set once both load together. The
		// admitted set is validated ALONE first so that a registry that is
		// already broken independent of the candidate is reported as exactly
		// that — still a fail-closed rejection, but attributed to registry
		// state rather than blaming the candidate for another plugin's fault.
		if err := sdk.ValidateBundles(append([]sdk.Bundle{}, listing.Registry...)); err != nil {
			return fmt.Sprintf("registry admitted set is invalid independent of candidate %s (fail-closed): %v", listing.Bundle.ID, err), false
		}
		if err := sdk.ValidateBundles(append(append([]sdk.Bundle{}, listing.Registry...), listing.Bundle)); err != nil {
			return err.Error(), false
		}
		return "", true
	case GateCompatRange:
		for _, entry := range compatibility.Entries() {
			if entry.ConsumerID != listing.Bundle.ID {
				continue
			}
			if err := compatibility.Verify(listing.Bundle, sdkVersion); err != nil {
				return err.Error(), false
			}
			return "", true
		}
		if !sdk.CompatibleWithSDK(listing.Bundle, sdkVersion) {
			return fmt.Sprintf("plugin %s: SDK range [%s, %s] does not support SDK %s", listing.Bundle.ID, listing.Bundle.SDKFloor, listing.Bundle.SDKCeiling, sdkVersion), false
		}
		return "", true
	case GateConformance:
		if !listing.ConformancePassed {
			return fmt.Sprintf("plugin %s: sdktest conformance suite did not pass", listing.Bundle.ID), false
		}
		return "", true
	default:
		return fmt.Sprintf("unknown admission gate class %q", gate), false
	}
}
