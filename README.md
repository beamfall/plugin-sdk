<p align="center">
  <img src=".github/brand/beamfall-logo.png" alt="Beamfall" width="96" height="96">
</p>

# Beamfall Plugin SDK

**The Go contract for teaching Beamfall Core a new kind of library.**
Every browse experience in [Beamfall Core](https://github.com/beamfall/core) (films and shows,
music, podcasts, audiobooks, photos, family archives) comes from a plugin built on this SDK.
A plugin declares what its media is, how to read it from disk, how to identify it, and how to
lay it out. Core does the scanning, storage, streaming, and rendering.

It is free and open source under [AGPL-3.0-only](LICENSE). Like Core, it needs no account, and it has
no telemetry.

## Why build on it

- **Describe, don't reimplement.** A plugin is mostly a declarative `Bundle`: entity kinds, typed
  fields, layouts, scrapers, importers, and labels. Core turns that into a working library, so
  your code stays small and focused on your domain.
- **Your domain stays yours.** The SDK has no domain vocabulary and no store. Words like "album",
  "season", or "episode" live in your plugin, and Core's schema stays generic.
- **Breakage shows up in your CI.** The `sdktest` harness pins your parser, scorer, bundle, and
  importer to golden vectors. A contract break fails your own tests and your tagged release, not
  a user's server.
- **One small dependency.** The module depends on the Go standard library and `golang.org/x/text`,
  and nothing else. Importing it adds almost nothing to your supply chain.

## What you get

| Package | What it provides |
| --- | --- |
| [`sdk`](docs/reference/sdk.md) (module root) | The `Bundle` manifest and its fail-closed validation (`ValidateBundle`, `ValidateBundles`, `CompatibleWithSDK`); fixed vocabularies for archetypes, artwork slots, sections, sources, field types, scrapers, permissions, and actions; `EntityKind`, `FieldSpec`, `LayoutSpec`, `ScraperSpec`, `ImporterSpec`; the `FilenameParser` and `MatchScorer` scan seams; the Migration Wizard `Source` seam; and `LocaleString` with `L()` for BCP-47 display strings. |
| [`sdktest`](docs/reference/sdktest.md) | The conformance harness: `RunBundleConformance`, `RunConformance`, `RunScoreConformance`, `RunMatchCorpus` (a hermetic matching-regression corpus), `RunImporterConformance`, `RunScraperTraceConformance`, `RequireEntityKindVocab`, the `RequireAllowedImports` static import gate, and `Admit`, the mechanical registry admission check. |
| [`scrobble`](docs/reference/scrobble.md) | A neutral scrobble dispatcher (`Dispatcher`) with retry, dead-letter, and skip handling. Your plugin supplies `Adapter`s for services such as Trakt or Last.fm; outbound requests are held to each adapter's declared origins, and `RedirectGuard` closes the redirect gap. |
| [`catalogindex`](docs/reference/catalogindex.md) | Offline Ed25519 verification and local filtering of the signed plugin catalog index. No network I/O. |
| [`compatibility`](docs/reference/compatibility.md) | The recorded compatibility matrix: which exact first-party plugin artifact passed conformance against which SDK version, and `Verify` to check a bundle against it. |

## Quick start

A plugin is its own Go module. Require the SDK by tag:

```sh
go get github.com/beamfall/plugin-sdk@v0.3.2
```

Declare a `Bundle` and implement the scan seam. This is a complete plugin for home video clips
named like `Beach Day (2019).mp4`:

```go
// Package clips is a minimal Beamfall plugin for home video clips.
package clips

import (
	"path/filepath"
	"regexp"
	"strconv"

	sdk "github.com/beamfall/plugin-sdk"
)

// Bundle is the plugin's declarative manifest. Core validates it at startup.
var Bundle = sdk.Bundle{
	ID:           "clips",
	Version:      "0.1.0",
	SDKFloor:     "0.3.2",
	SDKCeiling:   "0.3.2",
	DisplayName:  sdk.L("Home Clips"),
	MediaLabel:   sdk.L("Clip"),
	MediaClasses: []string{"video"},
	Layouts:      []sdk.LayoutSpec{{ID: "browse", Role: "browse"}},
}

var titleYear = regexp.MustCompile(`^(.+) \((\d{4})\)\.\w+$`)

// Parser turns "Beach Day (2019).mp4" into a title and a year.
type Parser struct{}

func (Parser) Parse(path string, _ sdk.ParseProbe) (sdk.ParsedMedia, bool) {
	m := titleYear.FindStringSubmatch(filepath.Base(path))
	if m == nil {
		return sdk.ParsedMedia{}, false
	}
	year, _ := strconv.Atoi(m[2])
	return sdk.ParsedMedia{Title: m[1], Year: year}, true
}
```

Then prove it against the contract in `conformance_test.go`:

```go
package clips

import (
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
	"github.com/beamfall/plugin-sdk/sdktest"
)

func TestConformance(t *testing.T) {
	sdktest.RunBundleConformance(t, Bundle)
	sdktest.RunConformance(t, Parser{}, []sdktest.ParseVector{
		{
			Name:   "title and year",
			Path:   "/media/clips/Beach Day (2019).mp4",
			Want:   sdk.ParsedMedia{Title: "Beach Day", Year: 2019},
			WantOK: true,
		},
		{Name: "no year", Path: "/media/clips/notes.txt", WantOK: false},
	})
}
```

`go test ./...` now checks the bundle, the parser's golden vectors and determinism, and the
plugin's imports. From here, add entity kinds, scrapers, a `MatchScorer`, or an importer as your
library needs them. The [plugin authoring guide](docs/plugin-authoring.md) walks through each one.

## How a plugin fits

```
        Beamfall Core
                 │  imports
                 ▼
        plugin-sdk  ◄── imports ── plugin-{filmtv,music,podcasts,…}
                 │
        (Go standard library + golang.org/x/text only)
```

Core and every plugin import the SDK; neither imports the other. A plugin is compiled into Core,
not loaded at runtime. Core reads the plugin's `Bundle`, calls its seams, and maps the SDK's types
to its own storage on its side of the boundary.

The plugin catalog currently admits first-party plugins only. See the
[registry admission runbook](docs/registry-admission-runbook.md) for what admission checks.

## Documentation

- [Plugin authoring guide](docs/plugin-authoring.md): concepts, seams, and the conformance workflow.
- API reference, generated from the Go doc comments: [`sdk`](docs/reference/sdk.md),
  [`sdktest`](docs/reference/sdktest.md), [`scrobble`](docs/reference/scrobble.md),
  [`catalogindex`](docs/reference/catalogindex.md), and
  [`compatibility`](docs/reference/compatibility.md).
- [Registry admission runbook](docs/registry-admission-runbook.md): the mechanical gates
  (`sdktest.Admit`) and the manual legality and malware review.
- [`llms.txt`](llms.txt) and [`llms-full.txt`](llms-full.txt): the same documentation for
  language-model tools.

## Compatibility and versioning

The SDK is pre-1.0. Patch releases keep existing APIs source-compatible; minor releases may break
them, and ship with coordinated first-party plugin updates. Pin the SDK by tag in `go.mod` and set
`Bundle.SDKFloor` and `Bundle.SDKCeiling` to the range your plugin supports. Committed plugin
modules must not `replace` the SDK. The full policy and release checks are in
[COMPATIBILITY.md](COMPATIBILITY.md).

## First-party plugins

These plugins are built on this SDK:

- [plugin-filmtv](https://github.com/beamfall/plugin-filmtv): films and TV shows
- [plugin-music](https://github.com/beamfall/plugin-music): music
- [plugin-podcasts](https://github.com/beamfall/plugin-podcasts): podcasts
- [plugin-photos](https://github.com/beamfall/plugin-photos): photos
- [plugin-audiobook](https://github.com/beamfall/plugin-audiobook): audiobooks
- [plugin-familyarchive](https://github.com/beamfall/plugin-familyarchive): family archives
- [plugin-enrichment](https://github.com/beamfall/plugin-enrichment): deterministic content fingerprints
- [plugin-karaoke](https://github.com/beamfall/plugin-karaoke): karaoke libraries with timed lyrics

They are the best working examples of every seam in this SDK.

## Developing the SDK

You need Go at the version in `go.mod`, and `golangci-lint` 2.12.2 for the full local gate.

```sh
script/gate.sh      # formatting, vet, tests, per-package coverage floors, lint
script/gen-docs.sh  # regenerate docs/reference and llms*.txt after a doc-comment or API change
```

The single-dependency rule is part of the contract: `go.mod` requires `golang.org/x/text` and
nothing else, and no other third-party import may be added.

## Contributing

Contributions require a signed Contributor License Agreement. Read [CONTRIBUTING.md](CONTRIBUTING.md)
and [CLA.md](CLA.md) before you open a pull request.

## Security

Do not report vulnerabilities in public issues. Follow [SECURITY.md](SECURITY.md).

## License

GNU Affero General Public License v3.0 only (AGPL-3.0-only). See [LICENSE](LICENSE).
