// Package compatibility records producer-owned SDK compatibility evidence for
// the canonical first-party plugin set.
package compatibility

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"

	sdk "github.com/beamfall/plugin-sdk"
)

// Entry binds one exact plugin artifact and declared SDK range to the SDK
// version against which its conformance test passed.
type Entry struct {
	ConsumerID       sdk.ID `json:"consumerID"`
	ConsumerVersion  string `json:"consumerVersion"`
	ConsumerRevision string `json:"consumerRevision"`
	ConsumerDigest   string `json:"consumerDigest"`
	SDKVersion       string `json:"sdkVersion"`
	SDKFloor         string `json:"sdkFloor"`
	SDKCeiling       string `json:"sdkCeiling"`
	ConformanceTest  string `json:"conformanceTest"`
}

//go:embed matrix.json
var matrixJSON []byte

var entries = mustLoadEntries()

func mustLoadEntries() []Entry {
	var loaded []Entry
	if err := json.Unmarshal(matrixJSON, &loaded); err != nil {
		panic(fmt.Sprintf("compatibility matrix: %v", err))
	}
	return loaded
}

// Entries returns a copy of the canonical compatibility matrix.
func Entries() []Entry { return slices.Clone(entries) }

// Verify rejects a bundle unless the producer matrix contains exact evidence
// for its identity, version, declared range, and the requested SDK version.
func Verify(bundle sdk.Bundle, sdkVersion string) error {
	if !sdk.CompatibleWithSDK(bundle, sdkVersion) {
		return fmt.Errorf("plugin %s: SDK range [%s, %s] does not support SDK %s", bundle.ID, bundle.SDKFloor, bundle.SDKCeiling, sdkVersion)
	}
	for _, entry := range entries {
		if entry.ConsumerID != bundle.ID {
			continue
		}
		if entry.ConsumerVersion != bundle.Version {
			continue
		}
		if entry.SDKVersion != sdkVersion {
			continue
		}
		if entry.SDKFloor != bundle.SDKFloor || entry.SDKCeiling != bundle.SDKCeiling {
			return fmt.Errorf("plugin %s version %s: declared SDK range [%s, %s] lacks exact producer evidence", bundle.ID, bundle.Version, bundle.SDKFloor, bundle.SDKCeiling)
		}
		if entry.ConformanceTest == "" {
			return fmt.Errorf("plugin %s version %s: compatibility evidence has no conformance test", bundle.ID, bundle.Version)
		}
		return nil
	}
	return fmt.Errorf("plugin %s version %s: no producer compatibility evidence for SDK %s", bundle.ID, bundle.Version, sdkVersion)
}
