## ADDED Requirements

### Requirement: Scoped write and playback-read permission kinds

The closed Integration Plugin permission vocabulary SHALL include `write-metadata`,
`write-collections`, `write-artwork`, `write-watch-state` and `read-playback-state`, and each SHALL
have a host-owned policy that marks it high risk.

#### Scenario: New kind declared by a bundle

- **WHEN** a bundle declares one of the five kinds with the host-owned label and description
- **THEN** bundle validation accepts the permission

#### Scenario: Plugin tries to soften a new kind

- **WHEN** a bundle declares one of the five kinds with its own label or description
- **THEN** bundle validation rejects the bundle

#### Scenario: Risk is host-owned

- **WHEN** a host reads `PermissionPolicyFor` for any of the five kinds
- **THEN** the policy reports `HighRisk` as true
