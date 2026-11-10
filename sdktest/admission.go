package sdktest

import (
	"crypto/ed25519"
	"fmt"
	"time"

	sdk "github.com/beamfall/plugin-sdk"
	"github.com/beamfall/plugin-sdk/catalogindex"
	"github.com/beamfall/plugin-sdk/compatibility"
)

// GateClass names one of the five mechanical admission gates required
// before a plugin listing is admitted to the registry: declare a valid
// bundle manifest, respect the fail-closed lifecycle contract, declare an
// SDK compat range that covers the registry's SDK version, carry signed
// legal metadata that verifies against the catalog operator's signed licence
// policy, and pass the SDK conformance suite. The legal gate checks that the
// metadata is complete, consistent and attested by a reviewer the policy
// names; it does not decide whether a licence is acceptable (the policy
// does) and it is not a malware review — see
// docs/registry-admission-runbook.md.
type GateClass string

const (
	GateConformance GateClass = "conformance"
	GateManifest    GateClass = "manifest"
	GateLifecycle   GateClass = "lifecycle"
	GateCompatRange GateClass = "compat-range"
	GateLegal       GateClass = "legal"
)

// gateClasses is the fixed, ordered admission vocabulary. Order
// is evaluation order: cheapest/most-diagnostic failures surface first.
var gateClasses = []GateClass{GateManifest, GateLifecycle, GateCompatRange, GateLegal, GateConformance}

// LegalEvidence is the signed legal metadata for one listing: the catalog
// entry the registry would publish, the operator's signed licence policy, the
// registry root keys that sign that policy, the evaluation time (the issue
// time of the index the entry would join) and the bytes the entry's digests
// name. The SDK ships no policy, keyring or keys; the catalog operator
// supplies them.
type LegalEvidence struct {
	Entry  catalogindex.Entry
	Policy catalogindex.SignedPolicy
	Roots  map[string]ed25519.PublicKey
	At     time.Time
	Bytes  catalogindex.EntryBytes
}

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
	// Legal is the listing's signed legal metadata. A nil Legal fails the
	// legal gate: nothing is admitted without it.
	Legal *LegalEvidence
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

// Admit runs the five mechanical admission gates against a
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
	case GateLegal:
		return legalGate(listing)
	case GateConformance:
		if !listing.ConformancePassed {
			return fmt.Sprintf("plugin %s: sdktest conformance suite did not pass", listing.Bundle.ID), false
		}
		return "", true
	default:
		return fmt.Sprintf("unknown admission gate class %q", gate), false
	}
}

// legalGate binds the listing's bundle to its signed catalog entry and
// verifies the entry against the operator's signed policy and the supplied
// bytes. Every omission or inconsistency rejects.
func legalGate(listing Listing) (string, bool) {
	evidence, bundle := listing.Legal, listing.Bundle
	if evidence == nil {
		return fmt.Sprintf("plugin %s: no signed legal metadata (licence, notices, source, provenance, rights attestation)", bundle.ID), false
	}
	entry := evidence.Entry
	if entry.PluginID != string(bundle.ID) || entry.Version != bundle.Version || entry.SDKFloor != bundle.SDKFloor || entry.SDKCeiling != bundle.SDKCeiling {
		return fmt.Sprintf("plugin %s %s [%s, %s]: legal metadata names %s %s [%s, %s]", bundle.ID, bundle.Version, bundle.SDKFloor, bundle.SDKCeiling, entry.PluginID, entry.Version, entry.SDKFloor, entry.SDKCeiling), false
	}
	if err := catalogindex.VerifyEntry(entry, evidence.Policy, evidence.Roots, evidence.At); err != nil {
		return fmt.Sprintf("plugin %s: %v", bundle.ID, err), false
	}
	if err := catalogindex.VerifyEntryBytes(entry, evidence.Bytes); err != nil {
		return fmt.Sprintf("plugin %s: %v", bundle.ID, err), false
	}
	return "", true
}
