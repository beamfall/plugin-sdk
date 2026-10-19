package sdk

var sectionKinds = []string{"hero", "shelf", "grid", "list", "info", "filter-bar", "playback-affordances"}

// SectionKinds returns the fixed Section vocabulary: compiled
// client primitives, parameterized by card style, artwork slot, label, source.
func SectionKinds() []string { return copySlice(sectionKinds) }

var playbackAffordances = []string{"resume", "direct-play", "airplay"}

// PlaybackAffordances returns the fixed playback-affordances Section
// vocabulary. Clients derive the actual enabled/disabled state from
// playbackInfo.
func PlaybackAffordances() []string { return copySlice(playbackAffordances) }

var layoutRoles = []string{"browse", "detail"}

// LayoutRoles returns the fixed Layout role vocabulary, named by shape, never
// domain: "browse" is the home Layout plugin-bound Libraries open on (exactly
// one per bundle); "detail" Layouts render one entity's page (zero or more,
// each optionally bound to a declared Entity Kind so clients can pick the right
// detail Layout for an entity).
func LayoutRoles() []string { return copySlice(layoutRoles) }

var layoutArchetypes = []string{"item", "container", "ordered-children", "person", "collection"}

// LayoutArchetypes returns the fixed vocabulary of detail-Layout shapes
// (generalized off Film & TV's five detail Layouts, named by
// shape rather than domain): "item" (a single entity's own detail — info,
// extras, related siblings), "container" (an entity holding child entities —
// info plus the child list), "ordered-children" (a container's ordered child
// list itself, e.g. one container-level's episode/track list),
// "person" (a person's detail, e.g. filmography), and "collection" (an
// ordered grid of sibling entities). Clients ignore unknown entries
// (invariant 3); added rarely, in client releases.
func LayoutArchetypes() []string { return copySlice(layoutArchetypes) }

// LayoutSpec is one declarative Layout: an ordered set of Sections plus the
// role saying where clients render it (one of the fixed LayoutRoles).
type LayoutSpec struct {
	// ID is a stable lowercase slug, unique within the bundle — the handle
	// additional Layouts stay addressable by on the wire.
	ID string
	// Role is one of the fixed LayoutRoles ("browse", "detail").
	Role string
	// Archetype optionally names this "detail" Layout's fixed shape from
	// LayoutArchetypes, letting clients pick a compiled-in rendering for it.
	// Invalid on any other role. At most one detail Layout per bundle may
	// declare a given Archetype.
	Archetype string
	// EntityKind optionally binds a "detail" Layout to one declared Entity
	// Kind so clients can pick the right detail Layout for an entity.
	// Invalid on any other role.
	EntityKind string
	Sections   []SectionSpec
}

// SectionSpec is one Section: a fixed-kind client primitive parameterized by
// title, artwork slot, and Source.
type SectionSpec struct {
	ID   string
	Kind string
	// Title is a display string and therefore an optional locale map:
	// declare L("…") for English-only, a full map to ship
	// translations.
	Title       LocaleString
	ArtworkSlot string
	Source      SourceSpec
	// FilterOptions declares a "filter-bar" Section's selectable filter
	// affordances: each option, when selected, narrows the adjacent
	// Section's Source by equality on one declared text Media field —
	// the same FieldFilters mechanism, chosen at view time instead of
	// declaration time. Valid only on filter-bar Sections; the values are
	// plugin data, never core vocabulary.
	FilterOptions []FilterOption
	// PlaybackAffordances declares which playbackInfo-derived controls a
	// playback-affordances Section renders.
	PlaybackAffordances []string
}

// FilterOption is one filter-bar affordance: a declared text Media field,
// the required value, and the display label clients render for the chip
// (a locale map; an absent label falls back to the value).
type FilterOption struct {
	Field string
	Value string
	Label LocaleString
}
