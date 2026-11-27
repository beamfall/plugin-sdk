package sdk

import (
	"strconv"
	"strings"
	"testing"
)

type legacyFilenameParser struct{}

func (legacyFilenameParser) Parse(path string, _ ParseProbe) (ParsedMedia, bool) {
	return ParsedMedia{Title: path}, path != ""
}

var _ FilenameParser = legacyFilenameParser{}

// TestParseContextIsAdditiveAndBounded pins source compatibility and the
// three-state directory shape. In particular, the positional ParseProbe
// literal is old caller syntax that would stop compiling if ParseProbe grew.
func TestParseContextIsAdditiveAndBounded(t *testing.T) {
	oldPositionalProbe := ParseProbe{true, true, 60, map[string]string{"title": "x"}}
	oldKeyedProbe := ParseProbe{HasAudio: true, Tags: map[string]string{"title": "x"}}
	parser := legacyFilenameParser{}

	if got, ok := parser.Parse("x.mp3", oldPositionalProbe); !ok || got.Title != "x.mp3" {
		t.Fatalf("legacy parser with positional ParseProbe = %+v/%v", got, ok)
	}
	if got, ok := parser.Parse("x.mp3", oldKeyedProbe); !ok || got.Title != "x.mp3" {
		t.Fatalf("legacy parser with keyed ParseProbe = %+v/%v", got, ok)
	}

	var absent ParseContext
	if absent.DirectoryMediaFileCount != DirectoryMediaFileCountUnknown {
		t.Fatalf("zero ParseContext count = %q, want unknown", absent.DirectoryMediaFileCount)
	}
	if absent.FamilyScope != "" {
		t.Fatalf("zero ParseContext family scope = %q, want empty", absent.FamilyScope)
	}
	scoped := ParseContext{DirectoryMediaFileCount: DirectoryMediaFileCountSingle, FamilyScope: "tvshows"}
	if scoped.FamilyScope != "tvshows" {
		t.Fatalf("keyed ParseContext family scope = %q, want tvshows", scoped.FamilyScope)
	}
	states := map[DirectoryMediaFileCount]bool{
		DirectoryMediaFileCountUnknown:  true,
		DirectoryMediaFileCountSingle:   true,
		DirectoryMediaFileCountMultiple: true,
	}
	if len(states) != 3 {
		t.Fatalf("directory media-file count states are not distinct: %v", states)
	}
}

// TestCandidateOrdinalPresenceDistinguishesZeroFromAbsent asserts the
// SequencePresent/IndexPresent presence fields let a genuine ordinal 0 be
// told apart from "the source carries no ordinal at all" — the ambiguity the
// old bare "0 = not carried" convention on Sequence/Index could not express.
func TestCandidateOrdinalPresenceDistinguishesZeroFromAbsent(t *testing.T) {
	absent := Candidate{Title: "a"}
	zeroPresent := Candidate{Title: "a", Index: 0, IndexPresent: true}
	nonzeroPresent := Candidate{Title: "a", Index: 7, IndexPresent: true}

	if absent.IndexPresent {
		t.Fatalf("zero-value Candidate literal must default IndexPresent=false, got true")
	}
	if absent.Index != 0 {
		t.Fatalf("zero-value Candidate literal must default Index=0, got %d", absent.Index)
	}
	if !zeroPresent.IndexPresent {
		t.Fatalf("Candidate{Index: 0, IndexPresent: true} lost IndexPresent=true")
	}
	if zeroPresent.Index != 0 {
		t.Fatalf("Candidate{Index: 0, IndexPresent: true} must keep Index=0, got %d", zeroPresent.Index)
	}
	if absent.Index == zeroPresent.Index && absent.IndexPresent == zeroPresent.IndexPresent {
		t.Fatal("absent and zero-present candidates must be distinguishable via IndexPresent")
	}
	if !nonzeroPresent.IndexPresent || nonzeroPresent.Index != 7 {
		t.Fatalf("Candidate{Index: 7, IndexPresent: true} = %+v, want Index=7 IndexPresent=true", nonzeroPresent)
	}
}

// TestCandidateOrdinalPresenceOldCallerFailsSafe asserts a struct literal
// written before SequencePresent/IndexPresent existed — one that only sets
// the legacy Sequence/Index ints — keeps compiling and keeps its old
// "0 = not carried" reading: the new presence fields default to false rather
// than silently asserting presence the old caller never declared.
func TestCandidateOrdinalPresenceOldCallerFailsSafe(t *testing.T) {
	oldStyleAbsent := Candidate{Title: "a", Sequence: 0, Index: 0}
	oldStyleCarried := Candidate{Title: "a", Sequence: 4, Index: 5}

	if oldStyleAbsent.SequencePresent || oldStyleAbsent.IndexPresent {
		t.Fatalf("old-style caller unaware of the presence fields must not appear to assert presence: %+v", oldStyleAbsent)
	}
	if oldStyleCarried.SequencePresent || oldStyleCarried.IndexPresent {
		t.Fatalf("old-style caller unaware of the presence fields must not appear to assert presence even with nonzero ordinals: %+v", oldStyleCarried)
	}
	if oldStyleCarried.Sequence != 4 || oldStyleCarried.Index != 5 {
		t.Fatalf("old-style caller's Sequence/Index values must survive unchanged: %+v", oldStyleCarried)
	}
}

// TestParseHintsAreAdditive pins that a legacy parser (no hints) still
// satisfies the hint contract and that the positional-free literal it returns
// validates unchanged.
func TestParseHintsAreAdditive(t *testing.T) {
	got, ok := legacyFilenameParser{}.Parse("a", ParseProbe{})
	if !ok || got.Hints != nil || got.ProviderIDs != nil || got.HintSource != ParseHintSourceNone {
		t.Fatalf("legacy parse carried hints: %+v", got)
	}
	if err := ValidateParseHints(got); err != nil {
		t.Fatalf("legacy parse failed the hint contract: %v", err)
	}
}

func TestValidateNamedParseHints(t *testing.T) {
	atLimit := make(map[string]string)
	for i := range 15 {
		atLimit["key_"+strconv.Itoa(i)] = ""
	}
	atLimit[strings.Repeat("x", 32)] = strings.Repeat("é", 128)
	if err := ValidateParseHints(ParsedMedia{Hints: atLimit}); err != nil {
		t.Fatalf("ValidateParseHints rejected hints at the limits: %v", err)
	}
	atLimit["_"] = "extra"

	for _, tc := range []struct {
		name  string
		hints map[string]string
		want  string
	}{
		{"too many", atLimit, "at most 16 entries"},
		{"too many bytes", map[string]string{"key": strings.Repeat("é", 128) + "a"}, "at most 256 bytes"},
		{"empty key", map[string]string{"": "value"}, "must match"},
		{"uppercase key", map[string]string{"Key": "value"}, "must match"},
		{"unicode key", map[string]string{"clé": "value"}, "must match"},
		{"hyphenated key", map[string]string{"key-name": "value"}, "must match"},
		{"long key", map[string]string{strings.Repeat("x", 33): "value"}, "must match"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateParseHints(ParsedMedia{Hints: tc.hints})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateParseHints error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestProviderIDVocabulary(t *testing.T) {
	for _, key := range []string{"tmdb", "imdb", "tvdb", "anidb", "musicbrainz-release", "musicbrainz-artist"} {
		if !ValidProviderIDKey(key) {
			t.Errorf("ValidProviderIDKey(%q) = false, want true", key)
		}
	}
	for _, key := range []string{"", "TMDB", "musicbrainz", "musicbrainz-", "musicbrainz-<entity>", "musicbrainz-Release", "tmdb-id"} {
		if ValidProviderIDKey(key) {
			t.Errorf("ValidProviderIDKey(%q) = true, want false", key)
		}
	}
	if got := ProviderIDVocabulary(); len(got) != 5 || got[4] != "musicbrainz-<entity>" {
		t.Errorf("ProviderIDVocabulary() = %v", got)
	}
}

func TestParseHintSources(t *testing.T) {
	for _, s := range []ParseHintSource{ParseHintSourceFilename, ParseHintSourceNFO, ParseHintSourceSidecar} {
		if !ValidParseHintSource(s) {
			t.Errorf("ValidParseHintSource(%q) = false", s)
		}
	}
	for _, s := range []ParseHintSource{ParseHintSourceNone, "NFO", "folder"} {
		if ValidParseHintSource(s) {
			t.Errorf("ValidParseHintSource(%q) = true", s)
		}
	}
}

func TestValidateParseHints(t *testing.T) {
	ok := ParsedMedia{Title: "a", ProviderIDs: map[string]string{"tmdb": "123", "musicbrainz-release": "r1"}, HintSource: ParseHintSourceNFO}
	if err := ValidateParseHints(ok); err != nil {
		t.Fatalf("valid hint rejected: %v", err)
	}
	cases := map[string]ParsedMedia{
		"source without ids":  {Title: "a", HintSource: ParseHintSourceNFO},
		"ids without source":  {Title: "a", ProviderIDs: map[string]string{"tmdb": "123"}},
		"unknown source":      {Title: "a", ProviderIDs: map[string]string{"tmdb": "123"}, HintSource: "folder"},
		"unknown provider":    {Title: "a", ProviderIDs: map[string]string{"plex": "123"}, HintSource: ParseHintSourceFilename},
		"blank id":            {Title: "a", ProviderIDs: map[string]string{"tmdb": " "}, HintSource: ParseHintSourceFilename},
		"template key copied": {Title: "a", ProviderIDs: map[string]string{"musicbrainz-<entity>": "x"}, HintSource: ParseHintSourceSidecar},
	}
	for name, m := range cases {
		if err := ValidateParseHints(m); err == nil {
			t.Errorf("%s: ValidateParseHints accepted %+v", name, m)
		}
	}
}

func TestValidVersionLabel(t *testing.T) {
	for _, label := range []string{"", "1080p", "Extended", "Director's Cut", "4K HDR", "Cut \uFFFD"} {
		if !ValidVersionLabel(label) {
			t.Errorf("ValidVersionLabel(%q) = false, want true", label)
		}
		if err := ValidateVersionLabel(ParsedMedia{Title: "a", VersionLabel: label}); err != nil {
			t.Errorf("ValidateVersionLabel(%q): %v", label, err)
		}
	}
	for _, label := range []string{" 1080p", "1080p ", " ", "a\tb", "a\x00b", "a\x7fb", "1080p\u0085Extended", "a\u200bb", "a\u2028b", "a\u2029b", "\ufeff1080p", "a\u00a0b", "\xff", strings.Repeat("x", MaxVersionLabelLen+1)} {
		if ValidVersionLabel(label) {
			t.Errorf("ValidVersionLabel(%q) = true, want false", label)
		}
		if err := ValidateVersionLabel(ParsedMedia{Title: "a", VersionLabel: label}); err == nil {
			t.Errorf("ValidateVersionLabel(%q) accepted an invalid label", label)
		}
	}
}
