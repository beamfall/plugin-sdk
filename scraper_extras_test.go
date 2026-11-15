package sdk

import (
	"strings"
	"testing"
)

// optionalPINBundle is a login-bearer scraper whose key is the AuthRef and
// whose subscriber PIN is an optional login field (CORE-1127).
func optionalPINBundle() Bundle {
	b := scraperBundle()
	b.Scrapers[0].AuthStyle = "login-bearer"
	b.Scrapers[0].AuthRef = "example-key"
	b.Scrapers[0].Login = &LoginSpec{
		Path: "/login", SecretField: "apikey", TokenPath: "data.token",
		OptionalSecretFields: map[string]OptionalSecret{"pin": {
			AuthRef:   "example-pin",
			Label:     L("Subscriber PIN"),
			HelpURL:   "https://example.test/subscribe",
			HelpLabel: L("Subscribe at example.test"),
		}},
	}
	return b
}

// TestValidateBundleLoginOptionalSecretFields: an optional login secret needs
// a distinct body field, a plain AuthRef, an "en" label and an https help
// link with its own label.
func TestValidateBundleLoginOptionalSecretFields(t *testing.T) {
	if err := ValidateBundle(optionalPINBundle()); err != nil {
		t.Fatalf("ValidateBundle rejected an optional login secret: %v", err)
	}
	noHelp := optionalPINBundle()
	noHelp.Scrapers[0].Login.OptionalSecretFields = map[string]OptionalSecret{"pin": {AuthRef: "example-pin", Label: L("PIN")}}
	if err := ValidateBundle(noHelp); err != nil {
		t.Fatalf("ValidateBundle rejected an optional secret without a help link: %v", err)
	}
	for name, mutate := range map[string]func(*LoginSpec){
		"same as secret field": func(l *LoginSpec) { l.OptionalSecretFields["apikey"] = l.OptionalSecretFields["pin"] },
		"same as required field": func(l *LoginSpec) {
			l.SecretFields = map[string]string{"pin": "other-key"}
		},
		"empty field": func(l *LoginSpec) { l.OptionalSecretFields[""] = l.OptionalSecretFields["pin"] },
		"path ref": func(l *LoginSpec) {
			s := l.OptionalSecretFields["pin"]
			s.AuthRef = "../secret"
			l.OptionalSecretFields["pin"] = s
		},
		"empty ref": func(l *LoginSpec) {
			s := l.OptionalSecretFields["pin"]
			s.AuthRef = ""
			l.OptionalSecretFields["pin"] = s
		},
		"no en label": func(l *LoginSpec) {
			s := l.OptionalSecretFields["pin"]
			s.Label = LocaleString{"de": "PIN"}
			l.OptionalSecretFields["pin"] = s
		},
		"http help": func(l *LoginSpec) {
			s := l.OptionalSecretFields["pin"]
			s.HelpURL = "http://example.test/x"
			l.OptionalSecretFields["pin"] = s
		},
		"no help text": func(l *LoginSpec) {
			s := l.OptionalSecretFields["pin"]
			s.HelpLabel = nil
			l.OptionalSecretFields["pin"] = s
		},
		"help text without link": func(l *LoginSpec) {
			s := l.OptionalSecretFields["pin"]
			s.HelpURL = ""
			l.OptionalSecretFields["pin"] = s
		},
	} {
		bad := optionalPINBundle()
		mutate(bad.Scrapers[0].Login)
		if err := ValidateBundle(bad); err == nil || !strings.Contains(err.Error(), "optional secret field") {
			t.Errorf("%s: got %v, want an optional secret field error", name, err)
		}
	}
}

// TestValidateBundleReferenceDocuments: reference documents are http-json
// only, slug-named, /-rooted with config-only placeholders, cached for at
// most a year, and every {ref.<name>:<path>} placeholder names one.
func TestValidateBundleReferenceDocuments(t *testing.T) {
	valid := func() Bundle {
		b := scraperBundle()
		b.ConfigFields = []ConfigFieldSpec{{Name: "language", Type: "text", Label: L("Language")}}
		b.Scrapers[0].ReferenceDocuments = map[string]ReferenceDocument{
			"artwork-types": {Path: "/artwork/types"},
			"localized":     {Path: "/labels?lang={config.language}", CacheHours: 24},
		}
		b.Scrapers[0].ArtworkMap = map[string]string{
			"backdrop": "artworks[type={ref.artwork-types:data[slug=background,recordType=series].id}].image",
		}
		return b
	}
	if err := ValidateBundle(valid()); err != nil {
		t.Fatalf("ValidateBundle rejected reference documents: %v", err)
	}
	for name, mutate := range map[string]func(*ScraperSpec){
		"bad name":      func(s *ScraperSpec) { s.ReferenceDocuments["Artwork_Types"] = ReferenceDocument{Path: "/x"} },
		"relative path": func(s *ScraperSpec) { s.ReferenceDocuments["artwork-types"] = ReferenceDocument{Path: "artwork/types"} },
		"id placeholder": func(s *ScraperSpec) {
			s.ReferenceDocuments["artwork-types"] = ReferenceDocument{Path: "/artwork/{id}"}
		},
		"undeclared config": func(s *ScraperSpec) {
			s.ReferenceDocuments["artwork-types"] = ReferenceDocument{Path: "/artwork?lang={config.region}"}
		},
		"negative cache": func(s *ScraperSpec) {
			s.ReferenceDocuments["artwork-types"] = ReferenceDocument{Path: "/artwork/types", CacheHours: -1}
		},
		"cache over a year": func(s *ScraperSpec) {
			s.ReferenceDocuments["artwork-types"] = ReferenceDocument{Path: "/artwork/types", CacheHours: 8761}
		},
		"undeclared document": func(s *ScraperSpec) { s.FieldMap["title"] = "data[id={ref.genres:data.0.id}].name" },
		"graphql kind":        func(s *ScraperSpec) { s.Kind = "http-graphql" },
	} {
		bad := valid()
		mutate(&bad.Scrapers[0])
		if err := ValidateBundle(bad); err == nil || !strings.Contains(err.Error(), "reference document") {
			t.Errorf("%s: got %v, want a reference document error", name, err)
		}
	}
}
