# package scrobble

`import "github.com/beamfall/plugin-sdk/scrobble"`

Package scrobble is the neutral Scrobble dispatcher substrate. It provides the engine (Dispatcher), seams (Adapter/SecretSource/LinkSource/ DeadLetterSink/SkipSink/Doer), types (WatchedEvent/Scrobble/Service/MediaClass/ ExternalID/Link/DeadLetter/SkipReason/AdapterDescriptor), persistence (FileDeadLetter), and the registry-driven config loader (LoadAdapters). Service consts and concrete Adapter implementations live in the owning plugin modules (beamfall-plugin-music, beamfall-plugin-filmtv).

## func LoadAdapters

```go
func LoadAdapters(path string, secrets SecretSource, client Doer, registry map[Service]AdapterFactory) ([]Adapter, Config, error)
```

LoadAdapters reads the scrobble configuration at path and builds the activated adapters using the provided registry of factories, resolving each service's operator app credentials through the secure store. client is the shared outbound HTTP client. A missing config file returns (nil, zero, nil) — dormant. A present but malformed file, an unknown service, or an unresolvable operator credential is an error so a misconfiguration fails at boot.

## func RedirectGuard

```go
func RedirectGuard(endpoints func() []string) func(req *http.Request, via []*http.Request) error
```

RedirectGuard returns an http.Client.CheckRedirect function that rejects any redirect hop whose target origin (scheme + host + port) is not among the declared endpoints. Origin matching means a declared https endpoint that 30x-es to http:// on the SAME host is rejected — Go only strips Authorization/Cookie headers on a host change, so a scheme downgrade would otherwise replay the secret over cleartext. The composition root installs it on the http.Client it injects as the scrobble Doer, so the allowlist that guards initial request origins also covers server-driven redirects (see the Doer / allowlistDoer contract). endpoints is called per hop so it always reflects the currently declared endpoint set.

## type Adapter

```go
type Adapter interface {
	Service() Service
	// Class is the playback class this adapter scrobbles (video for Trakt, audio
	// for the music targets). An event of another class is never routed here.
	Class() MediaClass
	// IdentitySchemes are the External-ID schemes this adapter can consume, in
	// preference order. The dispatcher picks the first scheme present on the
	// Media; if none is present the event is skipped-with-reason, never
	// guessed (no fuzzy matching).
	IdentitySchemes() []string
	// DeclaredEndpoints are the well-known outbound endpoint origins this
	// adapter may call. A bare host (no scheme) declares the https origin on
	// the default port; cleartext http or a non-default port must be declared
	// explicitly (e.g. "http://127.0.0.1:8080" in tests). The injected Doer
	// rejects requests to any other origin — scheme and port included, so a
	// declared https endpoint never authorizes an http downgrade on the same
	// host.
	DeclaredEndpoints() []string
	// Submit pushes one resolved scrobble to the service. The ctx bounds the
	// request; the secret in s.Secret authorizes it. Returned errors must not
	// include s.Secret; the dispatcher redacts it again before logging or
	// persisting the error.
	Submit(ctx context.Context, s Scrobble) error
}
```

Adapter is the target-adapter seam: one external service behind one interface, so adding a service is a pure adapter addition. Each adapter declares its Service, the MediaClass it scrobbles, the External-ID schemes it can consume (preference order — the first present on the Media wins), and a Submit that pushes one watched/played event. Submit returns nil on success; a transient error is retried with backoff, a permanent one is dead-lettered after the attempt cap (the dispatcher does not distinguish — it retries every error up to the cap, mirroring the Webhook Emitter).

## type AdapterConfig

```go
type AdapterConfig struct {
	Service      Service `json:"service"`
	ClientIDRef  string  `json:"client_id_ref,omitempty"`
	APIKeyRef    string  `json:"api_key_ref,omitempty"`
	APISecretRef string  `json:"api_secret_ref,omitempty"`
	BaseURL      string  `json:"base_url,omitempty"`
}
```

AdapterConfig is the on-disk shape of one activated scrobble adapter. JSON tags must match the original config.go fileAdapter tags exactly.

## type AdapterDescriptor

```go
type AdapterDescriptor struct {
	Service         string
	Class           string
	IdentitySchemes []string
}
```

AdapterDescriptor is one configured adapter's non-secret descriptor for the status read: the service token, the media class it scrobbles, and the External-ID schemes it consumes.

## type AdapterFactory

```go
type AdapterFactory func(client Doer, cfg AdapterConfig, secrets SecretSource) (Adapter, error)
```

AdapterFactory builds one adapter from operator config + secret resolution.

## type Config

```go
type Config struct {
	MaxAttempts int
	BaseBackoff time.Duration
	// MaxBackoff caps the exponential backoff between retry attempts. Zero
	// takes the package default; it is never allowed to be unbounded, since an
	// operator-settable MaxAttempts must not be able to block the worker for
	// minutes on a single stuck event.
	MaxBackoff time.Duration
	DeadLetter DeadLetterSink
	Skips      SkipSink
}
```

Config tunes the dispatcher. Zero values take the package defaults.

## type DeadLetter

```go
type DeadLetter struct {
	ProfileID string    `json:"profile_id"`
	Service   string    `json:"service"`
	MediaID   string    `json:"media_id"`
	Attempts  int       `json:"attempts"`
	LastError string    `json:"last_error"`
	At        time.Time `json:"at"`
}
```

DeadLetter is one submission that exhausted its retries: which Profile/service failed on which Media, why, and when. It holds NO secret and NO identity handle beyond the Media id (which is already Profile-private engagement). The Webhook Emitter's FileDeadLetter is the persistence model.

## type DeadLetterReader

```go
type DeadLetterReader interface {
	Entries() []DeadLetter
}
```

DeadLetterReader is the optional read side of a DeadLetterSink (FileDeadLetter satisfies it). The dispatcher exposes the feed for the operator status read only when the wired sink can be read back.

## type DeadLetterSink

```go
type DeadLetterSink interface {
	Record(entry DeadLetter) error
}
```

DeadLetterSink persists exhausted submissions for operator inspection (the status panel). The Webhook Emitter's plugin-private JSON-on-disk is the model: no Core schema row (invariant #3).

## type Dispatcher

```go
type Dispatcher struct {
	// contains filtered or unexported fields
}
```

Dispatcher is the scrobble plugin's core: it consumes watched events (WatchedSink), maps each to the Profile's linked, eligible targets, and pushes each submission with backoff + dead-letter. It owns a bounded in-memory queue drained by a worker goroutine (Run), so the playback path that enqueues never blocks on a slow external service — the Webhook Emitter's resilience posture adapted to a callback source instead of an outbox poll.

### func New

```go
func New(cfg Config, adapters []Adapter, links LinkSource, secrets SecretSource, logger *slog.Logger) (*Dispatcher, error)
```

New builds a dispatcher, or returns (nil, nil) when no adapters are configured — the dormant case the app treats as "plugin off" (opt-in). links and secrets are required when any adapter is present. A duplicate adapter Service is a configuration error caught at boot.

### func (*Dispatcher) AdapterDescriptors

```go
func (d *Dispatcher) AdapterDescriptors() []AdapterDescriptor
```

AdapterDescriptors returns the configured adapters' non-secret descriptors, sorted by service for a stable read. It carries no credential.

### func (*Dispatcher) DeadLetters

```go
func (d *Dispatcher) DeadLetters() []DeadLetter
```

DeadLetters returns the dead-letter feed if the wired sink is readable, else nil (status read).

### func (*Dispatcher) Run

```go
func (d *Dispatcher) Run(ctx context.Context)
```

Run drains the queue until ctx is canceled, processing one event at a time. It is started as a long-lived goroutine by the app (like the Webhook Emitter's Run), not the job substrate — scrobble retry is per (event, service) and must not be conflated with job-level retry. On shutdown, any events still queued are not processed (a submission mid-retry must not race a canceled ctx) but the count is logged so the drop is observable rather than silent. A nil receiver (the dormant dispatcher New returns when scrobbling is not configured) returns immediately, matching Watched and the other nil-safe methods.

### func (*Dispatcher) Skips

```go
func (d *Dispatcher) Skips() []SkipReason
```

Skips returns the skip-with-reason feed if the wired sink is readable, else nil (status read).

### func (*Dispatcher) Watched

```go
func (d *Dispatcher) Watched(event WatchedEvent)
```

Watched is the WatchedSink seam the graph playback path calls on the unwatched→watched transition. It enqueues the event and returns immediately — the playback path must never block on a third-party service. A full queue (sustained outage) drops the event with a logged skip rather than blocking; the worker drains it in Run.

## type Doer

```go
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}
```

Doer is the HTTP client seam (interface at the consumer). \*http.Client satisfies it; tests inject a stub. The composition root passes the same outbound client the Webhook Emitter uses (the established Integration-Plugin posture: a well-known, operator-trusted public endpoint, not a user-supplied URL, so it shares the Webhook Emitter's delivery client rather than the scrape engine's SSRF-guarded dialer for user-typed hosts).

LoadAdapters wraps the injected Doer in an allowlistDoer that checks the INITIAL request origin (scheme + host + port) against the adapter's DeclaredEndpoints. That guard does not see redirects the Doer follows internally, so a declared origin that 30x-es to an undeclared one would escape the allowlist. For the "rejects requests to any other origin" promise to hold end-to-end, the composition root MUST make the injected Doer refuse cross-origin redirects: install RedirectGuard on http.Client.CheckRedirect, or disable redirect following.

## type ExternalID

```go
type ExternalID struct {
	Scheme string
	Value  string
}
```

ExternalID is one plugin-contributed identifier on the watched Media (the scrobble identity handle): scheme + value, e.g. {"tmdb","603"} or {"musicbrainz-recording","b1a9c0e9-..."}. The adapter declares which scheme(s) it can consume; the dispatcher skips-with-reason when none is present.

## type FileDeadLetter

```go
type FileDeadLetter struct {
	// contains filtered or unexported fields
}
```

FileDeadLetter persists exhausted scrobble submissions as plugin-private JSON under the data directory — the Webhook Emitter / qBittorrent Ledger model (visible in the plugin status panel without a Core schema row, invariant #3). It is written 0600 because it shares the secrets-adjacent data tree, even though a dead-letter row holds NO secret (invariant #5) — only the Profile/service/Media ids and the sanitized error string.

### func OpenDeadLetter

```go
func OpenDeadLetter(path string) (*FileDeadLetter, error)
```

OpenDeadLetter loads (or initializes) the dead-letter file at path. A missing file is a fresh feed; a corrupt file is an error rather than a silent reset, so an operator notices a clobbered record instead of losing prior failures.

### func (*FileDeadLetter) Entries

```go
func (dl *FileDeadLetter) Entries() []DeadLetter
```

Entries returns a copy of the current dead-letter feed, newest first — the operator-inspection read for the status panel.

### func (*FileDeadLetter) Record

```go
func (dl *FileDeadLetter) Record(entry DeadLetter) error
```

Record appends one exhausted submission and persists atomically. It implements DeadLetterSink.

## type Link

```go
type Link struct {
	Service      Service
	SecretRef    string
	AccountLabel string
}
```

Link is one resolved per-Profile service link the dispatcher routes by. It carries the non-secret SecretRef (a pointer to the 0600 file), never the token itself.

## type LinkSource

```go
type LinkSource interface {
	// LinksFor returns the Profile's active links: which services it has linked,
	// each with the non-secret SecretRef the dispatcher resolves through the
	// secure store at push time.
	LinksFor(ctx context.Context, profileID string) ([]Link, error)
	// LibraryEligible reports whether the Library is scrobble-eligible
	// (per-Library opt-in; adult/Restricted never eligible by default).
	LibraryEligible(ctx context.Context, libraryID string) (bool, error)
}
```

LinkSource resolves a Profile's active scrobble links and a Library's eligibility (interface at the consumer). \*store.Store satisfies it; the composition root adapts. The dispatcher reads these to gate each event without importing the store.

## type MediaClass

```go
type MediaClass string
```

MediaClass is the generic playback class the dispatcher routes on (mirrors store.MediaClass without importing it at the seam — invariant #1 keeps the seam generic). Trakt consumes video; Last.fm and ListenBrainz consume audio.

```go
const (
	ClassVideo MediaClass = "video"
	ClassAudio MediaClass = "audio"
)
```

## type MemorySkipSink

```go
type MemorySkipSink struct {
	// contains filtered or unexported fields
}
```

MemorySkipSink is an in-memory SkipSink for the status panel and tests: a bounded ring of recent skips. Entries reports them newest first. The composition root may use it to back the plugin's status read; tests assert on it directly.

### func NewMemorySkipSink

```go
func NewMemorySkipSink(max int) *MemorySkipSink
```

NewMemorySkipSink returns a skip sink retaining up to max recent entries (0 → a sane default).

### func (*MemorySkipSink) Entries

```go
func (m *MemorySkipSink) Entries() []SkipReason
```

Entries returns a copy of the recorded skips, newest first.

### func (*MemorySkipSink) Skip

```go
func (m *MemorySkipSink) Skip(entry SkipReason)
```

Skip records one skip, dropping the oldest past the cap.

## type MusicMetadata

```go
type MusicMetadata struct {
	Artist      string // artist display name (the related Artist entity's title)
	Album       string // album/release display name
	ArtistMBID  string // MusicBrainz artist id, when hydrated
	ReleaseMBID string // MusicBrainz release id, when hydrated
}
```

MusicMetadata is the optional richer outbound enrichment for an audio scrobble. Accepted exception: Last.fm/ListenBrainz-class submissions must carry hydrated artist, album, and MusicBrainz data when present, so these protocol terms remain named here even though the generic seam otherwise stays shape-first. Every field is optional; an empty field is omitted from the submission, never sent as an empty value. The identity gate is still the recording MBID on ExternalIDs — this only enriches a submission that already resolved an identity, it never becomes a new identity handle (no fuzzy artist matching).

## type Scrobble

```go
type Scrobble struct {
	Event    WatchedEvent
	Identity ExternalID
	// Secret is the Profile's resolved OAuth token / session key bytes (from the
	// 0600 secure store). Adapter-private use only; never logged.
	Secret string
	// AccountLabel is the optional non-secret linked-account display name.
	AccountLabel string
}
```

Scrobble is the resolved per-(event, service) submission an adapter pushes. It pairs the watched event with the resolved identity handle the adapter declared it needs, plus the Profile's resolved secret for that service. The Secret is held transiently to authorize the single request and is never persisted or logged (invariant #5).

## type SecretSource

```go
type SecretSource interface {
	Secret(ref string) (string, error)
}
```

SecretSource resolves a per-Profile-link SecretRef to the credential bytes in the 0600 secure store (the same secretFiles reader the scrape engine, qBittorrent ingest, and the Webhook Emitter use). The link record on disk names a ref, never the secret (invariant #5).

## type Service

```go
type Service string
```

Service is the adapter vocabulary value stored in store.ScrobbleLink.Service and used as the routing key. Concrete values ("trakt", "lastfm", "listenbrainz") are declared in the owning plugin modules.

## type SkipReader

```go
type SkipReader interface {
	Entries() []SkipReason
}
```

SkipReader is the optional read side of a SkipSink (MemorySkipSink satisfies it) for the status read.

## type SkipReason

```go
type SkipReason struct {
	ProfileID string
	MediaID   string
	Service   string // "" when the skip is service-independent (e.g. ineligible Library)
	Reason    string
	At        time.Time
}
```

SkipReason records a watched event that was NOT scrobbled and why — the observable "skip-with-reason" the status panel surfaces. A skip is not a failure: a Media with no matching External ID, an ineligible Library, or no linked service is skipped cleanly, never retried, never dead-lettered.

## type SkipSink

```go
type SkipSink interface {
	Skip(entry SkipReason)
}
```

SkipSink records skip-with-reason outcomes for the status panel. The default (nil) discards them — the dispatcher still logs each skip at debug.

## type TargetHint

```go
type TargetHint string
```

TargetHint is the bounded, domain-neutral shape an adapter uses to choose its external submission envelope. The zero value is deliberately unknown: producers compiled before this contract and producers carrying a future value fail closed instead of falling through to an adapter-specific default.

```go
const (
	// TargetStandalone identifies one independently-addressed item.
	TargetStandalone TargetHint = "standalone"
	// TargetIndexed identifies one item indexed within a parent sequence.
	TargetIndexed TargetHint = "indexed"
)
```

### func (TargetHint) Known

```go
func (h TargetHint) Known() bool
```

Known reports whether h belongs to the target-hint vocabulary understood by this SDK version. Adapters must not infer a default for absent or future values; Dispatcher enforces the same rule before resolving credentials.

## type WatchedEvent

```go
type WatchedEvent struct {
	ProfileID string
	MediaID   string
	LibraryID string
	Class     MediaClass
	// TargetHint is the generic external target shape selected from activated
	// plugin data. Absent and unknown values are skipped before submission so
	// older producers and version-skewed producers fail closed.
	TargetHint  TargetHint
	Title       string
	Year        int
	ExternalIDs []ExternalID
	// Music carries optional richer outbound metadata (artist/album display
	// names + MusicBrainz ids) the composition root resolves for audio events
	// from the activated Music plugin's declared relations/fields.
	// It is by SHAPE (outbound-metadata fields), not domain: a video
	// event leaves it zero, and an audio event with no hydrated metadata leaves
	// it zero too — the adapter enriches when present and degrades when absent.
	Music MusicMetadata
	// WatchedAt is the instant the watched threshold was crossed.
	WatchedAt time.Time
}
```

WatchedEvent is the generic signal the graph playback path emits on the unwatched→watched transition. It carries only what an adapter needs to map a submission, plus the routing facts (Profile, Library, class) the dispatcher gates on. It is NOT a new Core event kind — it is an in-process callback payload, so it adds nothing to the SSE vocabulary.

