package sdk

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// The Migration Wizard importer seam.
//
// An importer is a first-party compiled plugin hook — the third alongside the
// filename Parser and MatchScorer (scan.go) — that a plugin *module* implements:
// it yields the foreign items for one import run. The seam's data types and the
// Source interface are published here (store-free) so a plugin module can
// implement them; core's internal/migrate consumes these types via aliases and
// adapts ExternalID -> store.MediaExternalID at its match boundary. JobKind,
// Payload, Handler, and the app-side SourceResolver wiring stay in core.

// Import-category vocabulary: the fixed set of data categories a foreign item
// can carry. A Source reports the subset it carries for a run via Categories();
// the handler applies a category's state only when the run admits it.
const (
	CategoryWatchState    = "watch-state"
	CategoryRatings       = "ratings"
	CategoryCollections   = "collections"
	CategorySubscriptions = "subscriptions"
	CategoryItemState     = "item-state"
)

// ExternalID is a plugin-contributed media identifier (scheme + value) a source
// recorded when it has no trustworthy file path — the generic external-ID match
// key. It is the store-free seam shape; core adapts it to its persisted
// external-ID type at the match boundary.
type ExternalID struct {
	Scheme string
	Value  string
}

// ForeignItem is one record a source parser yields: a single item from the
// predecessor server with its mapped behavioral state. It is keyed for matching
// by Path (the absolute file path the source recorded). The handler runs the
// match chain on Path to find the core Media, then applies whichever state
// fields the run's categories admit. Fields outside the run's selected
// categories are ignored even when the parser populates them.
type ForeignItem struct {
	// Path is the absolute file path the source recorded for this item — the
	// first match-chain key after external IDs. A file-keyed source
	// (Plex/Emby/Jellyfin/NFO) sets it; an item with no Path and no Scrobble
	// or external IDs cannot be matched and is recorded unmatched.
	Path string
	// ExternalIDs are plugin-contributed media identifiers the source recorded
	// when it has no trustworthy file path. The handler treats them as a
	// generic match key: exactly one Media for a scheme/value is accepted,
	// duplicates stay unmatched rather than guessed.
	ExternalIDs []ExternalID
	// WatchState carries resume position + completion, applied when the run
	// includes the watch-state category. Zero value = no watch-state.
	WatchState WatchState
	// Scrobble carries a metadata-keyed play event (artist/track/album +
	// timestamp) for a source that records no file path — a Last.fm scrobble
	// When Present, the handler resolves the item by metadata (the
	// Scrobble match chain) instead of by Path, and applies a play through the
	// play-count/last-played projection. Zero value = no scrobble.
	Scrobble ScrobbleRecord
	// Subscription carries one feed subscription from a source that records a
	// feed list rather than file-keyed Media. When Present, the handler applies
	// it to the target Library in the run payload and follows it for the target
	// Profile. Zero value = no subscription.
	Subscription Subscription
	// ItemState carries one feed-scoped serial item state from a source that
	// keys items by feed URL + item GUID rather than by file path. When Present,
	// the handler records it in the item-state replay ledger for the target
	// Library subscription. Zero value = no item-state.
	ItemState ItemState
	// SmartCollection carries one smart collection/playlist the source recorded
	// as a rule-driven (auto-updating) container. The handler imports it as a
	// STATIC current-member snapshot and surfaces it on the report's distinct
	// smart-collection→static warning line rather
	// than folding the collection itself into the match-rate number. Zero value
	// = no smart collection.
	SmartCollection SmartCollection
	// Rating carries one imported per-Profile numeric rating, applied when the
	// run includes the ratings category. Zero value = no
	// rating.
	Rating Rating
	// CollectionMember carries this item's ordered membership in one static
	// (non-rule-driven) collection/playlist, applied when the run includes the
	// collections category. Zero value = no membership.
	CollectionMember CollectionMember
}

// Rating is one imported per-Profile numeric rating ("Numeric
// or star ratings → per-Profile ratings"). It carries the value on the
// source's own scale rather than a pre-normalized number, so the handler
// rescales generically instead of the seam assuming any one source's
// 5-star/10-point/100-point convention (invariant #1 — generic by shape).
type Rating struct {
	// Present is true when the parser populated a rating for this item, so an
	// unrated item is distinguishable from a genuine 0-value rating.
	Present bool
	// Value is the rating on the source's native scale (e.g. 4 of a 5-star
	// scale, 8.5 of a 10-point scale).
	Value float64
	// Scale is the maximum value of the source's rating scale (e.g. 5, 10,
	// 100) — required whenever Present, since the handler cannot interpret
	// Value without knowing the scale it was recorded on.
	Scale float64
}

// CollectionMember is this foreign item's ordered position in one static
// (non-rule-driven) collection/playlist the source recorded (playlists and
// collections import as ordered Collections) — distinct from
// SmartCollection above, which covers the rule-driven case the seam still
// cannot import member-by-member. The source yields one CollectionMember per
// (item, collection) membership; the handler groups memberships that share a
// Name and orders them by Position to reconstruct the collection, using each
// member item's own Path/ExternalIDs as its identity — no separate member
// identity list is needed here.
type CollectionMember struct {
	// Present is true when the parser populated a static-collection membership
	// for this item, so "no membership" is distinguishable from membership at
	// position 0.
	Present bool
	// Name is the collection's title — the grouping key memberships are
	// collected under. Required whenever Present.
	Name string
	// Position is this item's zero-based order within the collection, the
	// ordering key that reconstructs the source's playlist order.
	Position int
}

// SmartCollection is one imported smart collection/playlist: a rule-driven
// container the source auto-maintained. Beamfall imports it as a STATIC snapshot
// of its current members ("a smart collection imports its current
// members, statically") because the seam carries no rule/predicate
// representation, so the imported collection will not auto-update. The handler
// counts it toward the report's distinct smart-collection→static warning line so
// the operator sees the silent behavioral downgrade explicitly. It is generic by
// shape: an opaque stable identity, display name, and ordered member identities,
// never a source-system or domain term.
type SmartCollection struct {
	// Present is true when the parser populated a smart collection for this item,
	// so an empty-name collection is still distinguishable from "no collection".
	Present bool
	// Identity is the source's opaque stable identity for this collection. It is
	// required whenever Present and remains stable across renames and replays.
	// Core treats it only as an identity component; it is never display text.
	Identity string
	// Name is the source's title for the collection — display text only, never a
	// match key or a domain term. Required whenever Present.
	Name string
	// CurrentMembersPresent is true when the source explicitly supplied the
	// current-member snapshot. It distinguishes an intentionally empty snapshot
	// from an older/count-only producer that cannot safely claim fidelity.
	CurrentMembersPresent bool
	// CurrentMembers carries the current members in source order. Each entry is
	// the existing generic match input shape; handlers inspect only its identity
	// fields (ExternalIDs, Path, and Scrobble metadata fallback), never its
	// behavioral-state fields.
	CurrentMembers []ForeignItem
	// MemberCount is the number of current members captured in the static
	// snapshot at import time. A handler verifies it equals len(CurrentMembers)
	// before reporting the collection as imported.
	MemberCount int
}

// Subscription is one imported feed subscription: a feed URL,
// optional title, and optional target root. The run payload supplies the
// target Library; Root here overrides the payload root only when the parser has
// a per-feed download root.
type Subscription struct {
	// Present is true when the parser populated a subscription for this item,
	// so an empty title/root is still distinguishable from "no subscription".
	Present bool
	FeedURL string
	Root    string
	Title   string
}

// ItemState is one imported serial item-state row keyed by feed URL + item
// GUID. It is a generic shape used by podcast importers today; the source
// plugin owns the source-system parsing.
type ItemState struct {
	Present     bool
	FeedURL     string
	GUID        string
	Played      bool
	HasProgress bool
	Progress    float64
	Starred     bool
}

// ScrobbleRecord is one metadata-keyed play event a source recorded with no
// file path — a Last.fm scrobble. The handler resolves
// it to a core Media by metadata (track title, confirmed by artist; album is a
// tiebreaker) and applies it as a play (increment count + advance last-played)
// to the importing Profile's engagement. It is plain media vocab (artist/
// track/album — invariant #1), never a domain term.
type ScrobbleRecord struct {
	// Present is true when the parser populated a scrobble for this item, so a
	// zero-timestamp scrobble is still distinguishable from "no scrobble".
	Present bool
	// Artist is the recorded performing artist — the required disambiguator on
	// a title match (two libraries can share a track title across artists).
	Artist string
	// Track is the recorded track title — the metadata match-chain key.
	// Required; an empty Track cannot be matched and is recorded unmatched.
	Track string
	// Album is the recorded album title; an optional tiebreaker when more than
	// one artist-matched candidate survives. Empty when the export omits it.
	Album string
	// PlayedAt is the true historical instant the source recorded the play —
	// the last-played timestamp the apply advances toward. Zero when the export
	// recorded none (last-played then stays unset; the play still counts).
	PlayedAt time.Time
}

// WatchState is a foreign item's playback history: the resume position, the
// completion flag, and the true historical instant the source last recorded
// play. The handler writes it through the import-provenance engagement path so
// it keeps its real timestamp without masquerading as a fresh play.
type WatchState struct {
	// Present is true when the parser populated watch-state for this item; a
	// zero-position-but-watched record is meaningful, so a flag distinguishes
	// "no data" from "watched at position 0".
	Present bool
	// PositionSeconds is the resume offset; 0 for a completed or never-resumed
	// item.
	PositionSeconds int
	// DurationSeconds is the item's total runtime when the source knew it; 0
	// when unknown (the engagement threshold then can't fire on duration, so
	// Watched must carry completion).
	DurationSeconds int
	// Watched is the source's completion flag.
	Watched bool
	// OccurredAt is the true historical instant the source last played the
	// item; zero when the source recorded none (the handler then leaves the
	// historical timestamp unset, and the event time is the import instant).
	OccurredAt time.Time
}

// MaxImportPageItems is the largest page a StreamingSource may hand to its
// callback. Keeping the bound in the contract lets consumers apply arbitrarily
// large imports without retaining the complete source in memory.
const MaxImportPageItems = 256

var (
	// ErrInvalidImportCheckpoint reports a malformed or out-of-range legacy
	// continuation token. StreamingSource implementations define their own
	// opaque token syntax and should wrap this error for invalid tokens.
	ErrInvalidImportCheckpoint = errors.New("sdk: invalid import checkpoint")
	// ErrInvalidImportPage reports a StreamingSource contract violation.
	ErrInvalidImportPage = errors.New("sdk: invalid import page")
)

// Source is the legacy per-run parser seam. Categories reports
// which data categories this source carries for the run.
//
// Deprecated: implement StreamingSource in addition to Source during the 0.x
// compatibility window. Items remains temporarily required by existing
// consumers, but it necessarily materializes the complete import and is not
// suitable for large sources. New consumers should call StreamSource, which
// prefers StreamingSource and falls back to this interface.
type Source interface {
	// Categories returns the data categories the items carry, from the fixed
	// vocabulary. The handler applies a category's state only when it appears
	// here.
	Categories() []string
	// Items yields every foreign item the source holds for the run.
	//
	// Deprecated: implement StreamingSource.Stream. Items is retained only for
	// source compatibility with pre-streaming consumers.
	Items(ctx context.Context) ([]ForeignItem, error)
}

// StreamingSource is the bounded, cancellable Migration Wizard importer seam.
// During the Source compatibility window an importer implements both
// interfaces; consumers discover this interface with a type assertion.
//
// checkpoint is an opaque exclusive continuation token returned by a previous
// successful emit callback. The empty checkpoint starts at the first item.
// Every non-empty page carries a non-empty next checkpoint and contains at most
// MaxImportPageItems items. For unchanged input, a source must produce the same
// ordered items and checkpoints from the same starting checkpoint.
//
// emit is synchronous. It must atomically apply the page and persist its next
// checkpoint before returning nil. If emit returns an error, Stream must stop
// and return that error without emitting another page; the caller resumes from
// the last checkpoint whose callback returned nil. Implementations must check
// ctx before reading or emitting more work and return its cancellation error
// promptly.
type StreamingSource interface {
	Categories() []string
	Stream(
		ctx context.Context,
		checkpoint string,
		emit func(context.Context, []ForeignItem, string) error,
	) error
}

// StreamSource consumes source through StreamingSource when available. It
// validates page bounds and checkpoint progress and preserves an emit callback
// error exactly. A legacy Source is adapted into bounded callback pages, but
// its deprecated Items call still materializes the complete import.
func StreamSource(
	ctx context.Context,
	source Source,
	checkpoint string,
	emit func(context.Context, []ForeignItem, string) error,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if emit == nil {
		return fmt.Errorf("%w: nil callback", ErrInvalidImportPage)
	}
	if streaming, ok := source.(StreamingSource); ok {
		return streamValidated(ctx, streaming, checkpoint, emit)
	}
	return streamLegacy(ctx, source, checkpoint, emit)
}

func streamValidated(
	ctx context.Context,
	source StreamingSource,
	checkpoint string,
	emit func(context.Context, []ForeignItem, string) error,
) error {
	lastCheckpoint := checkpoint
	var callbackErr error
	err := source.Stream(ctx, checkpoint, func(
		pageCtx context.Context,
		items []ForeignItem,
		nextCheckpoint string,
	) error {
		if callbackErr != nil {
			return callbackErr
		}
		callbackErr = importPageViolation(ctx, pageCtx, items, lastCheckpoint, nextCheckpoint)
		if callbackErr != nil {
			return callbackErr
		}
		callbackErr = emit(pageCtx, items, nextCheckpoint)
		if callbackErr == nil {
			lastCheckpoint = nextCheckpoint
		}
		return callbackErr
	})
	if callbackErr != nil {
		return callbackErr
	}
	if err != nil {
		return err
	}
	return ctx.Err()
}

func importPageViolation(
	ctx context.Context,
	pageCtx context.Context,
	items []ForeignItem,
	lastCheckpoint string,
	nextCheckpoint string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if pageCtx == nil {
		return fmt.Errorf("%w: nil context", ErrInvalidImportPage)
	}
	if err := pageCtx.Err(); err != nil {
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("%w: empty page", ErrInvalidImportPage)
	}
	if len(items) > MaxImportPageItems {
		return fmt.Errorf("%w: %d items exceeds limit %d", ErrInvalidImportPage, len(items), MaxImportPageItems)
	}
	if nextCheckpoint == "" {
		return fmt.Errorf("%w: empty checkpoint", ErrInvalidImportPage)
	}
	if nextCheckpoint == lastCheckpoint {
		return fmt.Errorf("%w: checkpoint did not advance", ErrInvalidImportPage)
	}
	return nil
}

func streamLegacy(
	ctx context.Context,
	source Source,
	checkpoint string,
	emit func(context.Context, []ForeignItem, string) error,
) error {
	items, err := source.Items(ctx)
	if err != nil {
		return err
	}
	start, err := legacyImportOffset(checkpoint, len(items))
	if err != nil {
		return err
	}
	for start < len(items) {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := min(start+MaxImportPageItems, len(items))
		if err := emit(ctx, items[start:end], strconv.Itoa(end)); err != nil {
			return err
		}
		start = end
	}
	return ctx.Err()
}

func legacyImportOffset(checkpoint string, itemCount int) (int, error) {
	if checkpoint == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(checkpoint)
	if err != nil || offset < 0 || offset > itemCount {
		return 0, fmt.Errorf("%w %q", ErrInvalidImportCheckpoint, checkpoint)
	}
	return offset, nil
}
