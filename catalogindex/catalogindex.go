// Package catalogindex verifies and filters Beamfall's public global plugin
// catalog. Callers obtain the one static artifact without plugin identifiers,
// verify it offline, then filter it locally.
//
// Every entry carries signed legal metadata: an SPDX licence expression,
// notice files, the exact source and build provenance, an optional source
// offer and a rights attestation by a policy reviewer. What is acceptable is
// not decided here. The catalog operator signs a licence Policy (allowed SPDX
// expressions, each with its source-offer rule, the reviewer keyring and the
// maximum attestation age), the document embeds it, and the index payload is
// bound to its digest. This package ships no production policy, keyring or
// key; it only verifies the operator's signed data and fails closed on any
// omission or mismatch.
package catalogindex

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SchemaVersion is the only catalog index schema ParseAndVerify accepts.
// Version 2 adds the signed legal metadata (licence, notices, source,
// provenance, rights attestation) and the embedded licence policy; version 1
// documents carry none of it and are rejected with ErrLegacySchema.
const SchemaVersion = 2

// legacySchemaVersion is the pre-legal schema. It is never admitted; see
// MigrateV1.
const legacySchemaVersion = 1

// firstPartyCatalogOnly keeps catalog admission closed to third-party plugins.
// The index can now carry notice and source-offer metadata, but opening
// third-party admission is a separate owner decision.
const firstPartyCatalogOnly = true

var firstPartyPluginIDs = map[string]struct{}{
	"adult":         {},
	"audiobook":     {},
	"familyarchive": {},
	"filmtv":        {},
	"music":         {},
	"photos":        {},
	"podcasts":      {},
}

// Entry is one plugin release in the catalog. Every field, including Legal,
// is covered by the registry signature.
type Entry struct {
	PluginID       string `json:"plugin_id"`
	Version        string `json:"version"`
	SDKFloor       string `json:"sdk_floor"`
	SDKCeiling     string `json:"sdk_ceiling"`
	ArtifactSHA256 string `json:"artifact_sha256"`
	DeliveryRef    string `json:"delivery_ref"`
	Legal          Legal  `json:"legal"`
}

// Index is the signed catalog payload. PolicySHA256 binds the index to the
// licence policy embedded in the SignedIndex document (see PolicyDigest).
type Index struct {
	SchemaVersion int       `json:"schema_version"`
	Sequence      uint64    `json:"sequence"`
	IssuedAt      time.Time `json:"issued_at"`
	PolicySHA256  string    `json:"policy_sha256"`
	Entries       []Entry   `json:"entries"`
}

// SignedIndex is the complete static catalog document: the index, the
// operator-signed licence policy it is bound to, and the registry signature
// over the index and RegistryKeyID.
type SignedIndex struct {
	Index         Index        `json:"index"`
	Policy        SignedPolicy `json:"policy"`
	RegistryKeyID string       `json:"registry_key_id"`
	Signature     string       `json:"signature"`
}

type signingPayload struct {
	Index         Index  `json:"index"`
	RegistryKeyID string `json:"registry_key_id"`
}

var (
	ErrInvalid            = errors.New("catalogindex: invalid index")
	ErrSignature          = errors.New("catalogindex: invalid signature")
	ErrUnknownKey         = errors.New("catalogindex: unknown registry key")
	ErrSequenceRegression = errors.New("catalogindex: sequence must increase monotonically")
	// ErrLegacySchema rejects a schema version 1 document: it carries no
	// licence, notice, provenance or rights metadata and is never admitted.
	// Re-issue it as version 2 (see MigrateV1).
	ErrLegacySchema = errors.New("catalogindex: schema version 1 carries no legal metadata; re-issue as version 2")
)

// ParseAndVerify decodes a complete static catalog, verifies it against caller-
// supplied bundled roots, and rejects equal or older publications. It verifies,
// in order: the schema version, the sequence, the registry signature over the
// index (which covers the policy digest and every entry's legal fields), the
// embedded licence policy's own signature and its digest binding, and then
// every entry's legal metadata against that policy. Any failure returns an
// empty Index.
func ParseAndVerify(raw []byte, roots map[string]ed25519.PublicKey, lastKnownSequence uint64) (Index, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var signed SignedIndex
	if err := dec.Decode(&signed); err != nil {
		return Index{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return Index{}, fmt.Errorf("%w: trailing data", ErrInvalid)
	}
	if signed.Index.SchemaVersion == legacySchemaVersion {
		return Index{}, ErrLegacySchema
	}
	if signed.Index.Sequence <= lastKnownSequence {
		return Index{}, ErrSequenceRegression
	}
	key, ok := roots[signed.RegistryKeyID]
	if !ok || len(key) != ed25519.PublicKeySize {
		return Index{}, ErrUnknownKey
	}
	if err := validate(signed.Index); err != nil {
		return Index{}, err
	}
	if err := verifySignature(key, signingPayload{Index: signed.Index, RegistryKeyID: signed.RegistryKeyID}, signed.Signature); err != nil {
		return Index{}, err
	}
	policyDigest, err := verifyPolicy(signed.Policy, roots)
	if err != nil {
		return Index{}, err
	}
	if policyDigest != signed.Index.PolicySHA256 {
		return Index{}, fmt.Errorf("%w: index is bound to policy %s, embedded policy is %s", ErrPolicy, signed.Index.PolicySHA256, policyDigest)
	}
	if signed.Policy.Policy.IssuedAt.After(signed.Index.IssuedAt) {
		return Index{}, fmt.Errorf("%w: policy issued after the index", ErrPolicy)
	}
	for _, entry := range signed.Index.Entries {
		if err := verifyLegal(entry, signed.Policy.Policy, policyDigest, signed.Index.IssuedAt); err != nil {
			return Index{}, err
		}
	}
	return signed.Index, nil
}

func verifySignature(key ed25519.PublicKey, payload any, signature string) error {
	sig, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return ErrSignature
	}
	encoded, err := canonicalJSON(payload)
	if err != nil {
		return err
	}
	if !ed25519.Verify(key, encoded, sig) {
		return ErrSignature
	}
	return nil
}

// Filter returns only installed plugin IDs from an already verified index. It
// performs no I/O; plugin IDs remain local.
func Filter(index Index, pluginIDs map[string]struct{}) []Entry {
	out := make([]Entry, 0, len(pluginIDs))
	for _, entry := range index.Entries {
		if _, ok := pluginIDs[entry.PluginID]; ok {
			out = append(out, entry)
		}
	}
	return out
}

func validate(index Index) error {
	if index.SchemaVersion != SchemaVersion || index.Sequence == 0 || index.IssuedAt.IsZero() || !isSHA256(index.PolicySHA256) {
		return ErrInvalid
	}
	seenIDs := make(map[string]struct{}, len(index.Entries))
	seenRefs := make(map[string]struct{}, len(index.Entries))
	for _, entry := range index.Entries {
		if firstPartyCatalogOnly {
			if _, ok := firstPartyPluginIDs[entry.PluginID]; !ok {
				return ErrInvalid
			}
		}
		if err := validateEntry(entry); err != nil {
			return err
		}
		if _, exists := seenIDs[entry.PluginID]; exists {
			return ErrInvalid
		}
		seenIDs[entry.PluginID] = struct{}{}
		if _, exists := seenRefs[entry.DeliveryRef]; exists {
			return ErrInvalid
		}
		seenRefs[entry.DeliveryRef] = struct{}{}
	}
	return nil
}

// validateEntry checks one entry's release fields (not its legal metadata).
func validateEntry(entry Entry) error {
	for _, value := range []string{entry.PluginID, entry.Version, entry.ArtifactSHA256, entry.DeliveryRef} {
		if strings.TrimSpace(value) == "" {
			return ErrInvalid
		}
	}
	if !isSHA256(entry.ArtifactSHA256) {
		return ErrInvalid
	}
	if strings.ContainsAny(entry.DeliveryRef, "/?#") || strings.Contains(entry.DeliveryRef, "://") {
		return ErrInvalid
	}
	_, versionOK := parseSemver(entry.Version)
	floor, floorOK := parseSemver(entry.SDKFloor)
	ceiling, ceilingOK := parseSemver(entry.SDKCeiling)
	if !versionOK || (entry.SDKFloor != "" && !floorOK) || (entry.SDKCeiling != "" && !ceilingOK) || (floorOK && ceilingOK && compareSemver(floor, ceiling) > 0) {
		return ErrInvalid
	}
	return nil
}

// isSHA256 reports whether value is a lowercase hex SHA-256 digest.
func isSHA256(value string) bool {
	return isLowerHex(value, 32)
}

func isLowerHex(value string, size int) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == size && value == strings.ToLower(value)
}

func parseSemver(value string) ([3]uint64, bool) {
	var parsed [3]uint64
	if value == "" {
		return parsed, false
	}
	parts := strings.Split(strings.SplitN(value, "-", 2)[0], ".")
	if len(parts) != 3 {
		return parsed, false
	}
	for i, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return parsed, false
		}
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return parsed, false
		}
		parsed[i] = n
	}
	return parsed, true
}

func compareSemver(a, b [3]uint64) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

func canonicalJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	return marshalCanonical(generic)
}

func marshalCanonical(v any) ([]byte, error) {
	switch value := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		buf := []byte{'{'}
		for i, key := range keys {
			if i > 0 {
				buf = append(buf, ',')
			}
			encodedKey, err := json.Marshal(key)
			if err != nil {
				return nil, err
			}
			encodedValue, err := marshalCanonical(value[key])
			if err != nil {
				return nil, err
			}
			buf = append(buf, encodedKey...)
			buf = append(buf, ':')
			buf = append(buf, encodedValue...)
		}
		return append(buf, '}'), nil
	case []any:
		buf := []byte{'['}
		for i, item := range value {
			if i > 0 {
				buf = append(buf, ',')
			}
			encoded, err := marshalCanonical(item)
			if err != nil {
				return nil, err
			}
			buf = append(buf, encoded...)
		}
		return append(buf, ']'), nil
	default:
		return json.Marshal(value)
	}
}
