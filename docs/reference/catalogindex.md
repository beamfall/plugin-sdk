# package catalogindex

`import "github.com/beamfall/plugin-sdk/catalogindex"`

Package catalogindex verifies and filters Beamfall's public global plugin catalog. Callers obtain the one static artifact without plugin identifiers, verify it offline, then filter it locally.

## Constants

```go
const SchemaVersion = 1
```

## Variables

```go
var (
	ErrInvalid            = errors.New("catalogindex: invalid index")
	ErrSignature          = errors.New("catalogindex: invalid signature")
	ErrUnknownKey         = errors.New("catalogindex: unknown registry key")
	ErrSequenceRegression = errors.New("catalogindex: sequence must increase monotonically")
)
```

## type Entry

```go
type Entry struct {
	PluginID       string `json:"plugin_id"`
	Version        string `json:"version"`
	SDKFloor       string `json:"sdk_floor"`
	SDKCeiling     string `json:"sdk_ceiling"`
	ArtifactSHA256 string `json:"artifact_sha256"`
	DeliveryRef    string `json:"delivery_ref"`
}
```

### func Filter

```go
func Filter(index Index, pluginIDs map[string]struct{}) []Entry
```

Filter returns only installed plugin IDs from an already verified index. It performs no I/O; plugin IDs remain local.

## type Index

```go
type Index struct {
	SchemaVersion int       `json:"schema_version"`
	Sequence      uint64    `json:"sequence"`
	IssuedAt      time.Time `json:"issued_at"`
	Entries       []Entry   `json:"entries"`
}
```

### func ParseAndVerify

```go
func ParseAndVerify(raw []byte, roots map[string]ed25519.PublicKey, lastKnownSequence uint64) (Index, error)
```

ParseAndVerify decodes a complete static catalog, verifies it against caller- supplied bundled roots, and rejects equal or older publications.

## type SignedIndex

```go
type SignedIndex struct {
	Index         Index  `json:"index"`
	RegistryKeyID string `json:"registry_key_id"`
	Signature     string `json:"signature"`
}
```

