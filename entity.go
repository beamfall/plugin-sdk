package sdk

import (
	"fmt"
	"slices"
	"strings"
)

var archetypes = []string{"person", "container", "category"}

// Archetypes returns the fixed vocabulary bridging declared Entity Kinds to
// client rendering. Added rarely, in client releases.
func Archetypes() []string { return copySlice(archetypes) }

var artworkSlots = []string{"poster", "backdrop", "thumb", "logo", "portrait", "banner", "clearart", "disc", "season-poster"}

// ArtworkSlots returns the fixed artwork slot vocabulary:
// poster, backdrop, thumb, logo, portrait, banner, clearart, disc, and season-poster.
func ArtworkSlots() []string { return copySlice(artworkSlots) }

var fieldTypes = []string{"text", "number", "date", "flag", "url"}

// FieldTypes returns the fixed Entity Kind field-schema vocabulary.
func FieldTypes() []string { return copySlice(fieldTypes) }

var builtinMediaFields = []string{"title", "summary", "year"}

// candidateSignalFields is the closed set of bare FieldMap keys a scraper may
// map that feed the MatchScorer's Candidate ranking only — they are NOT Media
// metadata and are never persisted at apply time. sequence/index carry the
// candidate's ordinals (shape-named, mirroring ParsedMedia) for the scorer's
// ordinal agreement; popularity carries the tiebreak signal. Kept a bare-key
// set (not "media."-namespaced) so a plugin need not declare a throwaway Media
// field for a value that never lands on a Media row.
var candidateSignalFields = []string{"popularity", "sequence", "index"}

// BuiltinMediaFields returns the closed set of bare FieldMap keys: the core
// Media descriptive metadata identification writes directly
// (UpdateMediaMetadata). A bare FieldMap/SearchFieldMap key outside this set is
// a load-time error — it would silently no-op at identification today (the typo
// trap). Plugin Media fields route through the "media." namespace
// (MediaFieldKeyPrefix), not as bare keys.
func BuiltinMediaFields() []string { return copySlice(builtinMediaFields) }

// MediaFieldKeyPrefix namespaces a FieldMap/ImpliedMediaFields key onto a
// declared Bundle.MediaFields FieldSpec: "media.<name>" routes the resolved
// value through the Media typed-field side tables at identification time. The
// "entity." namespace is deliberately rejected — entity routing is EntityMap's
// job, not FieldMap's.
const MediaFieldKeyPrefix = "media."

// entityKeyPrefix is the rejected namespace: a FieldMap key under it points
// the author at EntityMap (entity hydration), never silently no-ops.
const entityKeyPrefix = "entity."

// MediaFieldRoute is one resolved FieldMap/ImpliedMediaFields key: whether it
// targets a core built-in or a plugin Media field, and (for plugin fields) the
// declared FieldSpec name and type the value parses against.
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

// classifyMediaFieldKey resolves one FieldMap/ImpliedMediaFields key against
// the namespacing contract, given the bundle's declared Media field types
// (name → type). It is the single source of truth both load-time validation
// and identification-time routing share, so a key the validator accepts is
// exactly a key routing knows how to write.
func classifyMediaFieldKey(key string, mediaFields map[string]string) (MediaFieldRoute, error) {
	if strings.HasPrefix(key, entityKeyPrefix) {
		return MediaFieldRoute{}, fmt.Errorf("field key %q uses the entity namespace; entity routing is EntityMap's job, not FieldMap's", key)
	}
	if name, ok := strings.CutPrefix(key, MediaFieldKeyPrefix); ok {
		fieldType, declared := mediaFields[name]
		if !declared {
			return MediaFieldRoute{}, fmt.Errorf("field key %q names media field %q, which the bundle does not declare", key, name)
		}
		return MediaFieldRoute{MediaField: name, Type: fieldType}, nil
	}
	if slices.Contains(builtinMediaFields, key) {
		return MediaFieldRoute{Builtin: true}, nil
	}
	if slices.Contains(candidateSignalFields, key) {
		return MediaFieldRoute{CandidateSignal: true}, nil
	}
	return MediaFieldRoute{}, fmt.Errorf("field key %q is not a built-in (%s), a candidate-signal (%s), and is not namespaced %q for a plugin media field", key, strings.Join(builtinMediaFields, ", "), strings.Join(candidateSignalFields, ", "), MediaFieldKeyPrefix+"<name>")
}

// ClassifyMediaFieldKey resolves one FieldMap/ImpliedMediaFields key for a
// validated bundle's declared Media fields.
func ClassifyMediaFieldKey(key string, mediaFields map[string]string) (MediaFieldRoute, error) {
	return classifyMediaFieldKey(key, mediaFields)
}

// MediaFieldTypes indexes the bundle's declared Media field names → declared
// types, the shape ClassifyMediaFieldKey resolves "media.<name>" keys against.
func (b Bundle) MediaFieldTypes() map[string]string {
	types := make(map[string]string, len(b.MediaFields))
	for _, field := range b.MediaFields {
		types[field.Name] = field.Type
	}
	return types
}

// EntityFieldTypes indexes the named Entity Kind's declared field names →
// declared types — the shape entity hydration parses an EntityMap.FieldMap's
// resolved values against. An undeclared kind yields an empty map.
func (b Bundle) EntityFieldTypes(kind string) map[string]string {
	for _, k := range b.EntityKinds {
		if k.Name != kind {
			continue
		}
		types := make(map[string]string, len(k.Fields))
		for _, field := range k.Fields {
			types[field.Name] = field.Type
		}
		return types
	}
	return map[string]string{}
}

// EntityKind declares one plugin-owned entity kind: its schema name, display
// label, archetype, typed fields, relations to other kinds (or to Media), and
// artwork slots. Kind names are a global namespace across active plugins
// (ValidateBundles rejects collisions).
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

var orphanPolicies = []string{"retain", "gc"}

// OrphanPolicies returns the fixed vocabulary for EntityKind.OrphanPolicy:
// "retain" (the default) keeps an instance that has lost every Media relation,
// "gc" opts the kind into store-side orphan garbage collection.
func OrphanPolicies() []string { return copySlice(orphanPolicies) }

var titleJoinRules = []string{"slash"}

// TitleJoinRules returns the fixed vocabulary for EntityKind.TitleJoinRule:
// the compiled-in archetypes for composing a display title when core spans
// multiple of a container kind's children into one Media (Film & TV's
// multi-episode file). "slash" joins each
// spanned child's title with " / " ("{E01 title} / {E02 title}"), the sole
// archetype needed today. Generic by shape: core reads only the
// declared archetype name and applies its compiled join; the child titles
// themselves stay plugin data.
func TitleJoinRules() []string { return copySlice(titleJoinRules) }

var ratingScaleArchetypes = []string{"ordered-certification"}

// RatingScaleArchetypes returns the fixed vocabulary for
// EntityKind.RatingScaleArchetype: the compiled-in shapes a container kind's
// rating scale takes. "ordered-certification" is an ordered scale plus
// ceiling (country × media-family × ordered certification values) backing
// per-Profile enforced filter ceilings and fail-closed unrated handling
// — the sole archetype needed today. Generic by shape: core reads only the declared archetype name;
// the scale's actual values (the certification levels, their order, and each
// instance's certification) are plugin data.
func RatingScaleArchetypes() []string { return copySlice(ratingScaleArchetypes) }

// FieldSpec is one declared field on an Entity Kind (or on the wire entity
// Media itself, via Bundle.MediaFields).
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

// MediaKindName is the reserved relation target naming the wire entity
// itself: relations run between Entities and Media.
const MediaKindName = "media"

var relationCardinalities = []string{"one", "many"}

// RelationCardinalities returns the fixed relation-cardinality vocabulary:
// "one" means each target is claimed by at most one source entity for the
// relation (an item belongs to exactly one container), "many" (the default)
// places no constraint.
func RelationCardinalities() []string { return copySlice(relationCardinalities) }

// RelationSpec is a declared relation between kinds. Cast/credit-style links
// are relations with attributes, never entities. ToKind names
// a declared kind or MediaKindName.
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

// RelationAttributeSpec is one typed attribute a relation's rows may carry:
// a schema-identifier name plus a declared type (one of FieldTypes). It is the
// typed counterpart to RelationSpec.Attributes' bare strings.
type RelationAttributeSpec struct {
	// Name is the schema identifier the attribute value keys on — never
	// localized.
	Name string
	// Type is one of FieldTypes; the type the resolved attribute value parses
	// against at hydration.
	Type string
}
