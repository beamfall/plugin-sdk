# package sdktest

`import "github.com/beamfall/plugin-sdk/sdktest"`

Package sdktest is the cross-plugin conformance harness.

The SDK ships types and interfaces only; every first-party plugin is its own module with its own CI. These runners let each plugin pin golden vectors and bundle invariants against the SDK contract, so a scan or locale contract break reds that plugin's CI and its tagged release (semver-gated) rather than drifting silently.

Four entry points, composed by each plugin's conformance\_test.go:

  - RunConformance        — golden filename-parse vectors (FilenameParser).
  - RunContextConformance — bounded scan-context filename-parse vectors (ContextFilenameParser), including unknown/future-value fallback.
  - RunScoreConformance   — golden match-score vectors (Scorer).
  - RunBundleConformance — declarative Bundle + locale invariants; every plugin has a Bundle, so this is the one runner all plugins call.

## Constants

```go
const DefaultMatchThreshold = 0.5
```

DefaultMatchThreshold is the fail-closed acceptance floor a corpus case uses when it names no MinScore of its own: a candidate is only accepted as the match if its score is strictly above this floor. Below it, the file routes to the unmatched queue — the fail-closed baseline. It is a deliberately low, single-shared constant so a family exercises the parser and scorer, not a per-plugin tuning knob.

## Variables

```go
var PodcastADR0030KindVocab = []KindVocab{
	{Name: "show", Archetype: "container"},
	{Name: "person", Archetype: "person"},
	{Name: "topic", Archetype: "category"},
}
```

PodcastADR0030KindVocab is the Podcast plugin's EntityKind vocabulary of record: one container Kind for the show, one person-archetype Kind carrying both hosts and guests, and one flat category Kind seeded from the Apple genre taxonomy. Order matches the bundle's declared EntityKinds.

## func RequireAllowedImports

```go
func RequireAllowedImports(t testing.TB)
```

RequireAllowedImports is a targeted static gate over the calling module's non-test Go files, NOT a full dependency allowlist (third-party modules are reviewed via go.mod in registry admission). It fails when plugin domain code:

  - imports a denied package: "net", "net/smtp", "os/exec", "plugin", "syscall", or "unsafe";
  - imports any other network-capable net/\* package (net/rpc, net/textproto, net/http/httputil, …) anywhere — the transitive-reach guard, since a module-wide walk catches such a package pulled in through a local helper; only the parse-only value packages net/url and net/netip are permitted;
  - imports "net/http" outside a registered scrobble.Adapter seam file, or references an HTTP outbound function (call or captured function value) outside Adapter.Submit;
  - constructs an http.Client/http.Transport or touches http.DefaultClient/http.DefaultTransport anywhere — adapters must use the injected scrobble Doer, which carries the endpoint allowlist;
  - references a filesystem-mutating os function (or ioutil.WriteFile) outside SDK-declared hook seams, including capturing it as a function value, or calls os.StartProcess/os.FindProcess;
  - uses a dot import, which would defeat the selector-based checks above.

The gate is syntactic and per-file: it cannot see through local aliasing of package identifiers to variables, code generation, or what a third-party dependency does internally. It is one admission gate among several, not a sandbox.

## func RequireBundleCompatibility

```go
func RequireBundleCompatibility(t testing.TB, b sdk.Bundle)
```

RequireBundleCompatibility fails unless the producer-owned matrix contains exact conformance evidence for a canonical first-party bundle against the current SDK. Other bundles must declare a range compatible with the current SDK but do not require a first-party matrix row.

## func RequireBundleDefaultsRecognized

```go
func RequireBundleDefaultsRecognized(t testing.TB, b sdk.Bundle)
```

RequireBundleDefaultsRecognized fails when the SDK adds a LibraryDefaults field before sdktest names it as governed conformance vocabulary.

## func RequireBundleValid

```go
func RequireBundleValid(t testing.TB, b sdk.Bundle)
```

RequireBundleValid fails the test if b violates sdk.ValidateBundle.

## func RequireBundlesReject

```go
func RequireBundlesReject(t testing.TB, bundles ...sdk.Bundle)
```

RequireBundlesReject fails the test if sdk.ValidateBundles accepts the given bundles — the cross-plugin reject-path assertion (e.g. a colliding kind name).

## func RequireBundlesValid

```go
func RequireBundlesValid(t testing.TB, bundles ...sdk.Bundle)
```

RequireBundlesValid fails the test if the bundles violate the cross-plugin contract (sdk.ValidateBundles) — most notably two plugins declaring the same Entity Kind name, which the store's global kind namespace forbids. Use this when a test activates more than one plugin's bundle together.

## func RequireEntityKindVocab

```go
func RequireEntityKindVocab(t testing.TB, b sdk.Bundle, want []KindVocab)
```

RequireEntityKindVocab fails the test if b's declared EntityKinds (name + archetype, in order) don't match want exactly. Use this to pin a bundle's Kind vocabulary against a design-of-record so a rename or archetype change reds the plugin's own gate instead of drifting silently out of sync with the ADR that names it.

## func RequirePermissionPolicyNegativeCorpus

```go
func RequirePermissionPolicyNegativeCorpus(t testing.TB, b sdk.Bundle)
```

RequirePermissionPolicyNegativeCorpus proves the SDK rejects the shared malicious permission declarations: unknown capabilities, replaced host labels or warnings, and cleared minimum high-risk flags.

## func RunBundleConformance

```go
func RunBundleConformance(t *testing.T, b sdk.Bundle)
```

RunBundleConformance asserts a plugin's declarative Bundle obeys the SDK contract every plugin shares.

## func RunConformance

```go
func RunConformance(t *testing.T, p sdk.FilenameParser, vectors []ParseVector)
```

RunConformance asserts a FilenameParser matches its golden vectors and obeys the scan contract: each Parse is deterministic (same input → same output).

The vector table must also clear a coverage floor (2026-07-05 test-quality review): at least one WantOK:false vector (a hostile input the parser correctly refuses) and at least one WantOK:true vector whose Want sets two or more ParsedMedia fields (a multi-field parse, not just a bare title). A happy-path-only or single-field-only table passes every per-vector assertion below while leaving the parser's failure and multi-field paths completely unexercised, so the floor fails fast here instead.

## func RunContextConformance

```go
func RunContextConformance(t *testing.T, p sdk.ContextFilenameParser, vectors []ContextParseVector)
```

RunContextConformance asserts a ContextFilenameParser matches its golden vectors, is deterministic, and preserves its legacy Parse result when the context is absent or unrecognized. A vector table must cover the bounded unknown/single/multiple directory-media-file shapes in addition to the same refusal and multi-field coverage floor as RunConformance.

## func RunImporterConformance

```go
func RunImporterConformance(t *testing.T, vectors []ImporterVector)
```

RunImporterConformance grades a plugin's bounded StreamingSource and its deprecated Source.Items compatibility path against golden vectors.

RequireAllowedImports runs FIRST, before any source method is called. Because the import gate is a static, whole-module AST check, an importer whose code is capable of outbound network — directly or through a local helper package — reds the run here, before execution, no matter what Stream does at runtime (an importer cannot launder a network capability past the gate by swallowing its own transport error, because no request is ever reached).

Per-vector the runner asserts, in order:

  - Categories() is the declared WantCategories set, and every entry is a recognized ImportCategories value;
  - Stream emits WantItems in pages no larger than sdk.MaxImportPageItems;
  - a second Stream produces identical items and checkpoints;
  - resuming after the first and a later committed page produces no duplicate/lost items;
  - Stream(cancelledCtx) returns an error and emits no items;
  - deprecated Items returns the same result during the compatibility window;
  - on a WantErr vector, both APIs fail before emitting any item.

## func RunMatchCorpus

```go
func RunMatchCorpus(t *testing.T, p sdk.FilenameParser, s sdk.MatchScorer, families []MatchFamily)
```

RunMatchCorpus drives a plugin's parser+scorer through its matching-regression corpus and is the corpus sibling of RunConformance / RunScoreConformance. For each case it parses the path/probe, scores every candidate in the frozen set, applies the fail-closed pick (highest score strictly above the case floor; an ambiguous top band routes to unmatched unless the top scores are EXACTLY tied and a unique popularity signal breaks that tie — popularity never promotes a lower score), and asserts the resolved ID equals WantID — a wrong match is the regression, an unmatched WantID=="" case is the fail-closed baseline.

The run is hermetic: http.DefaultTransport is swapped for a transport that both errors on and \*records\* every request for the duration of the corpus. A parser or scorer that reaches a live provider mid-run fails the test even if it swallows the transport error, because the harness reds any run in which an outbound request was attempted — a network round-trip CI cannot reproduce is a hermeticity break, not something the plugin's own error handling can hide. The seal covers the default transport only: code that constructs its own http.Client/Transport or dials the network directly bypasses it, which is why RequireAllowedImports (run first) bans exactly those constructions.

Because the guard mutates the process-global http.DefaultTransport, calls to RunMatchCorpus are serialized with each other and must not run inside a parallel subtest (do not call t.Parallel() around it).

## func RunScoreConformance

```go
func RunScoreConformance(t *testing.T, s Scorer, vectors []ScoreVector)
```

RunScoreConformance asserts a Scorer matches its golden vectors, stays within the documented \[0,1] confidence range, and is deterministic.

## func RunScraperTraceConformance

```go
func RunScraperTraceConformance(t *testing.T, vectors []ScraperTraceVector)
```

RunScraperTraceConformance grades a plugin's declarative ScraperSpecs against golden reference traces. It is diagnostic by construction: a method, URL, auth-placement, pagination, response-path, transform, or artwork divergence each reds with a message naming the dimension and showing got vs want. Like the other runners it runs RequireAllowedImports first.

## type AdmissionResult

```go
type AdmissionResult struct {
	Admitted    bool
	FailedGate  GateClass
	Reason      string
	GatesPassed []GateClass
}
```

AdmissionResult is the outcome of Admit: which gate classes passed, and the first failure with a human-readable reason (mechanical, not taste: every rejection traces to exactly one named gate class).

### func Admit

```go
func Admit(listing Listing, sdkVersion string) AdmissionResult
```

Admit runs the four mechanical admission gates against a listing, in order, stopping at the first failure. sdkVersion is the registry's reference plugin-sdk release (e.g. sdk.Version of the SDK build the registry curates against).

## type AuthPlacement

```go
type AuthPlacement struct {
	// Location is "header", "query", or "" (no auth / no AuthRef).
	Location string
	// Name is the header or query-parameter name carrying the secret.
	Name string
	// Scheme is a header value prefix ("Bearer"); empty for non-bearer forms.
	Scheme string
	// InURL is true when the secret rides in the request URL (the "query:<param>"
	// form) — the placement the SDK doc warns leaks secrets to logs and proxies.
	InURL bool
}
```

AuthPlacement is where a scraper's AuthRef secret rides on a request — the placement the trace predicts from AuthStyle, never the secret value itself.

## type ContextParseVector

```go
type ContextParseVector struct {
	Name    string
	Path    string
	Probe   sdk.ParseProbe
	Context sdk.ParseContext
	Want    sdk.ParsedMedia
	WantOK  bool
}
```

ContextParseVector is one golden contextual filename-parse case. It is separate from ParseVector so adding the optional context seam does not alter existing plugin fixture literals.

## type GateClass

```go
type GateClass string
```

GateClass names one of the four mechanical admission gates required before a plugin listing is admitted to the registry: pass the SDK conformance suite, declare a valid bundle manifest, respect the fail-closed lifecycle contract, and declare an SDK compat range that covers the registry's SDK version. Admission beyond these gates (legality/malware) is a manual review function, not mechanical — see docs/registry-admission-runbook.md.

```go
const (
	GateConformance GateClass = "conformance"
	GateManifest    GateClass = "manifest"
	GateLifecycle   GateClass = "lifecycle"
	GateCompatRange GateClass = "compat-range"
)
```

## type ImporterVector

```go
type ImporterVector struct {
	Name string
	// Source is the plugin's compiled importer for this run.
	Source sdk.Source //nolint:staticcheck // deliberate 0.x compatibility path for pre-streaming sources
	// WantCategories is the exact set of data categories the run carries, from
	// the fixed ImportCategories vocabulary — order-insensitive. Every entry must
	// be a recognized category; an importer that carries nothing is meaningless.
	WantCategories []string
	// WantItems is the foreign-item slice a successful run yields, compared with
	// reflect.DeepEqual. Ignored when WantErr is set.
	WantItems []sdk.ForeignItem
	// WantErr asserts both Stream and the deprecated Items compatibility path
	// fail before emitting items. Repeatability, resume, and cancellation are
	// not graded on an error vector.
	WantErr bool
}
```

ImporterVector is one golden Migration Wizard importer case: a Source under test plus the categories and foreign items one run must stream.

## type KindVocab

```go
type KindVocab struct {
	Name      string
	Archetype string
}
```

KindVocab is one declared EntityKind's name + archetype pair. Plugins pin their bundle's Kind vocabulary against a design of record with this shape, so a future rename or archetype change fails the plugin's own gate instead of drifting silently out of sync with the doc.

## type Listing

```go
type Listing struct {
	Bundle sdk.Bundle
	// ConformancePassed reports whether the artifact's sdktest-based
	// conformance suite (RunConformance / RunScoreConformance /
	// RunBundleConformance) passed. Required because conformance runs as
	// that plugin's own `go test`, outside this process.
	ConformancePassed bool
	// Registry holds the Bundles of plugins already admitted to the target
	// registry listing set. The lifecycle gate checks the candidate against
	// this set (fails-closed, all-or-nothing load: a new revision
	// must not silently collide with another plugin's ID or Entity Kind
	// namespace once both are loaded together).
	Registry []sdk.Bundle
}
```

Listing is the registry-facing manifest for one plugin admission attempt: the plugin's Bundle plus the out-of-band conformance signal (did \`go test ./...\` against sdktest's runners pass for this artifact). The registry computes ConformancePassed by running the artifact's own module tests before calling Admit; sdktest cannot execute another module's tests from within this package.

## type MatchCase

```go
type MatchCase struct {
	Name       string
	Path       string
	Probe      sdk.ParseProbe
	Candidates []sdk.Candidate
	WantID     string
	// MinScore overrides DefaultMatchThreshold for this case; zero uses the
	// default. The winning candidate's score must be strictly greater.
	MinScore float64
	// HintProvider, when set, asserts the case resolves by identification hint:
	// the parser must carry ProviderIDs[HintProvider],
	// that ID must name a candidate in the frozen set, and the pick must come
	// from the hint alone — the scorer is never consulted and no outbound
	// request is attempted ("matched via hint, zero search calls"). WantID
	// still names the expected candidate. Empty keeps the scored pick.
	HintProvider string
}
```

MatchCase is one corpus fixture: an unidentified file (path plus the embedded tags scan would probe) matched against a frozen candidate set. WantID is the ID of the candidate the parser+scorer must resolve to; an empty WantID means the case must route to the unmatched queue (no candidate clears the floor).

The regression the corpus guards is a \*wrong match\*: resolving to a candidate whose ID differs from WantID. A WantID case that comes back unmatched is a fail-closed miss, still a regression against a case asserted to match; but a WantID=="" case that stays unmatched is the intended baseline and passes. Fixtures are hermetic: Candidates is the whole world the case sees — no live provider call is made, and one attempted mid-run reds the case.

## type MatchFamily

```go
type MatchFamily struct {
	Name  string
	Cases []MatchCase
}
```

MatchFamily is a named group of corpus cases — one hostile-input class such as scene naming, anime absolute numbering, or non-Latin titles. The plugin owns the families; the SDK owns how they run.

## type ParseVector

```go
type ParseVector struct {
	Name   string
	Path   string
	Probe  sdk.ParseProbe
	Want   sdk.ParsedMedia
	WantOK bool
}
```

ParseVector is one golden filename-parse case: a path + probe in, the expected ParsedMedia and ok out.

## type ScoreVector

```go
type ScoreVector struct {
	Name         string
	CandTitle    string
	CandYear     int
	CandSequence int
	CandIndex    int
	// CandSequencePresent and CandIndexPresent are the vector-table counterparts
	// of Candidate.SequencePresent/IndexPresent (scan.go): set true to assert a
	// scorer's behavior on a genuine ordinal 0, distinct from the zero-value
	// "not carried" a vector leaves false.
	CandSequencePresent bool
	CandIndexPresent    bool
	CandPopularity      float64
	Parsed              sdk.ParsedMedia
	Want                float64
	Tol                 float64
}
```

ScoreVector is one golden match-score case. Tol is the absolute tolerance for the float comparison; zero defaults to 1e-9.

## type Scorer

```go
type Scorer = sdk.MatchScorer
```

Scorer is the match-scoring seam: higher = a stronger match.

## type ScraperTrace

```go
type ScraperTrace struct {
	// Requests is the ordered request plan (login, then search pages, then lookup).
	Requests []TraceRequest
	// Fields is the emitted lookup-document core-field → value map, after
	// transforms; an unresolved or unparseable value is omitted (never present as
	// an empty string).
	Fields map[string]string
	// Artwork is the emitted lookup-document artwork-slot → absolute URL map.
	Artwork map[string]string
	// Candidates is the emitted search-result elements, in document order.
	Candidates []TraceCandidate
}
```

ScraperTrace is the compiled reference trace for one input: the predicted request plan plus the emitted mapping outputs.

### func CompileScraperTrace

```go
func CompileScraperTrace(spec sdk.ScraperSpec, input TraceInput) ScraperTrace
```

CompileScraperTrace re-derives the reference trace for spec against input, purely and offline. It is the oracle RunScraperTraceConformance grades against and the executor is checked against — not a fetch.

## type ScraperTraceVector

```go
type ScraperTraceVector struct {
	Name  string
	Spec  sdk.ScraperSpec
	Input TraceInput
	Want  ScraperTrace
}
```

ScraperTraceVector is one golden reference-trace case: a spec + input, and the trace the spec must compile to.

## type TraceCandidate

```go
type TraceCandidate struct {
	// ID is the value at CandidateIDPath — what a chosen candidate feeds into the
	// lookup {id}. Empty when no CandidateIDPath is declared or it is unresolved.
	ID      string
	Fields  map[string]string
	Artwork map[string]string
}
```

TraceCandidate is one search-result element's emitted values, resolved through SearchFieldMap/SearchArtworkMap (falling back to FieldMap/ArtworkMap).

## type TraceInput

```go
type TraceInput struct {
	// Query is the free-text {query} substitution for the search request; empty
	// compiles no search request.
	Query string
	// ID is the {id} substitution for the lookup request; empty compiles no
	// lookup request (unless fingerprints alone drive it — see LookupTemplate).
	ID string
	// Fingerprints supplies the content-identity variables {phash}, {oshash},
	// {duration}. An unknown/absent fingerprint substitutes as the empty string,
	// matching the executor contract (ScraperSpec.LookupTemplate).
	Fingerprints map[string]string
	// LookupDoc is the decoded JSON lookup document FieldMap/ArtworkMap/Transforms
	// resolve against. Nil resolves nothing (every mapped value is omitted).
	LookupDoc map[string]any
	// SearchDoc is the decoded JSON search response CandidatesPath resolves the
	// candidate array from. Nil compiles no candidates.
	SearchDoc map[string]any
}
```

TraceInput is the offline stimulus a reference trace is compiled against: the search/lookup identifiers to substitute and the decoded JSON documents the mappings resolve against. No field triggers a network call.

## type TraceRequest

```go
type TraceRequest struct {
	// Role is "search", "lookup", or "login".
	Role string
	// Page is the 1-based search page; 0 for lookup/login.
	Page int
	// Method is the implicit HTTP method the scraper kind uses.
	Method string
	// URL is the composed target: Endpoint plus the substituted path/query
	// template (GET kinds) or bare Endpoint (POST kinds carry the body instead).
	URL string
	// Body is the request body — the substituted GraphQL query for http-graphql,
	// or the login exchange payload; empty for GET requests.
	Body string
	// Auth is where the AuthRef secret rides on this request; the login exchange
	// carries no Auth (it mints the token).
	Auth AuthPlacement
}
```

TraceRequest is one outbound request the reference trace predicts.

