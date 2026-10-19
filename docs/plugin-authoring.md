# Plugin authoring guide

How to build a Beamfall plugin module against `github.com/beamfall/plugin-sdk`.
This is the hand-authored concepts layer; exact signatures and field-level
contracts live in the generated [API reference](reference/sdk.md)
([scrobble](reference/scrobble.md), [sdktest](reference/sdktest.md),
[catalogindex](reference/catalogindex.md),
[compatibility](reference/compatibility.md)), and the
architecture + dependency invariants live in the [README](../README.md).

## The shape of a plugin

A plugin is its own Go module, compiled into Core (first-party, not loaded at
runtime). It contributes exactly four kinds of things, all declared or
implemented through SDK seams:

1. **A `Bundle`** — the declarative manifest: identity, Entity Kinds, typed
   Media fields, Layouts, importers, scrapers, actions, and vocabulary label
   maps. Core validates it with `sdk.ValidateBundle` /
   `sdk.ValidateBundles` at startup; an invalid bundle is a boot failure,
   not a runtime surprise.
2. **Scan hooks** — a `FilenameParser` (filename → `ParsedMedia` best guess)
   and a `MatchScorer` (score a scraper `Candidate` against a parse), plus the
   optional `MediaFieldParser` / `TypedMediaFieldParser` extensions for
   deriving declared Media fields from file-local metadata. Core may invoke
   each implemented hook separately for the same file in one scan pass — the
   seam has no cross-call caching, so a plugin whose hooks share expensive
   work (e.g. reading a sidecar) should read once per call and pass the
   result down internally rather than re-deriving it per hook.
3. **A Migration Wizard `Source`** (optional) — yields `ForeignItem`s from a
   predecessor server for one import run, keyed by file path and/or
   `ExternalID`s.
4. **Scrobble `Adapter`s** (optional) — concrete integrations (e.g. Trakt,
   Last.fm) implemented in the plugin module over the neutral
   `scrobble` dispatcher substrate.

Core consumes all of these through the SDK's types only. The SDK holds no
store, no rendering, and no domain vocabulary — domain labels stay in the
plugin, Core's wire schema stays generic (`Media` is the only wire entity;
plugin Entity Kinds relate to it).

## Declaring the Bundle

`Bundle` is the root of everything declarative. The load-bearing parts:

- **Identity**: `ID`, `Version`, `SDKFloor` — the minimum SDK semver the
  bundle requires; Core rejects bundles whose floor is newer than the SDK it
  embeds — and `SDKCeiling`, the required upper bound of the listing compat
  range `[SDKFloor, SDKCeiling]` that `CompatibleWithSDK` and the `sdktest.Admit` compat gate check. Both bounds must be non-empty valid
  semvers, share a major version, and satisfy `SDKFloor <= SDKCeiling`.
- **Display**: `DisplayName` and `MediaLabel` are `LocaleString` maps
  (BCP-47), and English (`"en"`) is the required fallback anchor — `L()`
  resolves through it. Clients render `MediaLabel` for the wire entity inside
  this plugin's Libraries; the schema name stays `Media`.
- **Vocabularies, not free text**: archetypes, artwork slots, section kinds,
  source kinds, field types, scraper kinds, etc. are fixed SDK vocabularies
  (`Archetypes()`, `FieldTypes()`, …). `ValidateBundle` rejects values outside
  them. New vocabulary values are SDK/client releases, not plugin releases.
- **Entity Kinds** (`EntityKind`): the plugin's domain entities (artist,
  album, series, …) with typed fields (`FieldSpec`) and relations to `Media`
  (`MediaKindName` is the reserved relation target).
- **Typed Media fields** (`MediaFields`): fields on the wire entity itself,
  assigned at parse/identification time; Core stores and serves them without
  interpreting values. A plugin that subdivides its Media declares a text
  field named `class` keyed to `MediaClassLabels` by convention.
- **Store sensitivity**: `StoreSensitivity` ("adult" /
  "acquisition-adjacent") is a declared floor that store-distributed clients
  use to suppress the plugin's surfaces; it complements, not replaces, the
  structural client-side denylist.
- **Scrapers** (`ScraperKinds()`): declarative fetch-and-map definitions —
  network kinds (`http-json`, `http-graphql`, `rss`) require an https
  endpoint + URL templates; `exif` is the lone local-file kind and runs in
  the scan pass instead of the network identification pipeline.

## Implementing the scan seam

`FilenameParser.Parse(path, probe)` returns the best-guess `ParsedMedia` for
one unidentified file (the `ParseProbe` carries scan's FFmpeg view: streams,
duration, tags). Parsing must be **deterministic** — same input, same output —
and the conformance harness enforces that. `MatchScorer.Score(candidate,
parsed)` ranks scraper search candidates against the parse; identification
policy (`IdentificationPolicy` on the Bundle) decides whether Core may act on
scores automatically (`auto`, the empty default) or routes everything to the
manual queue (`queue`).

## Implementing an importer Source

`Source` is called once per import run: `Categories()` declares which of the
fixed import categories (`CategoryWatchState`, `CategoryRatings`,
`CategoryCollections`, `CategorySubscriptions`, `CategoryItemState`) the
items carry, and `Items(ctx)` yields every `ForeignItem`. The match key
follows the item's shape: a file-keyed item resolves by its recorded file
`Path`, then by `ExternalID`s; an item carrying a `Scrobble` (a
metadata-keyed play) resolves by the scrobble metadata chain — track,
confirmed by artist, album as tiebreaker — *instead of* Path;
`Subscription` / `ItemState` items are keyed by feed URL (plus item GUID),
never by Path; and a `SmartCollection` imports as a static current-member
snapshot, surfaced on the report's distinct smart-collection→static warning
line. An item with no Path, no Scrobble, and no external IDs is recorded
unmatched. Runs are re-runnable — an error aborts the whole job rather than
half-applying. Fields outside the run's selected categories are ignored even
when populated.

## Implementing scrobble Adapters

The `scrobble` package is the engine and seam layer only: `Dispatcher`
routes `WatchedEvent`s by `Service` and `MediaClass` (`video` / `audio`),
handles retry/dead-letter/skip, and loads adapter config via the registry
(`LoadAdapters`). The plugin module contributes the `Service` constants and
`Adapter` implementations, using the seam's `SecretSource` / `LinkSource` /
`Doer` for credentials, account links, and HTTP. Concrete adapters live in
their owning plugin modules (e.g. `github.com/beamfall/plugin-music`,
`github.com/beamfall/plugin-filmtv`), never in the SDK. See
[reference/scrobble.md](reference/scrobble.md) for the exact signatures.

**What an adapter declares.** Each `Adapter` returns its outbound endpoint
origins from `DeclaredEndpoints() []string`. A bare host (no scheme) declares
the https origin on the default port; cleartext http or a non-default port
must be declared explicitly (e.g. `"http://127.0.0.1:8080"` in tests).
`LoadAdapters` wraps the injected `Doer` in an allowlist that matches the
**full origin** — scheme + host + port — so a declared https endpoint never
authorizes an http downgrade on the same host, and a request to any
undeclared origin is rejected.

**The allowlist + redirect-guard contract.** The allowlist checks the
**initial** request origin only; it cannot see redirects the `Doer` follows
internally. So a declared origin that 30x-es to an undeclared one would escape
the allowlist unless the composition root closes that gap. For the "rejects
requests to any other origin" promise to hold end-to-end, the composition root
**must** make the injected `Doer` refuse cross-origin redirects — install
`scrobble.RedirectGuard(endpoints func() []string)` on
`http.Client.CheckRedirect`, or configure the client to not follow redirects.
`RedirectGuard` applies the same scheme+host+port origin check to every
redirect hop, calling `endpoints` per hop so it always reflects the currently
declared set.

## Conformance: proving the contract in your CI

Every plugin pins golden vectors with `sdktest` in its own
`conformance_test.go`:

- `sdktest.RunBundleConformance` — Bundle + locale invariants. Every plugin
  has a Bundle, so **every plugin calls this one**.
- `sdktest.RunConformance` — golden filename-parse vectors for the
  `FilenameParser` (includes the determinism check). The vector table must
  also clear a coverage floor: at least one `WantOK: false` vector (a
  hostile input the parser correctly refuses) and at least one
  `WantOK: true` vector whose `Want` sets two or more `ParsedMedia` fields.
- `sdktest.RunScoreConformance` — golden match-score vectors for the
  `MatchScorer`. `Score` must be **deterministic** and return a value in
  [0,1]; NaN or an out-of-range value is a contract violation the runner
  fails loudly rather than picks on.
- `sdktest.RunMatchCorpus` — the matching-regression corpus:
  `MatchFamily` groups of `MatchCase` fixtures drive the parser + scorer
  through a fail-closed pick against a frozen candidate set (the winner must
  score strictly above the case's `MinScore`, defaulting to
  `DefaultMatchThreshold`; a `MinScore` outside [0,1] or NaN is a fixture
  error). Candidate popularity breaks EXACT score ties at the top only — it
  never promotes a lower score. Fixtures are hermetic: the harness swaps
  `http.DefaultTransport` for a sealing transport and reds any case that
  attempts an outbound request, even one whose error the plugin swallows.
  Because that guard mutates process-global state, corpus runs are
  serialized — do **not** call `RunMatchCorpus` under `t.Parallel()`.
- `sdktest.RunImporterConformance` — golden Migration Wizard importer vectors
  for `sdk.Source`. Each `ImporterVector` grades one Source on
  the categories it declares (from the fixed `ImportCategories` vocabulary),
  deterministic `ForeignItems`, **repeatability** (two runs must agree),
  **cancellation** (a cancelled context yields an error and no items), and
  **failure atomicity** (a `WantErr` run reports an error and emits no partial
  items). Category, determinism, cancellation, and atomicity violations each red
  with a message naming the dimension.
- `sdktest.RunScraperTraceConformance` — golden **reference traces** for
  declarative `ScraperSpec`s. `CompileScraperTrace` re-derives, purely and
  offline, what a spec *should* produce for an input: the request plan (implicit
  method, path/query substitution, auth placement, pagination) and the mapping
  outputs (response-path resolution, transforms, artwork composition, emitted
  values). It is a **reference-trace validator, not the scraper executor** —
  Core owns the executor; this is the independent oracle it is checked against,
  and it never fetches anything. A method, URL, auth-placement, pagination,
  response-path, transform, or artwork divergence each reds diagnostically.
- `sdktest.RequireEntityKindVocab` — pins the bundle's declared `EntityKind`
  name + archetype pairs (`KindVocab`) against a design of record, so a
  rename or archetype change reds the plugin's own gate instead of drifting.
- `sdktest.RequireBundlesValid` / `sdktest.RequireBundlesReject` — the
  accept and reject paths of the cross-plugin `sdk.ValidateBundles` contract
  (e.g. a colliding kind name), for tests that activate more than one
  bundle together.

The runners also invoke `sdktest.RequireAllowedImports`, a **targeted static
gate** over the plugin module's non-test Go files — NOT a full dependency
allowlist (third-party modules are reviewed via `go.mod` in registry
admission). It denies importing `net`, `net/smtp`, `os/exec`, `plugin`,
`syscall`, and `unsafe`, plus every other network-capable `net/*` package
(`net/rpc`, `net/textproto`, `net/http/httputil`, …) — the transitive-reach
guard, since the module-wide walk catches such a package pulled in through a
local helper; only the parse-only value packages `net/url` and `net/netip` are
permitted from that tree. It allows `net/http` only in registered
`scrobble.Adapter` seam files (outbound calls only inside `Adapter.Submit`);
bans constructing an `http.Client`/`http.Transport` or touching the HTTP
defaults anywhere (adapters must use the injected scrobble `Doer`); bans
filesystem-mutating `os` functions outside SDK-declared hook seams —
capturing one as a function value counts — plus `os.StartProcess` /
`os.FindProcess`; and bans dot imports, which would defeat the selector
checks. A contract break reds the plugin's CI and its tagged release instead
of drifting silently.

## Versioning and dependencies

- Pin the SDK by **tagged semver** in `go.mod` — committed plugin modules
  must not `replace github.com/beamfall/plugin-sdk`. Local development may
  use an uncommitted `go.work`. Policy details: [COMPATIBILITY.md](../COMPATIBILITY.md).
- Set `Bundle.SDKFloor` to the oldest SDK your bundle actually works with.
- The SDK's dependency surface is the Go standard library plus
  `golang.org/x/text` (Unicode normalization in bundle validation); don't
  propose SDK changes that widen it.

## Regenerating these docs

The API reference and `llms.txt` / `llms-full.txt` are generated:

```sh
script/gen-docs.sh
```

Run it after any exported-API or doc-comment change; hand-edit only this
guide, the README, COMPATIBILITY.md, and the
[registry admission runbook](registry-admission-runbook.md).
