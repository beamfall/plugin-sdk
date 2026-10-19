# package compatibility

`import "github.com/beamfall/plugin-sdk/compatibility"`

Package compatibility records producer-owned SDK compatibility evidence for the canonical first-party plugin set.

## func Verify

```go
func Verify(bundle sdk.Bundle, sdkVersion string) error
```

Verify rejects a bundle unless the producer matrix contains exact evidence for its identity, version, declared range, and the requested SDK version.

## type Entry

```go
type Entry struct {
	ConsumerID       sdk.ID `json:"consumerID"`
	ConsumerVersion  string `json:"consumerVersion"`
	ConsumerRevision string `json:"consumerRevision"`
	ConsumerDigest   string `json:"consumerDigest"`
	SDKVersion       string `json:"sdkVersion"`
	SDKFloor         string `json:"sdkFloor"`
	SDKCeiling       string `json:"sdkCeiling"`
	ConformanceTest  string `json:"conformanceTest"`
}
```

Entry binds one exact plugin artifact and declared SDK range to the SDK version against which its conformance test passed.

### func Entries

```go
func Entries() []Entry
```

Entries returns a copy of the canonical compatibility matrix.

