package catalogindex

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// legacyEntry, legacyIndex and legacySignedIndex are the schema version 1
// wire shapes, kept only so MigrateV1 can verify a v1 signature.
type legacyEntry struct {
	PluginID       string `json:"plugin_id"`
	Version        string `json:"version"`
	SDKFloor       string `json:"sdk_floor"`
	SDKCeiling     string `json:"sdk_ceiling"`
	ArtifactSHA256 string `json:"artifact_sha256"`
	DeliveryRef    string `json:"delivery_ref"`
}

type legacyIndex struct {
	SchemaVersion int           `json:"schema_version"`
	Sequence      uint64        `json:"sequence"`
	IssuedAt      time.Time     `json:"issued_at"`
	Entries       []legacyEntry `json:"entries"`
}

type legacySignedIndex struct {
	Index         legacyIndex `json:"index"`
	RegistryKeyID string      `json:"registry_key_id"`
	Signature     string      `json:"signature"`
}

type legacySigningPayload struct {
	Index         legacyIndex `json:"index"`
	RegistryKeyID string      `json:"registry_key_id"`
}

// MigrateV1 is the only migration path for an existing schema version 1
// index. It verifies the v1 document's registry signature against roots,
// then returns its entries as v2 entries carrying legal[plugin ID]. It fails
// closed when any v1 entry has no legal record or legal names a plugin the
// v1 index does not list: nothing is grandfathered. The result is not an
// admitted catalog: the operator still has every entry attested under its
// signed policy (SignRightsAttestation), signs a new v2 index at a higher
// sequence (SignIndex) and checks it with ParseAndVerify.
func MigrateV1(raw []byte, roots map[string]ed25519.PublicKey, legal map[string]Legal) ([]Entry, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var signed legacySignedIndex
	if err := dec.Decode(&signed); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("%w: trailing data", ErrInvalid)
	}
	if signed.Index.SchemaVersion != legacySchemaVersion || signed.Index.Sequence == 0 || signed.Index.IssuedAt.IsZero() {
		return nil, fmt.Errorf("%w: not a schema version 1 index", ErrInvalid)
	}
	key, ok := roots[signed.RegistryKeyID]
	if !ok || len(key) != ed25519.PublicKeySize {
		return nil, ErrUnknownKey
	}
	if err := verifySignature(key, legacySigningPayload{Index: signed.Index, RegistryKeyID: signed.RegistryKeyID}, signed.Signature); err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(signed.Index.Entries))
	for _, old := range signed.Index.Entries {
		record, ok := legal[old.PluginID]
		if !ok {
			return nil, fmt.Errorf("%w: no legal record for v1 plugin %q", ErrInvalid, old.PluginID)
		}
		entry := Entry{
			PluginID:       old.PluginID,
			Version:        old.Version,
			SDKFloor:       old.SDKFloor,
			SDKCeiling:     old.SDKCeiling,
			ArtifactSHA256: old.ArtifactSHA256,
			DeliveryRef:    old.DeliveryRef,
			Legal:          record,
		}
		if err := validateEntry(entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if len(legal) != len(entries) {
		return nil, fmt.Errorf("%w: legal records name plugins the v1 index does not list", ErrInvalid)
	}
	return entries, nil
}
