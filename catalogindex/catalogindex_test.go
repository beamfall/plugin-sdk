package catalogindex

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

const fixturePublicKey = "eDUCARIrsMG5h2Pm3Vy9_7nMZPkTm4vdA8mWgJ7j32U"

func TestCloudFixtureVerifiesAndFiltersLocally(t *testing.T) {
	raw, err := os.ReadFile("testdata/catalog-index.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	pub, err := base64.RawURLEncoding.DecodeString(fixturePublicKey)
	if err != nil {
		t.Fatal(err)
	}
	roots := map[string]ed25519.PublicKey{"catalog-fixture-1": pub}
	index, err := ParseAndVerify(raw, roots, 6)
	if err != nil {
		t.Fatal(err)
	}
	got := Filter(index, map[string]struct{}{"music": {}})
	if len(got) != 1 || got[0].PluginID != "music" {
		t.Fatalf("Filter = %#v", got)
	}
	if _, err := ParseAndVerify(raw, roots, 7); !errors.Is(err, ErrSequenceRegression) {
		t.Fatalf("equal sequence error = %v", err)
	}
}

func TestTamperedWholeIndexFailsOfflineVerification(t *testing.T) {
	raw, err := os.ReadFile("testdata/catalog-index.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	pub, _ := base64.RawURLEncoding.DecodeString(fixturePublicKey)
	tampered := append([]byte(nil), raw...)
	for i := range tampered {
		if tampered[i] == '1' {
			tampered[i] = '9'
			break
		}
	}
	invalidSignature := bytes.Replace(raw, []byte(`"signature":"`), []byte(`"signature":"!`), 1)
	for name, candidate := range map[string][]byte{"tampered index": tampered, "invalid signature": invalidSignature} {
		t.Run(name, func(t *testing.T) {
			index, err := ParseAndVerify(candidate, map[string]ed25519.PublicKey{"catalog-fixture-1": pub}, 0)
			if !errors.Is(err, ErrSignature) {
				t.Fatalf("ParseAndVerify error = %v", err)
			}
			if index.Entries != nil || len(Filter(index, map[string]struct{}{"filmtv": {}})) != 0 {
				t.Fatalf("ParseAndVerify returned usable index: %#v", index)
			}
		})
	}
}

func TestShippedCatalogContainsOnlyFirstPartyPlugins(t *testing.T) {
	raw, err := os.ReadFile("testdata/catalog-index.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var signed SignedIndex
	if err := json.Unmarshal(raw, &signed); err != nil {
		t.Fatal(err)
	}
	for _, entry := range signed.Index.Entries {
		if _, ok := firstPartyPluginIDs[entry.PluginID]; !ok {
			t.Errorf("shipped catalog contains non-first-party plugin ID %q", entry.PluginID)
		}
	}
}

func TestValidationRejectsNonFirstPartyPlugin(t *testing.T) {
	index := Index{
		SchemaVersion: SchemaVersion,
		Sequence:      1,
		IssuedAt:      time.Unix(1, 0),
		Entries: []Entry{{
			PluginID:       "third-party-example",
			Version:        "1.0.0",
			SDKFloor:       "1.0.0",
			SDKCeiling:     "1.0.0",
			ArtifactSHA256: strings.Repeat("a", 64),
			DeliveryRef:    "delivery-third-party-1",
		}},
	}
	if err := validate(index); !errors.Is(err, ErrInvalid) {
		t.Fatalf("validate error = %v, want %v", err, ErrInvalid)
	}
}

func TestUnknownSchemaAndUnknownKeyFailClosed(t *testing.T) {
	raw, err := os.ReadFile("testdata/catalog-index.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAndVerify(raw, nil, 0); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("unknown key error = %v", err)
	}
	pub, _ := base64.RawURLEncoding.DecodeString(fixturePublicKey)
	mutated := []byte(string(raw))
	needle := []byte(`"schema_version":1`)
	for i := 0; i+len(needle) <= len(mutated); i++ {
		if string(mutated[i:i+len(needle)]) == string(needle) {
			mutated[i+len(needle)-1] = '2'
			break
		}
	}
	if _, err := ParseAndVerify(mutated, map[string]ed25519.PublicKey{"catalog-fixture-1": pub}, 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("schema error = %v", err)
	}
}

func FuzzParseAndVerify(f *testing.F) {
	raw, err := os.ReadFile("testdata/catalog-index.v1.json")
	if err != nil {
		f.Fatal(err)
	}
	pub, err := base64.RawURLEncoding.DecodeString(fixturePublicKey)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(raw)
	f.Add([]byte(`{"index":null}`))
	f.Add([]byte("not json"))
	roots := map[string]ed25519.PublicKey{"catalog-fixture-1": pub}
	f.Fuzz(func(t *testing.T, candidate []byte) {
		_, _ = ParseAndVerify(candidate, roots, 0)
	})
}
