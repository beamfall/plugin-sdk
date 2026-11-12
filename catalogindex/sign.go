package catalogindex

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
)

// errPrivateKey rejects a signing key of the wrong size.
var errPrivateKey = errors.New("catalogindex: invalid Ed25519 private key")

// SignPolicy validates policy and signs it with the registry key keyID. The
// SDK holds no keys; the catalog operator supplies its own.
func SignPolicy(policy Policy, keyID string, key ed25519.PrivateKey) (SignedPolicy, error) {
	if len(key) != ed25519.PrivateKeySize {
		return SignedPolicy{}, errPrivateKey
	}
	if err := validatePolicy(policy); err != nil {
		return SignedPolicy{}, err
	}
	sig, err := sign(key, policySigningPayload{Policy: policy, RegistryKeyID: keyID})
	if err != nil {
		return SignedPolicy{}, err
	}
	return SignedPolicy{Policy: policy, RegistryKeyID: keyID, Signature: sig}, nil
}

// SignRightsAttestation returns entry's rights attestation with SHA256 and
// Signature set: the reviewer key signs AttestationStatement(entry,
// policySHA256). The reviewer key id, identity, review date and disposition
// are taken from entry.Legal.RightsAttestation as the reviewer supplied them.
func SignRightsAttestation(entry Entry, policySHA256 string, key ed25519.PrivateKey) (RightsAttestation, error) {
	if len(key) != ed25519.PrivateKeySize {
		return RightsAttestation{}, errPrivateKey
	}
	statement, err := AttestationStatement(entry, policySHA256)
	if err != nil {
		return RightsAttestation{}, err
	}
	att := entry.Legal.RightsAttestation
	att.SHA256 = sha256Hex(statement)
	att.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, statement))
	return att, nil
}

// SignIndex binds index to policy (setting Index.PolicySHA256) and signs it
// with the registry key keyID. It does not verify the entries; run
// ParseAndVerify on the result before publishing it.
func SignIndex(index Index, policy SignedPolicy, keyID string, key ed25519.PrivateKey) (SignedIndex, error) {
	if len(key) != ed25519.PrivateKeySize {
		return SignedIndex{}, errPrivateKey
	}
	digest, err := PolicyDigest(policy.Policy)
	if err != nil {
		return SignedIndex{}, err
	}
	index.PolicySHA256 = digest
	sig, err := sign(key, signingPayload{Index: index, RegistryKeyID: keyID})
	if err != nil {
		return SignedIndex{}, err
	}
	return SignedIndex{Index: index, Policy: policy, RegistryKeyID: keyID, Signature: sig}, nil
}

func sign(key ed25519.PrivateKey, payload any) (string, error) {
	encoded, err := canonicalJSON(payload)
	if err != nil {
		return "", fmt.Errorf("catalogindex: encode signing payload: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, encoded)), nil
}
