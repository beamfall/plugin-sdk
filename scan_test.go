package sdk

import "testing"

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
	if !ok || got.ProviderIDs != nil || got.HintSource != ParseHintSourceNone {
		t.Fatalf("legacy parse carried hints: %+v", got)
	}
	if err := ValidateParseHints(got); err != nil {
		t.Fatalf("legacy parse failed the hint contract: %v", err)
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
