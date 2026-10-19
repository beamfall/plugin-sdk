package sdk

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// ParsedMedia is a filename parser's best guess for one unidentified file.
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

// ParseHintSource is the origin class of a ParsedMedia.ProviderIDs hint — the
// trust axis core ranks identification hints by (an NFO written by a prior
// manager outranks a folder tag; a sidecar the operator authored outranks both).
type ParseHintSource string

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

var parseHintSources = []string{string(ParseHintSourceFilename), string(ParseHintSourceNFO), string(ParseHintSourceSidecar)}

// ParseHintSources returns the fixed ParsedMedia.HintSource vocabulary.
func ParseHintSources() []string { return copySlice(parseHintSources) }

// ValidParseHintSource reports whether s is a legal non-empty HintSource.
func ValidParseHintSource(s ParseHintSource) bool {
	return slices.Contains(parseHintSources, string(s))
}

var providerIDVocabulary = []string{"tmdb", "imdb", "tvdb", "anidb", "musicbrainz-<entity>"}

// ProviderIDVocabulary returns the closed ProviderIDs key vocabulary. The
// "musicbrainz-<entity>" entry is a template: any key of the form
// "musicbrainz-" plus a non-empty lowercase entity name is legal.
func ProviderIDVocabulary() []string { return copySlice(providerIDVocabulary) }

// ValidProviderIDKey reports whether key is a legal ProviderIDs provider name.
func ValidProviderIDKey(key string) bool {
	if slices.Contains(providerIDVocabulary[:4], key) {
		return true
	}
	entity, ok := strings.CutPrefix(key, "musicbrainz-")
	if !ok || entity == "" {
		return false
	}
	return entity == strings.ToLower(entity) && !strings.ContainsAny(entity, "<>")
}

// ValidateParseHints checks the ProviderIDs/HintSource pairing contract on a
// parse result: nil ProviderIDs requires an empty HintSource; a populated map
// requires a recognized HintSource, recognized provider keys, and non-empty ID
// values. A parser that carries no hints passes unchanged.
func ValidateParseHints(m ParsedMedia) error {
	if len(m.ProviderIDs) == 0 {
		if m.HintSource != ParseHintSourceNone {
			return fmt.Errorf("HintSource %q set without ProviderIDs", m.HintSource)
		}
		return nil
	}
	if !ValidParseHintSource(m.HintSource) {
		return fmt.Errorf("ProviderIDs carried with HintSource %q; want one of %s", m.HintSource, strings.Join(parseHintSources, ", "))
	}
	for key, id := range m.ProviderIDs {
		if !ValidProviderIDKey(key) {
			return fmt.Errorf("ProviderIDs key %q is outside the vocabulary %s", key, strings.Join(providerIDVocabulary, ", "))
		}
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("ProviderIDs[%q] is empty", key)
		}
	}
	return nil
}

// ParsedTextField is a plugin parser's best-effort text Media field derived
// from file-local metadata.
type ParsedTextField struct {
	Name string
	Text string
}

// ParsedField is a parser-derived Media field with the small typed vocabulary
// scan can safely persist absent-only. Exactly one of Text and Date carries the
// value: a text field sets Text and leaves Date zero; a date field sets Date and
// leaves Text empty. TypedMediaFieldParser yields these (its date-capable shape);
// the older MediaFieldParser yields text-only ParsedTextField values.
type ParsedField struct {
	Name string
	Text string
	Date time.Time
}

// DirectoryMediaFileCount is bounded evidence about how many media files share
// the current file's directory. It deliberately carries only the shape needed
// by parsers, never an exact unbounded count or sibling names.
type DirectoryMediaFileCount string

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

// ParseContext is bounded, scan-owned evidence that is not available from one
// file's path or probe alone. A zero ParseContext is absent context. Producers
// set a non-zero field only from complete evidence; parsers MUST treat absent
// or unrecognized values as unknown and preserve their legacy Parse fallback.
type ParseContext struct {
	DirectoryMediaFileCount DirectoryMediaFileCount
}

// ParseProbe is scan's view of the FFmpeg probe handed to the parser.
type ParseProbe struct {
	HasVideo        bool
	HasAudio        bool
	DurationSeconds int
	Tags            map[string]string
}

// FilenameParser is a first-party compiled plugin hook.
type FilenameParser interface {
	Parse(path string, probe ParseProbe) (ParsedMedia, bool)
}

// ContextFilenameParser is the optional additive FilenameParser extension for
// parsers that can use bounded scan-owned context. ParseWithContext MUST return
// the same result as Parse when context is zero or contains an unrecognized
// value. Existing parsers and callers continue to use FilenameParser unchanged.
type ContextFilenameParser interface {
	FilenameParser
	ParseWithContext(path string, probe ParseProbe, context ParseContext) (ParsedMedia, bool)
}

// Candidate is one identification candidate from a scraper search.
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

// MatchScorer scores how confidently one identification Candidate matches a
// file's parsed guess. Score MUST return a value in [0,1] (0 = no confidence,
// 1 = certain) and must be deterministic for the same inputs; NaN and
// out-of-range values are contract violations that sdktest.RunScoreConformance
// and sdktest.RunMatchCorpus fail loudly rather than pick on.
type MatchScorer interface {
	Score(candidate Candidate, parsed ParsedMedia) float64
}

// MediaFieldParser is an optional extension for parser hooks that can derive
// plugin-declared text Media fields from the same file-local metadata.
type MediaFieldParser interface {
	ParseMediaFields(path string, probe ParseProbe) []ParsedTextField
}

// TypedMediaFieldParser is the typed extension of MediaFieldParser for hooks
// that need date fields from local metadata without turning them into text.
// Each returned ParsedField sets exactly one of Text or Date (see ParsedField).
type TypedMediaFieldParser interface {
	ParseTypedMediaFields(path string, probe ParseProbe) []ParsedField
}
