## ADDED Requirements

### Requirement: Signed legal metadata on every catalog entry

The catalog index verifier SHALL accept only schema version 2 documents in which every entry carries
an SPDX licence expression, at least one notice file reference with its SHA-256, the source
repository, commit and source-archive SHA-256, a build-provenance SHA-256 and a rights attestation,
and SHALL reject any document in which one of these is missing or malformed.

#### Scenario: Missing licence or notice

- **WHEN** an entry has no SPDX expression, an expression that does not parse, a non-canonical
  expression, `NONE` or `NOASSERTION`, or no notice reference
- **THEN** `ParseAndVerify` returns an error and no index

#### Scenario: Legacy document

- **WHEN** a schema version 1 document is verified
- **THEN** `ParseAndVerify` returns `ErrLegacySchema` and no index

### Requirement: Licence policy is signed data bound into the index

The verifier SHALL take the allowed SPDX expressions, each expression's source-offer rule, the
policy-reviewer keyring and the maximum attestation age only from an operator-signed policy embedded
in the document, verified against the caller's registry roots, and SHALL require the index payload
to carry that policy's SHA-256. The SDK SHALL ship no production policy.

#### Scenario: Policy mismatch or tampering

- **WHEN** the embedded policy is unsigned, signed by an unknown key, altered after signing, or its
  digest differs from the index's policy digest
- **THEN** verification fails closed

#### Scenario: Expression outside the policy

- **WHEN** an entry's SPDX expression is not listed in the policy, or the policy requires a source
  offer for that expression and the entry has none
- **THEN** verification fails closed

### Requirement: Rights attestation by a keyring reviewer

The verifier SHALL accept an entry only when its rights attestation names a reviewer key in the
policy keyring with the keyring's identity, carries the disposition `approved`, has a statement
digest equal to the digest recomputed from the entry and policy digest, carries a valid Ed25519
signature by that reviewer over the statement, and was made no earlier than the policy's issue time,
no later than the index issue time and within the policy's maximum age.

#### Scenario: Stale or unsigned attestation

- **WHEN** any entry field covered by the statement changes after the reviewer signed, the signature
  is missing or invalid, or the review is older than the policy or the maximum age
- **THEN** verification fails closed

#### Scenario: Reviewer outside the keyring

- **WHEN** the attestation names a key id the policy keyring does not contain, or an identity that
  differs from the keyring's
- **THEN** verification fails closed

### Requirement: Admission checks legal evidence against bytes

`sdktest.Admit` SHALL run a `legal` gate that rejects a listing without a catalog entry for the same
plugin ID and version, without a verified signed policy, with an entry that fails the rules above,
or whose artifact, source-archive or notice bytes do not match the entry's digests.

#### Scenario: Digest mismatch

- **WHEN** the supplied artifact, source archive or a notice file hashes to a value other than the
  entry's
- **THEN** `Admit` rejects the listing at the `legal` gate

### Requirement: Generated documentation has no drift

The repository gate SHALL regenerate the reference and LLM documentation twice in a scratch copy and
fail when either run differs from the committed files.

#### Scenario: Stale generated docs

- **WHEN** an exported doc comment changes without `script/gen-docs.sh` being rerun
- **THEN** `script/gen-docs_test.sh` fails
