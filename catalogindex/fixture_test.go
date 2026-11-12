package catalogindex

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"
)

// Everything in this file is a TEST FIXTURE. The keys are derived from
// public seeds, the policy is not a licence decision, and the reviewer is not
// a person. Production policy, keyring and keys belong to the catalog
// operator and never live in this repository.

const (
	fixtureRegistryKeyID = "catalog-fixture-2"
	fixtureReviewerKeyID = "reviewer-fixture-1"
	fixtureReviewer      = "Test fixture reviewer (not a real person)"
	fixturePolicyID      = "TEST-FIXTURE-POLICY-not-for-production"
	// fixtureOpenLicense is a test-only LicenseRef the fixture policy lists
	// without a source-offer requirement.
	fixtureOpenLicense = "LicenseRef-Test-Fixture-No-Offer"
)

var (
	fixturePolicyIssued = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	fixtureReviewedAt   = time.Date(2026, 10, 15, 9, 30, 0, 0, time.UTC)
	fixtureIssuedAt     = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
)

func fixtureKey(label string) ed25519.PrivateKey {
	seed := sha256.Sum256([]byte("beamfall catalogindex TEST FIXTURE key: " + label))
	return ed25519.NewKeyFromSeed(seed[:])
}

func fixtureRoots() map[string]ed25519.PublicKey {
	return map[string]ed25519.PublicKey{fixtureRegistryKeyID: fixtureKey("registry").Public().(ed25519.PublicKey)}
}

func fixturePolicy() Policy {
	return Policy{
		PolicyID: fixturePolicyID,
		IssuedAt: fixturePolicyIssued,
		Licenses: []PolicyLicense{
			{Expression: "AGPL-3.0-only", SourceOfferRequired: true},
			{Expression: fixtureOpenLicense},
		},
		Reviewers: []PolicyReviewer{{
			KeyID:     fixtureReviewerKeyID,
			Identity:  fixtureReviewer,
			PublicKey: base64.RawURLEncoding.EncodeToString(fixtureKey("reviewer").Public().(ed25519.PublicKey)),
		}},
		AttestationMaxAgeDays: 365,
	}
}

// fixtureBytes are the source bytes each fixture entry's digests identify.
func fixtureBytes(pluginID, version string) EntryBytes {
	tag := pluginID + " " + version
	return EntryBytes{
		Artifact:   []byte("TEST FIXTURE artifact bytes for " + tag + "\n"),
		Source:     []byte("TEST FIXTURE source archive bytes for " + tag + "\n"),
		Provenance: []byte(`{"fixture":"TEST FIXTURE build provenance for ` + tag + `"}` + "\n"),
		Notices: map[string][]byte{
			"LICENSE":                []byte("TEST FIXTURE licence notice for " + tag + "\n"),
			"THIRD_PARTY_NOTICES.md": []byte("TEST FIXTURE third-party notices for " + tag + "\n"),
		},
	}
}

// fixtureEntry returns an unattested entry whose digests name fixtureBytes.
func fixtureEntry(pluginID, version, deliveryRef string) Entry {
	b := fixtureBytes(pluginID, version)
	return Entry{
		PluginID:       pluginID,
		Version:        version,
		SDKFloor:       "1.0.0",
		SDKCeiling:     "1.9.0",
		ArtifactSHA256: sha256Hex(b.Artifact),
		DeliveryRef:    deliveryRef,
		Legal: Legal{
			LicenseExpression: "AGPL-3.0-only",
			Notices: []Notice{
				{Path: "LICENSE", SHA256: sha256Hex(b.Notices["LICENSE"])},
				{Path: "THIRD_PARTY_NOTICES.md", SHA256: sha256Hex(b.Notices["THIRD_PARTY_NOTICES.md"])},
			},
			SourceRepository: "https://example.invalid/fixture/plugin-" + pluginID,
			SourceCommit:     sha256Hex([]byte("commit " + pluginID))[:40],
			SourceSHA256:     sha256Hex(b.Source),
			SourceOffer:      "https://example.invalid/fixture/source-offer",
			ProvenanceSHA256: sha256Hex(b.Provenance),
			RightsAttestation: RightsAttestation{
				ReviewerKeyID:    fixtureReviewerKeyID,
				ReviewerIdentity: fixtureReviewer,
				ReviewedAt:       fixtureReviewedAt,
				Disposition:      DispositionApproved,
			},
		},
	}
}

// fixtureOpts mutate one stage of fixture construction.
type fixtureOpts struct {
	policy      func(*Policy) // before the policy is signed
	entry       func(*Entry)  // before the attestation is signed
	afterAttest func(*Entry)  // after the attestation, before the index is signed
	index       func(*Index)  // before the index is signed
	reviewerKey ed25519.PrivateKey
	attestUnder string // policy digest the reviewer signs under; default the real one
}

func buildSigned(t testing.TB, o fixtureOpts) SignedIndex {
	t.Helper()
	policy := fixturePolicy()
	if o.policy != nil {
		o.policy(&policy)
	}
	signedPolicy, err := SignPolicy(policy, fixtureRegistryKeyID, fixtureKey("registry"))
	if err != nil {
		t.Fatal(err)
	}
	digest, err := PolicyDigest(policy)
	if err != nil {
		t.Fatal(err)
	}
	reviewerKey := o.reviewerKey
	if reviewerKey == nil {
		reviewerKey = fixtureKey("reviewer")
	}
	attestUnder := o.attestUnder
	if attestUnder == "" {
		attestUnder = digest
	}
	entries := []Entry{
		fixtureEntry("filmtv", "1.4.2", "delivery-filmtv-1"),
		fixtureEntry("music", "2.1.0", "delivery-music-2"),
	}
	for i := range entries {
		if o.entry != nil && i == 0 {
			o.entry(&entries[i])
		}
		att, err := SignRightsAttestation(entries[i], attestUnder, reviewerKey)
		if err != nil {
			t.Fatal(err)
		}
		entries[i].Legal.RightsAttestation = att
		if o.afterAttest != nil && i == 0 {
			o.afterAttest(&entries[i])
		}
	}
	index := Index{SchemaVersion: SchemaVersion, Sequence: 8, IssuedAt: fixtureIssuedAt, Entries: entries}
	if o.index != nil {
		o.index(&index)
	}
	signed, err := SignIndex(index, signedPolicy, fixtureRegistryKeyID, fixtureKey("registry"))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func encodeSigned(t testing.TB, signed SignedIndex) []byte {
	t.Helper()
	raw, err := canonicalJSON(signed)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func buildDoc(t testing.TB, o fixtureOpts) []byte {
	t.Helper()
	return encodeSigned(t, buildSigned(t, o))
}
