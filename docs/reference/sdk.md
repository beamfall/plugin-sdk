# package sdk

`import "github.com/beamfall/plugin-sdk"`

## Constants

```go
const (
	CategoryWatchState    = "watch-state"
	CategoryRatings       = "ratings"
	CategoryCollections   = "collections"
	CategorySubscriptions = "subscriptions"
	CategoryItemState     = "item-state"
)
```

Import-category vocabulary: the fixed set of data categories a foreign item can carry. A Source reports the subset it carries for a run via Categories(); the handler applies a category's state only when the run admits it.

```go
const FeatureAudioStation = "audio-station"
```

FeatureAudioStation is the Bundle.FeatureEligibility capability that lets the first-party audio-station feature operate on Libraries bound to the declaring plugin.

```go
const MaxImportPageItems = 256
```

MaxImportPageItems is the largest page a StreamingSource may hand to its callback. Keeping the bound in the contract lets consumers apply arbitrarily large imports without retaining the complete source in memory.

```go
const MediaFieldKeyPrefix = "media."
```

MediaFieldKeyPrefix namespaces a FieldMap/ImpliedMediaFields key onto a declared Bundle.MediaFields FieldSpec: "media.\<name>" routes the resolved value through the Media typed-field side tables at identification time. The "entity." namespace is deliberately rejected — entity routing is EntityMap's job, not FieldMap's.

```go
const MediaKindName = "media"
```

MediaKindName is the reserved relation target naming the wire entity itself: relations run between Entities and Media.

```go
const Version = "0.3.2"
```

Version is the exact SDK semver identity embedded by a Core build.

## Variables

```go
var (
	// ErrInvalidImportCheckpoint reports a malformed or out-of-range legacy
	// continuation token. StreamingSource implementations define their own
	// opaque token syntax and should wrap this error for invalid tokens.
	ErrInvalidImportCheckpoint = errors.New("sdk: invalid import checkpoint")
	// ErrInvalidImportPage reports a StreamingSource contract violation.
	ErrInvalidImportPage = errors.New("sdk: invalid import page")
)
```

## func ActionRiskLevels

```go
func ActionRiskLevels() []string
```

ActionRiskLevels returns the fixed risk vocabulary for Plugin Action Descriptors.

## func Archetypes

```go
func Archetypes() []string
```

Archetypes returns the fixed vocabulary bridging declared Entity Kinds to client rendering. Added rarely, in client releases.

## func ArtworkSlots

```go
func ArtworkSlots() []string
```

ArtworkSlots returns the fixed artwork slot vocabulary: poster, backdrop, thumb, logo, portrait, banner, clearart, disc, and season-poster.

## func AuthStyleForms

```go
func AuthStyleForms() []string
```

AuthStyleForms returns the fixed scraper auth-style vocabulary: the shapes a spec's AuthStyle may take, i.e. how the AuthRef secret is presented on requests. Three forms are parameterized by a name after the colon; the secret itself always comes from the CredentialSource, never the spec.

	"bearer"          Authorization: Bearer <secret> (the default)
	"header:<name>"   request header <name>: <secret> (X-Api-Key services)
	"query:<param>"   query parameter ?<param>=<secret> (TMDB v3 api_key)
	"login-bearer"    declarative token exchange (LoginSpec), then Bearer
	"signature:<name>" declarative per-request-signature archetype named
	                  <name> from SignatureArchetypes (SignatureSpec supplies
	                  the archetype's declared signing inputs; Podcast Index's
	                  X-Auth-Key/X-Auth-Date/Authorization SHA-1 triple)

## func AvailabilityValues

```go
func AvailabilityValues() []string
```

AvailabilityValues returns the closed vocabulary for the "availability" Source's Value: the media\_file-provenance band to surface.

## func BuiltinMediaFields

```go
func BuiltinMediaFields() []string
```

BuiltinMediaFields returns the closed set of bare FieldMap keys: the core Media descriptive metadata identification writes directly (UpdateMediaMetadata). A bare FieldMap/SearchFieldMap key outside this set is a load-time error — it would silently no-op at identification today (the typo trap). Plugin Media fields route through the "media." namespace (MediaFieldKeyPrefix), not as bare keys.

## func CompatibleWithSDK

```go
func CompatibleWithSDK(bundle Bundle, sdkVersion string) bool
```

CompatibleWithSDK reports whether sdkVersion (a released plugin-sdk semver, e.g. sdk.Version) falls within bundle's required \[SDKFloor, SDKCeiling] compat range and supported major. sdkVersion must be semver; malformed metadata is treated as incompatible rather than panicking.

## func ConflictStrategies

```go
func ConflictStrategies() []string
```

ConflictStrategies returns the fixed Importer conflict-resolution vocabulary. Only the additive strategy ships: source data fills gaps or advances strictly-ahead positions, never overwrites or downgrades Beamfall's own engagement, ratings, or collection membership.

## func ControlPolicies

```go
func ControlPolicies() []string
```

ControlPolicies returns the fixed Manager access posture vocabulary.

## func DirectoryValues

```go
func DirectoryValues() []string
```

DirectoryValues returns the closed vocabulary for the "directory" Source's Value: the external-directory query shape a Discover Section requests.

## func FeatureEligibilityCapabilities

```go
func FeatureEligibilityCapabilities() []string
```

FeatureEligibilityCapabilities returns the closed vocabulary for Bundle.FeatureEligibility.

## func FieldTypes

```go
func FieldTypes() []string
```

FieldTypes returns the fixed Entity Kind field-schema vocabulary.

## func FilterableSourceKinds

```go
func FilterableSourceKinds() []string
```

FilterableSourceKinds returns the Source kinds that accept FieldFilters: the whole-catalog feeds, where "the items whose field equals this value" is a meaningful narrowing of an otherwise unscoped list.

## func ImportCategories

```go
func ImportCategories() []string
```

ImportCategories returns the fixed Migration Wizard data-category vocabulary: the behavioral-state categories an Importer may declare it can carry. Importers never touch metadata, scraper output, file organization, or server settings — Beamfall re-scrapes content from its own scrapers. Categories are named by shape; the source system a plugin imports from is plugin data, never core vocabulary.

Note: the per-Media played-state category is shape-named "item-state" is used here instead — the per-Media played/new flag, domain-agnostic, because the core vocabulary firewall (TestVocab) forbids serial-content domain words.

	"watch-state"    resume positions + completion → engagement events
	"ratings"        numeric/star ratings → per-Profile ratings
	"collections"    playlists/collections → ordered Collections
	"subscriptions"  feed subscriptions → Subscriptions
	"item-state"     per-Media new/played flag (serial-content importers)

## func InputForms

```go
func InputForms() []string
```

InputForms returns the fixed Importer input vocabulary: how a user supplies the source the importer reads from.

## func LayoutArchetypes

```go
func LayoutArchetypes() []string
```

LayoutArchetypes returns the fixed vocabulary of detail-Layout shapes (generalized off Film & TV's five detail Layouts, named by shape rather than domain): "item" (a single entity's own detail — info, extras, related siblings), "container" (an entity holding child entities — info plus the child list), "ordered-children" (a container's ordered child list itself, e.g. one container-level's episode/track list), "person" (a person's detail, e.g. filmography), and "collection" (an ordered grid of sibling entities). Clients ignore unknown entries (invariant 3); added rarely, in client releases.

## func LayoutRoles

```go
func LayoutRoles() []string
```

LayoutRoles returns the fixed Layout role vocabulary, named by shape, never domain: "browse" is the home Layout plugin-bound Libraries open on (exactly one per bundle); "detail" Layouts render one entity's page (zero or more, each optionally bound to a declared Entity Kind so clients can pick the right detail Layout for an entity).

## func LocalFileScraperKind

```go
func LocalFileScraperKind(kind string) bool
```

LocalFileScraperKind reports whether a scraper kind reads from the Media's local file rather than a remote endpoint. Local-file kinds skip the https-endpoint and URL-template requirements (they have neither) and run as part of the scan pass, not the network identification pipeline.

## func OrphanPolicies

```go
func OrphanPolicies() []string
```

OrphanPolicies returns the fixed vocabulary for EntityKind.OrphanPolicy: "retain" (the default) keeps an instance that has lost every Media relation, "gc" opts the kind into store-side orphan garbage collection.

## func ParseHintSources

```go
func ParseHintSources() []string
```

ParseHintSources returns the fixed ParsedMedia.HintSource vocabulary.

## func PermissionKinds

```go
func PermissionKinds() []string
```

PermissionKinds returns the Core-owned Integration Plugin permission vocabulary. These are Host/API grants, not public GraphQL auth scopes.

## func PlaybackAffordances

```go
func PlaybackAffordances() []string
```

PlaybackAffordances returns the fixed playback-affordances Section vocabulary. Clients derive the actual enabled/disabled state from playbackInfo.

## func ProviderIDVocabulary

```go
func ProviderIDVocabulary() []string
```

ProviderIDVocabulary returns the closed ProviderIDs key vocabulary. The "musicbrainz-\<entity>" entry is a template: any key of the form "musicbrainz-" plus a non-empty lowercase entity name is legal.

## func RatingScaleArchetypes

```go
func RatingScaleArchetypes() []string
```

RatingScaleArchetypes returns the fixed vocabulary for EntityKind.RatingScaleArchetype: the compiled-in shapes a container kind's rating scale takes. "ordered-certification" is an ordered scale plus ceiling (country × media-family × ordered certification values) backing per-Profile enforced filter ceilings and fail-closed unrated handling — the sole archetype needed today. Generic by shape: core reads only the declared archetype name; the scale's actual values (the certification levels, their order, and each instance's certification) are plugin data.

## func RelationCardinalities

```go
func RelationCardinalities() []string
```

RelationCardinalities returns the fixed relation-cardinality vocabulary: "one" means each target is claimed by at most one source entity for the relation (an item belongs to exactly one container), "many" (the default) places no constraint.

## func ResumeProgressValues

```go
func ResumeProgressValues() []string
```

ResumeProgressValues returns the closed vocabulary for the "resume-progress" Source's Value: the band of the playback-progress axis to surface. Only "in-progress" is server-evaluated today (the full axis is unplayed/in-progress/played; the other bands have dedicated feeds — "played" is recently-played/most-played; "unplayed" has no engagement row).

## func ScraperKinds

```go
func ScraperKinds() []string
```

ScraperKinds returns the fixed scraper transport vocabulary (declarative fetch-and-map definitions). "http-json", "http-graphql", and "rss" fetch a remote document over the network. "exif" is the lone local-file kind: it reads embedded EXIF metadata from the image Media's own file — no endpoint, no network, no templates — so the scraper validation below exempts it from the https-endpoint and template requirements the network kinds carry.

## func SectionKinds

```go
func SectionKinds() []string
```

SectionKinds returns the fixed Section vocabulary: compiled client primitives, parameterized by card style, artwork slot, label, source.

## func SignatureArchetypes

```go
func SignatureArchetypes() []string
```

SignatureArchetypes returns the fixed vocabulary for the "signature:\<name>" AuthStyle's archetype name: the compiled-in per-request-signature shapes core knows how to compute. "podcast-index-sha1" is the Podcast Index v2 triple: headers \`X-Auth-Key: \<AuthRef api key>\` and \`X-Auth-Date: \<unix seconds>\`, plus \`Authorization: sha1(api-key + api-secret + X-Auth-Date)\` hex-encoded — computed fresh per request, never a static secret. Generic by shape: core reads only the declared archetype name and computes its fixed signing recipe; the actual key/secret come from the AuthRef CredentialSource, never the spec.

## func SourceKinds

```go
func SourceKinds() []string
```

SourceKinds returns the fixed Source vocabulary: bounded, server-evaluated item feeds Sections draw from. Grown rarely; clients ignore unknown kinds.

## func StoreSensitivities

```go
func StoreSensitivities() []string
```

StoreSensitivities returns the closed vocabulary for Bundle.StoreSensitivity — the plugin categories consumer app stores prohibit. Empty StoreSensitivity means no restriction.

## func StreamSource

```go
func StreamSource(
	ctx context.Context,
	source Source,
	checkpoint string,
	emit func(context.Context, []ForeignItem, string) error,
) error
```

StreamSource consumes source through StreamingSource when available. It validates page bounds and checkpoint progress and preserves an emit callback error exactly. A legacy Source is adapted into bounded callback pages, but its deprecated Items call still materializes the complete import.

## func TitleJoinRules

```go
func TitleJoinRules() []string
```

TitleJoinRules returns the fixed vocabulary for EntityKind.TitleJoinRule: the compiled-in archetypes for composing a display title when core spans multiple of a container kind's children into one Media (Film & TV's multi-episode file). "slash" joins each spanned child's title with " / " ("{E01 title} / {E02 title}"), the sole archetype needed today. Generic by shape: core reads only the declared archetype name and applies its compiled join; the child titles themselves stay plugin data.

## func TransformKinds

```go
func TransformKinds() []string
```

TransformKinds returns the fixed value-transform vocabulary: per-field post-processing of mapped FieldMap values, named by shape, never domain. Deliberately not a DSL — kinds are added rarely, when a real scraper needs one.

	"year-from-date"      YYYY-MM-DD… or RFC-822 date → "YYYY"
	"date-from-rfc822"    RFC-822 date (RSS pubDate) → "YYYY-MM-DD"
	"seconds-from-clock"  "HH:MM:SS"/"MM:SS"/plain seconds → seconds

## func ValidParseHintSource

```go
func ValidParseHintSource(s ParseHintSource) bool
```

ValidParseHintSource reports whether s is a legal non-empty HintSource.

## func ValidProviderIDKey

```go
func ValidProviderIDKey(key string) bool
```

ValidProviderIDKey reports whether key is a legal ProviderIDs provider name.

## func ValidateAuthStyle

```go
func ValidateAuthStyle(s string) bool
```

ValidateAuthStyle reports whether s is a legal ScraperSpec.AuthStyle value: the empty string (the documented "bearer" default — AuthStyleForms lists the template sentinels, not this contract-legal shorthand, so "" is valid even though it is absent from that slice), the bare forms ("bearer", "login-bearer"), or one of the three parameterized forms ("header:\<name>", "query:\<param>", "signature:\<name>") with its colon-suffixed name non-empty and not a literal template sentinel (a name containing "\<" or ">" is the AuthStyleForms placeholder copied verbatim, never a real header, parameter, or archetype name). A "query:\<param>" style is contract-legal but puts the secret in the request URL — logs, proxies, and browser history all see it — so callers choosing between forms should prefer "header:\<name>" or "bearer" wherever the upstream API supports it. A "signature:\<name>" style further requires \<name> to name a declared SignatureArchetypes entry — checked by ValidateBundle, not here, since that vocabulary is core-compiled rather than a shape this function alone can validate.

## func ValidateBundle

```go
func ValidateBundle(bundle Bundle) error
```

ValidateBundle checks a Bundle against the fixed SDK vocabularies and the declarative authoring invariants Core enforces before activating a plugin.

## func ValidateBundles

```go
func ValidateBundles(bundles []Bundle) error
```

ValidateBundles validates each bundle individually and then enforces the cross-plugin invariants a single ValidateBundle cannot see. Entity Kind names are a global namespace (store rows, relations, and Layouts key on the bare kind name), so two active plugins declaring the same kind name would collide on those rows; this rejects that. Bundles must carry distinct plugin IDs — a duplicate ID is itself a collision.

## func ValidateLookupByIDTemplates

```go
func ValidateLookupByIDTemplates(scraper ScraperSpec) error
```

ValidateLookupByIDTemplates checks a scraper's LookupByIDTemplate map: every key is a ProviderIDVocabulary provider, every template is non-blank and contains the {id} placeholder, and a local-file scraper kind declares none (it has no endpoint to resolve an ID against). A nil or empty map is valid — the field is optional and a scraper omitting it is unchanged.

## func ValidateParseHints

```go
func ValidateParseHints(m ParsedMedia) error
```

ValidateParseHints checks the ProviderIDs/HintSource pairing contract on a parse result: nil ProviderIDs requires an empty HintSource; a populated map requires a recognized HintSource, recognized provider keys, and non-empty ID values. A parser that carries no hints passes unchanged.

## func ValuedSourceKinds

```go
func ValuedSourceKinds() map[string][]string
```

ValuedSourceKinds returns the Source kinds that take a Value parameter mapped to their closed value vocabulary. A kind absent from this map takes no Value.

## func WatchedByAnyProfileValues

```go
func WatchedByAnyProfileValues() []string
```

WatchedByAnyProfileValues returns the closed vocabulary for the "watched-by-any-profile" Source's Value.

## type ActionSpec

```go
type ActionSpec struct {
	ID    string
	Label LocaleString
	// Icon is an optional client-known symbol name for the action button.
	// Empty lets clients choose their default action glyph.
	Icon                string
	Risk                string
	RequiredPermissions []string
	InputFields         []ConfigFieldSpec
	Confirmation        LocaleString
}
```

ActionSpec is one server-performed Plugin Action Descriptor.

## type Bundle

```go
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
```

Bundle is one plugin's complete declarative contribution. Pure data.

### func (Bundle) BrowseLayout

```go
func (b Bundle) BrowseLayout() (LayoutSpec, bool)
```

BrowseLayout returns the bundle's browse Layout — the home Layout plugin-bound Libraries open on — and whether one was found. Validation guarantees exactly one exists; an unvalidated bundle without one yields the zero LayoutSpec and ok=false.

### func (Bundle) EntityFieldTypes

```go
func (b Bundle) EntityFieldTypes(kind string) map[string]string
```

EntityFieldTypes indexes the named Entity Kind's declared field names → declared types — the shape entity hydration parses an EntityMap.FieldMap's resolved values against. An undeclared kind yields an empty map.

### func (Bundle) FamilyScopeValid

```go
func (b Bundle) FamilyScopeValid(value string) bool
```

FamilyScopeValid reports whether value is a family scope this bundle offers. The empty string is always valid — an unset scope means "no creation-time choice", which preserves the plugin's default per-file behavior.

### func (Bundle) MediaFieldTypes

```go
func (b Bundle) MediaFieldTypes() map[string]string
```

MediaFieldTypes indexes the bundle's declared Media field names → declared types, the shape ClassifyMediaFieldKey resolves "media.\<name>" keys against.

## type Candidate

```go
type Candidate struct {
	ID      string
	Title   string
	Summary string
	Year    int
	Artwork map[string]string
	// Sequence and Index are the candidate's ordinal position when the source
	// carries item-level identity (0 = not carried, for a source that predates
	// SequencePresent/IndexPresent below), the candidate-side counterparts of
	// ParsedMedia.Sequence/Index so a MatchScorer can agree/disagree with the
	// parse. Note the deliberate asymmetry: the parsed-side Sequence is
	// decimal-compatible TEXT (it can carry "4.5"), while the candidate side is
	// an integer ordinal — a scorer comparing the two must parse the text side
	// and cannot expect ordinal agreement for fractional positions. Named by
	// shape (never by domain), the same firewall the parsed twin keeps.
	Sequence int
	Index    int
	// SequencePresent and IndexPresent are true when the source explicitly
	// carried Sequence/Index for this candidate — the presence counterpart of
	// the "0 = not carried" convention above, so a genuine ordinal 0 (e.g. a
	// zero-indexed opener) stays distinguishable from "the source carries no
	// item-level identity at all". A candidate producer built before these
	// fields existed leaves them at their zero value (false), which preserves
	// its prior 0-means-absent behavior unchanged: it is source-compatible,
	// not a breaking change. A producer that does carry a genuine ordinal 0
	// must set the matching Present field to true for a scorer to tell the two
	// cases apart.
	SequencePresent bool
	IndexPresent    bool
	// Popularity is the source's relative popularity signal (e.g. TMDB's
	// popularity), 0 when the source carries none. It is a tiebreak-only input:
	// a MatchScorer may nudge ranking between otherwise-equal candidates by it,
	// but it must never let a wrong-title or wrong-year candidate outrank a
	// right one.
	Popularity float64
}
```

Candidate is one identification candidate from a scraper search.

## type CollectionMember

```go
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
```

CollectionMember is this foreign item's ordered position in one static (non-rule-driven) collection/playlist the source recorded (playlists and collections import as ordered Collections) — distinct from SmartCollection above, which covers the rule-driven case the seam still cannot import member-by-member. The source yields one CollectionMember per (item, collection) membership; the handler groups memberships that share a Name and orders them by Position to reconstruct the collection, using each member item's own Path/ExternalIDs as its identity — no separate member identity list is needed here.

## type ConfigFieldSpec

```go
type ConfigFieldSpec struct {
	Name     string
	Type     string
	Label    LocaleString
	Required bool
	Default  string
	Options  []ConfigOption
}
```

ConfigFieldSpec declares one operator-editable configuration field of a plugin (or one input field of an ActionSpec). Its Type reuses the small FieldTypes vocabulary so clients already know how to render and validate the value shape.

## type ConfigOption

```go
type ConfigOption struct {
	Value string
	Label LocaleString
}
```

ConfigOption constrains a config field to one selectable value.

## type ContextFilenameParser

```go
type ContextFilenameParser interface {
	FilenameParser
	ParseWithContext(path string, probe ParseProbe, context ParseContext) (ParsedMedia, bool)
}
```

ContextFilenameParser is the optional additive FilenameParser extension for parsers that can use bounded scan-owned context. ParseWithContext MUST return the same result as Parse when context is zero or contains an unrecognized value. Existing parsers and callers continue to use FilenameParser unchanged.

## type DirectoryMediaFileCount

```go
type DirectoryMediaFileCount string
```

DirectoryMediaFileCount is bounded evidence about how many media files share the current file's directory. It deliberately carries only the shape needed by parsers, never an exact unbounded count or sibling names.

```go
const (
	// DirectoryMediaFileCountUnknown means directory evidence was absent,
	// incomplete, or not recognized. It is the zero value for compatibility.
	DirectoryMediaFileCountUnknown DirectoryMediaFileCount = ""
	// DirectoryMediaFileCountSingle means the current file is the directory's
	// only media file.
	DirectoryMediaFileCountSingle DirectoryMediaFileCount = "single"
	// DirectoryMediaFileCountMultiple means the directory contains at least two
	// media files. The exact count is intentionally saturated away.
	DirectoryMediaFileCountMultiple DirectoryMediaFileCount = "multiple"
)
```

## type Engagement

```go
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
```

Engagement is the bundle's declarative engagement posture (resume eligibility is a property, not a media-class convention — music audio never resumes, podcast audio always does).

## type EntityFilter

```go
type EntityFilter struct {
	Kind string
	Name string
}
```

EntityFilter names one entity by declared Kind and Name. The Kind must be a declared Entity Kind of the bundle; the Name is plugin data matched against the entity's stored title (hydration keys entities by Library + kind + name, so the pair is a stable handle). An entity that does not exist — or is hidden behind the Reveal contract — yields an empty feed, never an error.

## type EntityKind

```go
type EntityKind struct {
	// Name is the schema identifier (lowercase slug) Layouts, relations, and
	// store rows key on — never localized.
	Name string
	// DisplayName is the optional human label clients render for the kind
	// (a locale map). Absent means clients fall back to Name.
	DisplayName LocaleString
	Archetype   string
	Fields      []FieldSpec
	Relations   []RelationSpec
	// OrderedChildren marks container kinds whose children carry a defined
	// play order (play-next semantics).
	OrderedChildren bool
	// OrderingSourceField names the declared "number" field on this kind that
	// supplies each child's ordinal within an OrderedChildren container — the
	// source of the ordered sequence (episode/track number). Generic by shape:
	// the ordinal VALUE is plugin data resolved at hydration; core reads only
	// the declared field name to sequence children. Only meaningful on an
	// OrderedChildren kind; empty leaves ordering to core's default (insertion
	// order). A non-empty value must name a declared number field.
	OrderingSourceField string
	// SortLastField names a declared "flag" field on this kind whose set value
	// sorts a child *after* every ordinally-ordered sibling, regardless of its
	// ordinal — the container policy for out-of-sequence extras a plugin keeps
	// at the end of an ordered container (Film & TV "specials"/season-0).
	// Generic by shape: the flag VALUE is plugin data resolved at
	// hydration; core only reads the declared field name to place set children
	// last within OrderedChildren ordering. Only meaningful on an
	// OrderedChildren kind; empty leaves ordinal order untouched.
	SortLastField string
	// ExcludeAutoAdvanceField names a declared "flag" field on this kind whose
	// set value removes a child from auto-advance / play-next candidacy while
	// keeping it in the ordered listing — the container policy for children a
	// viewer may open explicitly but that must never be auto-selected next.
	// Generic by shape: the flag VALUE is plugin data; core reads
	// only the declared field name. Only meaningful on an OrderedChildren kind;
	// empty keeps every child auto-advance-eligible.
	ExcludeAutoAdvanceField string
	// TitleJoinRule names this container kind's fixed title-join archetype
	// from TitleJoinRules, used when core spans multiple children into one
	// Media (Film & TV's multi-episode file).
	// Generic by shape: the archetype is compiled-in; the child titles being
	// joined are plugin data. Only meaningful on a "container" archetype kind;
	// empty means core never spans this kind's children into one Media.
	TitleJoinRule string
	// RatingScaleArchetype names this container kind's fixed rating-scale
	// archetype from RatingScaleArchetypes (an ordered scale plus a
	// ceiling). Generic by shape: the archetype is compiled-in; the scale's
	// actual certification values and each instance's certification are
	// plugin data. Only meaningful on a "container" archetype kind; empty
	// means this kind carries no rating scale.
	RatingScaleArchetype string
	// Hierarchical marks kinds whose instances form a parent/child tree
	// (depth-aware rollups).
	Hierarchical bool
	// FullTextSearchable opts this kind's instances into the store's full-text
	// search index alongside Media, so a browse search surfaces matching
	// entities (a person, a container) and not only Media. Declarative by shape:
	// the SDK records the participation intent; the core store owns the actual
	// index. Default false keeps a kind out of FTS.
	FullTextSearchable bool
	// OrphanPolicy declares what the store does with an instance of this kind
	// that has lost every Media relation (an orphan): one of OrphanPolicies.
	// Empty means "retain" (the default) — orphans linger until explicitly
	// deleted; "gc" opts the kind into store-side orphan garbage collection.
	// Declarative by shape: the SDK records the policy; the core store owns the
	// GC job.
	OrphanPolicy string
	// CursorPaginated declares that list reads over this kind's instances use
	// stable cursor pagination rather than offset paging — the capability a
	// large kind (many thousands of instances) opts into so deep listing stays
	// O(page) under concurrent writes. Declarative by shape: the SDK records the
	// capability; the core store owns the cursor query. Default false keeps
	// offset paging.
	CursorPaginated bool
	// ArtworkSlots lists the slots instances of this kind carry.
	ArtworkSlots []string
}
```

EntityKind declares one plugin-owned entity kind: its schema name, display label, archetype, typed fields, relations to other kinds (or to Media), and artwork slots. Kind names are a global namespace across active plugins (ValidateBundles rejects collisions).

## type EntityMap

```go
type EntityMap struct {
	// Kind names a declared Entity Kind the hydrated Entities instantiate.
	Kind string
	// Relation names a Media-targeted relation (ToKind == MediaKindName)
	// declared on Kind — hydration writes one entity→Media relation row per
	// hydrated Entity under this name (relations run from the declaring
	// kind, matching the store's from-entity/to-media row shape).
	Relation string
	// Path is the lookup-document response path of the value(s): a scalar
	// yields one Entity, an array yields one per element. Unresolved paths
	// are omitted, never errors — same posture as FieldMap.
	Path string
	// NamePath resolves an Entity's name within one element when Path
	// yields a list of objects ("name" for TMDB's genres). Empty means the
	// resolved value (or each scalar element) is the name itself.
	NamePath string
	// ArtworkPath optionally resolves an image URL for the hydrated Entity
	// within one element (element-relative, like NamePath). Site-relative
	// values compose with the scraper's ArtworkBaseURL the same way
	// ArtworkMap values do. Empty means no entity artwork; unresolved paths
	// are omitted, never errors.
	ArtworkPath string
	// FieldMap declares the hydrated Entity's typed field values: declared
	// field name on Kind → element-relative response path (the NamePath/
	// ArtworkPath addressing). Each resolved value parses against the named
	// FieldSpec's declared type; an unparseable or unresolved value is omitted,
	// never an error — the FieldMap posture. Keys must name a field the Kind
	// declares. Empty means name-only hydration.
	FieldMap map[string]string
	// ParentNamePath resolves, within one element, the name of this Entity's
	// parent — a same-Kind Entity the hydrated Entity links to under
	// ParentRelation (kind hierarchy: a sub-kind instance nested
	// under its parent instance of the same Kind). The parent is hydrated as a
	// bare same-Kind Entity. Only declarable on a Hierarchical Kind; empty means
	// no parent. Unresolved or blank parent names are omitted, never errors.
	ParentNamePath string
	// ParentRelation names the same-Kind relation (ToKind == Kind) the hydrated
	// Entity links to its parent under. Required when ParentNamePath is set.
	ParentRelation string
	// OrdinalField names a FieldMap key whose resolved numeric value supplies
	// the relation's sort position (relation.sort_order) — the ordering source
	// that makes OrderedChildren and the next-up Source non-empty for an
	// ordered container. The plugin declares which document value
	// is the ordinal (an installment or sequence number it carries); a missing
	// or non-numeric value falls back to the document array index, the same
	// FieldMap "omit, never error" posture. Generic by shape: the value is an
	// integer position, never a domain label. Empty means order by array index
	// only. Must name a key declared in FieldMap.
	OrdinalField string
	// LookupTemplate optionally fetches a referenced JSON document per EntityMap
	// element before resolving NamePath/ArtworkPath/FieldMap/ParentNamePath,
	// up to MaxLookups. LookupIDPath resolves the per-element ID fed into {id}.
	// Empty keeps the legacy element-local behavior.
	LookupTemplate string
	LookupIDPath   string
	// MaxLookups caps secondary LookupTemplate fetches for this EntityMap. It
	// must be positive when LookupTemplate is set; zero means no secondary
	// lookup is declared.
	MaxLookups int
	// ArtworkURLTemplate renders this EntityMap's ArtworkPath scalar into an
	// absolute HTTPS URL. Empty uses the scraper's ArtworkBaseURL composition.
	ArtworkURLTemplate string
}
```

EntityMap is one declarative lookup-document → Entity mapping. Named by shape, never domain: the Kind and Relation are the declaring plugin's data, the map itself is core vocabulary.

## type ExternalID

```go
type ExternalID struct {
	Scheme string
	Value  string
}
```

ExternalID is a plugin-contributed media identifier (scheme + value) a source recorded when it has no trustworthy file path — the generic external-ID match key. It is the store-free seam shape; core adapts it to its persisted external-ID type at the match boundary.

## type FamilyScopeSpec

```go
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
```

FamilyScopeSpec declares one Library family scope a plugin offers at creation time. The scope tells identification the media family deterministically for a Library instead of guessing per file. The Value is plugin-owned data (a lowercase slug — e.g. the Film & TV plugin's "films" / "tvshows" / "mixed"); core stores it verbatim on the Library row and never interprets it. Exactly one scope per plugin should be marked Default — the value a Library bound to the plugin takes when the manager makes no explicit choice (Film & TV's "mixed", preserving today's per-file classification).

## type FieldSpec

```go
type FieldSpec struct {
	// Name is the schema identifier field values key on — never localized.
	Name string
	Type string
	// Label is the optional display label clients render for the field
	// (a locale map). Absent means clients fall back to Name.
	Label LocaleString
	// Filterable declares the field as an equality-filter target: the store
	// keeps a composite index over its values and list reads may narrow by
	// it. Any typed field may be filterable.
	Filterable bool
	// Sortable declares the field as an ordering key: the store keeps a
	// composite index over its values and list reads may order by it. Any
	// typed field may be sortable.
	Sortable bool
	// Searchable folds the field's values into full-text search alongside
	// the entity name. Text fields only.
	Searchable bool
	// Sensitive marks values that may contain PII or other user-sensitive
	// data. It is declarative metadata only; false is the default and keeps
	// existing field behavior unchanged.
	Sensitive bool
}
```

FieldSpec is one declared field on an Entity Kind (or on the wire entity Media itself, via Bundle.MediaFields).

## type FilenameParser

```go
type FilenameParser interface {
	Parse(path string, probe ParseProbe) (ParsedMedia, bool)
}
```

FilenameParser is a first-party compiled plugin hook.

## type FilterOption

```go
type FilterOption struct {
	Field string
	Value string
	Label LocaleString
}
```

FilterOption is one filter-bar affordance: a declared text Media field, the required value, and the display label clients render for the chip (a locale map; an absent label falls back to the value).

## type ForeignItem

```go
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
```

ForeignItem is one record a source parser yields: a single item from the predecessor server with its mapped behavioral state. It is keyed for matching by Path (the absolute file path the source recorded). The handler runs the match chain on Path to find the core Media, then applies whichever state fields the run's categories admit. Fields outside the run's selected categories are ignored even when the parser populates them.

## type ID

```go
type ID string
```

ID is a stable plugin identifier (lowercase slug).

## type IdentificationPolicy

```go
type IdentificationPolicy string
```

IdentificationPolicy is the closed vocabulary for Bundle.IdentificationPolicy. Empty means auto.

```go
const (
	IdentificationPolicyAuto  IdentificationPolicy = "auto"
	IdentificationPolicyQueue IdentificationPolicy = "queue"
)
```

### func IdentificationPolicies

```go
func IdentificationPolicies() []IdentificationPolicy
```

IdentificationPolicies returns the closed vocabulary for Bundle.IdentificationPolicy.

## type ImporterSpec

```go
type ImporterSpec struct {
	// ID is a lowercase slug, unique within the bundle ("plex", "jellyfin").
	ID string
	// DisplayName is the human source name shown in the wizard menu
	// (a locale map). Required.
	DisplayName LocaleString
	// Categories are the data categories this importer can carry, from the
	// fixed ImportCategories vocabulary. At least one is required — an
	// importer that imports nothing is meaningless.
	Categories []string
	// Input declares how the user supplies the source: "credentials" (a live
	// API: Emby/Jellyfin) or "export-file" (an uploaded export: Plex XML,
	// OPML, AntennaPod db). One of InputForms; required.
	Input string
	// CredentialRef names the config/secret-store entry holding the live
	// source's credentials, parallel to ScraperSpec.AuthRef. The wizard
	// collects the secret and the operator stores it under this ref before the
	// import can run; the run references the ref, never the secret (invariant
	// #5: secrets live in the platform secure store / 0600 files, never in the
	// job payload). Required for an "input: credentials" importer; must be empty
	// for an "export-file" importer (a file path is the source, not a secret).
	CredentialRef string
	// Conflict declares the importer's conflict-resolution strategy against
	// Beamfall's own curated state, from ConflictStrategies. Re-runs are
	// strictly additive; "additive" is the only strategy
	// today, declared explicitly so a future overwrite mode is a vocabulary
	// addition, not a silent default change.
	Conflict string
}
```

ImporterSpec is one declarative Migration Wizard source: the Importer Extension Point, parallel to ScraperSpec. A plugin declares one per source system it can import behavioral state from (the Film & TV plugin declares Plex/Emby/Jellyfin/NFO; the Podcast plugin declares OPML/Overcast/ AntennaPod). The wizard reads these from the activated bundles; Core never hardcodes a source name. The spec is data only — no code, no secrets.

## type ItemState

```go
type ItemState struct {
	Present     bool
	FeedURL     string
	GUID        string
	Played      bool
	HasProgress bool
	Progress    float64
	Starred     bool
}
```

ItemState is one imported serial item-state row keyed by feed URL + item GUID. It is a generic shape used by podcast importers today; the source plugin owns the source-system parsing.

## type LabelMapSpec

```go
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
```

LabelMapSpec maps one literal AI label to one target in this plugin's vocabulary. Labels are matched by equality only after fixed normalization; matching beyond literal equality belongs in compiled first-party hooks, not this data.

## type LayoutSpec

```go
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
```

LayoutSpec is one declarative Layout: an ordered set of Sections plus the role saying where clients render it (one of the fixed LayoutRoles).

## type LibraryDefaults

```go
type LibraryDefaults struct {
	// Restricted Libraries live behind the Reveal contract by default
	// (the adult plugin's normal posture).
	Restricted bool
}
```

LibraryDefaults are the creation-time defaults for plugin-bound Libraries.

## type LocaleString

```go
type LocaleString map[string]string
```

LocaleString is one display string in every language it ships: BCP-47 locale tag (lowercase: "en", "fr", "fr-ca") → translated value. English ("en") is the required fallback anchor on any non-empty map. A nil or empty map means "no string declared" (where an empty string was legal before).

### func L

```go
func L(value string) LocaleString
```

L is the shorthand constructor: L("…") ≡ LocaleString{"en": "…"}, the in-Go equivalent of the contract's plain-string shorthand. L("") is the absent string (nil).

### func (LocaleString) MarshalJSON

```go
func (s LocaleString) MarshalJSON() ([]byte, error)
```

MarshalJSON keeps the wire shorthand: an absent map is "", a map carrying only English is the plain string, anything else is the full locale-map object (keys sorted by encoding/json — deterministic).

### func (LocaleString) Resolve

```go
func (s LocaleString) Resolve(locale string) string
```

Resolve returns the best value for a BCP-47 locale: exact tag, then progressively truncated tags (fr-ca → fr), then the English fallback. The empty locale means English. Resolution is deterministic: a map with no usable match (no "en" entry) yields the value of the lexicographically smallest tag, and an absent map yields "".

### func (*LocaleString) UnmarshalJSON

```go
func (s *LocaleString) UnmarshalJSON(data []byte) error
```

UnmarshalJSON accepts both contract forms: the plain-string shorthand (≡ {"en": …}) and the full locale-map object.

## type LoginSpec

```go
type LoginSpec struct {
	// Path is appended to the scraper Endpoint; must start with "/".
	Path string
	// SecretField is the JSON body key carrying the secret ("apikey").
	SecretField string
	// TokenPath is the JSON path of the issued token ("data.token").
	TokenPath string
}
```

LoginSpec declares the "login-bearer" token exchange as pure data (the TheTVDB v4 POST /login shape): the engine POSTs {SecretField: \<secret>} as JSON to Endpoint+Path and uses the string at TokenPath in the JSON response as the bearer token for subsequent requests.

## type MatchScorer

```go
type MatchScorer interface {
	Score(candidate Candidate, parsed ParsedMedia) float64
}
```

MatchScorer scores how confidently one identification Candidate matches a file's parsed guess. Score MUST return a value in \[0,1] (0 = no confidence, 1 = certain) and must be deterministic for the same inputs; NaN and out-of-range values are contract violations that sdktest.RunScoreConformance and sdktest.RunMatchCorpus fail loudly rather than pick on.

## type MediaFieldParser

```go
type MediaFieldParser interface {
	ParseMediaFields(path string, probe ParseProbe) []ParsedTextField
}
```

MediaFieldParser is an optional extension for parser hooks that can derive plugin-declared text Media fields from the same file-local metadata.

## type MediaFieldRoute

```go
type MediaFieldRoute struct {
	// Builtin is true for the bare BuiltinMediaFields keys.
	Builtin bool
	// CandidateSignal is true for the bare candidateSignalFields keys — a
	// scorer-only ranking input the consumer reads onto the Candidate and never
	// writes as Media metadata.
	CandidateSignal bool
	// MediaField is the declared MediaFields FieldSpec name a "media.<name>"
	// key targets (the prefix stripped). Empty for built-ins.
	MediaField string
	// Type is the declared FieldSpec type of MediaField (one of FieldTypes).
	// Empty for built-ins.
	Type string
}
```

MediaFieldRoute is one resolved FieldMap/ImpliedMediaFields key: whether it targets a core built-in or a plugin Media field, and (for plugin fields) the declared FieldSpec name and type the value parses against.

### func ClassifyMediaFieldKey

```go
func ClassifyMediaFieldKey(key string, mediaFields map[string]string) (MediaFieldRoute, error)
```

ClassifyMediaFieldKey resolves one FieldMap/ImpliedMediaFields key for a validated bundle's declared Media fields.

## type ParseContext

```go
type ParseContext struct {
	DirectoryMediaFileCount DirectoryMediaFileCount
}
```

ParseContext is bounded, scan-owned evidence that is not available from one file's path or probe alone. A zero ParseContext is absent context. Producers set a non-zero field only from complete evidence; parsers MUST treat absent or unrecognized values as unknown and preserve their legacy Parse fallback.

## type ParseHintSource

```go
type ParseHintSource string
```

ParseHintSource is the origin class of a ParsedMedia.ProviderIDs hint — the trust axis core ranks identification hints by (an NFO written by a prior manager outranks a folder tag; a sidecar the operator authored outranks both).

```go
const (
	// ParseHintSourceNone is the zero value: no hint carried.
	ParseHintSourceNone ParseHintSource = ""
	// ParseHintSourceFilename means the ID was parsed from the path itself
	// (folder or file name tags such as `{tmdb-123}` / `[imdbid-tt…]`).
	ParseHintSourceFilename ParseHintSource = "filename"
	// ParseHintSourceNFO means the ID came from an NFO document beside the
	// file (`<uniqueid type="…">`).
	ParseHintSourceNFO ParseHintSource = "nfo"
	// ParseHintSourceSidecar means the ID came from a non-NFO sidecar file the
	// operator or a prior tool authored.
	ParseHintSourceSidecar ParseHintSource = "sidecar"
)
```

## type ParseProbe

```go
type ParseProbe struct {
	HasVideo        bool
	HasAudio        bool
	DurationSeconds int
	Tags            map[string]string
}
```

ParseProbe is scan's view of the FFmpeg probe handed to the parser.

## type ParsedField

```go
type ParsedField struct {
	Name string
	Text string
	Date time.Time
}
```

ParsedField is a parser-derived Media field with the small typed vocabulary scan can safely persist absent-only. Exactly one of Text and Date carries the value: a text field sets Text and leaves Date zero; a date field sets Date and leaves Text empty. TypedMediaFieldParser yields these (its date-capable shape); the older MediaFieldParser yields text-only ParsedTextField values.

## type ParsedMedia

```go
type ParsedMedia struct {
	Title string
	Class string
	Year  int
	// Sequence is decimal-compatible text so plugins can carry fractional order
	// such as audiobook series position "4.5" without a second seam.
	Sequence  string
	Index     int
	IndexEnd  int
	PartGroup string
	// AbsoluteOrdinal is an optional absolute item-ordinal (shape-named, never
	// domain-named) for sources that number items continuously across a series
	// rather than resetting per season/volume (e.g. anime absolute episode
	// numbering). 0 = not carried. Unpopulated until an anime-numbering source
	// lands; present now so that source doesn't need a second seam change.
	AbsoluteOrdinal int
	// ProviderIDs carries external identification IDs the parser found in
	// file-local evidence — a `{tmdb-123}` folder tag, an NFO `<uniqueid>`, a
	// sidecar — keyed by provider name from the closed ProviderIDVocabulary:
	// "tmdb", "imdb", "tvdb", "anidb", and "musicbrainz-<entity>" (any
	// lowercase entity suffix, e.g. "musicbrainz-release"). Values are the
	// provider's own ID text, verbatim. Nil means no hint was carried (the
	// legacy shape). Core treats a carried ID as an identification Parse Hint:
	// a lookup-by-ID short circuit ranked by HintSource trust, never
	// a silent override of the scored match. Unknown provider keys are a
	// contract violation (ValidProviderIDKey) that sdktest surfaces loudly.
	ProviderIDs map[string]string
	// HintSource names where ProviderIDs came from so core can rank trust:
	// one of ParseHintSources ("filename", "nfo", "sidecar"). Empty when
	// ProviderIDs is nil; required non-empty when ProviderIDs is populated.
	HintSource ParseHintSource
}
```

ParsedMedia is a filename parser's best guess for one unidentified file.

## type ParsedTextField

```go
type ParsedTextField struct {
	Name string
	Text string
}
```

ParsedTextField is a plugin parser's best-effort text Media field derived from file-local metadata.

## type PermissionPolicy

```go
type PermissionPolicy struct {
	Label       LocaleString
	Description LocaleString
	HighRisk    bool
}
```

PermissionPolicy is the host-owned minimum presentation and risk policy for one permission kind. Plugins cannot replace its Label or Description, or lower its HighRisk classification.

### func PermissionPolicyFor

```go
func PermissionPolicyFor(kind string) (PermissionPolicy, bool)
```

PermissionPolicyFor returns the host-owned minimum policy for kind. Unknown kinds fail closed. Returned locale maps are copies.

## type PermissionSpec

```go
type PermissionSpec struct {
	Kind        string
	Label       LocaleString
	Description LocaleString
	Context     LocaleString
	HighRisk    bool
}
```

PermissionSpec declares one permission a plugin requests or needs for an action. Label and Description must equal PermissionPolicyFor(Kind); Context is the plugin's optional additive explanation. HighRisk cannot be lower than the host-owned classification.

## type Rating

```go
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
```

Rating is one imported per-Profile numeric rating ("Numeric or star ratings → per-Profile ratings"). It carries the value on the source's own scale rather than a pre-normalized number, so the handler rescales generically instead of the seam assuming any one source's 5-star/10-point/100-point convention (invariant #1 — generic by shape).

## type RelationAttributeSpec

```go
type RelationAttributeSpec struct {
	// Name is the schema identifier the attribute value keys on — never
	// localized.
	Name string
	// Type is one of FieldTypes; the type the resolved attribute value parses
	// against at hydration.
	Type string
}
```

RelationAttributeSpec is one typed attribute a relation's rows may carry: a schema-identifier name plus a declared type (one of FieldTypes). It is the typed counterpart to RelationSpec.Attributes' bare strings.

## type RelationSpec

```go
type RelationSpec struct {
	Name   string
	ToKind string
	// Attributes names the render-only string attributes relation rows may
	// carry. Untyped: every value is a string. Use TypedAttributes to declare a
	// non-string type for an attribute.
	Attributes []string
	// TypedAttributes declares relation-row attributes carrying a parsed type
	// (a billing "number", an aired "date") rather than a bare string. Each
	// name must not also appear in Attributes; each Type is one of FieldTypes.
	// The value stays plugin data; the SDK records only the declared name+type.
	TypedAttributes []RelationAttributeSpec
	// Cardinality is one of RelationCardinalities; empty means "many". The
	// store enforces "one" on Media-targeted relations (a second source
	// entity replaces the first, never duplicates).
	Cardinality string
}
```

RelationSpec is a declared relation between kinds. Cast/credit-style links are relations with attributes, never entities. ToKind names a declared kind or MediaKindName.

## type ScraperSpec

```go
type ScraperSpec struct {
	ID string
	// DisplayName is the optional human source name shown in scraper selection
	// surfaces (a locale map). Absent means clients fall back to ID.
	DisplayName LocaleString
	Kind        string
	// Endpoint is the base URL (https required outside tests).
	Endpoint string
	// AuthRef names the config/secret-store entry holding credentials.
	AuthRef string
	// OptionalAuth permits a scraper with an AuthRef to be omitted from the
	// active scraper set when that credential is unset.
	OptionalAuth bool
	// AuthStyle says how the AuthRef secret is presented — one of the fixed
	// AuthStyleForms. Empty means "bearer". Styles only make sense with an
	// AuthRef.
	AuthStyle string
	// Login declares the "login-bearer" style's token exchange; nil for
	// every other style.
	Login *LoginSpec
	// Signature declares the "signature:<name>" style's per-request-signature
	// archetype; nil for every other style. The declared Archetype must be one
	// of SignatureArchetypes and must match the AuthStyle's <name> suffix.
	Signature *SignatureSpec
	// LookupTemplate builds the per-item request: path (http-json), GraphQL
	// query body (http-graphql), or feed URL template (rss). Substitutions:
	// {id}, the fingerprint variables {phash}, {oshash}, {duration}
	// (content-identity lookups), and {config.<name>} for a
	// declared Bundle.ConfigFields entry. Fingerprint values the engine does
	// not know substitute as the empty string — placeholders never reach a
	// remote API literally.
	LookupTemplate string
	// LookupByIDTemplate optionally resolves a ParsedMedia.ProviderIDs hint
	// without a search round-trip: provider key (ProviderIDVocabulary) →
	// request template in LookupTemplate's shape, with {id} substituting the
	// hinted provider ID (TMDB's "/find/{id}?external_source=imdb_id" for an
	// "imdb" hint, "/movie/{id}" for "tmdb"). The fetched document is the
	// lookup document: FieldMap/ArtworkMap/EntityMaps resolve against it.
	// Nil or absent for a provider means the hint cannot short-circuit this
	// scraper and identification runs the ordinary search path. Each key must
	// pass ValidProviderIDKey and each template must contain {id}
	// (ValidateLookupByIDTemplates). It may also use LookupTemplate's
	// {config.<name>} substitutions. Network kinds only.
	LookupByIDTemplate map[string]string
	// SearchTemplate builds the free-text identification request; {query}
	// and the fingerprint variables substitute (a fingerprint-driven search
	// queries by content identity instead of free text). {page} substitutes
	// the one-based search page; {config.<name>} substitutes a declared
	// Bundle.ConfigFields entry. Requires CandidatesPath. Not valid on rss.
	SearchTemplate string
	// MaxPages caps the number of search pages the engine may fetch. Zero
	// means one page, preserving existing scrapers.
	MaxPages int
	// CandidatesPath is the response path of the search results array
	// ("results" for TMDB, "data" for TVDB, the root field for GraphQL);
	// FieldMap/ArtworkMap then apply per array element.
	CandidatesPath string
	// CandidateIDPath is the element-relative path of a candidate's id —
	// the value a chosen candidate feeds back into LookupTemplate's {id}.
	// Required when both templates are declared.
	CandidateIDPath string
	// FieldMap maps core field names (title, summary, year, …) to response
	// paths (dotted JSON path, GraphQL selection path, or RSS element). A JSON
	// path may use a name[key=value] predicate segment to select the first
	// array element whose key equals value; value may be a literal or a
	// {config.<name>} reference. Example:
	// release_dates.results[iso_3166_1={config.region}].release_dates.0.certification.
	// Paths resolve against the lookup document, and — when no SearchFieldMap
	// is declared — against one search candidate element too.
	FieldMap map[string]string
	// ArtworkMap maps artwork slots to response paths carrying image URLs.
	// Like FieldMap, it serves search candidates only when no
	// SearchArtworkMap is declared.
	ArtworkMap map[string]string
	// CandidateArtworkSlots declares which of this scraper's mapped artwork
	// slots retain every scraped candidate rather than only the applied
	// winner ("Artwork Slots retain candidates... kept and
	// browsable in a picker"). Each entry must name a slot the scraper maps
	// via ArtworkMap or SearchArtworkMap. Declarative only: the SDK records
	// which slots keep a candidate pool, the core store owns candidate
	// persistence and the picker. A mapped slot absent from this list keeps
	// today's single-winner behavior.
	CandidateArtworkSlots []string
	// FieldTypes optionally declares the type of mapped fields using the
	// FieldTypes vocabulary. Undeclared fields stay text for compatibility.
	FieldTypes map[string]string
	// SearchFieldMap, when non-nil, replaces FieldMap for search candidate
	// elements — for sources whose search-element shape diverges from the
	// lookup document. Nil falls back to FieldMap (same-shape sources keep
	// one map). Requires a SearchTemplate.
	SearchFieldMap map[string]string
	// SearchArtworkMap is SearchFieldMap's artwork twin: non-nil replaces
	// ArtworkMap for search candidates, nil falls back. Same slot
	// vocabulary as ArtworkMap. Requires a SearchTemplate.
	SearchArtworkMap map[string]string
	// Transforms post-processes mapped field values: field name → one of
	// the fixed TransformKinds. One vocabulary for both templates — a
	// transform applies to its core field name wherever the value resolved
	// from (lookup document or search element); there is no search-scoped
	// transform map. A value the transform cannot parse is omitted, like an
	// unresolved path.
	Transforms map[string]string
	// ArtworkBaseURL is prefixed onto site-relative ArtworkMap values
	// (TMDB's "/abc.jpg" poster_path → https://image.tmdb.org/t/p/original
	// + path). Absolute URLs pass through untouched. https required.
	ArtworkBaseURL string
	// ArtworkURLTemplates render non-URL artwork IDs into absolute URLs:
	// slot → HTTPS template containing {value}. The value is the scalar
	// resolved by ArtworkMap or SearchArtworkMap for the same slot.
	ArtworkURLTemplates map[string]string
	// EntityMaps declare lookup-document → Entity hydration (a
	// "field → Entity Kind relation" mapping): applying an identification
	// creates one Entity of the declared Kind per value at Path and links
	// each to the identified Media through the declared Relation. Each map may
	// also populate the Entity's typed fields (FieldMap), its artwork
	// (ArtworkPath), and a same-Kind parent link (ParentNamePath/
	// ParentRelation) for hierarchical kinds. JSON documents only (http-json,
	// http-graphql); requires a LookupTemplate.
	EntityMaps []EntityMap
	// ImpliedMediaFields declares the Media field values a successful
	// identification through this scraper implies: declared MediaFields name
	// → value. The choice of scraper carries the information — a bundle that
	// subdivides its Media declares one scraper per subdivision, each
	// implying the conventional "class" field's value. Keys must name a
	// declared text MediaField; values are plugin data, written verbatim.
	ImpliedMediaFields map[string]string
	// MinRequestIntervalMS is the minimum spacing, in milliseconds, between
	// any two engine requests for this scraper — declarative rate-limit
	// etiquette for sources that throttle clients (MusicBrainz: ~1 req/s).
	// Zero means unthrottled. Must not be negative.
	MinRequestIntervalMS int
	// ArchiveBackfill opts an "rss" scraper into archive backfill:
	// identifying every item the feed lists, not only new items since
	// the last poll. Requires ArchiveBackfillGUIDPath. Only valid on "rss"
	// scrapers; false means the engine polls new items only, today's
	// behavior. Declarative only: the SDK records the backfill intent, the
	// core ingest engine owns the actual archive walk.
	ArchiveBackfill bool
	// ArchiveBackfillGUIDPath names the per-item feed-element response path
	// resolving the item's GUID ("guid" for a standard RSS 2.0 item) — the
	// dedupe/refresh key ArchiveBackfill identifies archive items by, rather
	// than file path or the enclosure URL a DAI can rewrite freely.
	// Required when ArchiveBackfill is set; ignored otherwise.
	// Declarative only: the SDK records the guid source, core writes the
	// resolved value as the item's External ID at apply time.
	ArchiveBackfillGUIDPath string
}
```

ScraperSpec is a declarative fetch-and-map definition: URL templates plus mappings from response paths to core fields and artwork slots. No code, no secrets — AuthRef names a config secret, never holds it.

A scraper declares a SearchTemplate (free text → candidate list), a LookupTemplate (one id → one document), or both — identification is scraper search → candidates → lookup. FieldMap/ArtworkMap paths resolve against the lookup document, and against ONE element of the CandidatesPath array during search; sources whose search-element and lookup-document shapes diverge declare SearchFieldMap/SearchArtworkMap so each template gets the map that fits its shape (TVDB's top-level search elements vs its data.-enveloped details, StashDB's element-relative searchScene vs its findScene.-rooted lookup).

## type ScrobbleMatchSpec

```go
type ScrobbleMatchSpec struct {
	ArtistConfirm []string
	AlbumTieBreak []string
}
```

ScrobbleMatchSpec carries the bare, unqualified relation slugs the scrobble matcher uses. ArtistConfirm names the relation(s) that connect a credited Entity to a Track Media for artist confirmation; AlbumTieBreak names the Media-targeted relation(s) whose related Entity title disambiguates several artist-confirmed candidates by album. Only relations that target Media (queryable from a Media) belong here; the names must match the bare relation slugs the store persists.

## type ScrobbleRecord

```go
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
```

ScrobbleRecord is one metadata-keyed play event a source recorded with no file path — a Last.fm scrobble. The handler resolves it to a core Media by metadata (track title, confirmed by artist; album is a tiebreaker) and applies it as a play (increment count + advance last-played) to the importing Profile's engagement. It is plain media vocab (artist/ track/album — invariant #1), never a domain term.

## type ScrobbleTargetMapping

```go
type ScrobbleTargetMapping struct {
	Value string
	Hint  scrobble.TargetHint
}
```

ScrobbleTargetMapping maps one exact, plugin-owned Media field value to a bounded target hint. Values are compared as opaque data; no normalization or fallback is implied by this declaration.

## type ScrobbleTargetSpec

```go
type ScrobbleTargetSpec struct {
	Default    scrobble.TargetHint
	MediaField string
	Mappings   []ScrobbleTargetMapping
}
```

ScrobbleTargetSpec selects a bounded scrobble target shape without exposing plugin-owned domain vocabulary to Core. Exactly one mode must be declared: Default alone, or MediaField plus one or more Mappings.

## type SectionSpec

```go
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
```

SectionSpec is one Section: a fixed-kind client primitive parameterized by title, artwork slot, and Source.

## type SignatureSpec

```go
type SignatureSpec struct {
	// Archetype names the fixed signing recipe from SignatureArchetypes
	// ("podcast-index-sha1"). Must match the owning AuthStyle's <name> suffix.
	Archetype string
	// HeaderName is the request header the computed signature is sent on
	// ("Authorization" for Podcast Index). Must be non-empty.
	HeaderName string
}
```

SignatureSpec declares the "signature:\<name>" AuthStyle's per-request signature as pure data: which archetype computes the signature, and which header carries the computed value. The signing inputs themselves (the AuthRef key/secret, the current timestamp, the request path) are archetype- compiled — core reads only Archetype and HeaderName; it never evaluates plugin-supplied signing logic (there is none to evaluate).

## type SmartCollection

```go
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
```

SmartCollection is one imported smart collection/playlist: a rule-driven container the source auto-maintained. Beamfall imports it as a STATIC snapshot of its current members ("a smart collection imports its current members, statically") because the seam carries no rule/predicate representation, so the imported collection will not auto-update. The handler counts it toward the report's distinct smart-collection→static warning line so the operator sees the silent behavioral downgrade explicitly. It is generic by shape: an opaque stable identity, display name, and ordered member identities, never a source-system or domain term.

## type Source

```go
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
```

Source is the legacy per-run parser seam. Categories reports which data categories this source carries for the run.

Deprecated: implement StreamingSource in addition to Source during the 0.x compatibility window. Items remains temporarily required by existing consumers, but it necessarily materializes the complete import and is not suitable for large sources. New consumers should call StreamSource, which prefers StreamingSource and falls back to this interface.

## type SourceSpec

```go
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
```

SourceSpec is the canonical declarative Source reference.

## type StreamingSource

```go
type StreamingSource interface {
	Categories() []string
	Stream(
		ctx context.Context,
		checkpoint string,
		emit func(context.Context, []ForeignItem, string) error,
	) error
}
```

StreamingSource is the bounded, cancellable Migration Wizard importer seam. During the Source compatibility window an importer implements both interfaces; consumers discover this interface with a type assertion.

checkpoint is an opaque exclusive continuation token returned by a previous successful emit callback. The empty checkpoint starts at the first item. Every non-empty page carries a non-empty next checkpoint and contains at most MaxImportPageItems items. For unchanged input, a source must produce the same ordered items and checkpoints from the same starting checkpoint.

emit is synchronous. It must atomically apply the page and persist its next checkpoint before returning nil. If emit returns an error, Stream must stop and return that error without emitting another page; the caller resumes from the last checkpoint whose callback returned nil. Implementations must check ctx before reading or emitting more work and return its cancellation error promptly.

## type Subscription

```go
type Subscription struct {
	// Present is true when the parser populated a subscription for this item,
	// so an empty title/root is still distinguishable from "no subscription".
	Present bool
	FeedURL string
	Root    string
	Title   string
}
```

Subscription is one imported feed subscription: a feed URL, optional title, and optional target root. The run payload supplies the target Library; Root here overrides the payload root only when the parser has a per-feed download root.

## type TypedMediaFieldParser

```go
type TypedMediaFieldParser interface {
	ParseTypedMediaFields(path string, probe ParseProbe) []ParsedField
}
```

TypedMediaFieldParser is the typed extension of MediaFieldParser for hooks that need date fields from local metadata without turning them into text. Each returned ParsedField sets exactly one of Text or Date (see ParsedField).

## type WatchState

```go
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
```

WatchState is a foreign item's playback history: the resume position, the completion flag, and the true historical instant the source last recorded play. The handler writes it through the import-provenance engagement path so it keeps its real timestamp without masquerading as a fresh play.

