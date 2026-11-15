package sdk

import (
	"strings"
	"testing"
)

// pinHelpBundle is a login-bearer scraper whose AuthRef is a subscriber PIN
// presented with a label and a subscribe link (CORE-1127).
func pinHelpBundle() Bundle {
	b := scraperBundle()
	b.Scrapers[0].AuthStyle = "login-bearer"
	b.Scrapers[0].AuthRef = "example-pin"
	b.Scrapers[0].Login = &LoginSpec{
		Path: "/login", SecretField: "pin", TokenPath: "data.token",
		SecretFields: map[string]string{"apikey": "example-key"},
	}
	b.Scrapers[0].CredentialHelp = &CredentialHelp{
		Label:     L("Subscriber PIN"),
		HelpURL:   "https://example.test/subscribe",
		HelpLabel: L("Subscribe at example.test"),
	}
	return b
}

// TestValidateBundleCredentialHelp: credential help needs an AuthRef, an
// "en" label, and an https help link with its own label.
func TestValidateBundleCredentialHelp(t *testing.T) {
	if err := ValidateBundle(pinHelpBundle()); err != nil {
		t.Fatalf("ValidateBundle rejected credential help: %v", err)
	}
	noLink := pinHelpBundle()
	noLink.Scrapers[0].CredentialHelp = &CredentialHelp{Label: L("PIN")}
	if err := ValidateBundle(noLink); err != nil {
		t.Fatalf("ValidateBundle rejected credential help without a link: %v", err)
	}
	for name, mutate := range map[string]func(*ScraperSpec){
		"no auth ref": func(s *ScraperSpec) {
			s.AuthRef, s.AuthStyle, s.Login = "", "", nil
		},
		"no en label":            func(s *ScraperSpec) { s.CredentialHelp.Label = LocaleString{"de": "PIN"} },
		"http help":              func(s *ScraperSpec) { s.CredentialHelp.HelpURL = "http://example.test/x" },
		"relative help":          func(s *ScraperSpec) { s.CredentialHelp.HelpURL = "/subscribe" },
		"no help text":           func(s *ScraperSpec) { s.CredentialHelp.HelpLabel = nil },
		"help text without link": func(s *ScraperSpec) { s.CredentialHelp.HelpURL = "" },
	} {
		bad := pinHelpBundle()
		mutate(&bad.Scrapers[0])
		if err := ValidateBundle(bad); err == nil || !strings.Contains(err.Error(), "credential help") {
			t.Errorf("%s: got %v, want a credential help error", name, err)
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

// TestValidateBundleLookupByIDRecordIDPaths: an id-lookup record path turns
// one provider's LookupByIDTemplate into a resolver for the scraper's own
// record id, so it needs that template, a LookupTemplate and http-json.
func TestValidateBundleLookupByIDRecordIDPaths(t *testing.T) {
	valid := func() Bundle {
		b := scraperBundle()
		b.Scrapers[0].LookupTemplate = "/series/{id}/extended"
		b.Scrapers[0].LookupByIDTemplate = map[string]string{
			"tvdb": "/series/{id}/extended",
			"imdb": "/search/remoteid/{id}",
		}
		b.Scrapers[0].LookupByIDRecordIDPaths = map[string]string{"imdb": "data.0.series.id"}
		return b
	}
	if err := ValidateBundle(valid()); err != nil {
		t.Fatalf("ValidateBundle rejected an id-lookup record path: %v", err)
	}
	for name, mutate := range map[string]func(*ScraperSpec){
		"no lookup template": func(s *ScraperSpec) {
			s.LookupTemplate, s.SearchTemplate = "", "/search?query={query}"
		},
		"undeclared provider": func(s *ScraperSpec) { s.LookupByIDRecordIDPaths["tmdb"] = "data.0.series.id" },
		"empty path":          func(s *ScraperSpec) { s.LookupByIDRecordIDPaths["imdb"] = " " },
		"graphql kind":        func(s *ScraperSpec) { s.Kind = "http-graphql" },
	} {
		bad := valid()
		mutate(&bad.Scrapers[0])
		if err := ValidateBundle(bad); err == nil || !strings.Contains(err.Error(), "LookupByIDRecordIDPaths") {
			t.Errorf("%s: got %v, want a LookupByIDRecordIDPaths error", name, err)
		}
	}
}

// TestValidateBundleLocalFileScraperRejectsNetworkExtras: the local-file
// `continue` skips the network validators, so the exif kind rejects
// credential help, reference documents and an id-lookup record path outright.
func TestValidateBundleLocalFileScraperRejectsNetworkExtras(t *testing.T) {
	for name, mutate := range map[string]func(*ScraperSpec){
		"credential help": func(s *ScraperSpec) { s.CredentialHelp = &CredentialHelp{Label: L("PIN")} },
		"reference documents": func(s *ScraperSpec) {
			s.ReferenceDocuments = map[string]ReferenceDocument{"types": {Path: "/types"}}
		},
		"record id path": func(s *ScraperSpec) { s.LookupByIDRecordIDPaths = map[string]string{"imdb": "data.id"} },
	} {
		b := exifScraperBundle()
		mutate(&b.Scrapers[0])
		if err := ValidateBundle(b); err == nil || !strings.Contains(err.Error(), `is a local-file kind "exif" and must not declare credential help`) {
			t.Errorf("%s: got %v, want a local-file rejection", name, err)
		}
	}
}
