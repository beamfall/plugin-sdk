## ADDED Requirements

### Requirement: Login declarations may carry optional secrets

`LoginSpec` SHALL accept `OptionalSecretFields`, a map from a JSON body field to an `OptionalSecret`
whose AuthRef the engine sends in that field only when the secret is configured. When the secret is
not configured, the field SHALL be absent from the login body, never sent empty.

#### Scenario: Optional secret saved

- **WHEN** the owner has saved the optional secret's AuthRef
- **THEN** the login body carries the field with that secret beside the required ones

#### Scenario: Optional secret not saved

- **WHEN** the optional secret's AuthRef has no saved value
- **THEN** the login body carries only the required fields

### Requirement: Optional login secrets are validated

Validation SHALL reject an optional secret whose body field is empty or equals `SecretField` or a
`SecretFields` key, whose AuthRef is not a plain AuthRef, whose label has no `"en"` anchor, whose help
URL is not an absolute https URL, or whose help URL and help label are not declared together.

#### Scenario: Field collides with a required field

- **WHEN** an optional secret's body field is also a `SecretFields` key
- **THEN** `ValidateBundle` returns an optional secret field error
