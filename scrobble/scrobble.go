// Package scrobble is the neutral Scrobble dispatcher substrate.
// It provides the engine (Dispatcher), seams (Adapter/SecretSource/LinkSource/
// DeadLetterSink/SkipSink/Doer), types (WatchedEvent/Scrobble/Service/MediaClass/
// ExternalID/Link/DeadLetter/SkipReason/AdapterDescriptor), persistence
// (FileDeadLetter), and the registry-driven config loader (LoadAdapters).
// Service consts and concrete Adapter implementations live in the owning plugin
// modules (beamfall-plugin-music, beamfall-plugin-filmtv).
package scrobble

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Service is the adapter vocabulary value stored in store.ScrobbleLink.Service
// and used as the routing key. Concrete values ("trakt", "lastfm",
// "listenbrainz") are declared in the owning plugin modules.
type Service string

// MediaClass is the generic playback class the dispatcher routes on (mirrors
// store.MediaClass without importing it at the seam — invariant #1 keeps the
// seam generic). Trakt consumes video; Last.fm and ListenBrainz consume audio.
type MediaClass string

const (
	ClassVideo MediaClass = "video"
	ClassAudio MediaClass = "audio"
)

// TargetHint is the bounded, domain-neutral shape an adapter uses to choose
// its external submission envelope. The zero value is deliberately unknown:
// producers compiled before this contract and producers carrying a future
// value fail closed instead of falling through to an adapter-specific default.
type TargetHint string

const (
	// TargetStandalone identifies one independently-addressed item.
	TargetStandalone TargetHint = "standalone"
	// TargetIndexed identifies one item indexed within a parent sequence.
	TargetIndexed TargetHint = "indexed"
)

// Known reports whether h belongs to the target-hint vocabulary understood by
// this SDK version. Adapters must not infer a default for absent or future
// values; Dispatcher enforces the same rule before resolving credentials.
func (h TargetHint) Known() bool {
	return h == TargetStandalone || h == TargetIndexed
}

// ExternalID is one plugin-contributed identifier on the watched Media (the
// scrobble identity handle): scheme + value, e.g.
// {"tmdb","603"} or {"musicbrainz-recording","b1a9c0e9-..."}. The adapter
// declares which scheme(s) it can consume; the dispatcher skips-with-reason when
// none is present.
type ExternalID struct {
	Scheme string
	Value  string
}

// WatchedEvent is the generic signal the graph playback path emits on the
// unwatched→watched transition. It carries only what an adapter
// needs to map a submission, plus the routing facts (Profile, Library, class)
// the dispatcher gates on. It is NOT a new Core event kind — it is an in-process
// callback payload, so it adds nothing to the SSE vocabulary.
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

// MusicMetadata is the optional richer outbound enrichment for an audio
// scrobble. Accepted exception: Last.fm/ListenBrainz-class submissions must
// carry hydrated artist, album, and MusicBrainz data when present, so these
// protocol terms remain named here even though the generic seam otherwise stays
// shape-first. Every field is optional;
// an empty field is omitted from the submission, never sent as an empty value.
// The identity gate is still the recording MBID on ExternalIDs — this only
// enriches a submission that already resolved an identity, it never becomes a
// new identity handle (no fuzzy artist matching).
type MusicMetadata struct {
	Artist      string // artist display name (the related Artist entity's title)
	Album       string // album/release display name
	ArtistMBID  string // MusicBrainz artist id, when hydrated
	ReleaseMBID string // MusicBrainz release id, when hydrated
}

// Scrobble is the resolved per-(event, service) submission an adapter pushes. It
// pairs the watched event with the resolved identity handle the adapter
// declared it needs, plus the Profile's resolved secret for that service. The
// Secret is held transiently to authorize the single request and is never
// persisted or logged (invariant #5).
type Scrobble struct {
	Event    WatchedEvent
	Identity ExternalID
	// Secret is the Profile's resolved OAuth token / session key bytes (from the
	// 0600 secure store). Adapter-private use only; never logged.
	Secret string
	// AccountLabel is the optional non-secret linked-account display name.
	AccountLabel string
}

// Adapter is the target-adapter seam: one external service
// behind one interface, so adding a service is a pure adapter addition. Each
// adapter declares its Service, the MediaClass it scrobbles, the External-ID
// schemes it can consume (preference order — the first present on the Media
// wins), and a Submit that pushes one watched/played event. Submit returns nil
// on success; a transient error is retried with backoff, a permanent one is
// dead-lettered after the attempt cap (the dispatcher does not distinguish — it
// retries every error up to the cap, mirroring the Webhook Emitter).
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

// Doer is the HTTP client seam (interface at the consumer). *http.Client
// satisfies it; tests inject a stub. The composition root passes the same
// outbound client the Webhook Emitter uses (the established Integration-Plugin
// posture: a well-known, operator-trusted public endpoint, not a user-supplied
// URL, so it shares the Webhook Emitter's delivery client rather than the
// scrape engine's SSRF-guarded dialer for user-typed hosts).
//
// LoadAdapters wraps the injected Doer in an allowlistDoer that checks the
// INITIAL request origin (scheme + host + port) against the adapter's
// DeclaredEndpoints. That guard does not see redirects the Doer follows
// internally, so a declared origin that 30x-es to an undeclared one would
// escape the allowlist. For the "rejects requests to any other origin" promise
// to hold end-to-end, the composition root MUST make the injected Doer refuse
// cross-origin redirects: install RedirectGuard on http.Client.CheckRedirect,
// or disable redirect following.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// SecretSource resolves a per-Profile-link SecretRef to the credential bytes in
// the 0600 secure store (the same secretFiles reader the scrape engine,
// qBittorrent ingest, and the Webhook Emitter use). The link record on disk
// names a ref, never the secret (invariant #5).
type SecretSource interface {
	Secret(ref string) (string, error)
}

// LinkSource resolves a Profile's active scrobble links and a Library's
// eligibility (interface at the consumer). *store.Store satisfies it;
// the composition root adapts. The dispatcher reads these to gate each event
// without importing the store.
type LinkSource interface {
	// LinksFor returns the Profile's active links: which services it has linked,
	// each with the non-secret SecretRef the dispatcher resolves through the
	// secure store at push time.
	LinksFor(ctx context.Context, profileID string) ([]Link, error)
	// LibraryEligible reports whether the Library is scrobble-eligible
	// (per-Library opt-in; adult/Restricted never eligible by default).
	LibraryEligible(ctx context.Context, libraryID string) (bool, error)
}

// Link is one resolved per-Profile service link the dispatcher routes by. It
// carries the non-secret SecretRef (a pointer to the 0600 file), never the
// token itself.
type Link struct {
	Service      Service
	SecretRef    string
	AccountLabel string
}

// DeadLetter is one submission that exhausted its retries: which Profile/service
// failed on which Media, why, and when. It holds NO secret and NO identity
// handle beyond the Media id (which is already Profile-private engagement). The
// Webhook Emitter's FileDeadLetter is the persistence model.
type DeadLetter struct {
	ProfileID string    `json:"profile_id"`
	Service   string    `json:"service"`
	MediaID   string    `json:"media_id"`
	Attempts  int       `json:"attempts"`
	LastError string    `json:"last_error"`
	At        time.Time `json:"at"`
}

// DeadLetterSink persists exhausted submissions for operator inspection (the
// status panel). The Webhook Emitter's plugin-private JSON-on-disk is the
// model: no Core schema row (invariant #3).
type DeadLetterSink interface {
	Record(entry DeadLetter) error
}

// SkipReason records a watched event that was NOT scrobbled and why — the
// observable "skip-with-reason" the status panel surfaces. A skip is not
// a failure: a Media with no matching External ID, an ineligible Library, or no
// linked service is skipped cleanly, never retried, never dead-lettered.
type SkipReason struct {
	ProfileID string
	MediaID   string
	Service   string // "" when the skip is service-independent (e.g. ineligible Library)
	Reason    string
	At        time.Time
}

// SkipSink records skip-with-reason outcomes for the status panel. The default
// (nil) discards them — the dispatcher still logs each skip at debug.
type SkipSink interface {
	Skip(entry SkipReason)
}

const (
	defaultMaxAttempts = 5
	defaultBaseBackoff = time.Second
	// defaultMaxBackoff caps the exponential backoff so an operator-settable
	// MaxAttempts cannot block the worker for minutes at a time.
	defaultMaxBackoff = 30 * time.Second
	// queueDepth bounds the in-memory submission queue so a sustained outage
	// cannot grow it without limit; a full queue drops the incoming event with
	// a logged skip rather than blocking the playback path that enqueues.
	queueDepth = 1024
	// submitAttemptTimeout bounds a single adapter.Submit attempt. Without a
	// per-attempt deadline, one request that hangs without a response would
	// stall the single worker goroutine until the queue overflows and drops
	// events — the retry/backoff machinery never gets a turn.
	submitAttemptTimeout = 30 * time.Second
)

// Dispatcher is the scrobble plugin's core: it consumes watched events
// (WatchedSink), maps each to the Profile's linked, eligible targets, and pushes
// each submission with backoff + dead-letter. It owns a bounded in-memory queue
// drained by a worker goroutine (Run), so the playback path that enqueues never
// blocks on a slow external service — the Webhook Emitter's resilience posture
// adapted to a callback source instead of an outbox poll.
type Dispatcher struct {
	adapters map[Service]Adapter
	links    LinkSource
	secrets  SecretSource
	dead     DeadLetterSink
	skips    SkipSink
	logger   *slog.Logger

	maxAttempts int
	baseBackoff time.Duration
	maxBackoff  time.Duration

	queue  chan WatchedEvent
	now    func() time.Time
	sleep  func(ctx context.Context, d time.Duration) error
	jitter func(d time.Duration) time.Duration
}

// Config tunes the dispatcher. Zero values take the package defaults.
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

// New builds a dispatcher, or returns (nil, nil) when no adapters are configured
// — the dormant case the app treats as "plugin off" (opt-in). links
// and secrets are required when any adapter is present. A duplicate adapter
// Service is a configuration error caught at boot.
func New(cfg Config, adapters []Adapter, links LinkSource, secrets SecretSource, logger *slog.Logger) (*Dispatcher, error) {
	if len(adapters) == 0 {
		return nil, nil // dormant: no adapters wired
	}
	if links == nil {
		return nil, errors.New("scrobble: a link source is required")
	}
	if secrets == nil {
		return nil, errors.New("scrobble: a secret source is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	byService := make(map[Service]Adapter, len(adapters))
	for _, a := range adapters {
		svc := a.Service()
		if strings.TrimSpace(string(svc)) == "" {
			return nil, errors.New("scrobble: adapter has no service name")
		}
		if _, dup := byService[svc]; dup {
			return nil, fmt.Errorf("scrobble: duplicate adapter for service %q", svc)
		}
		byService[svc] = a
	}
	if cfg.MaxAttempts < 1 {
		cfg.MaxAttempts = defaultMaxAttempts
	}
	if cfg.BaseBackoff <= 0 {
		cfg.BaseBackoff = defaultBaseBackoff
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = defaultMaxBackoff
	}
	return &Dispatcher{
		adapters:    byService,
		links:       links,
		secrets:     secrets,
		dead:        cfg.DeadLetter,
		skips:       cfg.Skips,
		logger:      logger,
		maxAttempts: cfg.MaxAttempts,
		baseBackoff: cfg.BaseBackoff,
		maxBackoff:  cfg.MaxBackoff,
		queue:       make(chan WatchedEvent, queueDepth),
		now:         time.Now,
		sleep:       sleepCtx,
		jitter:      jitterBackoff,
	}, nil
}

// DeadLetterReader is the optional read side of a DeadLetterSink (FileDeadLetter
// satisfies it). The dispatcher exposes the feed for the operator status read
// only when the wired sink can be read back.
type DeadLetterReader interface {
	Entries() []DeadLetter
}

// SkipReader is the optional read side of a SkipSink (MemorySkipSink satisfies
// it) for the status read.
type SkipReader interface {
	Entries() []SkipReason
}

// AdapterDescriptor is one configured adapter's non-secret descriptor for the
// status read: the service token, the media class it
// scrobbles, and the External-ID schemes it consumes.
type AdapterDescriptor struct {
	Service         string
	Class           string
	IdentitySchemes []string
}

// AdapterDescriptors returns the configured adapters' non-secret descriptors,
// sorted by service for a stable read. It carries no credential.
func (d *Dispatcher) AdapterDescriptors() []AdapterDescriptor {
	if d == nil {
		return nil
	}
	out := make([]AdapterDescriptor, 0, len(d.adapters))
	for svc, a := range d.adapters {
		out = append(out, AdapterDescriptor{
			Service:         string(svc),
			Class:           string(a.Class()),
			IdentitySchemes: a.IdentitySchemes(),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Service < out[j].Service })
	return out
}

// DeadLetters returns the dead-letter feed if the wired sink is readable, else
// nil (status read).
func (d *Dispatcher) DeadLetters() []DeadLetter {
	if d == nil {
		return nil
	}
	if reader, ok := d.dead.(DeadLetterReader); ok {
		return reader.Entries()
	}
	return nil
}

// Skips returns the skip-with-reason feed if the wired sink is readable, else
// nil (status read).
func (d *Dispatcher) Skips() []SkipReason {
	if d == nil {
		return nil
	}
	if reader, ok := d.skips.(SkipReader); ok {
		return reader.Entries()
	}
	return nil
}

// Watched is the WatchedSink seam the graph playback path calls on the
// unwatched→watched transition. It enqueues the event and
// returns immediately — the playback path must never block on a third-party
// service. A full queue (sustained outage) drops the event with a logged skip
// rather than blocking; the worker drains it in Run.
func (d *Dispatcher) Watched(event WatchedEvent) {
	if d == nil {
		return
	}
	select {
	case d.queue <- event:
	default:
		d.logger.Warn("scrobble queue full, dropping watched event",
			"profile", event.ProfileID, "media", event.MediaID)
		d.recordSkip(SkipReason{
			ProfileID: event.ProfileID,
			MediaID:   event.MediaID,
			Reason:    "queue full (sustained delivery backlog)",
			At:        d.now().UTC(),
		})
	}
}

// Run drains the queue until ctx is canceled, processing one event at a time.
// It is started as a long-lived goroutine by the app (like the Webhook Emitter's
// Run), not the job substrate — scrobble retry is per (event, service) and must
// not be conflated with job-level retry. On shutdown, any events still queued
// are not processed (a submission mid-retry must not race a canceled ctx) but
// the count is logged so the drop is observable rather than silent. A nil
// receiver (the dormant dispatcher New returns when scrobbling is not
// configured) returns immediately, matching Watched and the other nil-safe
// methods.
func (d *Dispatcher) Run(ctx context.Context) {
	if d == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			d.logDroppedOnShutdown()
			return
		case event := <-d.queue:
			d.process(ctx, event)
		}
	}
}

// logDroppedOnShutdown logs how many events were still queued when Run's ctx
// was canceled, so a shutdown never silently discards in-flight work.
func (d *Dispatcher) logDroppedOnShutdown() {
	if n := len(d.queue); n > 0 {
		d.logger.Warn("scrobble worker shutting down, dropping queued events", "count", n)
	}
}

// process maps one watched event to every linked, eligible target and submits
// each. The order of gates is the contract: eligibility first (a
// service-independent skip if the Library is not opted in / is Restricted), then
// per-service link + identity resolution.
func (d *Dispatcher) process(ctx context.Context, event WatchedEvent) {
	if !event.TargetHint.Known() {
		d.recordSkip(SkipReason{
			ProfileID: event.ProfileID,
			MediaID:   event.MediaID,
			Reason:    "scrobble target hint is absent or unknown",
			At:        d.now().UTC(),
		})
		return
	}
	eligible, err := d.links.LibraryEligible(ctx, event.LibraryID)
	if err != nil {
		d.logger.Error("scrobble eligibility lookup failed", "library", event.LibraryID, "error", err)
		return
	}
	if !eligible {
		// Per-Library opt-in: a Library the operator never marked eligible
		// — which is every Library by default, including every adult/Restricted
		// one — is skipped cleanly. "Adult never default" falls out of this.
		d.recordSkip(SkipReason{
			ProfileID: event.ProfileID,
			MediaID:   event.MediaID,
			Reason:    "library not scrobble-eligible",
			At:        d.now().UTC(),
		})
		return
	}
	links, err := d.links.LinksFor(ctx, event.ProfileID)
	if err != nil {
		d.logger.Error("scrobble link lookup failed", "profile", event.ProfileID, "error", err)
		return
	}
	if len(links) == 0 {
		d.recordSkip(SkipReason{
			ProfileID: event.ProfileID,
			MediaID:   event.MediaID,
			Reason:    "profile has no linked scrobble service",
			At:        d.now().UTC(),
		})
		return
	}
	for _, link := range links {
		d.dispatchToService(ctx, event, link)
	}
}

// dispatchToService resolves one linked service's adapter, class match, identity
// handle, and secret, then submits with retry. Each gate that fails is a clean
// skip-with-reason — never a dead-letter (a dead-letter is for a
// submission that was attempted and exhausted its retries).
func (d *Dispatcher) dispatchToService(ctx context.Context, event WatchedEvent, link Link) {
	adapter, ok := d.adapters[link.Service]
	if !ok {
		// A linked service with no wired adapter (operator deactivated it): skip.
		d.recordSkip(SkipReason{
			ProfileID: event.ProfileID, MediaID: event.MediaID, Service: string(link.Service),
			Reason: "no adapter configured for linked service", At: d.now().UTC(),
		})
		return
	}
	if adapter.Class() != event.Class {
		// Class mismatch (a video target for an audio play): not an error, the
		// service simply has no vocabulary for it. Skip silently-with-reason.
		d.recordSkip(SkipReason{
			ProfileID: event.ProfileID, MediaID: event.MediaID, Service: string(link.Service),
			Reason: "media class not eligible for this service", At: d.now().UTC(),
		})
		return
	}
	identity, ok := resolveIdentity(adapter.IdentitySchemes(), event.ExternalIDs)
	if !ok {
		// Skip-with-reason: the Media lacks the External ID this service
		// needs. Never fuzzy-match — scrobbling the wrong title corrupts a
		// permanent external history. Recorded, not failed.
		d.recordSkip(SkipReason{
			ProfileID: event.ProfileID, MediaID: event.MediaID, Service: string(link.Service),
			Reason: "no external id for " + strings.Join(adapter.IdentitySchemes(), "/"),
			At:     d.now().UTC(),
		})
		return
	}
	secret, err := d.secrets.Secret(link.SecretRef)
	if err != nil {
		// The link names a secret the store cannot resolve (cleared file, bad
		// mode). Not retryable — record a skip so the operator sees the broken
		// link, but never log the ref's contents.
		d.recordSkip(SkipReason{
			ProfileID: event.ProfileID, MediaID: event.MediaID, Service: string(link.Service),
			Reason: "linked credential unavailable", At: d.now().UTC(),
		})
		return
	}
	d.submitWithRetry(ctx, adapter, Scrobble{
		Event:        event,
		Identity:     identity,
		Secret:       secret,
		AccountLabel: link.AccountLabel,
	})
}

// submitWithRetry pushes one scrobble with exponential-backoff retry, dead-
// lettering on exhaustion. The error string carries the service and the
// adapter's sanitized message only — never the secret. Backoff is capped at
// d.maxBackoff and jittered so an operator-settable MaxAttempts cannot block
// the worker for minutes and concurrent retries do not thunder in lockstep.
func (d *Dispatcher) submitWithRetry(ctx context.Context, adapter Adapter, s Scrobble) {
	var lastErr error
	backoff := d.baseBackoff
	for attempt := 1; attempt <= d.maxAttempts; attempt++ {
		if attempt > 1 {
			if err := d.sleep(ctx, d.jitter(backoff)); err != nil {
				return // ctx canceled mid-backoff: stop, the event is lost (best-effort)
			}
			backoff *= 2
			if backoff > d.maxBackoff {
				backoff = d.maxBackoff
			}
		}
		// Each attempt gets its own deadline so one hung request cannot
		// stall the single worker indefinitely (the queue would silently
		// fill and drop events). Well-behaved adapters honor ctx; the
		// deadline bounds the ones that don't get a response back.
		attemptCtx, cancel := context.WithTimeout(ctx, submitAttemptTimeout)
		lastErr = adapter.Submit(attemptCtx, s)
		cancel()
		if lastErr == nil {
			return
		}
		if ctx.Err() != nil {
			return
		}
	}
	errText := sanitize(lastErr.Error(), s.Secret)
	d.logger.Warn("scrobble submission exhausted, dead-lettering",
		"service", adapter.Service(), "profile", s.Event.ProfileID, "media", s.Event.MediaID,
		"attempts", d.maxAttempts, "error", errText)
	if d.dead == nil {
		return
	}
	if err := d.dead.Record(DeadLetter{
		ProfileID: s.Event.ProfileID,
		Service:   string(adapter.Service()),
		MediaID:   s.Event.MediaID,
		Attempts:  d.maxAttempts,
		LastError: errText,
		At:        d.now().UTC(),
	}); err != nil {
		d.logger.Error("scrobble dead-letter write failed",
			"service", adapter.Service(), "media", s.Event.MediaID, "error", err)
	}
}

func sanitize(msg, secret string) string {
	if secret == "" {
		return msg
	}
	for _, form := range secretForms(secret) {
		if form != "" {
			msg = strings.ReplaceAll(msg, form, "[REDACTED]")
		}
	}
	return msg
}

// secretForms returns the raw secret plus its common wire encodings
// (URL-escaped and the four base64 variants), so an adapter error that embeds
// the credential in an encoded form — a URL query, a Basic auth header — is
// still redacted before logging or persisting.
func secretForms(secret string) []string {
	return []string{
		secret,
		url.QueryEscape(secret),
		base64.StdEncoding.EncodeToString([]byte(secret)),
		base64.URLEncoding.EncodeToString([]byte(secret)),
		base64.RawStdEncoding.EncodeToString([]byte(secret)),
		base64.RawURLEncoding.EncodeToString([]byte(secret)),
	}
}

// recordSkip logs and records one skip-with-reason. A nil SkipSink still logs at
// debug so the outcome is observable in logs even without the status panel.
func (d *Dispatcher) recordSkip(entry SkipReason) {
	d.logger.Debug("scrobble skipped",
		"profile", entry.ProfileID, "media", entry.MediaID, "service", entry.Service, "reason", entry.Reason)
	if d.skips != nil {
		d.skips.Skip(entry)
	}
}

// resolveIdentity picks the first External ID whose scheme the adapter declared,
// in the adapter's preference order (a defined order, never a guess). It
// returns ok=false when none is present — the skip-with-reason trigger.
func resolveIdentity(schemes []string, ids []ExternalID) (ExternalID, bool) {
	for _, scheme := range schemes {
		for _, id := range ids {
			if id.Scheme == scheme && strings.TrimSpace(id.Value) != "" {
				return id, true
			}
		}
	}
	return ExternalID{}, false
}

// jitterBackoff returns a duration uniformly distributed in [d/2, d]
// (inclusive of both bounds), so concurrent retries after a shared outage don't
// thunder in lockstep. d<=0 returns 0 (nothing to jitter).
func jitterBackoff(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	half := d / 2
	return half + time.Duration(rand.Int63n(int64(d-half)+1))
}

// sleepCtx waits d or returns ctx.Err() if the context is canceled first
// (mirrors the Webhook Emitter's helper).
func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// MemorySkipSink is an in-memory SkipSink for the status panel and tests: a
// bounded ring of recent skips. Entries reports them newest first. The
// composition root may use it to back the plugin's status read; tests assert on
// it directly.
type MemorySkipSink struct {
	mu      sync.Mutex
	max     int
	entries []SkipReason
}

// NewMemorySkipSink returns a skip sink retaining up to max recent entries
// (0 → a sane default).
func NewMemorySkipSink(max int) *MemorySkipSink {
	if max <= 0 {
		max = 256
	}
	return &MemorySkipSink{max: max}
}

// Skip records one skip, dropping the oldest past the cap.
func (m *MemorySkipSink) Skip(entry SkipReason) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, entry)
	if len(m.entries) > m.max {
		m.entries = m.entries[len(m.entries)-m.max:]
	}
}

// Entries returns a copy of the recorded skips, newest first.
func (m *MemorySkipSink) Entries() []SkipReason {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]SkipReason, len(m.entries))
	copy(out, m.entries)
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out
}

// --- Dead-letter persistence --------------------------------------------------

// maxDeadLetters bounds the persisted dead-letter feed so the file never grows
// without limit; the oldest entries are dropped first. Mirrors the Webhook
// Emitter's bounded feed (internal/webhook/deadletter.go).
const (
	maxDeadLetters     = 500
	maxDeadLetterBytes = 8 << 20
)

// FileDeadLetter persists exhausted scrobble submissions as plugin-private JSON
// under the data directory — the Webhook Emitter / qBittorrent Ledger model
// (visible in the plugin status panel without a Core schema row,
// invariant #3). It is written 0600 because it shares the secrets-adjacent data
// tree, even though a dead-letter row holds NO secret (invariant #5) — only the
// Profile/service/Media ids and the sanitized error string.
type FileDeadLetter struct {
	path  string
	mu    sync.Mutex
	state deadLetterState
}

type deadLetterState struct {
	Version int          `json:"version"`
	Entries []DeadLetter `json:"entries"`
}

// OpenDeadLetter loads (or initializes) the dead-letter file at path. A missing
// file is a fresh feed; a corrupt file is an error rather than a silent reset,
// so an operator notices a clobbered record instead of losing prior failures.
func OpenDeadLetter(path string) (*FileDeadLetter, error) {
	dl := &FileDeadLetter{path: path, state: deadLetterState{Version: 1}}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return dl, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxDeadLetterBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxDeadLetterBytes {
		return nil, fmt.Errorf("scrobble dead-letter file exceeds %d bytes", maxDeadLetterBytes)
	}
	var state deadLetterState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	state.Version = 1
	// Re-apply the retention cap on load: a file written by a different
	// build (or edited by hand) may exceed it, and waiting for the next
	// Record would leave an unbounded feed in memory and on disk until then.
	if len(state.Entries) > maxDeadLetters {
		state.Entries = state.Entries[len(state.Entries)-maxDeadLetters:]
	}
	dl.state = state
	return dl, nil
}

// Record appends one exhausted submission and persists atomically. It implements
// DeadLetterSink.
func (dl *FileDeadLetter) Record(entry DeadLetter) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.state.Entries = append(dl.state.Entries, entry)
	if len(dl.state.Entries) > maxDeadLetters {
		dl.state.Entries = dl.state.Entries[len(dl.state.Entries)-maxDeadLetters:]
	}
	return dl.persistLocked()
}

// Entries returns a copy of the current dead-letter feed, newest first — the
// operator-inspection read for the status panel.
func (dl *FileDeadLetter) Entries() []DeadLetter {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	out := make([]DeadLetter, len(dl.state.Entries))
	copy(out, dl.state.Entries)
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out
}

func (dl *FileDeadLetter) persistLocked() error {
	dl.state.Version = 1
	data, err := json.MarshalIndent(dl.state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dl.path), 0o700); err != nil {
		return err
	}
	// A unique temp name (not a fixed ".tmp" sibling) keeps concurrent
	// writers from clobbering each other's staging file; os.CreateTemp
	// creates it 0600, preserving the secrets-adjacent tree's mode.
	tmp, err := os.CreateTemp(filepath.Dir(dl.path), filepath.Base(dl.path)+".*.tmp")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	// Sync before rename: without it a crash after the rename can leave a
	// zero-length or truncated file at the final path — the rename is atomic
	// with respect to the NAME, not the not-yet-flushed contents.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), dl.path)
}

// --- Registry-driven config loader --------------------------------------------

// AdapterConfig is the on-disk shape of one activated scrobble adapter.
// JSON tags must match the original config.go fileAdapter tags exactly.
type AdapterConfig struct {
	Service      Service `json:"service"`
	ClientIDRef  string  `json:"client_id_ref,omitempty"`
	APIKeyRef    string  `json:"api_key_ref,omitempty"`
	APISecretRef string  `json:"api_secret_ref,omitempty"`
	BaseURL      string  `json:"base_url,omitempty"`
}

// AdapterFactory builds one adapter from operator config + secret resolution.
type AdapterFactory func(client Doer, cfg AdapterConfig, secrets SecretSource) (Adapter, error)

// fileConfig is the on-disk scrobble configuration
// (<DataDir>/scrobble-adapters.json). A missing file means the plugin is
// unconfigured and stays dormant (New returns a nil dispatcher).
type fileConfig struct {
	Adapters    []AdapterConfig `json:"adapters"`
	MaxAttempts int             `json:"max_attempts,omitempty"`
}

// LoadAdapters reads the scrobble configuration at path and builds the activated
// adapters using the provided registry of factories, resolving each service's
// operator app credentials through the secure store. client is the shared
// outbound HTTP client. A missing config file returns (nil, zero, nil) — dormant.
// A present but malformed file, an unknown service, or an unresolvable operator
// credential is an error so a misconfiguration fails at boot.
func LoadAdapters(path string, secrets SecretSource, client Doer, registry map[Service]AdapterFactory) ([]Adapter, Config, error) {
	if client == nil {
		return nil, Config{}, errors.New("scrobble config: an HTTP client is required")
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, Config{}, nil // dormant: no config file
	}
	if err != nil {
		return nil, Config{}, err
	}
	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, Config{}, fmt.Errorf("scrobble config %s: %w", path, err)
	}
	if len(fc.Adapters) == 0 {
		return nil, Config{}, nil // dormant: configured with no adapters
	}
	if secrets == nil {
		return nil, Config{}, errors.New("scrobble config: a secret source is required")
	}
	var adapters []Adapter
	for i, fa := range fc.Adapters {
		factory, ok := registry[Service(strings.TrimSpace(string(fa.Service)))]
		if !ok {
			return nil, Config{}, fmt.Errorf("scrobble config: adapter %d names unknown service %q", i, fa.Service)
		}
		var adapter Adapter
		guarded := allowlistDoer{next: client, endpoints: func() []string {
			if adapter == nil {
				return nil
			}
			return adapter.DeclaredEndpoints()
		}}
		a, err := factory(guarded, AdapterConfig{
			Service:      Service(strings.TrimSpace(string(fa.Service))),
			ClientIDRef:  fa.ClientIDRef,
			APIKeyRef:    fa.APIKeyRef,
			APISecretRef: fa.APISecretRef,
			BaseURL:      fa.BaseURL,
		}, secrets)
		if err != nil {
			return nil, Config{}, err
		}
		adapter = a
		adapters = append(adapters, a)
	}
	return adapters, Config{MaxAttempts: fc.MaxAttempts}, nil
}

// allowlistDoer restricts outbound requests to the origins (scheme + host +
// port) the adapter declares via DeclaredEndpoints. Matching the full origin —
// not just the host — means a declared https endpoint does NOT authorize
// cleartext http to the same host, which would otherwise let a downgrade replay
// the Authorization-bearing request without TLS (Go strips credentials only on
// host CHANGE, not on scheme downgrade). It guards the INITIAL request URL
// only; it cannot observe redirects the underlying Doer follows internally. The
// composition root must pair it with RedirectGuard (installed on
// http.Client.CheckRedirect) or a redirect-disabled client so a declared origin
// cannot 30x to an undeclared one.
type allowlistDoer struct {
	next      Doer
	endpoints func() []string
}

func (d allowlistDoer) Do(req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, errors.New("scrobble: outbound request has no URL")
	}
	if strings.ToLower(req.URL.Hostname()) == "" {
		return nil, errors.New("scrobble: outbound request has no host")
	}
	if !originAllowed(req.URL, d.endpoints()) {
		return nil, fmt.Errorf("scrobble: outbound origin %q is not declared by adapter", requestOriginString(req.URL))
	}
	return d.next.Do(req)
}

// RedirectGuard returns an http.Client.CheckRedirect function that rejects any
// redirect hop whose target origin (scheme + host + port) is not among the
// declared endpoints. Origin matching means a declared https endpoint that
// 30x-es to http:// on the SAME host is rejected — Go only strips
// Authorization/Cookie headers on a host change, so a scheme downgrade would
// otherwise replay the secret over cleartext. The composition root installs it
// on the http.Client it injects as the scrobble Doer, so the allowlist that
// guards initial request origins also covers server-driven redirects (see the
// Doer / allowlistDoer contract). endpoints is called per hop so it always
// reflects the currently declared endpoint set.
func RedirectGuard(endpoints func() []string) func(req *http.Request, via []*http.Request) error {
	return func(req *http.Request, _ []*http.Request) error {
		if req == nil || req.URL == nil {
			return errors.New("scrobble: redirect target has no URL")
		}
		if strings.ToLower(req.URL.Hostname()) == "" {
			return errors.New("scrobble: redirect target has no host")
		}
		if !originAllowed(req.URL, endpoints()) {
			return fmt.Errorf("scrobble: redirect to undeclared origin %q rejected", requestOriginString(req.URL))
		}
		return nil
	}
}

// originAllowed reports whether the request URL's origin (scheme, host, port)
// exactly matches one of the declared endpoint origins. Fail-closed: an empty
// endpoint list, an unparseable declared endpoint, or a non-http(s) request
// scheme all deny.
func originAllowed(u *url.URL, endpoints []string) bool {
	scheme, host, port, ok := requestOrigin(u)
	if !ok {
		return false
	}
	for _, endpoint := range endpoints {
		es, eh, ep, ok := declaredEndpointOrigin(endpoint)
		if ok && es == scheme && eh == host && ep == port {
			return true
		}
	}
	return false
}

// requestOrigin normalizes an outbound request URL to a comparable
// (scheme, host, port) origin. Only http and https are recognized; the port
// defaults from the scheme when the URL carries none.
func requestOrigin(u *url.URL) (scheme, host, port string, ok bool) {
	scheme = strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", "", "", false
	}
	host = strings.ToLower(u.Hostname())
	if host == "" {
		return "", "", "", false
	}
	port = u.Port()
	if port == "" {
		port = defaultSchemePort(scheme)
	}
	return scheme, host, port, true
}

// declaredEndpointOrigin parses one DeclaredEndpoints entry into a comparable
// (scheme, host, port) origin. A bare host with no scheme declares the https
// origin on the default port — cleartext http and non-default ports must be
// declared explicitly (fail-closed default). Endpoints that do not parse, or
// that use a non-http(s) scheme, return ok=false and match nothing.
func declaredEndpointOrigin(endpoint string) (scheme, host, port string, ok bool) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", "", "", false
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		u, err = url.Parse("https://" + endpoint)
		if err != nil || u.Host == "" {
			return "", "", "", false
		}
	}
	scheme = strings.ToLower(u.Scheme)
	if scheme == "" {
		scheme = "https"
	}
	if scheme != "http" && scheme != "https" {
		return "", "", "", false
	}
	host = strings.ToLower(u.Hostname())
	if host == "" {
		return "", "", "", false
	}
	port = u.Port()
	if port == "" {
		port = defaultSchemePort(scheme)
	}
	return scheme, host, port, true
}

func defaultSchemePort(scheme string) string {
	if scheme == "http" {
		return "80"
	}
	return "443"
}

// requestOriginString renders a request URL's origin for error messages,
// normalizing case but preserving whatever scheme/port the request carried.
func requestOriginString(u *url.URL) string {
	origin := strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Hostname())
	if p := u.Port(); p != "" {
		origin += ":" + p
	}
	return origin
}
