// Package catalogindex verifies and filters Beamfall's public global plugin
// catalog. Callers obtain the one static artifact without plugin identifiers,
// verify it offline, then filter it locally.
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

const SchemaVersion = 1

// firstPartyCatalogOnly keeps catalog admission closed to third-party plugins
// until the catalog carries notice and source-offer metadata.
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

type Entry struct {
	PluginID       string `json:"plugin_id"`
	Version        string `json:"version"`
	SDKFloor       string `json:"sdk_floor"`
	SDKCeiling     string `json:"sdk_ceiling"`
	ArtifactSHA256 string `json:"artifact_sha256"`
	DeliveryRef    string `json:"delivery_ref"`
}

type Index struct {
	SchemaVersion int       `json:"schema_version"`
	Sequence      uint64    `json:"sequence"`
	IssuedAt      time.Time `json:"issued_at"`
	Entries       []Entry   `json:"entries"`
}

type SignedIndex struct {
	Index         Index  `json:"index"`
	RegistryKeyID string `json:"registry_key_id"`
	Signature     string `json:"signature"`
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
)

// ParseAndVerify decodes a complete static catalog, verifies it against caller-
// supplied bundled roots, and rejects equal or older publications.
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
	sig, err := base64.RawURLEncoding.DecodeString(signed.Signature)
	if err != nil {
		return Index{}, ErrSignature
	}
	encoded, err := canonicalJSON(signingPayload{Index: signed.Index, RegistryKeyID: signed.RegistryKeyID})
	if err != nil {
		return Index{}, err
	}
	if !ed25519.Verify(key, encoded, sig) {
		return Index{}, ErrSignature
	}
	return signed.Index, nil
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
	if index.SchemaVersion != SchemaVersion || index.Sequence == 0 || index.IssuedAt.IsZero() {
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
		for _, value := range []string{entry.PluginID, entry.Version, entry.ArtifactSHA256, entry.DeliveryRef} {
			if strings.TrimSpace(value) == "" {
				return ErrInvalid
			}
		}
		if _, exists := seenIDs[entry.PluginID]; exists {
			return ErrInvalid
		}
		seenIDs[entry.PluginID] = struct{}{}
		if _, exists := seenRefs[entry.DeliveryRef]; exists {
			return ErrInvalid
		}
		seenRefs[entry.DeliveryRef] = struct{}{}
		hash, err := hex.DecodeString(entry.ArtifactSHA256)
		if err != nil || len(hash) != 32 || entry.ArtifactSHA256 != strings.ToLower(entry.ArtifactSHA256) {
			return ErrInvalid
		}
		if strings.ContainsAny(entry.DeliveryRef, "/?#") || strings.Contains(entry.DeliveryRef, "://") {
			return ErrInvalid
		}
		version, versionOK := parseSemver(entry.Version)
		floor, floorOK := parseSemver(entry.SDKFloor)
		ceiling, ceilingOK := parseSemver(entry.SDKCeiling)
		if !versionOK || (entry.SDKFloor != "" && !floorOK) || (entry.SDKCeiling != "" && !ceilingOK) || (floorOK && ceilingOK && compareSemver(floor, ceiling) > 0) {
			return ErrInvalid
		}
		_ = version
	}
	return nil
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
