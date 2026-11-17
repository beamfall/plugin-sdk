package sdk

import (
	"strings"
	"testing"
)

func subtitleBundle(segments ...LocaleString) Bundle {
	b := validBundle()
	b.MediaFields = []FieldSpec{
		{Name: "part-title", Type: "text", Label: L("Part title")},
		{Name: "rating", Type: "number", Label: L("Rating")},
		{Name: "released", Type: "date", Label: L("Released")},
	}
	b.MediaSubtitle = segments
	return b
}

func TestValidateBundleAcceptsMediaSubtitle(t *testing.T) {
	b := subtitleBundle(L("S{sequence}"), LocaleString{"en": "E{index}", "fr": "É{index}"}, L("{index}-{index-end}"), L("{part-title}"), L("{rating}"))
	if err := ValidateBundle(b); err != nil {
		t.Fatalf("ValidateBundle: %v", err)
	}
}

func TestValidateBundleRejectsBadMediaSubtitle(t *testing.T) {
	cases := map[string]struct {
		segment LocaleString
		want    string
	}{
		"undeclared field": {L("{missing}"), "undeclared media field"},
		"date field":       {L("{released}"), "only text and number"},
		"no placeholder":   {L("Part"), "names no placeholder"},
		"unbalanced":       {L("{index} }"), "unbalanced brace"},
		"no english":       {LocaleString{"fr": "{index}"}, "English fallback"},
		"bad translation":  {LocaleString{"en": "{index}", "fr": "{nope}"}, "undeclared media field"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidateBundle(subtitleBundle(tc.segment))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateBundle error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestRenderMediaSubtitle(t *testing.T) {
	segments := []LocaleString{L("S{sequence}"), LocaleString{"en": "E{index}", "fr": "É{index}"}, L("{part-title}")}
	values := map[string]string{"sequence": "5", "index": "3", "part-title": " The Reunion "}
	lookup := func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
	if got := RenderMediaSubtitle(segments, "en", lookup); got != "S5 · E3 · The Reunion" {
		t.Fatalf("en render = %q", got)
	}
	if got := RenderMediaSubtitle(segments, "fr-ca", lookup); got != "S5 · É3 · The Reunion" {
		t.Fatalf("fr-ca render = %q", got)
	}
	delete(values, "part-title")
	values["sequence"] = ""
	if got := RenderMediaSubtitle(segments, "en", lookup); got != "E3" {
		t.Fatalf("partial render = %q, want incomplete segments dropped", got)
	}
	if got := RenderMediaSubtitle(segments, "en", func(string) (string, bool) { return "", false }); got != "" {
		t.Fatalf("empty render = %q", got)
	}
}

func TestMediaSubtitleLocalesAndFields(t *testing.T) {
	segments := []LocaleString{L("S{sequence}"), LocaleString{"en": "E{index}", "fr": "É{index}"}, L("{part-title}")}
	if got := strings.Join(MediaSubtitleLocales(segments), ","); got != "en,fr" {
		t.Fatalf("locales = %q", got)
	}
	if got := strings.Join(MediaSubtitleFields(segments), ","); got != "index,part-title,sequence" {
		t.Fatalf("fields = %q", got)
	}
	if got := MediaSubtitleLocales(nil); len(got) != 0 {
		t.Fatalf("nil locales = %v", got)
	}
}
