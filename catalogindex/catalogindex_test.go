package catalogindex

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"strings"
	"testing"
	"time"
)

// legacyFixturePublicKey verifies the schema version 1 Cloud fixture.
const legacyFixturePublicKey = "eDUCARIrsMG5h2Pm3Vy9_7nMZPkTm4vdA8mWgJ7j32U"

const v2FixturePath = "testdata/catalog-index.v2.json"

// updateFixture rewrites testdata/catalog-index.v2.json from buildSigned:
// go test ./catalogindex -run TestV2FixtureIsReproducible -update-fixture
var updateFixture = flag.Bool("update-fixture", false, "rewrite "+v2FixturePath)

func legacyRoots(t testing.TB) map[string]ed25519.PublicKey {
	t.Helper()
	pub, err := base64.RawURLEncoding.DecodeString(legacyFixturePublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]ed25519.PublicKey{"catalog-fixture-1": pub}
}

func TestV2FixtureIsReproducible(t *testing.T) {
	want := append(buildDoc(t, fixtureOpts{}), '\n')
	if *updateFixture {
		if err := os.WriteFile(v2FixturePath, want, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(v2FixturePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s drifted from buildSigned; regenerate with -update-fixture", v2FixturePath)
	}
}

func TestV2FixtureVerifiesAndFiltersLocally(t *testing.T) {
	raw, err := os.ReadFile(v2FixturePath)
	if err != nil {
		t.Fatal(err)
	}
	index, err := ParseAndVerify(raw, fixtureRoots(), 7)
	if err != nil {
		t.Fatal(err)
	}
	got := Filter(index, map[string]struct{}{"music": {}})
	if len(got) != 1 || got[0].PluginID != "music" {
		t.Fatalf("Filter = %#v", got)
	}
	// Every fixture entry identifies its source bytes and carries an
	// authenticated disposition.
	for _, entry := range index.Entries {
		if err := VerifyEntryBytes(entry, fixtureBytes(entry.PluginID, entry.Version)); err != nil {
			t.Errorf("%s: %v", entry.PluginID, err)
		}
		if entry.Legal.RightsAttestation.Disposition != DispositionApproved || entry.Legal.RightsAttestation.Signature == "" {
			t.Errorf("%s: attestation %#v", entry.PluginID, entry.Legal.RightsAttestation)
		}
	}
	if _, err := ParseAndVerify(raw, fixtureRoots(), 8); !errors.Is(err, ErrSequenceRegression) {
		t.Fatalf("equal sequence error = %v", err)
	}
}

func TestRoundTripPreservesEveryField(t *testing.T) {
	signed := buildSigned(t, fixtureOpts{})
	index, err := ParseAndVerify(encodeSigned(t, signed), fixtureRoots(), 0)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := canonicalJSON(signed.Index)
	got, _ := canonicalJSON(index)
	if !bytes.Equal(want, got) {
		t.Fatalf("round trip changed the index:\nwant %s\ngot  %s", want, got)
	}
}

func TestLegacyV1IndexIsRejected(t *testing.T) {
	raw, err := os.ReadFile("testdata/catalog-index.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	index, err := ParseAndVerify(raw, legacyRoots(t), 0)
	if !errors.Is(err, ErrLegacySchema) {
		t.Fatalf("ParseAndVerify(v1) error = %v, want %v", err, ErrLegacySchema)
	}
	if index.Entries != nil {
		t.Fatalf("v1 returned usable index: %#v", index)
	}
}

func fixtureLegal(pluginID, version string) Legal {
	return fixtureEntry(pluginID, version, "unused").Legal
}

func TestMigrateV1RequiresALegalRecordPerEntry(t *testing.T) {
	raw, err := os.ReadFile("testdata/catalog-index.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	full := map[string]Legal{"filmtv": fixtureLegal("filmtv", "1.4.2"), "music": fixtureLegal("music", "2.1.0")}
	entries, err := MigrateV1(raw, legacyRoots(t), full)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].PluginID != "filmtv" || entries[0].Legal.LicenseExpression != "AGPL-3.0-only" || entries[1].DeliveryRef != "delivery-music-2" {
		t.Fatalf("MigrateV1 entries = %#v", entries)
	}

	// The migrated entries become a v2 catalog only after attestation and a
	// new registry signature.
	policy, err := SignPolicy(fixturePolicy(), fixtureRegistryKeyID, fixtureKey("registry"))
	if err != nil {
		t.Fatal(err)
	}
	digest, _ := PolicyDigest(policy.Policy)
	for i := range entries {
		att, err := SignRightsAttestation(entries[i], digest, fixtureKey("reviewer"))
		if err != nil {
			t.Fatal(err)
		}
		entries[i].Legal.RightsAttestation = att
	}
	signed, err := SignIndex(Index{SchemaVersion: SchemaVersion, Sequence: 8, IssuedAt: fixtureIssuedAt, Entries: entries}, policy, fixtureRegistryKeyID, fixtureKey("registry"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAndVerify(encodeSigned(t, signed), fixtureRoots(), 7); err != nil {
		t.Fatalf("migrated catalog: %v", err)
	}

	tampered := bytes.Replace(raw, []byte(`"sequence":7`), []byte(`"sequence":9`), 1)
	v2 := buildDoc(t, fixtureOpts{})
	for name, tc := range map[string]struct {
		raw   []byte
		roots map[string]ed25519.PublicKey
		legal map[string]Legal
		want  error
	}{
		"missing legal record": {raw, legacyRoots(t), map[string]Legal{"filmtv": full["filmtv"]}, ErrInvalid},
		"extra legal record":   {raw, legacyRoots(t), map[string]Legal{"filmtv": full["filmtv"], "music": full["music"], "photos": full["music"]}, ErrInvalid},
		"tampered v1":          {tampered, legacyRoots(t), full, ErrSignature},
		"unknown key":          {raw, nil, full, ErrUnknownKey},
		"v2 document":          {v2, fixtureRoots(), full, ErrInvalid},
		"trailing data":        {append(append([]byte(nil), raw...), []byte("{}")...), legacyRoots(t), full, ErrInvalid},
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := MigrateV1(tc.raw, tc.roots, tc.legal); !errors.Is(err, tc.want) || got != nil {
				t.Fatalf("MigrateV1 = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}

// TestLegalMetadataMutationsFailClosed breaks the signed fixture one way at
// a time. Each case is re-signed by the registry, so the rejection comes from
// the legal rule, not from the outer signature.
func TestLegalMetadataMutationsFailClosed(t *testing.T) {
	stranger := fixtureKey("not-in-keyring")
	cases := []struct {
		name string
		opts fixtureOpts
		want error
	}{
		{"missing SPDX", fixtureOpts{entry: func(e *Entry) { e.Legal.LicenseExpression = "" }}, ErrLicense},
		{"unparsable SPDX", fixtureOpts{entry: func(e *Entry) { e.Legal.LicenseExpression = "AGPL-3.0-only AND" }}, ErrLicense},
		{"non-canonical SPDX", fixtureOpts{entry: func(e *Entry) { e.Legal.LicenseExpression = "AGPL-3.0-only  " }}, ErrLicense},
		{"lower-case operator", fixtureOpts{entry: func(e *Entry) { e.Legal.LicenseExpression = "MIT and Apache-2.0" }}, ErrLicense},
		{"NOASSERTION", fixtureOpts{entry: func(e *Entry) { e.Legal.LicenseExpression = "NOASSERTION" }}, ErrLicense},
		{"NONE", fixtureOpts{entry: func(e *Entry) { e.Legal.LicenseExpression = "NONE" }}, ErrLicense},
		{"licence not in policy", fixtureOpts{entry: func(e *Entry) { e.Legal.LicenseExpression = "MIT" }}, ErrLicense},
		{"no notices", fixtureOpts{entry: func(e *Entry) { e.Legal.Notices = nil }}, ErrNotice},
		{"escaping notice path", fixtureOpts{entry: func(e *Entry) { e.Legal.Notices[0].Path = "../LICENSE" }}, ErrNotice},
		{"absolute notice path", fixtureOpts{entry: func(e *Entry) { e.Legal.Notices[0].Path = "/LICENSE" }}, ErrNotice},
		{"duplicate notice path", fixtureOpts{entry: func(e *Entry) { e.Legal.Notices[1].Path = "LICENSE" }}, ErrNotice},
		{"bad notice digest", fixtureOpts{entry: func(e *Entry) { e.Legal.Notices[0].SHA256 = "abc" }}, ErrNotice},
		{"absent required source offer", fixtureOpts{entry: func(e *Entry) { e.Legal.SourceOffer = "" }}, ErrSourceOffer},
		{"non-https source offer", fixtureOpts{entry: func(e *Entry) { e.Legal.SourceOffer = "http://example.invalid/offer" }}, ErrSourceOffer},
		{"missing source repository", fixtureOpts{entry: func(e *Entry) { e.Legal.SourceRepository = "" }}, ErrProvenance},
		{"source repository with credentials", fixtureOpts{entry: func(e *Entry) { e.Legal.SourceRepository = "https://user@example.invalid/x" }}, ErrProvenance},
		{"short source commit", fixtureOpts{entry: func(e *Entry) { e.Legal.SourceCommit = "abc123" }}, ErrProvenance},
		{"upper-case source commit", fixtureOpts{entry: func(e *Entry) { e.Legal.SourceCommit = strings.ToUpper(e.Legal.SourceCommit) }}, ErrProvenance},
		{"missing source digest", fixtureOpts{entry: func(e *Entry) { e.Legal.SourceSHA256 = "" }}, ErrProvenance},
		{"missing provenance digest", fixtureOpts{entry: func(e *Entry) { e.Legal.ProvenanceSHA256 = "" }}, ErrProvenance},
		{"reviewer outside keyring", fixtureOpts{entry: func(e *Entry) { e.Legal.RightsAttestation.ReviewerKeyID = "someone-else" }}, ErrReviewer},
		{"reviewer identity differs from keyring", fixtureOpts{entry: func(e *Entry) { e.Legal.RightsAttestation.ReviewerIdentity = "An agent" }}, ErrReviewer},
		{"disposition not approved", fixtureOpts{entry: func(e *Entry) { e.Legal.RightsAttestation.Disposition = "rejected" }}, ErrAttestation},
		{"no disposition", fixtureOpts{entry: func(e *Entry) { e.Legal.RightsAttestation.Disposition = "" }}, ErrAttestation},
		{"no review date", fixtureOpts{entry: func(e *Entry) { e.Legal.RightsAttestation.ReviewedAt = time.Time{} }}, ErrAttestation},
		{"reviewed before the policy", fixtureOpts{entry: func(e *Entry) { e.Legal.RightsAttestation.ReviewedAt = fixturePolicyIssued.Add(-time.Hour) }}, ErrAttestation},
		{"reviewed after the index", fixtureOpts{entry: func(e *Entry) { e.Legal.RightsAttestation.ReviewedAt = fixtureIssuedAt.Add(time.Hour) }}, ErrAttestation},
		{"attestation older than max age", fixtureOpts{policy: func(p *Policy) { p.AttestationMaxAgeDays = 1 }}, ErrAttestation},
		{"stale: artifact digest changed", fixtureOpts{afterAttest: func(e *Entry) { e.ArtifactSHA256 = strings.Repeat("c", 64) }}, ErrAttestation},
		{"stale: source commit changed", fixtureOpts{afterAttest: func(e *Entry) { e.Legal.SourceCommit = strings.Repeat("d", 40) }}, ErrAttestation},
		{"stale: licence changed", fixtureOpts{afterAttest: func(e *Entry) { e.Legal.LicenseExpression = fixtureOpenLicense }}, ErrAttestation},
		{"stale: version changed", fixtureOpts{afterAttest: func(e *Entry) { e.Version = "1.4.3" }}, ErrAttestation},
		{"stale: digest and signature recomputed by a stranger", fixtureOpts{reviewerKey: stranger}, ErrAttestation},
		{"stale: attested under another policy", fixtureOpts{attestUnder: strings.Repeat("e", 64)}, ErrAttestation},
		{"unsigned attestation", fixtureOpts{afterAttest: func(e *Entry) { e.Legal.RightsAttestation.Signature = "" }}, ErrAttestation},
		{"malformed attestation signature", fixtureOpts{afterAttest: func(e *Entry) { e.Legal.RightsAttestation.Signature = "!" }}, ErrAttestation},
		{"policy issued after the index", fixtureOpts{policy: func(p *Policy) { p.IssuedAt = fixtureIssuedAt.Add(time.Hour) }}, ErrPolicy},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			index, err := ParseAndVerify(buildDoc(t, tc.opts), fixtureRoots(), 0)
			if !errors.Is(err, tc.want) {
				t.Fatalf("ParseAndVerify error = %v, want %v", err, tc.want)
			}
			if index.Entries != nil {
				t.Fatalf("returned usable index: %#v", index)
			}
		})
	}
}

// TestRegistrySignatureCoversReviewerAndPolicyFields tampers with fields
// after the registry signed: every one must break the outer signature.
func TestRegistrySignatureCoversReviewerAndPolicyFields(t *testing.T) {
	cases := map[string]func(*SignedIndex){
		"reviewer identity": func(s *SignedIndex) { s.Index.Entries[0].Legal.RightsAttestation.ReviewerIdentity = "Someone else" },
		"reviewer key id":   func(s *SignedIndex) { s.Index.Entries[0].Legal.RightsAttestation.ReviewerKeyID = "reviewer-fixture-2" },
		"review date": func(s *SignedIndex) {
			s.Index.Entries[0].Legal.RightsAttestation.ReviewedAt = fixtureReviewedAt.Add(time.Second)
		},
		"policy digest":       func(s *SignedIndex) { s.Index.PolicySHA256 = strings.Repeat("f", 64) },
		"licence expression":  func(s *SignedIndex) { s.Index.Entries[1].Legal.LicenseExpression = fixtureOpenLicense },
		"source offer":        func(s *SignedIndex) { s.Index.Entries[1].Legal.SourceOffer = "https://example.invalid/other" },
		"notice digest":       func(s *SignedIndex) { s.Index.Entries[1].Legal.Notices[0].SHA256 = strings.Repeat("0", 64) },
		"attestation digest":  func(s *SignedIndex) { s.Index.Entries[0].Legal.RightsAttestation.SHA256 = strings.Repeat("1", 64) },
		"sequence":            func(s *SignedIndex) { s.Index.Sequence++ },
		"registry key id":     func(s *SignedIndex) { s.RegistryKeyID = "catalog-fixture-3" },
		"signature encoding":  func(s *SignedIndex) { s.Signature = "!" + s.Signature },
		"signature truncated": func(s *SignedIndex) { s.Signature = s.Signature[:10] },
	}
	roots := fixtureRoots()
	roots["catalog-fixture-3"] = roots[fixtureRegistryKeyID]
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			signed := buildSigned(t, fixtureOpts{})
			mutate(&signed)
			index, err := ParseAndVerify(encodeSigned(t, signed), roots, 0)
			if !errors.Is(err, ErrSignature) {
				t.Fatalf("ParseAndVerify error = %v, want %v", err, ErrSignature)
			}
			if index.Entries != nil || len(Filter(index, map[string]struct{}{"filmtv": {}})) != 0 {
				t.Fatalf("returned usable index: %#v", index)
			}
		})
	}
}

func TestEmbeddedPolicyMustBeSignedAndBound(t *testing.T) {
	t.Run("policy altered after signing", func(t *testing.T) {
		signed := buildSigned(t, fixtureOpts{})
		signed.Policy.Policy.Licenses = append(signed.Policy.Policy.Licenses, PolicyLicense{Expression: "MIT"})
		if _, err := ParseAndVerify(encodeSigned(t, signed), fixtureRoots(), 0); !errors.Is(err, ErrPolicy) {
			t.Fatalf("error = %v, want %v", err, ErrPolicy)
		}
	})
	t.Run("keyring altered after signing", func(t *testing.T) {
		signed := buildSigned(t, fixtureOpts{})
		signed.Policy.Policy.Reviewers[0].PublicKey = base64.RawURLEncoding.EncodeToString(fixtureKey("not-in-keyring").Public().(ed25519.PublicKey))
		if _, err := ParseAndVerify(encodeSigned(t, signed), fixtureRoots(), 0); !errors.Is(err, ErrPolicy) {
			t.Fatalf("error = %v, want %v", err, ErrPolicy)
		}
	})
	t.Run("policy signed by unknown key", func(t *testing.T) {
		signed := buildSigned(t, fixtureOpts{})
		signed.Policy.RegistryKeyID = "nobody"
		if _, err := ParseAndVerify(encodeSigned(t, signed), fixtureRoots(), 0); !errors.Is(err, ErrPolicy) {
			t.Fatalf("error = %v, want %v", err, ErrPolicy)
		}
	})
	t.Run("missing policy", func(t *testing.T) {
		signed := buildSigned(t, fixtureOpts{})
		signed.Policy = SignedPolicy{}
		if _, err := ParseAndVerify(encodeSigned(t, signed), fixtureRoots(), 0); !errors.Is(err, ErrPolicy) {
			t.Fatalf("error = %v, want %v", err, ErrPolicy)
		}
	})
	t.Run("index bound to another policy", func(t *testing.T) {
		signed := buildSigned(t, fixtureOpts{})
		other := fixturePolicy()
		other.PolicyID = "TEST-FIXTURE-POLICY-other"
		otherSigned, err := SignPolicy(other, fixtureRegistryKeyID, fixtureKey("registry"))
		if err != nil {
			t.Fatal(err)
		}
		signed.Policy = otherSigned
		if _, err := ParseAndVerify(encodeSigned(t, signed), fixtureRoots(), 0); !errors.Is(err, ErrPolicy) {
			t.Fatalf("error = %v, want %v", err, ErrPolicy)
		}
	})
}

func TestValidatePolicyRejectsIncompletePolicies(t *testing.T) {
	cases := map[string]func(*Policy){
		"no id":                 func(p *Policy) { p.PolicyID = " " },
		"no issue time":         func(p *Policy) { p.IssuedAt = time.Time{} },
		"no max age":            func(p *Policy) { p.AttestationMaxAgeDays = 0 },
		"no licences":           func(p *Policy) { p.Licenses = nil },
		"no reviewers":          func(p *Policy) { p.Reviewers = nil },
		"invalid expression":    func(p *Policy) { p.Licenses[0].Expression = "AGPL-3.0-only OR" },
		"ambiguous expression":  func(p *Policy) { p.Licenses[0].Expression = "NOASSERTION" },
		"duplicate expression":  func(p *Policy) { p.Licenses[1].Expression = p.Licenses[0].Expression },
		"reviewer without name": func(p *Policy) { p.Reviewers[0].Identity = "" },
		"reviewer without id":   func(p *Policy) { p.Reviewers[0].KeyID = "" },
		"duplicate reviewer":    func(p *Policy) { p.Reviewers = append(p.Reviewers, p.Reviewers[0]) },
		"short reviewer key":    func(p *Policy) { p.Reviewers[0].PublicKey = "AAAA" },
		"undecodable key":       func(p *Policy) { p.Reviewers[0].PublicKey = "!!" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			policy := fixturePolicy()
			mutate(&policy)
			if err := validatePolicy(policy); !errors.Is(err, ErrPolicy) {
				t.Fatalf("validatePolicy error = %v", err)
			}
			if _, err := SignPolicy(policy, fixtureRegistryKeyID, fixtureKey("registry")); !errors.Is(err, ErrPolicy) {
				t.Fatalf("SignPolicy error = %v", err)
			}
		})
	}
}

func TestVerifyEntryChecksOneEntryAgainstTheSignedPolicy(t *testing.T) {
	signed := buildSigned(t, fixtureOpts{})
	entry := signed.Index.Entries[0]
	if err := VerifyEntry(entry, signed.Policy, fixtureRoots(), fixtureIssuedAt); err != nil {
		t.Fatal(err)
	}
	if err := VerifyEntry(entry, signed.Policy, fixtureRoots(), time.Time{}); !errors.Is(err, ErrPolicy) {
		t.Fatalf("zero time error = %v", err)
	}
	if err := VerifyEntry(entry, signed.Policy, nil, fixtureIssuedAt); !errors.Is(err, ErrPolicy) {
		t.Fatalf("unknown root error = %v", err)
	}
	broken := entry
	broken.DeliveryRef = "a/b"
	if err := VerifyEntry(broken, signed.Policy, fixtureRoots(), fixtureIssuedAt); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid entry error = %v", err)
	}
	if err := VerifyEntry(entry, signed.Policy, fixtureRoots(), fixtureReviewedAt.AddDate(2, 0, 0)); !errors.Is(err, ErrAttestation) {
		t.Fatalf("expired attestation error = %v", err)
	}
}

func TestVerifyEntryBytesMatchesEveryDigest(t *testing.T) {
	entry := fixtureEntry("filmtv", "1.4.2", "delivery-filmtv-1")
	if err := VerifyEntryBytes(entry, fixtureBytes("filmtv", "1.4.2")); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*EntryBytes){
		"artifact":           func(b *EntryBytes) { b.Artifact = []byte("other") },
		"source":             func(b *EntryBytes) { b.Source = append(b.Source, 'x') },
		"missing provenance": func(b *EntryBytes) { b.Provenance = nil },
		"missing notice":     func(b *EntryBytes) { delete(b.Notices, "LICENSE") },
		"renamed notice": func(b *EntryBytes) {
			b.Notices["NOTICE"] = b.Notices["LICENSE"]
			delete(b.Notices, "LICENSE")
		},
		"extra notice":   func(b *EntryBytes) { b.Notices["NOTICE"] = []byte("x") },
		"notice content": func(b *EntryBytes) { b.Notices["LICENSE"] = []byte("x") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			b := fixtureBytes("filmtv", "1.4.2")
			mutate(&b)
			if err := VerifyEntryBytes(entry, b); !errors.Is(err, ErrDigestMismatch) {
				t.Fatalf("VerifyEntryBytes error = %v", err)
			}
		})
	}
}

func TestSigningHelpersRejectBadKeys(t *testing.T) {
	if _, err := SignPolicy(fixturePolicy(), "k", nil); !errors.Is(err, errPrivateKey) {
		t.Fatalf("SignPolicy error = %v", err)
	}
	if _, err := SignRightsAttestation(fixtureEntry("filmtv", "1.4.2", "d"), strings.Repeat("a", 64), ed25519.PrivateKey{1}); !errors.Is(err, errPrivateKey) {
		t.Fatalf("SignRightsAttestation error = %v", err)
	}
	if _, err := SignIndex(Index{}, SignedPolicy{}, "k", nil); !errors.Is(err, errPrivateKey) {
		t.Fatalf("SignIndex error = %v", err)
	}
}

func TestCheckSPDX(t *testing.T) {
	valid := []string{
		"MIT",
		"AGPL-3.0-only",
		"GPL-2.0+",
		"LicenseRef-Vendor-1",
		"DocumentRef-spdx-tool-1.2:LicenseRef-MIT-Style-2",
		"MIT OR Apache-2.0",
		"(MIT OR Apache-2.0) AND BSD-3-Clause",
		"GPL-2.0-or-later WITH Classpath-exception-2.0",
		"Apache-2.0 WITH AdditionRef-Vendor-Exception",
		"((MIT))",
	}
	for _, expr := range valid {
		if err := checkSPDX(expr); err != nil {
			t.Errorf("checkSPDX(%q) = %v", expr, err)
		}
	}
	invalid := []string{
		"", "AND", "MIT AND", "MIT OR OR Apache-2.0", "(MIT", "MIT)", "( MIT)", "MIT  OR Apache-2.0",
		"MIT or Apache-2.0", "MIT With Classpath-exception-2.0", "MIT WITH", "MIT WITH OR",
		"(MIT) WITH Classpath-exception-2.0", "NONE", "noassertion", "MIT_2", "LicenseRef-", "LicenseRef-a+",
		"DocumentRef-x:MIT", "Doc:LicenseRef-a", "MIT\tOR Apache-2.0", "MIT ÖR Apache-2.0", "MIT Apache-2.0",
		strings.Repeat("A", maxSPDXLength+1),
	}
	for _, expr := range invalid {
		if err := checkSPDX(expr); err == nil {
			t.Errorf("checkSPDX(%q) accepted", expr)
		}
	}
}

func TestShippedCatalogContainsOnlyFirstPartyPlugins(t *testing.T) {
	for _, path := range []string{"testdata/catalog-index.v1.json", v2FixturePath} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var signed struct {
			Index struct {
				Entries []struct {
					PluginID string `json:"plugin_id"`
				} `json:"entries"`
			} `json:"index"`
		}
		if err := json.Unmarshal(raw, &signed); err != nil {
			t.Fatal(err)
		}
		for _, entry := range signed.Index.Entries {
			if _, ok := firstPartyPluginIDs[entry.PluginID]; !ok {
				t.Errorf("%s contains non-first-party plugin ID %q", path, entry.PluginID)
			}
		}
	}
}

func TestValidationRejectsNonFirstPartyPlugin(t *testing.T) {
	raw := buildDoc(t, fixtureOpts{entry: func(e *Entry) { e.PluginID = "third-party-example" }})
	if _, err := ParseAndVerify(raw, fixtureRoots(), 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ParseAndVerify error = %v, want %v", err, ErrInvalid)
	}
}

func TestIndexShapeFailsClosed(t *testing.T) {
	cases := map[string]func(*Index){
		"duplicate plugin":    func(i *Index) { i.Entries[1].PluginID = i.Entries[0].PluginID },
		"duplicate delivery":  func(i *Index) { i.Entries[1].DeliveryRef = i.Entries[0].DeliveryRef },
		"delivery URL":        func(i *Index) { i.Entries[0].DeliveryRef = "https://x" },
		"upper-case digest":   func(i *Index) { i.Entries[0].ArtifactSHA256 = strings.ToUpper(i.Entries[0].ArtifactSHA256) },
		"blank version":       func(i *Index) { i.Entries[0].Version = " " },
		"floor above ceiling": func(i *Index) { i.Entries[0].SDKFloor = "2.0.0" },
		"bad ceiling":         func(i *Index) { i.Entries[0].SDKCeiling = "01.0.0" },
		"zero sequence":       func(i *Index) { i.Sequence = 0 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseAndVerify(buildDoc(t, fixtureOpts{index: mutate}), fixtureRoots(), 0); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

func TestUnknownSchemaAndUnknownKeyFailClosed(t *testing.T) {
	raw := buildDoc(t, fixtureOpts{})
	if _, err := ParseAndVerify(raw, nil, 0); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("unknown key error = %v", err)
	}
	mutated := bytes.Replace(raw, []byte(`"schema_version":2`), []byte(`"schema_version":3`), 1)
	if bytes.Equal(mutated, raw) {
		t.Fatal("fixture has no schema_version 2")
	}
	if _, err := ParseAndVerify(mutated, fixtureRoots(), 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("schema error = %v", err)
	}
	unknownField := bytes.Replace(raw, []byte(`"policy_sha256"`), []byte(`"extra":1,"policy_sha256"`), 1)
	if _, err := ParseAndVerify(unknownField, fixtureRoots(), 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown field error = %v", err)
	}
	if _, err := ParseAndVerify(append(append([]byte(nil), raw...), []byte("{}")...), fixtureRoots(), 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("trailing data error = %v", err)
	}
}

func FuzzParseAndVerify(f *testing.F) {
	raw, err := os.ReadFile(v2FixturePath)
	if err != nil {
		f.Fatal(err)
	}
	legacy, err := os.ReadFile("testdata/catalog-index.v1.json")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(raw)
	f.Add(legacy)
	f.Add([]byte(`{"index":null}`))
	f.Add([]byte("not json"))
	roots := fixtureRoots()
	f.Fuzz(func(t *testing.T, candidate []byte) {
		index, err := ParseAndVerify(candidate, roots, 0)
		if err != nil && index.Entries != nil {
			t.Fatalf("error %v returned entries", err)
		}
	})
}
