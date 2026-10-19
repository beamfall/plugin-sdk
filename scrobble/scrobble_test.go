package scrobble

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// local test-only service/scheme constants (real values live in plugin modules)
const (
	testServiceVideo Service = "testvideo"
	testServiceAudio Service = "testaudio"
)

const (
	testSchemeTMDB  = "tmdb"
	testSchemeMBRec = "musicbrainz-recording"
)

// --- test doubles ------------------------------------------------------------

// stubLinks is an in-memory LinkSource: a fixed per-Profile link set and a fixed
// per-Library eligibility map.
type stubLinks struct {
	links    map[string][]Link
	eligible map[string]bool
}

func (s stubLinks) LinksFor(_ context.Context, profileID string) ([]Link, error) {
	return s.links[profileID], nil
}

func (s stubLinks) LibraryEligible(_ context.Context, libraryID string) (bool, error) {
	return s.eligible[libraryID], nil
}

// stubSecrets resolves a ref to a fixed token, or errors for an unknown ref.
type stubSecrets struct{ byRef map[string]string }

func (s stubSecrets) Secret(ref string) (string, error) {
	if v, ok := s.byRef[ref]; ok {
		return v, nil
	}
	return "", errors.New("no such secret")
}

// recordingAdapter captures the scrobbles it received and can be told to fail a
// fixed number of times before succeeding (to exercise backoff + dead-letter).
type recordingAdapter struct {
	mu            sync.Mutex
	service       Service
	class         MediaClass
	schemes       []string
	got           []Scrobble
	failTimes     int
	failPermanent bool
	failMessage   string
}

func (a *recordingAdapter) Service() Service          { return a.service }
func (a *recordingAdapter) Class() MediaClass         { return a.class }
func (a *recordingAdapter) IdentitySchemes() []string { return a.schemes }
func (a *recordingAdapter) DeclaredEndpoints() []string {
	return []string{"https://api.example.test"}
}

func (a *recordingAdapter) Submit(_ context.Context, s Scrobble) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.got = append(a.got, s)
	if a.failPermanent {
		msg := a.failMessage
		if msg == "" {
			msg = "permanent failure"
		}
		return errors.New(msg)
	}
	if a.failTimes > 0 {
		a.failTimes--
		return errors.New("transient failure")
	}
	return nil
}

func (a *recordingAdapter) calls() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.got)
}

func (a *recordingAdapter) last() Scrobble {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.got[len(a.got)-1]
}

// memDeadLetter collects dead-lettered submissions in memory.
type memDeadLetter struct {
	mu      sync.Mutex
	entries []DeadLetter
}

func (m *memDeadLetter) Record(e DeadLetter) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, e)
	return nil
}

func (m *memDeadLetter) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.entries)
}

type recordingDoer struct {
	mu    sync.Mutex
	calls int
	host  string
}

func (d *recordingDoer) Do(req *http.Request) (*http.Response, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls++
	d.host = req.URL.Hostname()
	return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Request: req}, nil
}

func (d *recordingDoer) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.calls
}

type clientAdapter struct {
	client    Doer
	target    string
	endpoints []string
}

func (a *clientAdapter) Service() Service          { return testServiceVideo }
func (a *clientAdapter) Class() MediaClass         { return ClassVideo }
func (a *clientAdapter) IdentitySchemes() []string { return []string{testSchemeTMDB} }
func (a *clientAdapter) DeclaredEndpoints() []string {
	return a.endpoints
}

func (a *clientAdapter) Submit(ctx context.Context, _ Scrobble) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.target, nil)
	if err != nil {
		return err
	}
	resp, err := a.client.Do(req)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	return err
}

// newDispatcher builds a dispatcher with no-wait backoff for deterministic tests.
func newDispatcher(t *testing.T, adapters []Adapter, links LinkSource, secrets SecretSource, dl DeadLetterSink, skips SkipSink) *Dispatcher {
	t.Helper()
	d, err := New(Config{BaseBackoff: time.Millisecond, DeadLetter: dl, Skips: skips}, adapters, links, secrets, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if d == nil {
		t.Fatal("New returned dormant dispatcher for a configured adapter")
	}
	d.sleep = func(context.Context, time.Duration) error { return nil }
	return d
}

func videoEvent() WatchedEvent {
	return WatchedEvent{
		ProfileID:   "profile-1",
		MediaID:     "media-1",
		LibraryID:   "library-films",
		Class:       ClassVideo,
		TargetHint:  TargetStandalone,
		Title:       "Sintel",
		Year:        2010,
		ExternalIDs: []ExternalID{{Scheme: testSchemeTMDB, Value: "45745"}},
		WatchedAt:   time.Unix(1_700_000_000, 0).UTC(),
	}
}

// --- tests -------------------------------------------------------------------

func TestTargetHintVocabularyIsBounded(t *testing.T) {
	tests := []struct {
		name string
		hint TargetHint
		want bool
	}{
		{name: "standalone", hint: TargetStandalone, want: true},
		{name: "indexed", hint: TargetIndexed, want: true},
		{name: "absent", hint: "", want: false},
		{name: "future unknown", hint: "future-shape", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.hint.Known(); got != tt.want {
				t.Fatalf("TargetHint(%q).Known() = %v, want %v", tt.hint, got, tt.want)
			}
		})
	}
}

// TestTargetHintCompatibility pins the additive contract: both known shapes
// reach adapters, while an old producer's zero value and a future SDK value
// fail closed before any external submission.
func TestTargetHintCompatibility(t *testing.T) {
	tests := []struct {
		name       string
		hint       TargetHint
		wantCalls  int
		wantReason string
	}{
		{name: "standalone reaches adapter", hint: TargetStandalone, wantCalls: 1},
		{name: "indexed reaches adapter", hint: TargetIndexed, wantCalls: 1},
		{name: "old caller absent fails closed", wantReason: "scrobble target hint is absent or unknown"},
		{name: "future value fails closed", hint: "future-shape", wantReason: "scrobble target hint is absent or unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := &recordingAdapter{service: testServiceVideo, class: ClassVideo, schemes: []string{testSchemeTMDB}}
			links := stubLinks{
				links:    map[string][]Link{"profile-1": {{Service: testServiceVideo, SecretRef: "ref-1"}}},
				eligible: map[string]bool{"library-films": true},
			}
			skips := NewMemorySkipSink(0)
			d := newDispatcher(t, []Adapter{adapter}, links, stubSecrets{byRef: map[string]string{"ref-1": "token-abc"}}, &memDeadLetter{}, skips)

			event := videoEvent()
			event.TargetHint = tt.hint
			d.process(context.Background(), event)

			if got := adapter.calls(); got != tt.wantCalls {
				t.Fatalf("adapter calls = %d, want %d", got, tt.wantCalls)
			}
			entries := skips.Entries()
			if tt.wantReason == "" {
				if len(entries) != 0 {
					t.Fatalf("known target hint recorded skips: %+v", entries)
				}
				return
			}
			if len(entries) != 1 || entries[0].Reason != tt.wantReason {
				t.Fatalf("skip entries = %+v, want one reason %q", entries, tt.wantReason)
			}
		})
	}
}

// TestEligibleWatchedEventScrobbles: an eligible Library + a linked service + a
// present External ID => the adapter receives one submission with the resolved
// identity and the Profile's secret.
func TestEligibleWatchedEventScrobbles(t *testing.T) {
	adapter := &recordingAdapter{service: testServiceVideo, class: ClassVideo, schemes: []string{testSchemeTMDB, "imdb"}}
	links := stubLinks{
		links:    map[string][]Link{"profile-1": {{Service: testServiceVideo, SecretRef: "ref-1", AccountLabel: "alice"}}},
		eligible: map[string]bool{"library-films": true},
	}
	secrets := stubSecrets{byRef: map[string]string{"ref-1": "token-abc"}}
	d := newDispatcher(t, []Adapter{adapter}, links, secrets, &memDeadLetter{}, nil)

	d.process(context.Background(), videoEvent())

	if adapter.calls() != 1 {
		t.Fatalf("want 1 submission, got %d", adapter.calls())
	}
	got := adapter.last()
	if got.Identity.Scheme != testSchemeTMDB || got.Identity.Value != "45745" {
		t.Errorf("wrong identity resolved: %+v", got.Identity)
	}
	if got.Secret != "token-abc" {
		t.Errorf("adapter did not receive the resolved secret")
	}
	if got.AccountLabel != "alice" {
		t.Errorf("account label not carried: %q", got.AccountLabel)
	}
}

// TestSkipWithReasonWhenNoExternalID: a Media with no External ID the adapter can
// consume is skipped-with-reason, never submitted (no fuzzy match).
func TestSkipWithReasonWhenNoExternalID(t *testing.T) {
	adapter := &recordingAdapter{service: testServiceVideo, class: ClassVideo, schemes: []string{testSchemeTMDB}}
	links := stubLinks{
		links:    map[string][]Link{"profile-1": {{Service: testServiceVideo, SecretRef: "ref-1"}}},
		eligible: map[string]bool{"library-films": true},
	}
	secrets := stubSecrets{byRef: map[string]string{"ref-1": "token-abc"}}
	skips := NewMemorySkipSink(0)
	d := newDispatcher(t, []Adapter{adapter}, links, secrets, &memDeadLetter{}, skips)

	event := videoEvent()
	event.ExternalIDs = nil // no identity handle
	d.process(context.Background(), event)

	if adapter.calls() != 0 {
		t.Fatalf("a Media with no external id must not be submitted (no fuzzy match), got %d calls", adapter.calls())
	}
	entries := skips.Entries()
	if len(entries) != 1 {
		t.Fatalf("want 1 recorded skip, got %d", len(entries))
	}
	if !strings.Contains(entries[0].Reason, "no external id") {
		t.Errorf("skip reason should name the missing external id, got %q", entries[0].Reason)
	}
}

// TestAdultLibraryExcludedByDefault: a Library with no eligibility row (the
// default, including every adult/Restricted Library) is skipped service-
// independently — "adult never default" falls out of the opt-in default.
func TestAdultLibraryExcludedByDefault(t *testing.T) {
	adapter := &recordingAdapter{service: testServiceVideo, class: ClassVideo, schemes: []string{testSchemeTMDB}}
	links := stubLinks{
		links:    map[string][]Link{"profile-1": {{Service: testServiceVideo, SecretRef: "ref-1"}}},
		eligible: map[string]bool{}, // no row for the event's library => not eligible
	}
	secrets := stubSecrets{byRef: map[string]string{"ref-1": "token-abc"}}
	skips := NewMemorySkipSink(0)
	d := newDispatcher(t, []Adapter{adapter}, links, secrets, &memDeadLetter{}, skips)

	event := videoEvent()
	event.LibraryID = "library-adult"
	d.process(context.Background(), event)

	if adapter.calls() != 0 {
		t.Fatalf("an ineligible (default) Library must never scrobble, got %d calls", adapter.calls())
	}
	entries := skips.Entries()
	if len(entries) != 1 || !strings.Contains(entries[0].Reason, "not scrobble-eligible") {
		t.Fatalf("want a 'not scrobble-eligible' skip, got %+v", entries)
	}
}

// TestNoLinkedServiceSkips: an eligible Library but a Profile with no linked
// service emits nothing (a Profile with no linked account emits nothing).
func TestNoLinkedServiceSkips(t *testing.T) {
	adapter := &recordingAdapter{service: testServiceVideo, class: ClassVideo, schemes: []string{testSchemeTMDB}}
	links := stubLinks{
		links:    map[string][]Link{}, // profile-1 linked nothing
		eligible: map[string]bool{"library-films": true},
	}
	skips := NewMemorySkipSink(0)
	d := newDispatcher(t, []Adapter{adapter}, links, stubSecrets{}, &memDeadLetter{}, skips)

	d.process(context.Background(), videoEvent())

	if adapter.calls() != 0 {
		t.Fatalf("a Profile with no link must emit nothing, got %d calls", adapter.calls())
	}
	if len(skips.Entries()) != 1 {
		t.Fatalf("want 1 skip for no-link, got %d", len(skips.Entries()))
	}
}

// TestClassMismatchSkips: an audio target linked, a video play => the audio
// adapter is class-skipped (eligible classes).
func TestClassMismatchSkips(t *testing.T) {
	audio := &recordingAdapter{service: testServiceAudio, class: ClassAudio, schemes: []string{testSchemeMBRec}}
	links := stubLinks{
		links:    map[string][]Link{"profile-1": {{Service: testServiceAudio, SecretRef: "ref-1"}}},
		eligible: map[string]bool{"library-films": true},
	}
	secrets := stubSecrets{byRef: map[string]string{"ref-1": "tok"}}
	d := newDispatcher(t, []Adapter{audio}, links, secrets, &memDeadLetter{}, nil)

	d.process(context.Background(), videoEvent()) // video event, audio adapter

	if audio.calls() != 0 {
		t.Fatalf("an audio adapter must not receive a video play, got %d calls", audio.calls())
	}
}

// TestRetryThenDeadLetter: a permanently failing submission retries up to the
// attempt cap, then dead-letters with the service + reason.
func TestRetryThenDeadLetter(t *testing.T) {
	adapter := &recordingAdapter{service: testServiceVideo, class: ClassVideo, schemes: []string{testSchemeTMDB}, failPermanent: true}
	links := stubLinks{
		links:    map[string][]Link{"profile-1": {{Service: testServiceVideo, SecretRef: "ref-1"}}},
		eligible: map[string]bool{"library-films": true},
	}
	secrets := stubSecrets{byRef: map[string]string{"ref-1": "tok"}}
	dl := &memDeadLetter{}
	d, err := New(Config{MaxAttempts: 3, BaseBackoff: time.Millisecond, DeadLetter: dl}, []Adapter{adapter}, links, secrets, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	d.sleep = func(context.Context, time.Duration) error { return nil }

	d.process(context.Background(), videoEvent())

	if adapter.calls() != 3 {
		t.Fatalf("want 3 attempts (the cap), got %d", adapter.calls())
	}
	if dl.count() != 1 {
		t.Fatalf("want 1 dead-letter after exhaustion, got %d", dl.count())
	}
	dl.mu.Lock()
	entry := dl.entries[0]
	dl.mu.Unlock()
	if entry.Service != string(testServiceVideo) || entry.MediaID != "media-1" || entry.Attempts != 3 {
		t.Errorf("dead-letter entry mismatch: %+v", entry)
	}
}

func TestDeadLetterLastErrorRedactsSecret(t *testing.T) {
	const secret = "tok-secret"
	adapter := &recordingAdapter{
		service:       testServiceVideo,
		class:         ClassVideo,
		schemes:       []string{testSchemeTMDB},
		failPermanent: true,
		failMessage:   "401 token=" + secret,
	}
	links := stubLinks{
		links:    map[string][]Link{"profile-1": {{Service: testServiceVideo, SecretRef: "ref-1"}}},
		eligible: map[string]bool{"library-films": true},
	}
	secrets := stubSecrets{byRef: map[string]string{"ref-1": secret}}
	dl := &memDeadLetter{}
	d := newDispatcher(t, []Adapter{adapter}, links, secrets, dl, nil)

	d.process(context.Background(), videoEvent())

	if dl.count() != 1 {
		t.Fatalf("want 1 dead-letter after exhaustion, got %d", dl.count())
	}
	dl.mu.Lock()
	entry := dl.entries[0]
	dl.mu.Unlock()
	if strings.Contains(entry.LastError, secret) {
		t.Fatalf("LastError leaked secret: %q", entry.LastError)
	}
	if !strings.Contains(entry.LastError, "[REDACTED]") {
		t.Fatalf("LastError = %q, want redaction marker", entry.LastError)
	}
}

// TestRetryRecoversBeforeDeadLetter: a submission that fails twice then succeeds
// is delivered and NOT dead-lettered (backoff recovery).
func TestRetryRecoversBeforeDeadLetter(t *testing.T) {
	adapter := &recordingAdapter{service: testServiceVideo, class: ClassVideo, schemes: []string{testSchemeTMDB}, failTimes: 2}
	links := stubLinks{
		links:    map[string][]Link{"profile-1": {{Service: testServiceVideo, SecretRef: "ref-1"}}},
		eligible: map[string]bool{"library-films": true},
	}
	secrets := stubSecrets{byRef: map[string]string{"ref-1": "tok"}}
	dl := &memDeadLetter{}
	d := newDispatcher(t, []Adapter{adapter}, links, secrets, dl, nil)

	d.process(context.Background(), videoEvent())

	if adapter.calls() != 3 {
		t.Fatalf("want 3 calls (2 fail + 1 success), got %d", adapter.calls())
	}
	if dl.count() != 0 {
		t.Fatalf("a recovered submission must not dead-letter, got %d", dl.count())
	}
}

// TestWatchedThroughQueueAndRun drives the full async path: Watched enqueues,
// Run drains, the adapter receives the submission.
func TestWatchedThroughQueueAndRun(t *testing.T) {
	adapter := &recordingAdapter{service: testServiceVideo, class: ClassVideo, schemes: []string{testSchemeTMDB}}
	links := stubLinks{
		links:    map[string][]Link{"profile-1": {{Service: testServiceVideo, SecretRef: "ref-1"}}},
		eligible: map[string]bool{"library-films": true},
	}
	secrets := stubSecrets{byRef: map[string]string{"ref-1": "tok"}}
	d := newDispatcher(t, []Adapter{adapter}, links, secrets, &memDeadLetter{}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go d.Run(ctx)

	d.Watched(videoEvent())

	deadline := time.After(2 * time.Second)
	for adapter.calls() == 0 {
		select {
		case <-deadline:
			t.Fatal("watched event was not scrobbled through the queue within the deadline")
		case <-time.After(time.Millisecond):
		}
	}
}

// TestBackoffNeverExceedsCap: with a low MaxBackoff and enough attempts for
// the exponential growth to blow past it, every observed backoff (pre-jitter
// base, captured via d.sleep) must stay at or under the cap.
func TestBackoffNeverExceedsCap(t *testing.T) {
	adapter := &recordingAdapter{service: testServiceVideo, class: ClassVideo, schemes: []string{testSchemeTMDB}, failPermanent: true}
	links := stubLinks{
		links:    map[string][]Link{"profile-1": {{Service: testServiceVideo, SecretRef: "ref-1"}}},
		eligible: map[string]bool{"library-films": true},
	}
	secrets := stubSecrets{byRef: map[string]string{"ref-1": "tok"}}
	const maxBackoff = 10 * time.Millisecond
	d, err := New(Config{MaxAttempts: 8, BaseBackoff: time.Millisecond, MaxBackoff: maxBackoff, DeadLetter: &memDeadLetter{}},
		[]Adapter{adapter}, links, secrets, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	var mu sync.Mutex
	var waited []time.Duration
	d.sleep = func(_ context.Context, wait time.Duration) error {
		mu.Lock()
		waited = append(waited, wait)
		mu.Unlock()
		return nil
	}

	d.process(context.Background(), videoEvent())

	mu.Lock()
	defer mu.Unlock()
	if len(waited) == 0 {
		t.Fatal("expected at least one backoff wait")
	}
	for i, w := range waited {
		if w > maxBackoff {
			t.Errorf("wait[%d] = %v exceeds cap %v", i, w, maxBackoff)
		}
	}
}

// TestBackoffIsJittered: two dispatchers with identical config and RNG left
// at its default source produce backoff sequences that are not byte-identical
// to the raw unjittered exponential sequence — i.e. jitter is actually applied
// rather than passing the base duration straight through to sleep.
func TestBackoffIsJittered(t *testing.T) {
	adapter := &recordingAdapter{service: testServiceVideo, class: ClassVideo, schemes: []string{testSchemeTMDB}, failPermanent: true}
	links := stubLinks{
		links:    map[string][]Link{"profile-1": {{Service: testServiceVideo, SecretRef: "ref-1"}}},
		eligible: map[string]bool{"library-films": true},
	}
	secrets := stubSecrets{byRef: map[string]string{"ref-1": "tok"}}
	d, err := New(Config{MaxAttempts: 6, BaseBackoff: 100 * time.Millisecond, MaxBackoff: time.Hour, DeadLetter: &memDeadLetter{}},
		[]Adapter{adapter}, links, secrets, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	var mu sync.Mutex
	var waited []time.Duration
	d.sleep = func(_ context.Context, wait time.Duration) error {
		mu.Lock()
		waited = append(waited, wait)
		mu.Unlock()
		return nil
	}

	d.process(context.Background(), videoEvent())

	mu.Lock()
	defer mu.Unlock()
	base := 100 * time.Millisecond
	allExact := true
	for _, w := range waited {
		if w != base {
			allExact = false
		}
		if w < base/2 || w > base {
			t.Errorf("jittered wait %v out of expected [%v, %v] range for base %v", w, base/2, base, base)
		}
		base *= 2
	}
	if allExact {
		t.Error("no jitter observed: every wait matched the raw exponential base exactly")
	}
}

// TestRunLogsDroppedEventsOnShutdown: events still queued when ctx is
// canceled must be logged as dropped, not silently discarded. Run's select
// may still win a race and drain one event before observing ctx.Done (Go's
// select has no case priority), so this only pins the log contract, not the
// count of events processed.
func TestRunLogsDroppedEventsOnShutdown(t *testing.T) {
	adapter := &recordingAdapter{service: testServiceVideo, class: ClassVideo, schemes: []string{testSchemeTMDB}}
	links := stubLinks{
		links:    map[string][]Link{"profile-1": {{Service: testServiceVideo, SecretRef: "ref-1"}}},
		eligible: map[string]bool{"library-films": true},
	}
	secrets := stubSecrets{byRef: map[string]string{"ref-1": "tok"}}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	d, err := New(Config{BaseBackoff: time.Millisecond, DeadLetter: &memDeadLetter{}}, []Adapter{adapter}, links, secrets, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	d.sleep = func(context.Context, time.Duration) error { return nil }

	ctx, cancel := context.WithCancel(context.Background())
	// Fill the queue past what a single Run iteration can drain before
	// canceling, so at least one event is guaranteed to still be queued when
	// ctx.Done is observed.
	for i := 0; i < queueDepth; i++ {
		d.Watched(videoEvent())
	}
	cancel()

	done := make(chan struct{})
	go func() {
		d.Run(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after ctx cancellation")
	}

	if !strings.Contains(buf.String(), "dropping queued events") {
		t.Fatalf("expected a shutdown drop log, got: %s", buf.String())
	}
}

// TestNewDormantWithoutAdapters: no adapters => a nil (dormant) dispatcher, the
// opt-in default.
func TestNewDormantWithoutAdapters(t *testing.T) {
	d, err := New(Config{}, nil, stubLinks{}, stubSecrets{}, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if d != nil {
		t.Fatal("a dispatcher with no adapters must be dormant (nil)")
	}
}

// TestDispatcherStatusReadsExposeFeeds: the dispatcher's status
// accessors surface the adapter + skip feeds for the operator read, and degrade
// to nil for a dead-letter sink that is not readable (memDeadLetter has no
// Entries; the production FileDeadLetter does).
func TestDispatcherStatusReadsExposeFeeds(t *testing.T) {
	skips := NewMemorySkipSink(0)
	adapter := &recordingAdapter{service: testServiceAudio, class: ClassAudio, schemes: []string{testSchemeMBRec}}
	d := newDispatcher(t, []Adapter{adapter},
		stubLinks{
			links:    map[string][]Link{"p1": {{Service: testServiceAudio, SecretRef: "ref"}}},
			eligible: map[string]bool{"lib": true},
		},
		stubSecrets{byRef: map[string]string{"ref": "s"}}, &memDeadLetter{}, skips)

	descs := d.AdapterDescriptors()
	if len(descs) != 1 || descs[0].Service != string(testServiceAudio) || descs[0].Class != string(ClassAudio) {
		t.Fatalf("adapter descriptors = %+v", descs)
	}
	// memDeadLetter implements no Entries(), so DeadLetters() degrades to nil.
	if got := d.DeadLetters(); got != nil {
		t.Errorf("unreadable dead-letter sink should yield nil, got %v", got)
	}
	// Drive an ineligible-Library event so a skip is recorded, then read it back.
	d.process(context.Background(), WatchedEvent{ProfileID: "p1", MediaID: "m1", LibraryID: "other", Class: ClassAudio, TargetHint: TargetStandalone,
		ExternalIDs: []ExternalID{{Scheme: testSchemeMBRec, Value: "mbid"}}})
	got := d.Skips()
	if len(got) == 0 {
		t.Fatalf("MemorySkipSink must surface the recorded skip, got %v", got)
	}
}

// TestFileDeadLetterReadableThroughDispatcher: the production FileDeadLetter is
// readable, so DeadLetters() returns its feed.
func TestFileDeadLetterReadableThroughDispatcher(t *testing.T) {
	dl, err := OpenDeadLetter(t.TempDir() + "/dl.json")
	if err != nil {
		t.Fatalf("OpenDeadLetter: %v", err)
	}
	_ = dl.Record(DeadLetter{ProfileID: "p1", Service: "testaudio", MediaID: "m1", Attempts: 5, LastError: "boom", At: time.Unix(1, 0)})
	adapter := &recordingAdapter{service: testServiceAudio, class: ClassAudio, schemes: []string{testSchemeMBRec}}
	d := newDispatcher(t, []Adapter{adapter},
		stubLinks{eligible: map[string]bool{}}, stubSecrets{}, dl, NewMemorySkipSink(0))
	if got := d.DeadLetters(); len(got) != 1 || got[0].MediaID != "m1" {
		t.Fatalf("readable dead-letter feed = %+v", got)
	}
}

// TestWatchedDropsOnFullQueue: a full submission queue drops the newest event
// with a logged skip rather than blocking the playback path (queueDepth).
func TestWatchedDropsOnFullQueue(t *testing.T) {
	adapter := &recordingAdapter{service: testServiceVideo, class: ClassVideo, schemes: []string{testSchemeTMDB}}
	skips := NewMemorySkipSink(0)
	d := newDispatcher(t, []Adapter{adapter}, stubLinks{}, stubSecrets{}, &memDeadLetter{}, skips)
	// Nothing drains the queue (Run is never started), so it fills deterministically.
	for i := 0; i < queueDepth; i++ {
		d.Watched(videoEvent())
	}
	if len(d.queue) != queueDepth {
		t.Fatalf("queue = %d entries, want full at %d", len(d.queue), queueDepth)
	}

	d.Watched(videoEvent()) // one more: must drop, not block

	if len(d.queue) != queueDepth {
		t.Fatalf("queue = %d entries after overflow, want still %d", len(d.queue), queueDepth)
	}
	entries := skips.Entries()
	if len(entries) != 1 {
		t.Fatalf("want 1 recorded skip for the dropped event, got %d", len(entries))
	}
	if !strings.Contains(entries[0].Reason, "queue full") {
		t.Errorf("skip reason should name the full queue, got %q", entries[0].Reason)
	}
}

// TestFileDeadLetterCapEviction: Record beyond maxDeadLetters drops the
// oldest entries, keeping only the most recent maxDeadLetters.
func TestFileDeadLetterCapEviction(t *testing.T) {
	dl, err := OpenDeadLetter(t.TempDir() + "/dl.json")
	if err != nil {
		t.Fatalf("OpenDeadLetter: %v", err)
	}
	total := maxDeadLetters + 10
	for i := 0; i < total; i++ {
		if err := dl.Record(DeadLetter{
			ProfileID: "p1",
			Service:   "testvideo",
			MediaID:   "media",
			Attempts:  1,
			At:        time.Unix(int64(i), 0),
		}); err != nil {
			t.Fatalf("Record %d: %v", i, err)
		}
	}
	entries := dl.Entries()
	if len(entries) != maxDeadLetters {
		t.Fatalf("entries = %d, want capped at %d", len(entries), maxDeadLetters)
	}
	// The oldest (i=0..9) must have been evicted; the newest (i=total-1) must remain.
	dl.mu.Lock()
	oldest := dl.state.Entries[0].At.Unix()
	dl.mu.Unlock()
	if oldest < 10 {
		t.Errorf("oldest surviving entry at %d, want the first 10 evicted", oldest)
	}
}

// TestOpenDeadLetterMissingFileIsFreshFeed: a missing dead-letter file is not
// an error — it's a fresh, empty feed.
func TestOpenDeadLetterMissingFileIsFreshFeed(t *testing.T) {
	dl, err := OpenDeadLetter(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("OpenDeadLetter: %v", err)
	}
	if got := dl.Entries(); len(got) != 0 {
		t.Fatalf("fresh feed entries = %v, want empty", got)
	}
}

// TestOpenDeadLetterCorruptFileErrors: a present but corrupt (non-JSON)
// dead-letter file is a load error, not a silent reset — an operator must
// notice a clobbered record instead of losing prior failures.
func TestOpenDeadLetterCorruptFileErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dl.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("seed corrupt file: %v", err)
	}
	if _, err := OpenDeadLetter(path); err == nil {
		t.Fatal("want an error opening a corrupt dead-letter file, got nil")
	}
}

func TestOpenDeadLetterRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversized.json")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxDeadLetterBytes + 1); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenDeadLetter(path); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized dead-letter error = %v, want size-limit error", err)
	}
}

// --- LoadAdapters --------------------------------------------------------

func stubFactory(t *testing.T) AdapterFactory {
	t.Helper()
	return func(client Doer, cfg AdapterConfig, secrets SecretSource) (Adapter, error) {
		return &recordingAdapter{service: cfg.Service, class: ClassVideo, schemes: []string{testSchemeTMDB}}, nil
	}
}

// TestLoadAdaptersMissingFileIsDormant: no config file at path is not an
// error — the plugin stays dormant.
func TestLoadAdaptersMissingFileIsDormant(t *testing.T) {
	adapters, cfg, err := LoadAdapters(filepath.Join(t.TempDir(), "missing.json"), stubSecrets{}, &recordingDoer{}, nil)
	if err != nil {
		t.Fatalf("LoadAdapters: %v", err)
	}
	if adapters != nil || cfg != (Config{}) {
		t.Fatalf("got adapters=%v cfg=%+v, want dormant (nil, zero)", adapters, cfg)
	}
}

// TestLoadAdaptersEmptyAdaptersIsDormant: a config file present but with an
// empty adapters list is also dormant.
func TestLoadAdaptersEmptyAdaptersIsDormant(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg.json")
	if err := os.WriteFile(path, []byte(`{"adapters":[]}`), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	adapters, cfg, err := LoadAdapters(path, stubSecrets{}, &recordingDoer{}, nil)
	if err != nil {
		t.Fatalf("LoadAdapters: %v", err)
	}
	if adapters != nil || cfg != (Config{}) {
		t.Fatalf("got adapters=%v cfg=%+v, want dormant (nil, zero)", adapters, cfg)
	}
}

// TestLoadAdaptersMalformedFileErrors: a present but malformed (non-JSON)
// config file fails at boot rather than silently degrading.
func TestLoadAdaptersMalformedFileErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	if _, _, err := LoadAdapters(path, stubSecrets{}, &recordingDoer{}, nil); err == nil {
		t.Fatal("want an error loading a malformed config file, got nil")
	}
}

func TestLoadAdaptersNilDoerErrors(t *testing.T) {
	if _, _, err := LoadAdapters(filepath.Join(t.TempDir(), "missing.json"), stubSecrets{}, nil, nil); err == nil {
		t.Fatal("want an error with a nil HTTP client, got nil")
	}
}

// TestLoadAdaptersNilSecretsErrors: a config naming adapters with no secret
// source wired is a boot-time error.
func TestLoadAdaptersNilSecretsErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg.json")
	body := `{"adapters":[{"service":"testvideo"}]}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	registry := map[Service]AdapterFactory{testServiceVideo: stubFactory(t)}
	if _, _, err := LoadAdapters(path, nil, &recordingDoer{}, registry); err == nil {
		t.Fatal("want an error with a nil secret source, got nil")
	}
}

// TestLoadAdaptersUnknownServiceErrors: a config naming a service absent
// from the registry fails at boot (a misconfiguration, not a silent skip).
func TestLoadAdaptersUnknownServiceErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg.json")
	body := `{"adapters":[{"service":"unknown-service"}]}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	registry := map[Service]AdapterFactory{testServiceVideo: stubFactory(t)}
	if _, _, err := LoadAdapters(path, stubSecrets{}, &recordingDoer{}, registry); err == nil {
		t.Fatal("want an error for an unknown service, got nil")
	}
}

// TestLoadAdaptersFactoryErrorPropagates: an error from the adapter factory
// (e.g. an unresolvable operator credential) fails LoadAdapters.
func TestLoadAdaptersFactoryErrorPropagates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg.json")
	body := `{"adapters":[{"service":"testvideo","api_key_ref":"missing-ref"}]}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	wantErr := errors.New("credential unresolvable")
	registry := map[Service]AdapterFactory{
		testServiceVideo: func(client Doer, cfg AdapterConfig, secrets SecretSource) (Adapter, error) {
			return nil, wantErr
		},
	}
	if _, _, err := LoadAdapters(path, stubSecrets{}, &recordingDoer{}, registry); err == nil {
		t.Fatal("want the factory error to propagate, got nil")
	}
}

// TestLoadAdaptersSuccessInjectsClientAndMaxAttempts: a well-formed config
// builds the declared adapters, injects the provided client, and carries the
// configured MaxAttempts through.
func TestLoadAdaptersSuccessInjectsClientAndMaxAttempts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg.json")
	body := `{"adapters":[{"service":"testvideo"}],"max_attempts":7}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	baseClient := &recordingDoer{}
	var gotClient Doer
	registry := map[Service]AdapterFactory{
		testServiceVideo: func(client Doer, cfg AdapterConfig, secrets SecretSource) (Adapter, error) {
			gotClient = client
			return &recordingAdapter{service: cfg.Service, class: ClassVideo}, nil
		},
	}
	adapters, cfg, err := LoadAdapters(path, stubSecrets{}, baseClient, registry)
	if err != nil {
		t.Fatalf("LoadAdapters: %v", err)
	}
	if len(adapters) != 1 {
		t.Fatalf("adapters = %d, want 1", len(adapters))
	}
	if cfg.MaxAttempts != 7 {
		t.Errorf("cfg.MaxAttempts = %d, want 7", cfg.MaxAttempts)
	}
	if gotClient == nil || gotClient == baseClient {
		t.Errorf("factory did not receive guarded client")
	}
}

func TestLoadAdaptersDeclaredEndpointAllowlistAllowsDeclaredHost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg.json")
	if err := os.WriteFile(path, []byte(`{"adapters":[{"service":"testvideo"}]}`), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	baseClient := &recordingDoer{}
	registry := map[Service]AdapterFactory{
		testServiceVideo: func(client Doer, cfg AdapterConfig, secrets SecretSource) (Adapter, error) {
			return &clientAdapter{
				client:    client,
				target:    "https://api.allowed.example/scrobble",
				endpoints: []string{"https://api.allowed.example/v1"},
			}, nil
		},
	}
	adapters, _, err := LoadAdapters(path, stubSecrets{}, baseClient, registry)
	if err != nil {
		t.Fatalf("LoadAdapters: %v", err)
	}
	if err := adapters[0].Submit(context.Background(), Scrobble{}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if baseClient.count() != 1 {
		t.Fatalf("base client calls = %d, want 1", baseClient.count())
	}
}

func TestLoadAdaptersDeclaredEndpointAllowlistDeniesUndeclaredHost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg.json")
	if err := os.WriteFile(path, []byte(`{"adapters":[{"service":"testvideo"}]}`), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	baseClient := &recordingDoer{}
	registry := map[Service]AdapterFactory{
		testServiceVideo: func(client Doer, cfg AdapterConfig, secrets SecretSource) (Adapter, error) {
			return &clientAdapter{
				client:    client,
				target:    "https://api.denied.example/scrobble",
				endpoints: []string{"https://api.allowed.example"},
			}, nil
		},
	}
	adapters, _, err := LoadAdapters(path, stubSecrets{}, baseClient, registry)
	if err != nil {
		t.Fatalf("LoadAdapters: %v", err)
	}
	if err := adapters[0].Submit(context.Background(), Scrobble{}); err == nil {
		t.Fatal("want undeclared host error, got nil")
	}
	if baseClient.count() != 0 {
		t.Fatalf("base client calls = %d, want 0", baseClient.count())
	}
}

func TestRedirectGuard(t *testing.T) {
	endpoints := func() []string { return []string{"https://api.allowed.example/v1"} }
	guard := RedirectGuard(endpoints)

	toDeclared := &http.Request{URL: mustURL(t, "https://api.allowed.example/next")}
	if err := guard(toDeclared, nil); err != nil {
		t.Fatalf("redirect to a declared host rejected: %v", err)
	}

	toUndeclared := &http.Request{URL: mustURL(t, "https://evil.example/next")}
	if err := guard(toUndeclared, nil); err == nil {
		t.Fatal("redirect to an undeclared host accepted, want rejection")
	}

	noEndpoints := RedirectGuard(func() []string { return nil })
	if err := noEndpoints(toDeclared, nil); err == nil {
		t.Fatal("redirect with no declared endpoints accepted, want fail-closed rejection")
	}

	if err := guard(&http.Request{}, nil); err == nil {
		t.Fatal("redirect with nil URL accepted, want rejection")
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
}

// TestAllowlistDoerOriginMatching: the allowlist matches the full origin —
// scheme, host, and port — so a declared https endpoint never authorizes an
// http downgrade on the same host, bare-host declarations default to
// https:443, explicit ports match exactly, and non-http(s) schemes fail
// closed on both sides.
func TestAllowlistDoerOriginMatching(t *testing.T) {
	cases := []struct {
		name     string
		declared []string
		target   string
		allowed  bool
	}{
		{"declared https origin allows https", []string{"https://api.allowed.example"}, "https://api.allowed.example/x", true},
		{"declared https origin rejects the http downgrade", []string{"https://api.allowed.example"}, "http://api.allowed.example/x", false},
		{"explicit port allows that origin", []string{"http://127.0.0.1:8080"}, "http://127.0.0.1:8080/x", true},
		{"explicit port rejects a different port", []string{"http://127.0.0.1:8080"}, "http://127.0.0.1:8081/x", false},
		{"bare host declaration allows https on the default port", []string{"api.allowed.example"}, "https://api.allowed.example/x", true},
		{"bare host declaration rejects http on the same host", []string{"api.allowed.example"}, "http://api.allowed.example/x", false},
		{"non-http(s) request scheme rejected", []string{"https://api.allowed.example"}, "ftp://api.allowed.example/x", false},
		{"non-http(s) declared endpoint matches nothing", []string{"ftp://api.allowed.example"}, "https://api.allowed.example/x", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := &recordingDoer{}
			guarded := allowlistDoer{next: base, endpoints: func() []string { return tc.declared }}
			resp, err := guarded.Do(&http.Request{URL: mustURL(t, tc.target)})
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			if tc.allowed {
				if err != nil {
					t.Fatalf("declared origin rejected: %v", err)
				}
				if base.count() != 1 {
					t.Fatalf("base client calls = %d, want 1", base.count())
				}
				return
			}
			if err == nil {
				t.Fatal("undeclared origin accepted, want rejection")
			}
			if base.count() != 0 {
				t.Fatalf("base client calls = %d, want 0 for a rejected origin", base.count())
			}
		})
	}
}

// TestLoadAdaptersDeclaredEndpointAllowlistDeniesSchemeDowngrade: through the
// full LoadAdapters wiring, a declared https endpoint never authorizes
// cleartext http to the same host — the downgrade would replay the
// Authorization-bearing request without TLS.
func TestLoadAdaptersDeclaredEndpointAllowlistDeniesSchemeDowngrade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg.json")
	if err := os.WriteFile(path, []byte(`{"adapters":[{"service":"testvideo"}]}`), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	baseClient := &recordingDoer{}
	registry := map[Service]AdapterFactory{
		testServiceVideo: func(client Doer, cfg AdapterConfig, secrets SecretSource) (Adapter, error) {
			return &clientAdapter{
				client:    client,
				target:    "http://api.allowed.example/x",
				endpoints: []string{"https://api.allowed.example"},
			}, nil
		},
	}
	adapters, _, err := LoadAdapters(path, stubSecrets{}, baseClient, registry)
	if err != nil {
		t.Fatalf("LoadAdapters: %v", err)
	}
	if err := adapters[0].Submit(context.Background(), Scrobble{}); err == nil {
		t.Fatal("want scheme-downgrade rejection, got nil")
	}
	if baseClient.count() != 0 {
		t.Fatalf("base client calls = %d, want 0", baseClient.count())
	}
}

// TestRedirectGuardRejectsSchemeDowngrade: a redirect from the declared https
// origin to http on the SAME host is rejected — Go strips credentials only on
// a host change, so the downgrade would replay the secret over cleartext.
func TestRedirectGuardRejectsSchemeDowngrade(t *testing.T) {
	guard := RedirectGuard(func() []string { return []string{"https://api.allowed.example"} })

	downgrade := &http.Request{URL: mustURL(t, "http://api.allowed.example/x")}
	if err := guard(downgrade, nil); err == nil {
		t.Fatal("redirect downgrading https to http on the declared host accepted, want rejection")
	}

	sameOrigin := &http.Request{URL: mustURL(t, "https://api.allowed.example/x")}
	if err := guard(sameOrigin, nil); err != nil {
		t.Fatalf("redirect within the declared origin rejected: %v", err)
	}
}

// TestNilDispatcherRunReturns: the dormant (nil) dispatcher New returns when
// scrobbling is unconfigured must survive a Run call — it returns immediately
// without panicking, matching Watched and the other nil-safe methods.
func TestNilDispatcherRunReturns(t *testing.T) {
	var d *Dispatcher
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		d.Run(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("nil dispatcher Run did not return")
	}
}

// TestSanitizeRedactsEncodedSecretForms: sanitize redacts not only the raw
// secret but every secretForms wire encoding — URL-escaped and the four
// base64 variants — so an adapter error embedding the credential in a query
// string or Basic auth header never reaches a log or dead-letter row.
func TestSanitizeRedactsEncodedSecretForms(t *testing.T) {
	const secret = "tok+secret/1?"
	forms := map[string]string{
		"raw":            secret,
		"url-escaped":    url.QueryEscape(secret),
		"base64-std":     base64.StdEncoding.EncodeToString([]byte(secret)),
		"base64-url":     base64.URLEncoding.EncodeToString([]byte(secret)),
		"base64-raw-std": base64.RawStdEncoding.EncodeToString([]byte(secret)),
		"base64-raw-url": base64.RawURLEncoding.EncodeToString([]byte(secret)),
	}
	for name, form := range forms {
		t.Run(name, func(t *testing.T) {
			got := sanitize("401 authorization "+form+" rejected", secret)
			if strings.Contains(got, form) {
				t.Fatalf("sanitize left the %s secret form in %q", name, got)
			}
			if !strings.Contains(got, "[REDACTED]") {
				t.Fatalf("sanitize did not mark the redaction: %q", got)
			}
		})
	}

	if got := sanitize("no secret here", ""); got != "no secret here" {
		t.Errorf("sanitize with an empty secret rewrote the message: %q", got)
	}
}

// TestOpenDeadLetterReappliesCapOnLoad: a dead-letter file exceeding
// maxDeadLetters (written by a different build, or edited by hand) is capped
// on load, keeping the newest entries — not deferred to the next Record.
func TestOpenDeadLetterReappliesCapOnLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dl.json")
	total := maxDeadLetters + 25
	state := deadLetterState{Version: 1}
	for i := 0; i < total; i++ {
		state.Entries = append(state.Entries, DeadLetter{
			ProfileID: "p1",
			Service:   "testvideo",
			MediaID:   "media",
			Attempts:  1,
			At:        time.Unix(int64(i), 0),
		})
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal oversized state: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("seed oversized file: %v", err)
	}

	dl, err := OpenDeadLetter(path)
	if err != nil {
		t.Fatalf("OpenDeadLetter: %v", err)
	}
	if got := dl.Entries(); len(got) != maxDeadLetters {
		t.Fatalf("entries = %d after load, want capped at %d", len(got), maxDeadLetters)
	}
	dl.mu.Lock()
	oldest := dl.state.Entries[0].At.Unix()
	dl.mu.Unlock()
	if oldest != int64(total-maxDeadLetters) {
		t.Errorf("oldest surviving entry at %d, want %d (the oldest 25 evicted)", oldest, total-maxDeadLetters)
	}
}

// deadlineAdapter records whether the ctx its Submit received carried a
// deadline — the per-attempt bound submitWithRetry must install.
type deadlineAdapter struct {
	mu          sync.Mutex
	hadDeadline bool
	deadline    time.Time
}

func (a *deadlineAdapter) Service() Service          { return testServiceVideo }
func (a *deadlineAdapter) Class() MediaClass         { return ClassVideo }
func (a *deadlineAdapter) IdentitySchemes() []string { return []string{testSchemeTMDB} }
func (a *deadlineAdapter) DeclaredEndpoints() []string {
	return []string{"https://api.example.test"}
}

func (a *deadlineAdapter) Submit(ctx context.Context, _ Scrobble) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.deadline, a.hadDeadline = ctx.Deadline()
	return nil
}

// TestSubmitAttemptCtxCarriesDeadline: each Submit attempt runs under a ctx
// with a deadline (submitAttemptTimeout), even when the caller's ctx has
// none — one hung request must not stall the single worker indefinitely.
func TestSubmitAttemptCtxCarriesDeadline(t *testing.T) {
	adapter := &deadlineAdapter{}
	links := stubLinks{
		links:    map[string][]Link{"profile-1": {{Service: testServiceVideo, SecretRef: "ref-1"}}},
		eligible: map[string]bool{"library-films": true},
	}
	secrets := stubSecrets{byRef: map[string]string{"ref-1": "tok"}}
	d := newDispatcher(t, []Adapter{adapter}, links, secrets, &memDeadLetter{}, nil)

	before := time.Now()
	d.process(context.Background(), videoEvent())

	adapter.mu.Lock()
	hadDeadline, deadline := adapter.hadDeadline, adapter.deadline
	adapter.mu.Unlock()
	if !hadDeadline {
		t.Fatal("Submit received a ctx without a deadline; each attempt must be bounded by submitAttemptTimeout")
	}
	if latest := before.Add(submitAttemptTimeout + time.Minute); deadline.After(latest) {
		t.Errorf("attempt deadline %v is later than submitAttemptTimeout plus slack (%v)", deadline, latest)
	}
}
