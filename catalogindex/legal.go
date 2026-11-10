package catalogindex

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"
)

// DispositionApproved is the only rights-attestation disposition that admits
// an entry. Any other value is rejected.
const DispositionApproved = "approved"

// attestationDomain separates rights-attestation signatures from every other
// signature a reviewer key might make.
const attestationDomain = "beamfall-catalog-rights-attestation/v1"

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

// Legal is the licensing, notice, provenance and rights metadata every
// catalog entry carries. All digests are lowercase hex SHA-256 of the exact
// bytes they name.
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

// Notice names one notice file by its path in the source tree and its digest.
type Notice struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// RightsAttestation is the signed disposition of the policy reviewer named
// in the signed policy keyring. The reviewer identity and review date are
// data from the catalog operator's signed policy and index, never constants
// of this package. SHA256 is the digest of the attestation statement (see
// AttestationStatement) and Signature the reviewer's Ed25519 signature over
// it, base64url without padding.
type RightsAttestation struct {
	ReviewerKeyID    string    `json:"reviewer_key_id"`
	ReviewerIdentity string    `json:"reviewer_identity"`
	ReviewedAt       time.Time `json:"reviewed_at"`
	Disposition      string    `json:"disposition"`
	SHA256           string    `json:"sha256"`
	Signature        string    `json:"signature"`
}

// Policy is the catalog operator's licence policy. It is data: the SDK
// ships none and decides nothing about licence compatibility. The operator
// signs it (see SignPolicy) and every index is bound to its digest.
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

// PolicyLicense is one allowed SPDX expression and its source-offer rule.
type PolicyLicense struct {
	Expression          string `json:"expression"`
	SourceOfferRequired bool   `json:"source_offer_required"`
}

// PolicyReviewer is one keyring member: a key id, the reviewer identity the
// operator names, and the Ed25519 public key (base64url, no padding).
type PolicyReviewer struct {
	KeyID     string `json:"key_id"`
	Identity  string `json:"identity"`
	PublicKey string `json:"public_key"`
}

// SignedPolicy is a Policy plus the registry signature over it and KeyID.
type SignedPolicy struct {
	Policy        Policy `json:"policy"`
	RegistryKeyID string `json:"registry_key_id"`
	Signature     string `json:"signature"`
}

type policySigningPayload struct {
	Policy        Policy `json:"policy"`
	RegistryKeyID string `json:"registry_key_id"`
}

// attestationStatement is what a reviewer signs: every legal field of the
// entry, the release it describes, the policy it was judged under, and the
// reviewer's own identity, date and disposition.
type attestationStatement struct {
	Domain            string    `json:"domain"`
	PluginID          string    `json:"plugin_id"`
	Version           string    `json:"version"`
	ArtifactSHA256    string    `json:"artifact_sha256"`
	PolicySHA256      string    `json:"policy_sha256"`
	LicenseExpression string    `json:"license_expression"`
	Notices           []Notice  `json:"notices"`
	SourceRepository  string    `json:"source_repository"`
	SourceCommit      string    `json:"source_commit"`
	SourceSHA256      string    `json:"source_sha256"`
	SourceOffer       string    `json:"source_offer"`
	ProvenanceSHA256  string    `json:"provenance_sha256"`
	ReviewerKeyID     string    `json:"reviewer_key_id"`
	ReviewerIdentity  string    `json:"reviewer_identity"`
	ReviewedAt        time.Time `json:"reviewed_at"`
	Disposition       string    `json:"disposition"`
}

// PolicyDigest returns the lowercase hex SHA-256 of the policy's canonical
// JSON. Index.PolicySHA256 and every attestation statement carry it.
func PolicyDigest(policy Policy) (string, error) {
	encoded, err := canonicalJSON(policy)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

// AttestationStatement returns the canonical bytes a policy reviewer signs
// for entry under the policy with digest policySHA256. The reviewer fields
// (key id, identity, date, disposition) are read from
// entry.Legal.RightsAttestation; its SHA256 and Signature are not part of
// the statement.
func AttestationStatement(entry Entry, policySHA256 string) ([]byte, error) {
	legal := entry.Legal
	att := legal.RightsAttestation
	return canonicalJSON(attestationStatement{
		Domain:            attestationDomain,
		PluginID:          entry.PluginID,
		Version:           entry.Version,
		ArtifactSHA256:    entry.ArtifactSHA256,
		PolicySHA256:      policySHA256,
		LicenseExpression: legal.LicenseExpression,
		Notices:           legal.Notices,
		SourceRepository:  legal.SourceRepository,
		SourceCommit:      legal.SourceCommit,
		SourceSHA256:      legal.SourceSHA256,
		SourceOffer:       legal.SourceOffer,
		ProvenanceSHA256:  legal.ProvenanceSHA256,
		ReviewerKeyID:     att.ReviewerKeyID,
		ReviewerIdentity:  att.ReviewerIdentity,
		ReviewedAt:        att.ReviewedAt,
		Disposition:       att.Disposition,
	})
}

// VerifyEntry verifies one entry outside a full index, as registry admission
// does before publication: the signed policy against roots, the entry's
// release fields, and its legal metadata against that policy as of at. It
// does not apply the first-party allow-set, which ParseAndVerify enforces on
// the published index.
func VerifyEntry(entry Entry, policy SignedPolicy, roots map[string]ed25519.PublicKey, at time.Time) error {
	digest, err := verifyPolicy(policy, roots)
	if err != nil {
		return err
	}
	if at.IsZero() || policy.Policy.IssuedAt.After(at) {
		return fmt.Errorf("%w: policy issued after the verification time", ErrPolicy)
	}
	if err := validateEntry(entry); err != nil {
		return err
	}
	return verifyLegal(entry, policy.Policy, digest, at)
}

// EntryBytes are the exact bytes an entry's digests name.
type EntryBytes struct {
	Artifact   []byte
	Source     []byte
	Provenance []byte
	// Notices maps each notice path to its file bytes.
	Notices map[string][]byte
}

// VerifyEntryBytes reports ErrDigestMismatch unless the artifact, source
// archive, provenance statement and exactly the entry's notice files hash to
// the entry's digests.
func VerifyEntryBytes(entry Entry, b EntryBytes) error {
	checks := []struct {
		name string
		data []byte
		want string
	}{
		{"artifact", b.Artifact, entry.ArtifactSHA256},
		{"source archive", b.Source, entry.Legal.SourceSHA256},
		{"provenance statement", b.Provenance, entry.Legal.ProvenanceSHA256},
	}
	for _, c := range checks {
		if c.data == nil || sha256Hex(c.data) != c.want {
			return fmt.Errorf("%w: plugin %s: %s", ErrDigestMismatch, entry.PluginID, c.name)
		}
	}
	if len(b.Notices) != len(entry.Legal.Notices) {
		return fmt.Errorf("%w: plugin %s: %d notice files supplied, entry names %d", ErrDigestMismatch, entry.PluginID, len(b.Notices), len(entry.Legal.Notices))
	}
	for _, notice := range entry.Legal.Notices {
		data, ok := b.Notices[notice.Path]
		if !ok || sha256Hex(data) != notice.SHA256 {
			return fmt.Errorf("%w: plugin %s: notice %s", ErrDigestMismatch, entry.PluginID, notice.Path)
		}
	}
	return nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// verifyPolicy checks the policy signature against roots and the policy's
// shape, and returns its digest.
func verifyPolicy(signed SignedPolicy, roots map[string]ed25519.PublicKey) (string, error) {
	key, ok := roots[signed.RegistryKeyID]
	if !ok || len(key) != ed25519.PublicKeySize {
		return "", fmt.Errorf("%w: policy signed by unknown key %q", ErrPolicy, signed.RegistryKeyID)
	}
	if err := validatePolicy(signed.Policy); err != nil {
		return "", err
	}
	if err := verifySignature(key, policySigningPayload{Policy: signed.Policy, RegistryKeyID: signed.RegistryKeyID}, signed.Signature); err != nil {
		return "", fmt.Errorf("%w: policy signature", ErrPolicy)
	}
	return PolicyDigest(signed.Policy)
}

func validatePolicy(policy Policy) error {
	if strings.TrimSpace(policy.PolicyID) == "" || policy.IssuedAt.IsZero() || policy.AttestationMaxAgeDays <= 0 {
		return fmt.Errorf("%w: policy id, issue time and attestation max age are required", ErrPolicy)
	}
	if len(policy.Licenses) == 0 || len(policy.Reviewers) == 0 {
		return fmt.Errorf("%w: policy must list licences and reviewers", ErrPolicy)
	}
	seenExpr := make(map[string]struct{}, len(policy.Licenses))
	for _, license := range policy.Licenses {
		if err := checkSPDX(license.Expression); err != nil {
			return fmt.Errorf("%w: policy licence %q: %v", ErrPolicy, license.Expression, err)
		}
		if _, dup := seenExpr[license.Expression]; dup {
			return fmt.Errorf("%w: duplicate policy licence %q", ErrPolicy, license.Expression)
		}
		seenExpr[license.Expression] = struct{}{}
	}
	seenKey := make(map[string]struct{}, len(policy.Reviewers))
	for _, reviewer := range policy.Reviewers {
		if strings.TrimSpace(reviewer.KeyID) == "" || strings.TrimSpace(reviewer.Identity) == "" {
			return fmt.Errorf("%w: reviewer key id and identity are required", ErrPolicy)
		}
		if _, dup := seenKey[reviewer.KeyID]; dup {
			return fmt.Errorf("%w: duplicate reviewer key id %q", ErrPolicy, reviewer.KeyID)
		}
		seenKey[reviewer.KeyID] = struct{}{}
		if _, err := reviewerKey(reviewer); err != nil {
			return err
		}
	}
	return nil
}

func reviewerKey(reviewer PolicyReviewer) (ed25519.PublicKey, error) {
	raw, err := base64.RawURLEncoding.DecodeString(reviewer.PublicKey)
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("%w: reviewer %q public key", ErrPolicy, reviewer.KeyID)
	}
	return ed25519.PublicKey(raw), nil
}

// verifyLegal checks entry's legal metadata against an already verified
// policy with digest policySHA256, as of at.
func verifyLegal(entry Entry, policy Policy, policySHA256 string, at time.Time) error {
	legal := entry.Legal
	id := entry.PluginID
	if err := checkSPDX(legal.LicenseExpression); err != nil {
		return fmt.Errorf("%w: plugin %s: %v", ErrLicense, id, err)
	}
	var rule *PolicyLicense
	for i := range policy.Licenses {
		if policy.Licenses[i].Expression == legal.LicenseExpression {
			rule = &policy.Licenses[i]
			break
		}
	}
	if rule == nil {
		return fmt.Errorf("%w: plugin %s: %q is not in policy %s", ErrLicense, id, legal.LicenseExpression, policy.PolicyID)
	}
	if err := checkNotices(legal.Notices); err != nil {
		return fmt.Errorf("%w: plugin %s: %v", ErrNotice, id, err)
	}
	if !isHTTPSURL(legal.SourceRepository) || !isGitCommit(legal.SourceCommit) ||
		!isSHA256(legal.SourceSHA256) || !isSHA256(legal.ProvenanceSHA256) {
		return fmt.Errorf("%w: plugin %s", ErrProvenance, id)
	}
	if legal.SourceOffer != "" && !isHTTPSURL(legal.SourceOffer) {
		return fmt.Errorf("%w: plugin %s: source offer must be an https URL", ErrSourceOffer, id)
	}
	if rule.SourceOfferRequired && legal.SourceOffer == "" {
		return fmt.Errorf("%w: plugin %s: policy requires a source offer for %q", ErrSourceOffer, id, legal.LicenseExpression)
	}
	return verifyAttestation(entry, policy, policySHA256, at)
}

func verifyAttestation(entry Entry, policy Policy, policySHA256 string, at time.Time) error {
	att := entry.Legal.RightsAttestation
	id := entry.PluginID
	var reviewer *PolicyReviewer
	for i := range policy.Reviewers {
		if policy.Reviewers[i].KeyID == att.ReviewerKeyID {
			reviewer = &policy.Reviewers[i]
			break
		}
	}
	if reviewer == nil {
		return fmt.Errorf("%w: plugin %s: key %q", ErrReviewer, id, att.ReviewerKeyID)
	}
	if att.ReviewerIdentity != reviewer.Identity {
		return fmt.Errorf("%w: plugin %s: identity %q does not match keyring", ErrReviewer, id, att.ReviewerIdentity)
	}
	if att.Disposition != DispositionApproved {
		return fmt.Errorf("%w: plugin %s: disposition %q", ErrAttestation, id, att.Disposition)
	}
	maxAge := time.Duration(policy.AttestationMaxAgeDays) * 24 * time.Hour
	if att.ReviewedAt.IsZero() || att.ReviewedAt.Before(policy.IssuedAt) || att.ReviewedAt.After(at) || at.Sub(att.ReviewedAt) > maxAge {
		return fmt.Errorf("%w: plugin %s: review date outside the policy window", ErrAttestation, id)
	}
	statement, err := AttestationStatement(entry, policySHA256)
	if err != nil {
		return err
	}
	if sha256Hex(statement) != att.SHA256 {
		return fmt.Errorf("%w: plugin %s: statement digest does not match the entry", ErrAttestation, id)
	}
	key, err := reviewerKey(*reviewer)
	if err != nil {
		return err
	}
	sig, err := base64.RawURLEncoding.DecodeString(att.Signature)
	if err != nil || !ed25519.Verify(key, statement, sig) {
		return fmt.Errorf("%w: plugin %s: reviewer signature", ErrAttestation, id)
	}
	return nil
}

func checkNotices(notices []Notice) error {
	if len(notices) == 0 {
		return errors.New("no notice files")
	}
	seen := make(map[string]struct{}, len(notices))
	for _, notice := range notices {
		p := notice.Path
		if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, `\`) || path.Clean(p) != p || p == "." || p == ".." || strings.HasPrefix(p, "../") {
			return fmt.Errorf("notice path %q must be a clean relative path", p)
		}
		if _, dup := seen[p]; dup {
			return fmt.Errorf("duplicate notice path %q", p)
		}
		seen[p] = struct{}{}
		if !isSHA256(notice.SHA256) {
			return fmt.Errorf("notice %q digest", p)
		}
	}
	return nil
}

func isHTTPSURL(value string) bool {
	u, err := url.Parse(value)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && !strings.ContainsAny(value, " \t\r\n")
}

// isGitCommit reports whether value is a full lowercase SHA-1 or SHA-256
// git object name.
func isGitCommit(value string) bool {
	return isLowerHex(value, 20) || isLowerHex(value, 32)
}
