package sdk

import (
	"encoding/json"
	"maps"
	"testing"
)

// TestL: the shorthand constructor maps a non-empty value to {"en": value},
// and the empty string to the absent (nil) map.
func TestL(t *testing.T) {
	if got := L("Hello"); len(got) != 1 || got["en"] != "Hello" {
		t.Fatalf("L(%q) = %+v", "Hello", got)
	}
	if got := L(""); got != nil {
		t.Fatalf("L(\"\") = %+v, want nil", got)
	}
}

// TestResolveExactAndTruncatedTag: an exact tag match wins; a missing exact
// tag falls back through progressively truncated tags (fr-ca -> fr).
func TestResolveExactAndTruncatedTag(t *testing.T) {
	s := LocaleString{"en": "Hello", "fr": "Bonjour", "fr-ca": "Allo"}
	if got := s.Resolve("fr-ca"); got != "Allo" {
		t.Errorf("exact tag: got %q, want %q", got, "Allo")
	}
	if got := s.Resolve("fr-be"); got != "Bonjour" {
		t.Errorf("truncated fallback: got %q, want %q", got, "Bonjour")
	}
}

// TestResolveNormalizesCaseAndUnderscore: the locale argument is
// case-folded and underscores are treated as hyphens.
func TestResolveNormalizesCaseAndUnderscore(t *testing.T) {
	s := LocaleString{"fr-ca": "Allo"}
	if got := s.Resolve("FR_CA"); got != "Allo" {
		t.Errorf("got %q, want %q", got, "Allo")
	}
}

// TestResolveEnglishFallback: an unmatched locale (including the empty
// locale, which means English) falls back to "en".
func TestResolveEnglishFallback(t *testing.T) {
	s := LocaleString{"en": "Hello", "fr": "Bonjour"}
	if got := s.Resolve("de"); got != "Hello" {
		t.Errorf("unmatched locale: got %q, want %q", got, "Hello")
	}
	if got := s.Resolve(""); got != "Hello" {
		t.Errorf("empty locale: got %q, want %q", got, "Hello")
	}
}

// TestResolveLexSmallestFallback: a map with no "en" and no matching tag
// resolves deterministically to the lexicographically smallest tag's value
// (locale.go:50-55).
func TestResolveLexSmallestFallback(t *testing.T) {
	s := LocaleString{"fr": "Bonjour", "de": "Hallo", "es": "Hola"}
	if got := s.Resolve("ja"); got != "Hallo" {
		t.Errorf("lex-smallest fallback: got %q, want %q (de)", got, "Hallo")
	}
}

// TestResolveEmptyMap: a nil or empty map always resolves to "".
func TestResolveEmptyMap(t *testing.T) {
	var nilMap LocaleString
	if got := nilMap.Resolve("en"); got != "" {
		t.Errorf("nil map: got %q, want \"\"", got)
	}
	if got := (LocaleString{}).Resolve("en"); got != "" {
		t.Errorf("empty map: got %q, want \"\"", got)
	}
}

// TestMarshalJSONShorthand: an absent map marshals to "", an English-only
// map marshals to the plain string, and a multi-locale map marshals to the
// full object.
func TestMarshalJSONShorthand(t *testing.T) {
	cases := []struct {
		name string
		in   LocaleString
		want string
	}{
		{"nil", nil, `""`},
		{"empty", LocaleString{}, `""`},
		{"english-only", LocaleString{"en": "Hello"}, `"Hello"`},
		{"multi", LocaleString{"en": "Hello", "fr": "Bonjour"}, `{"en":"Hello","fr":"Bonjour"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := json.Marshal(c.in)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(got) != c.want {
				t.Errorf("got %s, want %s", got, c.want)
			}
		})
	}
}

// TestMarshalJSONSingleNonEnglish: a map with exactly one entry that is not
// "en" still marshals as the full object (the shorthand only applies to
// English-only maps).
func TestMarshalJSONSingleNonEnglish(t *testing.T) {
	got, err := json.Marshal(LocaleString{"fr": "Bonjour"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(got) != `{"fr":"Bonjour"}` {
		t.Errorf("got %s, want %s", got, `{"fr":"Bonjour"}`)
	}
}

// TestUnmarshalJSONRoundTrips covers both wire forms the contract accepts:
// the plain-string shorthand and the full locale-map object.
func TestUnmarshalJSONRoundTrips(t *testing.T) {
	var shorthand LocaleString
	if err := json.Unmarshal([]byte(`"Hello"`), &shorthand); err != nil {
		t.Fatalf("Unmarshal shorthand: %v", err)
	}
	if len(shorthand) != 1 || shorthand["en"] != "Hello" {
		t.Errorf("shorthand = %+v", shorthand)
	}

	var full LocaleString
	if err := json.Unmarshal([]byte(`{"en":"Hello","fr":"Bonjour"}`), &full); err != nil {
		t.Fatalf("Unmarshal full: %v", err)
	}
	if len(full) != 2 || full["en"] != "Hello" || full["fr"] != "Bonjour" {
		t.Errorf("full = %+v", full)
	}

	var empty LocaleString
	if err := json.Unmarshal([]byte(`""`), &empty); err != nil {
		t.Fatalf("Unmarshal empty shorthand: %v", err)
	}
	if empty != nil {
		t.Errorf("empty shorthand = %+v, want nil", empty)
	}
}

// TestUnmarshalJSONRejectsInvalidShape: a value that is neither a JSON
// string nor a JSON object of strings is a decode error.
func TestUnmarshalJSONRejectsInvalidShape(t *testing.T) {
	var s LocaleString
	if err := json.Unmarshal([]byte(`42`), &s); err == nil {
		t.Fatal("want an error unmarshaling a number, got nil")
	}
	if err := json.Unmarshal([]byte(`{"en": 1}`), &s); err == nil {
		t.Fatal("want an error unmarshaling a non-string-valued map, got nil")
	}
}

func FuzzLocaleStringJSON(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`"Hello"`),
		[]byte(`{"en":"Hello","fr":"Bonjour"}`),
		[]byte(`{"en":1}`),
		[]byte("not json"),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		var decoded LocaleString
		if err := json.Unmarshal(data, &decoded); err != nil {
			return
		}
		raw, err := json.Marshal(decoded)
		if err != nil {
			t.Fatal(err)
		}
		var roundTrip LocaleString
		if err := json.Unmarshal(raw, &roundTrip); err != nil {
			t.Fatal(err)
		}
		if len(decoded) == 1 && decoded["en"] == "" && roundTrip == nil {
			return
		}
		if !maps.Equal(roundTrip, decoded) {
			t.Fatalf("round trip = %#v, want %#v", roundTrip, decoded)
		}
	})
}
