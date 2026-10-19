package sdk

import "github.com/beamfall/plugin-sdk/scrobble"

// ID is a stable plugin identifier (lowercase slug).
type ID string

// Version is the exact SDK semver identity embedded by a Core build.
const Version = "0.3.2"

var storeSensitivities = []string{"adult", "acquisition-adjacent"}

// StoreSensitivities returns the closed vocabulary for Bundle.StoreSensitivity
// — the plugin categories consumer app stores prohibit.
// Empty StoreSensitivity means no restriction.
func StoreSensitivities() []string { return copySlice(storeSensitivities) }

// FeatureAudioStation is the Bundle.FeatureEligibility capability that lets
// the first-party audio-station feature operate on Libraries bound to the
// declaring plugin.
const FeatureAudioStation = "audio-station"

var featureEligibilityCapabilities = []string{FeatureAudioStation}

// FeatureEligibilityCapabilities returns the closed vocabulary for
// Bundle.FeatureEligibility.
func FeatureEligibilityCapabilities() []string { return copySlice(featureEligibilityCapabilities) }

// IdentificationPolicy is the closed vocabulary for Bundle.IdentificationPolicy.
// Empty means auto.
type IdentificationPolicy string

const (
	IdentificationPolicyAuto  IdentificationPolicy = "auto"
	IdentificationPolicyQueue IdentificationPolicy = "queue"
)

var identificationPolicies = []IdentificationPolicy{IdentificationPolicyAuto, IdentificationPolicyQueue}

// IdentificationPolicies returns the closed vocabulary for
// Bundle.IdentificationPolicy.
func IdentificationPolicies() []IdentificationPolicy { return copySlice(identificationPolicies) }

// Bundle is one plugin's complete declarative contribution. Pure data.
type Bundle struct {
	ID      ID
	Version string
	// SDKFloor is the required minimum plugin-sdk semver this bundle supports.
	// Its major version declares the supported SDK major and must match
	// SDKCeiling. Core rejects bundles whose floor is newer than the SDK it
	// embeds.
	SDKFloor string
	// SDKCeiling is the required maximum plugin-sdk semver this bundle declares
	// compatibility with (the upper end of the listing-manifest compat
	// range). It must be >= SDKFloor and
	// share its major version; a registry rejects a listing whose range
	// excludes the SDK version it targets or lacks producer-owned conformance
	// evidence for the exact plugin and SDK versions.
	SDKCeiling string
	// DisplayName is the plugin's human name, as a locale map.
	DisplayName LocaleString
	// MediaLabel is the display label clients render for the wire entity
	// `Media` inside this plugin's Libraries ("Media" stays the schema
	// name). A locale map.
	MediaLabel LocaleString
	// MediaClasses this plugin's Libraries hold ("video", "audio", "image").
	MediaClasses []string
	// Accent is the optional Library accent color ("#RRGGBB") for Libraries
	// bound to this plugin (a theme token; see the design system's "Library
	// accents"). Clients normalize it into tint/focus variants — it never
	// changes type, spacing, focus behavior, or motion. Empty means the
	// client default.
	Accent string
	// StoreSensitivity declares whether this plugin's surfaces would violate
	// consumer app-store content policy: "adult" for adult-content plugins, "acquisition-adjacent"
	// for acquisition tooling. Store-distributed clients (App Store / Play)
	// suppress Libraries, navigation, and Layouts declared by a sensitive
	// plugin; empty means no store restriction. Shape-named core vocabulary
	// (StoreSensitivities); declaring it is a *floor*, not a substitute for
	// the structural client-side denylist of known first-party sensitive
	// plugin IDs.
	StoreSensitivity string
	// IdentificationPolicy declares whether Core may automatically identify
	// scanned media for this plugin. Empty means "auto"; "queue" routes scans
	// into the manual identification queue.
	IdentificationPolicy IdentificationPolicy
	// MediaFields declares typed fields on the wire entity Media itself,
	// against the same FieldTypes vocabulary as Entity Kind fields. Field
	// values are plugin data, assigned at identification/parse time — core
	// stores and serves them without interpreting any value. By convention
	// a plugin that subdivides its Media declares a text field named
	// "class" whose values key MediaClassLabels.
	MediaFields []FieldSpec
	// LockableMediaFields declares which Media fields accept a per-field user
	// lock ("user edits outrank all sources"). Each entry is a
	// FieldMap-shaped key — a BuiltinMediaFields name ("summary", "year") or a
	// declared plugin field under the MediaFieldKeyPrefix namespace
	// ("media.<name>") — naming a field identification may write but a locked
	// value freezes against every future scraper/importer write. Declarative
	// only: the SDK records which fields are lockable, the core store owns the
	// lock state and write-time enforcement. A field absent from this list
	// carries no lock seam.
	LockableMediaFields []string
	// MediaClassLabels overrides MediaLabel per value of the conventional
	// "class" Media field: declared class value → display label. Values a
	// plugin leaves unmapped fall back to MediaLabel.
	MediaClassLabels map[string]LocaleString
	// CertificationScale is the plugin's ordered certification scale:
	// the values of the conventional text Media field
	// "certification", ordered low→high. A Profile policy ceiling against
	// this scale filters Media ranked above it from that Profile's reads,
	// server-side, composed with reveal filtering. The scale is shape-named
	// core vocabulary; the values are plugin data. Declaring a scale
	// requires declaring the "certification" text Media field.
	CertificationScale []string
	// LibraryDefaults seeds new Libraries bound to this plugin (a Manager
	// can still override per Library).
	LibraryDefaults LibraryDefaults
	// InteractiveSidecars are the lowercase file extensions (no dot) this
	// plugin recognises as INTERACTIVE-control sidecar tracks: an
	// external sibling file discovered at scan and served behind a Stream Token
	// as raw bytes, distinct from the Core caption sidecar (.srt/.vtt → WebVTT).
	// "Interactive sidecar" is shape-named Core vocabulary; the extension VALUES
	// are plugin data (Core never names them). Scan records a matching sibling
	// as a store sidecar of the interactive kind; a plugin that declares none
	// simply contributes no interactive sidecars.
	InteractiveSidecars []string
	// Engagement declares the plugin's engagement posture.
	Engagement  Engagement
	EntityKinds []EntityKind
	// Layouts are the bundle's declarative Layouts: exactly one "browse"
	// (the home Layout for plugin-bound Libraries) plus any number of
	// "detail" Layouts, addressable by stable ID for entity detail views.
	Layouts  []LayoutSpec
	Scrapers []ScraperSpec
	// Importers are this plugin's Migration Wizard sources: the
	// declarative Importer Extension Point, parallel to Scrapers. Each entry
	// declares one source system the plugin can import behavioral state from.
	// Core never hardcodes a source-system name — it reads this list from the
	// activated bundles to build the wizard's source menu. A plugin that
	// declares none is simply absent from the wizard.
	Importers []ImporterSpec
	// ConfigFields declares non-secret, typed plugin configuration the Console
	// may edit. Secrets stay in secure stores via AuthRef or plugin-scoped
	// secret seams; these values persist in Core operational config.
	ConfigFields []ConfigFieldSpec
	// Permissions declares the Core-owned Integration Host/API permissions this
	// plugin may request. Plugins choose from the closed vocabulary below; they
	// never define auth scopes.
	Permissions []PermissionSpec
	// Actions are server-performed Plugin Action Descriptors. Clients render
	// trusted controls from this data and call Core back; they never execute
	// downloaded plugin UI code.
	Actions []ActionSpec
	// ControlPolicy is the Manager access posture for this plugin's action
	// surface. Empty means "manager-enabled".
	ControlPolicy string
	// FamilyScopes are the optional Library family scopes this plugin offers:
	// the set of Library "types" a manager picks at creation
	// time for a plugin-bound Library. "Family scope" is shape-named core
	// vocabulary; the scope *values* are plugin data (the Film & TV plugin
	// offers "films", "tvshows", "mixed"). Core stores the chosen value
	// opaquely on the Library row and never interprets it — identification
	// reads it to fix classification deterministically rather than guessing.
	// A plugin that declares none simply offers no creation-time scope choice
	// (an unset scope preserves today's per-file behavior).
	FamilyScopes []FamilyScopeSpec
	// ScrobbleMatch declares the plugin-owned relation slugs the generic
	// scrobble-import match chain reads, so core/migrate confirms an imported
	// play's artist/album without hardcoding a music relation name (the
	// matcher receives plugin-owned relation selectors). Nil for
	// plugins that import no scrobble history. Core reads the names off the
	// activated bundle and passes them to the matcher as data — shape-named
	// core vocabulary, plugin-data values (invariant #1).
	ScrobbleMatch *ScrobbleMatchSpec
	// ScrobbleTarget declares how Media in this plugin map to the bounded,
	// shape-named target vocabulary consumed by scrobble adapters. A plugin
	// either declares one Default hint for every Media item, or names one of
	// its text MediaFields and maps exact plugin-owned values to hints. Missing
	// and unmapped values have no implicit fallback and therefore fail closed.
	ScrobbleTarget *ScrobbleTargetSpec
	// LabelMaps map literal AI labels to this plugin's vocabulary. A plugin
	// that declares none contributes no AI label mappings.
	LabelMaps []LabelMapSpec
	// FeatureEligibility declares which optional first-party product features may
	// operate on Libraries bound to this plugin. Entries are closed SDK
	// vocabulary; plugin data remains in the bundle.
	FeatureEligibility []string
}

// LabelMapSpec maps one literal AI label to one target in this plugin's
// vocabulary. Labels are matched by equality only after fixed normalization;
// matching beyond literal equality belongs in compiled first-party hooks, not
// this data.
type LabelMapSpec struct {
	// Label is the literal AI label key. Required, non-empty after
	// normalization, and never a pattern.
	Label string
	// MinConfidence drops a matched label whose model confidence is below this
	// floor. Zero keeps the capability-level fallback floor.
	MinConfidence float64

	// Exactly one target group: Tag, or Kind+Relation(+EntityName).
	Tag        string
	Kind       string
	Relation   string
	EntityName string
}

// ScrobbleMatchSpec carries the bare, unqualified relation slugs the scrobble
// matcher uses. ArtistConfirm names the relation(s) that
// connect a credited Entity to a Track Media for artist confirmation;
// AlbumTieBreak names the Media-targeted relation(s) whose related Entity title
// disambiguates several artist-confirmed candidates by album. Only relations
// that target Media (queryable from a Media) belong here; the names must match
// the bare relation slugs the store persists.
type ScrobbleMatchSpec struct {
	ArtistConfirm []string
	AlbumTieBreak []string
}

// ScrobbleTargetSpec selects a bounded scrobble target shape without exposing
// plugin-owned domain vocabulary to Core. Exactly one mode must be declared:
// Default alone, or MediaField plus one or more Mappings.
type ScrobbleTargetSpec struct {
	Default    scrobble.TargetHint
	MediaField string
	Mappings   []ScrobbleTargetMapping
}

// ScrobbleTargetMapping maps one exact, plugin-owned Media field value to a
// bounded target hint. Values are compared as opaque data; no normalization or
// fallback is implied by this declaration.
type ScrobbleTargetMapping struct {
	Value string
	Hint  scrobble.TargetHint
}

// BrowseLayout returns the bundle's browse Layout — the home Layout
// plugin-bound Libraries open on — and whether one was found. Validation
// guarantees exactly one exists; an unvalidated bundle without one yields
// the zero LayoutSpec and ok=false.
func (b Bundle) BrowseLayout() (LayoutSpec, bool) {
	for _, layout := range b.Layouts {
		if layout.Role == "browse" {
			return layout, true
		}
	}
	return LayoutSpec{}, false
}

// LibraryDefaults are the creation-time defaults for plugin-bound Libraries.
type LibraryDefaults struct {
	// Restricted Libraries live behind the Reveal contract by default
	// (the adult plugin's normal posture).
	Restricted bool
}

// Engagement is the bundle's declarative engagement posture (resume
// eligibility is a property, not a media-class convention — music
// audio never resumes, podcast audio always does).
type Engagement struct {
	// ResumeMedia overrides the class default (video resumes, audio does
	// not) for Media in this plugin's Libraries. Nil keeps the default.
	ResumeMedia *bool
	// EventKinds are the custom engagement-event kinds this plugin declares on
	// the append-only engagement event log (the engagement_event table). Each
	// is an opaque, shape-agnostic string Core only ever stores, groups, and
	// compares — never parses or names (invariant #1). A client may record an
	// event of a declared kind through the generic engagement-event graph
	// surface; Core validates the requested kind against this declared set so a
	// plugin's kind string lives in the plugin, never hardcoded in Core. A plugin that
	// declares none contributes no recordable engagement-event kinds.
	EventKinds []string
}

// FamilyScopeSpec declares one Library family scope a plugin offers at
// creation time. The scope tells identification the media
// family deterministically for a Library instead of guessing per file. The
// Value is plugin-owned data (a lowercase slug — e.g. the Film & TV plugin's
// "films" / "tvshows" / "mixed"); core stores it verbatim on the Library row
// and never interprets it. Exactly one scope per plugin should be marked
// Default — the value a Library bound to the plugin takes when the manager
// makes no explicit choice (Film & TV's "mixed", preserving today's per-file
// classification).
type FamilyScopeSpec struct {
	// Value is the lowercase-slug scope identifier stored on the Library row
	// and read by identification. Plugin data, never core vocabulary.
	Value string
	// DisplayName is the human label clients render for the scope in the
	// creation picker, as a locale map. Required.
	DisplayName LocaleString
	// Default marks the scope a plugin-bound Library takes when no scope is
	// chosen at creation. Exactly one declared scope must be the default.
	Default bool
}

// FamilyScopeValid reports whether value is a family scope this bundle offers.
// The empty string is always valid — an unset scope means "no creation-time
// choice", which preserves the plugin's default per-file behavior.
func (b Bundle) FamilyScopeValid(value string) bool {
	if value == "" {
		return true
	}
	for _, scope := range b.FamilyScopes {
		if scope.Value == value {
			return true
		}
	}
	return false
}
