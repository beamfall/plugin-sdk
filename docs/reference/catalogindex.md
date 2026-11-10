# package catalogindex

`import "github.com/beamfall/plugin-sdk/catalogindex"`

Package catalogindex verifies and filters Beamfall's public global plugin catalog. Callers obtain the one static artifact without plugin identifiers, verify it offline, then filter it locally.

Every entry carries signed legal metadata: an SPDX licence expression, notice files, the exact source and build provenance, an optional source offer and a rights attestation by a policy reviewer. What is acceptable is not decided here. The catalog operator signs a licence Policy (allowed SPDX expressions, each with its source-offer rule, the reviewer keyring and the maximum attestation age), the document embeds it, and the index payload is bound to its digest. This package ships no production policy, keyring or key; it only verifies the operator's signed data and fails closed on any omission or mismatch.

## Constants

```go
const DispositionApproved = "approved"
```

DispositionApproved is the only rights-attestation disposition that admits an entry. Any other value is rejected.

```go
const SchemaVersion = 2
```

SchemaVersion is the only catalog index schema ParseAndVerify accepts. Version 2 adds the signed legal metadata (licence, notices, source, provenance, rights attestation) and the embedded licence policy; version 1 documents carry none of it and are rejected with ErrLegacySchema.

## Variables

```go
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
```

```go
var (
	// ErrPolicy rejects a licence policy that is missing, malformed, signed
	// by a key outside the registry roots, altered after signing, or not the
	// policy the index is bound to.
	ErrPolicy = errors.New("catalogindex: invalid or unbound licence policy")
	// ErrLicense rejects a missing, unparsable, non-canonical or ambiguous
	// (NONE / NOASSERTION) SPDX expression, or one the policy does not list.
	ErrLicense = errors.New("catalogindex: licence expression missing, invalid or not allowed by policy")
	// ErrNotice rejects an entry without notice references, or with a
	// malformed notice path or digest.
	ErrNotice = errors.New("catalogindex: notice reference missing or invalid")
	// ErrSourceOffer rejects a missing source offer the policy requires for
	// the entry's licence expression, or a malformed one.
	ErrSourceOffer = errors.New("catalogindex: source offer missing or invalid")
	// ErrProvenance rejects missing or malformed source repository, source
	// commit, source-archive digest or build-provenance digest.
	ErrProvenance = errors.New("catalogindex: source or provenance metadata missing or invalid")
	// ErrReviewer rejects a rights attestation whose reviewer key is not in
	// the policy keyring, or whose identity differs from the keyring's.
	ErrReviewer = errors.New("catalogindex: rights reviewer outside the policy keyring")
	// ErrAttestation rejects a rights attestation that is unsigned, signed
	// over different fields (stale), out of date, or not approved.
	ErrAttestation = errors.New("catalogindex: rights attestation unsigned, stale or not approved")
	// ErrDigestMismatch rejects artifact, source, provenance or notice bytes
	// that do not hash to the entry's recorded digest.
	ErrDigestMismatch = errors.New("catalogindex: bytes do not match the entry digest")
)
```

## func AttestationStatement

```go
func AttestationStatement(entry Entry, policySHA256 string) ([]byte, error)
```

AttestationStatement returns the canonical bytes a policy reviewer signs for entry under the policy with digest policySHA256. The reviewer fields (key id, identity, date, disposition) are read from entry.Legal.RightsAttestation; its SHA256 and Signature are not part of the statement.

## func PolicyDigest

```go
func PolicyDigest(policy Policy) (string, error)
```

PolicyDigest returns the lowercase hex SHA-256 of the policy's canonical JSON. Index.PolicySHA256 and every attestation statement carry it.

## func VerifyEntry

```go
func VerifyEntry(entry Entry, policy SignedPolicy, roots map[string]ed25519.PublicKey, at time.Time) error
```

VerifyEntry verifies one entry outside a full index, as registry admission does before publication: the signed policy against roots, the entry's release fields, and its legal metadata against that policy as of at. It does not apply the first-party allow-set, which ParseAndVerify enforces on the published index.

## func VerifyEntryBytes

```go
func VerifyEntryBytes(entry Entry, b EntryBytes) error
```

VerifyEntryBytes reports ErrDigestMismatch unless the artifact, source archive, provenance statement and exactly the entry's notice files hash to the entry's digests.

## type Entry

```go
type Entry struct {
	PluginID       string `json:"plugin_id"`
	Version        string `json:"version"`
	SDKFloor       string `json:"sdk_floor"`
	SDKCeiling     string `json:"sdk_ceiling"`
	ArtifactSHA256 string `json:"artifact_sha256"`
	DeliveryRef    string `json:"delivery_ref"`
	Legal          Legal  `json:"legal"`
}
```

Entry is one plugin release in the catalog. Every field, including Legal, is covered by the registry signature.

### func Filter

```go
func Filter(index Index, pluginIDs map[string]struct{}) []Entry
```

Filter returns only installed plugin IDs from an already verified index. It performs no I/O; plugin IDs remain local.

### func MigrateV1

```go
func MigrateV1(raw []byte, roots map[string]ed25519.PublicKey, legal map[string]Legal) ([]Entry, error)
```

MigrateV1 is the only migration path for an existing schema version 1 index. It verifies the v1 document's registry signature against roots, then returns its entries as v2 entries carrying legal\[plugin ID]. It fails closed when any v1 entry has no legal record or legal names a plugin the v1 index does not list: nothing is grandfathered. The result is not an admitted catalog: the operator still has every entry attested under its signed policy (SignRightsAttestation), signs a new v2 index at a higher sequence (SignIndex) and checks it with ParseAndVerify.

## type EntryBytes

```go
type EntryBytes struct {
	Artifact   []byte
	Source     []byte
	Provenance []byte
	// Notices maps each notice path to its file bytes.
	Notices map[string][]byte
}
```

EntryBytes are the exact bytes an entry's digests name.

## type Index

```go
type Index struct {
	SchemaVersion int       `json:"schema_version"`
	Sequence      uint64    `json:"sequence"`
	IssuedAt      time.Time `json:"issued_at"`
	PolicySHA256  string    `json:"policy_sha256"`
	Entries       []Entry   `json:"entries"`
}
```

Index is the signed catalog payload. PolicySHA256 binds the index to the licence policy embedded in the SignedIndex document (see PolicyDigest).

### func ParseAndVerify

```go
func ParseAndVerify(raw []byte, roots map[string]ed25519.PublicKey, lastKnownSequence uint64) (Index, error)
```

ParseAndVerify decodes a complete static catalog, verifies it against caller- supplied bundled roots, and rejects equal or older publications. It verifies, in order: the schema version, the sequence, the registry signature over the index (which covers the policy digest and every entry's legal fields), the embedded licence policy's own signature and its digest binding, and then every entry's legal metadata against that policy. Any failure returns an empty Index.

## type Legal

```go
type Legal struct {
	// LicenseExpression is the artifact's SPDX licence expression in
	// canonical form (single spaces, upper-case operators). It must equal
	// one of the signed policy's allowed expressions exactly.
	LicenseExpression string `json:"license_expression"`
	// Notices are the copyright, LICENSE and NOTICE files in the source tree
	// that must accompany the artifact.
	Notices []Notice `json:"notices"`
	// SourceRepository is the https URL of the corresponding source.
	SourceRepository string `json:"source_repository"`
	// SourceCommit is the full lowercase hex commit id (SHA-1 or SHA-256).
	SourceCommit string `json:"source_commit"`
	// SourceSHA256 is the digest of the source archive at SourceCommit.
	SourceSHA256 string `json:"source_sha256"`
	// SourceOffer is the https URL of the written source offer. Required
	// when the policy marks the licence expression source_offer_required.
	SourceOffer string `json:"source_offer,omitempty"`
	// ProvenanceSHA256 is the digest of the build-provenance statement that
	// binds SourceCommit to the artifact.
	ProvenanceSHA256 string `json:"provenance_sha256"`
	// RightsAttestation is the policy reviewer's signed disposition.
	RightsAttestation RightsAttestation `json:"rights_attestation"`
}
```

Legal is the licensing, notice, provenance and rights metadata every catalog entry carries. All digests are lowercase hex SHA-256 of the exact bytes they name.

## type Notice

```go
type Notice struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
```

Notice names one notice file by its path in the source tree and its digest.

## type Policy

```go
type Policy struct {
	PolicyID string    `json:"policy_id"`
	IssuedAt time.Time `json:"issued_at"`
	// Licenses are the allowed SPDX expressions, matched exactly.
	Licenses []PolicyLicense `json:"licenses"`
	// Reviewers is the keyring of policy reviewers whose rights
	// attestations are accepted.
	Reviewers []PolicyReviewer `json:"reviewers"`
	// AttestationMaxAgeDays bounds how old a rights attestation may be
	// relative to the index (or admission) time.
	AttestationMaxAgeDays int `json:"attestation_max_age_days"`
}
```

Policy is the catalog operator's licence policy. It is data: the SDK ships none and decides nothing about licence compatibility. The operator signs it (see SignPolicy) and every index is bound to its digest.

## type PolicyLicense

```go
type PolicyLicense struct {
	Expression          string `json:"expression"`
	SourceOfferRequired bool   `json:"source_offer_required"`
}
```

PolicyLicense is one allowed SPDX expression and its source-offer rule.

## type PolicyReviewer

```go
type PolicyReviewer struct {
	KeyID     string `json:"key_id"`
	Identity  string `json:"identity"`
	PublicKey string `json:"public_key"`
}
```

PolicyReviewer is one keyring member: a key id, the reviewer identity the operator names, and the Ed25519 public key (base64url, no padding).

## type RightsAttestation

```go
type RightsAttestation struct {
	ReviewerKeyID    string    `json:"reviewer_key_id"`
	ReviewerIdentity string    `json:"reviewer_identity"`
	ReviewedAt       time.Time `json:"reviewed_at"`
	Disposition      string    `json:"disposition"`
	SHA256           string    `json:"sha256"`
	Signature        string    `json:"signature"`
}
```

RightsAttestation is the signed disposition of the policy reviewer named in the signed policy keyring. The reviewer identity and review date are data from the catalog operator's signed policy and index, never constants of this package. SHA256 is the digest of the attestation statement (see AttestationStatement) and Signature the reviewer's Ed25519 signature over it, base64url without padding.

### func SignRightsAttestation

```go
func SignRightsAttestation(entry Entry, policySHA256 string, key ed25519.PrivateKey) (RightsAttestation, error)
```

SignRightsAttestation returns entry's rights attestation with SHA256 and Signature set: the reviewer key signs AttestationStatement(entry, policySHA256). The reviewer key id, identity, review date and disposition are taken from entry.Legal.RightsAttestation as the reviewer supplied them.

## type SignedIndex

```go
type SignedIndex struct {
	Index         Index        `json:"index"`
	Policy        SignedPolicy `json:"policy"`
	RegistryKeyID string       `json:"registry_key_id"`
	Signature     string       `json:"signature"`
}
```

SignedIndex is the complete static catalog document: the index, the operator-signed licence policy it is bound to, and the registry signature over the index and RegistryKeyID.

### func SignIndex

```go
func SignIndex(index Index, policy SignedPolicy, keyID string, key ed25519.PrivateKey) (SignedIndex, error)
```

SignIndex binds index to policy (setting Index.PolicySHA256) and signs it with the registry key keyID. It does not verify the entries; run ParseAndVerify on the result before publishing it.

## type SignedPolicy

```go
type SignedPolicy struct {
	Policy        Policy `json:"policy"`
	RegistryKeyID string `json:"registry_key_id"`
	Signature     string `json:"signature"`
}
```

SignedPolicy is a Policy plus the registry signature over it and KeyID.

### func SignPolicy

```go
func SignPolicy(policy Policy, keyID string, key ed25519.PrivateKey) (SignedPolicy, error)
```

SignPolicy validates policy and signs it with the registry key keyID. The SDK holds no keys; the catalog operator supplies its own.

