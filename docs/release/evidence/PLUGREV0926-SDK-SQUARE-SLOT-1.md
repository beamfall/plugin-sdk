# PLUGREV0926-SDK-SQUARE-SLOT-1 (CORE-0468)

Red first, before the change to `entity.go`:

```
--- FAIL: TestValidateBundleSquareArtworkSlot (0.00s)
    bundle_test.go:1021: ArtworkSlots() does not include square
FAIL
FAIL	github.com/beamfall/plugin-sdk	0.116s
FAIL
```

Green after adding `square` to `artworkSlots`: `go test -run SquareArtwork .` passes.
Reference docs regenerated with `script/gen-docs.sh`. Gate result: see below.

Gate: `script/gate.sh` on the branch head exits 1 at the coverage-floor step with
`missing coverage floor for github.com/beamfall/plugin-sdk/internal/jsonpath`, a failure already
on origin/main (CORE-3253). Every earlier step passed (gate contracts, docs drift, release identity,
gofmt, vet, tests); `golangci-lint run ./...` reports 0 issues and strict OpenSpec validation
passes. The gate result is pending until CORE-3253 lands and the gate is rerun.
