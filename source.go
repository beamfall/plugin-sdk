package sdk

var sourceKinds = []string{
	"recently-added",
	"continue-watching",
	// next-up: for each ordered container the Profile has
	// engagement in, the first unwatched child in play-next order —
	// deliberately a separate shelf from continue-watching (Plex's merged
	// On Deck is the cautionary tale). Generic by construction: works for
	// any container archetype, no domain vocabulary.
	"next-up",
	"recently-played",
	"most-played",
	"all-items",
	// on-this-day: a generic calendar-date projection over a
	// date Media field ("capture-date" today). It returns visible Media from
	// prior years whose month/day matches the server's current date. Domain
	// plugins may label the rail (Photos calls it "On This Day"), but the core
	// evaluator stays shape-based: date field in, Media rows out.
	"on-this-day",
	"entity-items",
	// entity-media yields the Media related to the Entity the rendering
	// view is scoped to (a detail Layout's "episodes of this container"
	// feed). The entity comes from the request context — an entityID on
	// the sourceItems query — so the spec carries no parameters.
	"entity-media",
	// random: a seeded shuffle rail over the visible catalog —
	// the same Source.Seed always yields the same order (a stable permutation
	// keyed on (seed, media_id)), different seeds reorder. Reveal/cert/extras
	// filtered like every catalog feed; generic by construction (no domain
	// vocabulary), so it serves shuffle walls for any archetype.
	"random",
	// resume-progress: per-Profile playback-progress projection
	// — the Media a Profile has "in-progress" (started past the resume floor but
	// not finished). A finer cut over the same engagement data continue-watching
	// reads; the Source.Value selects which band of the unplayed/in-progress/
	// played axis to surface (only "in-progress" is wired here — the band the
	// resume filter needs). Generic by construction (progress is engagement, not a
	// domain concept), reveal-filtered like every feed.
	"resume-progress",
	// availability: a Media-file-provenance projection over the
	// frozen media_file.provisional flag — archived (a kept file),
	// streamed-provisional (a proxy-tee'd reapable file), or not-downloaded (no
	// file). The Source.Value selects the band. A generic file-provenance axis
	// (invariant #1 — no podcast vocabulary), reveal-filtered through the same
	// Library funnel as the catalog feeds.
	"availability",
	// watched-by-any-profile is the household aggregate playback axis:
	// boolean bands over whether any Profile has watched the
	// Media, without exposing which Profile did.
	"watched-by-any-profile",
	// directory is the generic Discover browse seam for external directories
	// (for example algorithmic trending lists). Source.Value selects the
	// directory query shape while provenance/title copy stays plugin-owned.
	"directory",
}

// SourceKinds returns the fixed Source vocabulary: bounded, server-evaluated
// item feeds Sections draw from. Grown rarely; clients ignore unknown kinds.
func SourceKinds() []string { return copySlice(sourceKinds) }

var resumeProgressValues = []string{"in-progress"}

// ResumeProgressValues returns the closed vocabulary for the "resume-progress"
// Source's Value: the band of the playback-progress axis to surface. Only
// "in-progress" is server-evaluated today (the full axis is
// unplayed/in-progress/played; the other bands have dedicated feeds —
// "played" is recently-played/most-played; "unplayed" has no engagement row).
func ResumeProgressValues() []string { return copySlice(resumeProgressValues) }

var availabilityValues = []string{"archived", "streamed-provisional", "not-downloaded"}

// AvailabilityValues returns the closed vocabulary for the "availability"
// Source's Value: the media_file-provenance band to surface.
func AvailabilityValues() []string { return copySlice(availabilityValues) }

var watchedByAnyProfileValues = []string{"any", "none"}

// WatchedByAnyProfileValues returns the closed vocabulary for the
// "watched-by-any-profile" Source's Value.
func WatchedByAnyProfileValues() []string { return copySlice(watchedByAnyProfileValues) }

var directoryValues = []string{"trending"}

// DirectoryValues returns the closed vocabulary for the "directory" Source's
// Value: the external-directory query shape a Discover Section requests.
func DirectoryValues() []string { return copySlice(directoryValues) }

var valuedSourceKinds = map[string][]string{
	"resume-progress":        resumeProgressValues,
	"availability":           availabilityValues,
	"watched-by-any-profile": watchedByAnyProfileValues,
	"directory":              directoryValues,
}

// ValuedSourceKinds returns the Source kinds that take a Value parameter mapped
// to their closed value vocabulary. A kind absent from this map takes no Value.
func ValuedSourceKinds() map[string][]string {
	out := make(map[string][]string, len(valuedSourceKinds))
	for kind, values := range valuedSourceKinds {
		out[kind] = copySlice(values)
	}
	return out
}

// SourceSpec is the canonical declarative Source reference.
type SourceSpec struct {
	Kind string
	// EntityKind names the declared Entity Kind an "entity-items" Source
	// lists (the browse-grid case); empty for every other Source kind.
	EntityKind string
	// FieldFilters narrows a catalog feed by declared Bundle.MediaFields.
	// Equality values use the field's native shape: string for text/url,
	// JSON number for number, RFC3339/date string or time.Time for date,
	// and bool for flag. Range values are map[string]any objects with
	// "gte" and/or "lte" bounds, valid only for number and date fields.
	// Only the catalog feeds in FilterableSourceKinds accept filters; the
	// engagement feeds are per-Profile event projections and the entity
	// feeds are scoped by their entity, not by Media fields.
	FieldFilters map[string]any
	// EntityFilter narrows a catalog feed to the Media related to one named
	// entity of a declared Entity Kind ("the items linked to the kind-K
	// entity named N in this Library") — the declarative per-category shelf.
	// Like FieldFilters it rides only the FilterableSourceKinds and travels
	// the manifest wire for clients to echo back on sourceItems. Nil means
	// unfiltered.
	EntityFilter *EntityFilter
	// Seed parameterizes the "random" Source kind: the same seed yields the
	// same shuffle order, so a declared seed pins a stable rail and a
	// client-rotated seed (echoed back on sourceItems) reshuffles on demand.
	// Empty is a valid seed (the empty-string permutation). A non-empty Seed
	// is REJECTED by bundle validation on every other kind — like
	// FieldFilters and EntityFilter, it is an error there, not ignored.
	Seed string
	// Value selects the closed vocabulary entry for a valued Source
	// (ValuedSourceKinds): a predicate band or directory query shape. Required
	// and validated on those kinds; rejected on every other kind, like Seed.
	Value string
}

// EntityFilter names one entity by declared Kind and Name. The Kind must be
// a declared Entity Kind of the bundle; the Name is plugin data matched
// against the entity's stored title (hydration keys entities by Library +
// kind + name, so the pair is a stable handle). An entity that does not
// exist — or is hidden behind the Reveal contract — yields an empty feed,
// never an error.
type EntityFilter struct {
	Kind string
	Name string
}

var filterableSourceKinds = []string{"all-items", "recently-added"}

// FilterableSourceKinds returns the Source kinds that accept FieldFilters: the
// whole-catalog feeds, where "the items whose field equals this value" is a
// meaningful narrowing of an otherwise unscoped list.
func FilterableSourceKinds() []string { return copySlice(filterableSourceKinds) }
