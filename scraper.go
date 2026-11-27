package sdk

import (
	"fmt"
	"strings"
)

var scraperKinds = []string{"http-json", "http-graphql", "rss", "exif"}

// ScraperKinds returns the fixed scraper transport vocabulary
// (declarative fetch-and-map definitions). "http-json", "http-graphql", and
// "rss" fetch a remote document over the network. "exif" is
// the lone local-file kind: it reads embedded EXIF metadata from the image
// Media's own file — no endpoint, no network, no templates — so the scraper
// validation below exempts it from the https-endpoint and template requirements
// the network kinds carry.
func ScraperKinds() []string { return copySlice(scraperKinds) }

// LocalFileScraperKind reports whether a scraper kind reads from the Media's
// local file rather than a remote endpoint. Local-file kinds skip the
// https-endpoint and URL-template requirements (they have neither) and run as
// part of the scan pass, not the network identification pipeline.
func LocalFileScraperKind(kind string) bool {
	return kind == "exif"
}

// AuthStyleForms returns the fixed scraper auth-style vocabulary: the shapes a
// spec's AuthStyle may take, i.e. how the AuthRef secret is presented on
// requests. Three forms are parameterized by a name after the colon; the
// secret itself always comes from the CredentialSource, never the spec.
//
//	"bearer"          Authorization: Bearer <secret> (the default)
//	"header:<name>"   request header <name>: <secret> (X-Api-Key services)
//	"query:<param>"   query parameter ?<param>=<secret> (TMDB v3 api_key)
//	"login-bearer"    declarative token exchange (LoginSpec), then Bearer
//	"signature:<name>" declarative per-request-signature archetype named
//	                  <name> from SignatureArchetypes (SignatureSpec supplies
//	                  the archetype's declared signing inputs; Podcast Index's
//	                  X-Auth-Key/X-Auth-Date/Authorization SHA-1 triple)
func AuthStyleForms() []string { return copySlice(authStyleForms) }

var authStyleForms = []string{"bearer", "header:<name>", "query:<param>", "login-bearer", "signature:<name>"}

// ValidateAuthStyle reports whether s is a legal ScraperSpec.AuthStyle value:
// the empty string (the documented "bearer" default — AuthStyleForms lists
// the template sentinels, not this contract-legal shorthand, so "" is valid
// even though it is absent from that slice), the bare forms ("bearer",
// "login-bearer"), or one of the three parameterized forms ("header:<name>",
// "query:<param>", "signature:<name>") with its colon-suffixed name non-empty
// and not a literal template sentinel (a name containing "<" or ">" is the
// AuthStyleForms placeholder copied verbatim, never a real header, parameter,
// or archetype name). A "query:<param>" style is contract-legal but puts the
// secret in the request URL — logs, proxies, and browser history all see it —
// so callers choosing between forms should prefer "header:<name>" or "bearer"
// wherever the upstream API supports it. A "signature:<name>" style further
// requires <name> to name a declared SignatureArchetypes entry — checked by
// ValidateBundle, not here, since that vocabulary is core-compiled rather than
// a shape this function alone can validate.
func ValidateAuthStyle(s string) bool {
	if s == "" || s == "bearer" || s == "login-bearer" {
		return true
	}
	for _, prefix := range []string{"header:", "query:", "signature:"} {
		if name, ok := strings.CutPrefix(s, prefix); ok {
			return name != "" && !strings.ContainsAny(name, "<>")
		}
	}
	return false
}

// SignatureArchetypes returns the fixed vocabulary for the "signature:<name>"
// AuthStyle's archetype name: the compiled-in per-request-signature shapes
// core knows how to compute. "podcast-index-sha1" is the Podcast Index v2
// triple: headers `X-Auth-Key: <AuthRef api key>` and
// `X-Auth-Date: <unix seconds>`, plus `Authorization: sha1(api-key + api-secret
// + X-Auth-Date)` hex-encoded — computed fresh per request, never a static
// secret. Generic by shape: core reads only the declared archetype name and
// computes its fixed signing recipe; the actual key/secret come from the
// AuthRef CredentialSource, never the spec.
func SignatureArchetypes() []string { return copySlice(signatureArchetypes) }

var signatureArchetypes = []string{"podcast-index-sha1"}

// TransformKinds returns the fixed value-transform vocabulary: per-field
// post-processing of mapped FieldMap values, named by shape, never domain.
// Deliberately not a DSL — kinds are added rarely, when a real scraper needs one.
//
//	"year-from-date"      YYYY-MM-DD… or RFC-822 date → "YYYY"
//	"date-from-rfc822"    RFC-822 date (RSS pubDate) → "YYYY-MM-DD"
//	"seconds-from-clock"  "HH:MM:SS"/"MM:SS"/plain seconds → seconds
func TransformKinds() []string { return copySlice(transformKinds) }

var transformKinds = []string{"year-from-date", "date-from-rfc822", "seconds-from-clock"}

// ScraperSpec is a declarative fetch-and-map definition: URL
// templates plus mappings from response paths to core fields and artwork
// slots. No code, no secrets — AuthRef names a config secret, never holds it.
//
// A scraper declares a SearchTemplate (free text → candidate list), a
// LookupTemplate (one id → one document), or both — identification is
// scraper search → candidates → lookup. FieldMap/ArtworkMap
// paths resolve against the lookup document, and against ONE element of the
// CandidatesPath array during search; sources whose search-element and
// lookup-document shapes diverge declare SearchFieldMap/SearchArtworkMap so
// each template gets the map that fits its shape (TVDB's top-level search
// elements vs its data.-enveloped details, StashDB's element-relative
// searchScene vs its findScene.-rooted lookup).
type ScraperSpec struct {
	ID string
	// DisplayName is the optional human source name shown in scraper selection
	// surfaces (a locale map). Absent means clients fall back to ID.
	DisplayName LocaleString
	// Attribution is the credit the data source's terms require wherever its
	// metadata is shown; nil when the source requires none.
	Attribution *Attribution
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
	// CredentialHelp tells clients what the owner enters for AuthRef and
	// where to get it ("Subscriber PIN", "Subscribe at thetvdb.com"), so a
	// credential that is not an API key is not presented as one. Nil keeps
	// the default key presentation. Ignored without an AuthRef.
	CredentialHelp *CredentialHelp
	// Login declares the "login-bearer" style's token exchange; nil for
	// every other style.
	Login *LoginSpec
	// ReferenceDocuments names static lookup documents (name → declaration)
	// that FieldMap, ArtworkMap, SearchFieldMap and SearchArtworkMap paths
	// may read through a {ref.<name>:<path>} placeholder: the placeholder is
	// replaced by the scalar at <path> in the named document before the map
	// path resolves, so data.artworks[type={ref.artwork-types:data[slug=background,recordType=series].id}].image
	// selects artwork by the source's own type id. A placeholder that does
	// not resolve leaves its map path unresolved (omitted, never an error).
	// Names follow the config-field slug shape. http-json scrapers only.
	ReferenceDocuments map[string]ReferenceDocument
	// Signature declares the "signature:<name>" style's per-request-signature
	// archetype; nil for every other style. The declared Archetype must be one
	// of SignatureArchetypes and must match the AuthStyle's <name> suffix.
	Signature *SignatureSpec
	// LookupTemplate builds the per-item request: path (http-json), GraphQL
	// query body (http-graphql), or feed URL template (rss). Substitutions:
	// {id}, the fingerprint variables {phash}, {oshash}, {duration}
	// (content-identity lookups), the parsed ordinals {sequence} and {index}
	// (ParsedMedia.Sequence and ParsedMedia.Index of the looked-up Media), and
	// {config.<name>} for a declared Bundle.ConfigFields entry. Fingerprint
	// and ordinal values the engine does not know substitute as the empty
	// string — placeholders never reach a remote API literally.
	LookupTemplate string
	// ItemLookupTemplate optionally fetches a per-item document after the
	// LookupTemplate document: the same {id} plus {sequence} and {index}
	// address one item inside the looked-up container
	// ("/tv/{id}/season/{sequence}/episode/{index}"). ItemFieldMap and
	// ItemArtworkMap map that document, and their values override the
	// FieldMap/ArtworkMap values for the same field or slot; Transforms apply
	// by field name as usual. A template naming {sequence} or {index} the host
	// does not know skips the item fetch, and a failed item fetch leaves the
	// main mapping unchanged. http-json only; requires LookupTemplate and must
	// contain {id}.
	ItemLookupTemplate string
	// ItemFieldMap maps core field names to ItemLookupTemplate document paths
	// (FieldMap's addressing and vocabulary). Requires ItemLookupTemplate.
	ItemFieldMap map[string]string
	// ItemArtworkMap maps artwork slots to ItemLookupTemplate document paths
	// (ArtworkMap's addressing and vocabulary). Requires ItemLookupTemplate.
	ItemArtworkMap map[string]string
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
	// LookupByIDRecordIDPaths turns chosen LookupByIDTemplate entries into
	// resolvers, keyed by the same provider: the document that provider's
	// template fetches is read only for this scraper's own record id at the
	// path, and the result is the ordinary LookupTemplate lookup of that id.
	// A source whose external-id endpoint answers with a summary record
	// (TheTVDB's /search/remoteid/{id} → data[0].series.id) then yields the
	// same full record as a search match. Providers without an entry map the
	// fetched document directly. http-json only; requires LookupTemplate, and
	// every key must name a LookupByIDTemplate provider.
	LookupByIDRecordIDPaths map[string]string
	// SearchTemplate builds the free-text identification request; {query},
	// {query.lucene}, and the fingerprint variables substitute (a
	// fingerprint-driven search queries by content identity instead of free
	// text). {query.lucene} escapes the free-text query for Apache Lucene query
	// parser syntax (prefixing \, +, -, !, (, ), :, ^, [, ], ", {, }, ~, *, ?,
	// |, &, and / with \) before applying the same transport escaping as
	// {query} (net/url.QueryEscape in a URL query, path-escaping in a URL path,
	// or GraphQL string escaping in http-graphql); it is only valid in
	// SearchTemplate. {page} substitutes
	// the one-based search page; {config.<name>} substitutes a declared
	// Bundle.ConfigFields entry. {year} substitutes the year identification
	// parsed or knows for the item; when the year is unknown, a query
	// parameter whose whole value is {year} is dropped from the request
	// rather than sent empty. Within the URL query, {hint.<key>} substitutes
	// the value from ParsedMedia.Hints (key must satisfy ValidHintKey). The host
	// escapes each value with net/url.QueryEscape and substitutes an empty
	// string when the hint is missing or empty. GraphQL request bodies reject
	// hint placeholders until the SDK defines GraphQL value binding. Requires
	// CandidatesPath. Not valid on rss.
	SearchTemplate string
	// RequiredSearchHints lists hint keys referenced by SearchTemplate that
	// must have non-empty values. The host skips this scraper for that search
	// if any is missing or empty. Independent lookups are unaffected. Keys
	// must be unique, and at most 16 may be required.
	RequiredSearchHints []string `json:",omitempty"`
	// MaxPages caps the number of search pages the engine may fetch. Zero
	// means one page, preserving existing scrapers.
	MaxPages int
	// CandidatesPath is the response path of the search results array
	// ("results" for TMDB, "data" for TVDB, the root field for GraphQL);
	// FieldMap/ArtworkMap then apply per array element.
	// Use "$" for a bare response array or append "[?key=value,...]" to an
	// array key or "$" to keep every matching row in response order. A
	// filter-all predicate must end the path. Both forms are list-only
	// syntax shared with ChildListing.ItemsPath.
	CandidatesPath string
	// CandidateIDPath is the element-relative path of a candidate's id —
	// the value a chosen candidate feeds back into LookupTemplate's {id}.
	// Required when both templates are declared.
	CandidateIDPath string
	// Children declares up to three child listing levels below a search
	// candidate, walked in order by a manager picking a container and item
	// (ADR-0068 amendment H). Empty means the scraper offers no child listing.
	Children []ChildListing `json:",omitempty"`
	// FieldMap maps core field names (title, sortTitle, summary, year, …) to response
	// paths (dotted JSON path, GraphQL selection path, or RSS element). A JSON
	// path may use a name[key=value] predicate segment to select the first
	// array element whose key equals value; value may be a literal or a
	// {config.<name>} reference. Example:
	// release_dates.results[iso_3166_1={config.region}].release_dates.0.certification.
	// Both predicate forms may hold several comma-separated key=value
	// conditions, all of which must match ([slug=background,recordType=series]).
	// Each condition tries a literal row key first, then a row-relative
	// dotted path. Values compare as strings: JSON numbers use their numeric
	// string form (integral values omit the decimal point) and booleans use
	// "true" or "false". Null, missing values, arrays and objects never match.
	// A path may list alternatives separated by "||": the first alternative that
	// resolves wins, so a translated value can fall back to the original
	// (data.translations.nameTranslations[language={config.language}].name||data.name).
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
	// ParentRelation) for hierarchical kinds. A Container map instead hydrates
	// the existing container Entity holding the Media (EntityMap.Container).
	// JSON documents only (http-json, http-graphql); requires a LookupTemplate.
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

// ChildListing describes one level of rows below a scraper search candidate.
type ChildListing struct {
	// Level is the client-facing lower-kebab label, unique within Children.
	Level string
	// ListTemplate is an endpoint-relative path with an optional query. It
	// may use {id} for the search candidate and {config.<name>} for declared
	// config fields. Every level after the first must use at least one of
	// {parent}, {sequence}, or {index} from the previous row. The first level
	// cannot use those placeholders.
	ListTemplate string
	// ItemsPath identifies the response array in the CandidatesPath dialect,
	// including "$" for a bare array and terminal filter-all predicates.
	ItemsPath string
	// Key is the row-relative path to the value substituted as {parent}.
	Key string
	// Label is the row-relative path to the display label.
	Label string
	// Sequence is the optional row-relative path to an ordinal sequence.
	Sequence string `json:",omitempty"`
	// Index is the optional row-relative path to an ordinal index.
	Index string `json:",omitempty"`
	// Year is an optional row-relative path to a year or date.
	Year string `json:",omitempty"`
	// Artwork is an optional row-relative path to an artwork path or URL,
	// resolved with the scraper's ArtworkBaseURL.
	Artwork string `json:",omitempty"`
	// MaxItems caps rows returned at this level. Zero selects the default
	// of 200, and the maximum is 500.
	MaxItems int `json:",omitempty"`
}

// ValidChildKeySegment reports whether v is safe to use as a single child
// listing path segment. Core must also check that the key was returned by the
// previous level and URL-escape it before substituting it.
func ValidChildKeySegment(v string) bool {
	return v != "" && v != "." && v != ".." && !strings.ContainsAny(v, `/\?#`) &&
		!strings.Contains(v, "%2f") && !strings.Contains(v, "%2F")
}

// ValidateLookupByIDTemplates checks a scraper's LookupByIDTemplate map: every
// key is a ProviderIDVocabulary provider, every template is non-blank and
// contains the {id} placeholder, and a local-file scraper kind declares none
// (it has no endpoint to resolve an ID against). A nil or empty map is valid —
// the field is optional and a scraper omitting it is unchanged.
func ValidateLookupByIDTemplates(scraper ScraperSpec) error {
	if len(scraper.LookupByIDTemplate) == 0 {
		return nil
	}
	if LocalFileScraperKind(scraper.Kind) {
		return fmt.Errorf("scraper %q: local-file kind %q cannot declare LookupByIDTemplate", scraper.ID, scraper.Kind)
	}
	for provider, tmpl := range scraper.LookupByIDTemplate {
		if !ValidProviderIDKey(provider) {
			return fmt.Errorf("scraper %q: LookupByIDTemplate key %q is outside the vocabulary %s", scraper.ID, provider, strings.Join(ProviderIDVocabulary(), ", "))
		}
		if strings.TrimSpace(tmpl) == "" || !strings.Contains(tmpl, "{id}") {
			return fmt.Errorf("scraper %q: LookupByIDTemplate[%q] must contain {id}", scraper.ID, provider)
		}
	}
	return nil
}

// EntityMap is one declarative lookup-document → Entity mapping. Named by
// shape, never domain: the Kind and Relation are the declaring plugin's data,
// the map itself is core vocabulary.
type EntityMap struct {
	// Kind names a declared Entity Kind the hydrated Entities instantiate.
	Kind string
	// Relation names a Media-targeted relation (ToKind == MediaKindName)
	// declared on Kind — hydration writes one entity→Media relation row per
	// hydrated Entity under this name (relations run from the declaring
	// kind, matching the store's from-entity/to-media row shape). Must be
	// empty when Container is set.
	Relation string
	// Container makes the map hydrate the existing container Entity of Kind
	// that holds the looked-up Media — directly, or further up its chain of
	// container Entities — instead of creating and linking a new Entity. The
	// host built that Entity (from the folder layout or a local mapper); the
	// map only adds the ArtworkPath/ArtworkMap images, FieldMap values and
	// TitlePath display title it resolves, plus its ContainerRelations links,
	// keeping the Entity's identity and its other fields. No Entity of Kind
	// holding the Media means nothing is hydrated. Kind must be a
	// "container" archetype; Relation must be empty; NamePath, LookupTemplate,
	// ParentNamePath, ParentRelation and OrdinalField are rejected; at least
	// one of ArtworkPath, ArtworkMap, FieldMap or TitlePath is required. Path
	// may be empty (the lookup document root) and should resolve to a single
	// object.
	Container bool
	// Path is the lookup-document response path of the value(s): a scalar
	// yields one Entity, an array yields one per element. Unresolved paths
	// are omitted, never errors — same posture as FieldMap. It may name the
	// {sequence} and {index} placeholders (inside a predicate,
	// seasons[season_number={sequence}]); a map whose Path names a value the
	// host does not know is skipped. Required unless Container is set, where
	// empty means the document root.
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
	// OrdinalField names a FieldMap or RelationAttributeMap key whose resolved
	// numeric value supplies the relation's sort position (relation.sort_order)
	// — the ordering source that makes OrderedChildren and the next-up Source
	// non-empty for an ordered container, or a credit list's billing order.
	// The plugin declares which document value
	// is the ordinal (an installment or sequence number it carries); a missing
	// or non-numeric value falls back to the document array index, the same
	// FieldMap "omit, never error" posture. Generic by shape: the value is an
	// integer position, never a domain label. Empty means order by array index
	// only. Must name a key declared in FieldMap or RelationAttributeMap.
	OrdinalField string
	// OrdinalOffset is added to every relation sort position this map writes
	// (the OrdinalField value or the array index), so several maps writing one
	// relation can lay their rows out one after another: a second map at 1000
	// sorts after the first map's rows below 1000. Must not be negative; zero
	// leaves positions as they are. Rejected on a Container map.
	OrdinalOffset int
	// RelationAttributeMap declares the relation row's attribute values: an
	// attribute the map's Relation declares (RelationSpec.Attributes) → a path
	// resolved against the Path element itself, never a LookupTemplate
	// document, because an attribute describes this link (a credit's character
	// or job), not the referenced Entity. Unresolved or blank values are
	// omitted, never errors. When one identification links an Entity to the
	// Media under one relation more than once (one person credited for two
	// jobs, or by two maps), the host writes one row: its attributes merge,
	// distinct values of one attribute joining in map declaration order, then
	// element order, with ", ", and its sort position is the smallest of the
	// links'. Empty means rows without attributes. Rejected on a Container map.
	RelationAttributeMap map[string]string
	// ElementFilter keeps only the Path elements whose element-relative paths
	// resolve to accepted values: path → accepted values, and every entry must
	// match (a key=value test like a predicate, but keeping every match rather
	// than the first). A filtered-out element hydrates nothing and consumes no
	// MaxLookups; kept elements keep their array index as sort position. Paths
	// resolve against the Path element, never a LookupTemplate document. Empty
	// means every element hydrates. Rejected on a Container map.
	ElementFilter map[string][]string
	// LookupTemplate optionally fetches a referenced JSON document per EntityMap
	// element before resolving NamePath/ArtworkPath/FieldMap/ParentNamePath,
	// up to MaxLookups. LookupIDPath resolves the per-element ID fed into {id};
	// {sequence} and {index} substitute as in the scraper's LookupTemplate.
	// Empty keeps the legacy element-local behavior.
	LookupTemplate string
	LookupIDPath   string
	// MaxLookups caps secondary LookupTemplate fetches for this EntityMap. It
	// must be positive when LookupTemplate is set; zero means no secondary
	// lookup is declared. Elements past the cap still hydrate, from the element
	// itself as if the map declared no LookupTemplate; an Entity hydrated with
	// no resolved FieldMap value keeps the typed fields an earlier
	// identification wrote.
	MaxLookups int
	// ArtworkURLTemplate renders this EntityMap's ArtworkPath scalar into an
	// absolute HTTPS URL. Empty uses the scraper's ArtworkBaseURL composition.
	ArtworkURLTemplate string
	// ArtworkMap (Container maps only) declares more container artwork:
	// artwork slot (one of ArtworkSlots) → container-node-relative path. Each
	// value composes like the scraper's Media ArtworkMap: the slot's
	// ArtworkURLTemplates entry, else ArtworkBaseURL. "poster" must not be
	// declared both here and by ArtworkPath. Unresolved paths are omitted.
	ArtworkMap map[string]string
	// TitlePath (Container maps only) resolves, against the container node,
	// the container Entity's display title. A non-blank value replaces the
	// title; the Entity's identity is unchanged and the host-built name (the
	// folder spelling) stays the fallback when the path does not resolve.
	TitlePath string
	// ContainerRelations (Container maps only) links the container Entity to
	// Entities sibling maps hydrate: relation declared on Kind whose ToKind is
	// an Entity kind → the Relation of a non-container entity map of that
	// ToKind in the same scraper. No non-container map of another kind may
	// declare that Relation, so the host matches siblings by Relation alone.
	// The host links the container Entity to every Entity that sibling map
	// hydrated in the same identification, under the container relation,
	// keeping the sibling's sort order.
	ContainerRelations map[string]string
}

// Attribution is a data source's required credit as pure data: the text
// clients show (a locale map with an "en" anchor) and the https link it points
// at. Clients render it next to metadata the source provided and in their
// credits view.
type Attribution struct {
	Text LocaleString
	URL  string
}

// LoginSpec declares the "login-bearer" token exchange as pure data (the
// TheTVDB v4 POST /login shape): the engine POSTs {SecretField: <secret>}
// as JSON to Endpoint+Path and uses the string at TokenPath in the JSON
// response as the bearer token for subsequent requests.
type LoginSpec struct {
	// Path is appended to the scraper Endpoint; must start with "/".
	Path string
	// SecretField is the JSON body key carrying the secret ("apikey").
	SecretField string
	// TokenPath is the JSON path of the issued token ("data.token").
	TokenPath string
	// SecretFields adds further JSON body keys, each carrying the secret named
	// by its AuthRef value and resolved exactly like the scraper's AuthRef —
	// for example {"apikey": "tvdb-api-key"} beside a "pin" SecretField whose
	// AuthRef is the user's subscriber PIN. A missing one fails the exchange
	// like a missing AuthRef. Nil for a single-secret login.
	SecretFields map[string]string
}

// CredentialHelp presents a scraper's AuthRef credential to the owner.
type CredentialHelp struct {
	// Label names what the owner enters ("Subscriber PIN"); an "en" anchor
	// is required.
	Label LocaleString
	// HelpURL optionally links to where the owner obtains it; an absolute
	// https URL. Requires HelpLabel.
	HelpURL string
	// HelpLabel is the link text for HelpURL ("Subscribe at thetvdb.com").
	HelpLabel LocaleString
}

// ReferenceDocument declares a static lookup document a scraper's map paths
// can read from (TheTVDB's /artwork/types): fetched with the scraper's
// authorization and cached by the engine, so ids that the source assigns
// are looked up instead of hard-coded.
type ReferenceDocument struct {
	// Path is appended to the scraper Endpoint; must start with "/". It may
	// use {config.<name>} substitutions and no other placeholder.
	Path string
	// CacheHours is how long the engine reuses a fetched copy. Zero means
	// one week (168 hours), the usual guidance for static lookup endpoints;
	// it must not be negative or exceed 8760 (one year).
	CacheHours int
}

// SignatureSpec declares the "signature:<name>" AuthStyle's per-request
// signature as pure data: which archetype computes the signature, and which
// header carries the computed value. The signing inputs themselves (the
// AuthRef key/secret, the current timestamp, the request path) are archetype-
// compiled — core reads only Archetype and HeaderName; it never evaluates
// plugin-supplied signing logic (there is none to evaluate).
type SignatureSpec struct {
	// Archetype names the fixed signing recipe from SignatureArchetypes
	// ("podcast-index-sha1"). Must match the owning AuthStyle's <name> suffix.
	Archetype string
	// HeaderName is the request header the computed signature is sent on
	// ("Authorization" for Podcast Index). Must be non-empty.
	HeaderName string
}
