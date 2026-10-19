package sdk

import (
	"slices"
	"testing"
)

// TestLocalFileScraperKind: only "exif" is a local-file scraper kind.
func TestLocalFileScraperKind(t *testing.T) {
	if !LocalFileScraperKind("exif") {
		t.Error(`LocalFileScraperKind("exif") = false, want true`)
	}
	if LocalFileScraperKind("http-json") {
		t.Error(`LocalFileScraperKind("http-json") = true, want false`)
	}
}

// TestValidateAuthStyle covers the empty default, the two bare forms, the
// two parameterized forms (with and without a name), and an invalid style.
func TestValidateAuthStyle(t *testing.T) {
	valid := []string{"", "bearer", "login-bearer", "header:X-Api-Key", "query:api_key"}
	for _, s := range valid {
		if !ValidateAuthStyle(s) {
			t.Errorf("ValidateAuthStyle(%q) = false, want true", s)
		}
	}
	invalid := []string{"basic", "header:", "query:", "Bearer", "header:<name>", "query:<param>"}
	for _, s := range invalid {
		if ValidateAuthStyle(s) {
			t.Errorf("ValidateAuthStyle(%q) = true, want false", s)
		}
	}
}

// TestValidateAuthStyleSignatureForm: the "signature:<name>" form validates
// like the other parameterized forms — a non-empty name without the template
// sentinel — while the archetype-vocabulary check stays ValidateBundle's job.
func TestValidateAuthStyleSignatureForm(t *testing.T) {
	valid := []string{"signature:podcast-index-sha1", "signature:custom"}
	for _, s := range valid {
		if !ValidateAuthStyle(s) {
			t.Errorf("ValidateAuthStyle(%q) = false, want true", s)
		}
	}
	invalid := []string{"signature:", "signature:<name>"}
	for _, s := range invalid {
		if ValidateAuthStyle(s) {
			t.Errorf("ValidateAuthStyle(%q) = true, want false", s)
		}
	}
}

// TestScraperVocabularies pins the exported scraper vocabularies to their
// documented contents — additions are deliberate contract changes.
func TestScraperVocabularies(t *testing.T) {
	if got, want := ScraperKinds(), []string{"http-json", "http-graphql", "rss", "exif"}; !slices.Equal(got, want) {
		t.Errorf("ScraperKinds() = %v, want %v", got, want)
	}
	if got, want := AuthStyleForms(), []string{"bearer", "header:<name>", "query:<param>", "login-bearer", "signature:<name>"}; !slices.Equal(got, want) {
		t.Errorf("AuthStyleForms() = %v, want %v", got, want)
	}
	if got, want := SignatureArchetypes(), []string{"podcast-index-sha1"}; !slices.Equal(got, want) {
		t.Errorf("SignatureArchetypes() = %v, want %v", got, want)
	}
	if got, want := TransformKinds(), []string{"year-from-date", "date-from-rfc822", "seconds-from-clock"}; !slices.Equal(got, want) {
		t.Errorf("TransformKinds() = %v, want %v", got, want)
	}
}

// TestValidateBundleScraperIdentityAndTransport: scraper ID slug/duplicate
// checks, the kind vocabulary, the https endpoint requirement, and the
// at-least-one-template rule.
func TestValidateBundleScraperIdentityAndTransport(t *testing.T) {
	badSlug := scraperBundle()
	badSlug.Scrapers[0].ID = "Not-Slug"
	wantBundleError(t, badSlug, `scraper ID "Not-Slug" must be a lowercase slug`)

	duplicate := scraperBundle()
	duplicate.Scrapers = append(duplicate.Scrapers, duplicate.Scrapers[0])
	wantBundleError(t, duplicate, `duplicate scraper ID "example"`)

	unknownKind := scraperBundle()
	unknownKind.Scrapers[0].Kind = "ftp"
	wantBundleError(t, unknownKind, `scraper "example" uses unknown kind "ftp"`)

	insecure := scraperBundle()
	insecure.Scrapers[0].Endpoint = "http://example.test"
	wantBundleError(t, insecure, `scraper "example" endpoint must be https`)

	noTemplates := scraperBundle()
	noTemplates.Scrapers[0].LookupTemplate = ""
	wantBundleError(t, noTemplates, "declares neither a lookup nor a search template")

	searchMapsOnly := scraperBundle()
	searchMapsOnly.Scrapers[0].SearchFieldMap = map[string]string{"title": "name"}
	wantBundleError(t, searchMapsOnly, "declares search maps without a search template")

	badFieldMapKey := scraperBundle()
	badFieldMapKey.Scrapers[0].FieldMap = map[string]string{"bogus": "x"}
	wantBundleError(t, badFieldMapKey, `field key "bogus" is not a built-in`)
}

// TestValidateBundleScraperAuthStyles: valid and invalid cases per auth style
// — the unknown-style reject, the style-without-AuthRef reject, the
// login-bearer/Login mutual requirement, and the LoginSpec shape.
func TestValidateBundleScraperAuthStyles(t *testing.T) {
	for _, style := range []string{"bearer", "header:X-Api-Key", "query:api_key"} {
		b := scraperBundle()
		b.Scrapers[0].AuthStyle = style
		b.Scrapers[0].AuthRef = "example-key"
		if err := ValidateBundle(b); err != nil {
			t.Fatalf("ValidateBundle rejected auth style %q: %v", style, err)
		}
	}

	unknown := scraperBundle()
	unknown.Scrapers[0].AuthStyle = "basic"
	unknown.Scrapers[0].AuthRef = "example-key"
	wantBundleError(t, unknown, `uses unknown auth style "basic"`)

	noRef := scraperBundle()
	noRef.Scrapers[0].AuthStyle = "bearer"
	wantBundleError(t, noRef, `declares auth style "bearer" without an AuthRef`)

	loginBearerNoLogin := scraperBundle()
	loginBearerNoLogin.Scrapers[0].AuthStyle = "login-bearer"
	loginBearerNoLogin.Scrapers[0].AuthRef = "example-key"
	wantBundleError(t, loginBearerNoLogin, "the login-bearer style and the Login declaration require each other")

	loginWithoutStyle := scraperBundle()
	loginWithoutStyle.Scrapers[0].Login = &LoginSpec{Path: "/login", SecretField: "apikey", TokenPath: "data.token"}
	wantBundleError(t, loginWithoutStyle, "the login-bearer style and the Login declaration require each other")

	badLogin := scraperBundle()
	badLogin.Scrapers[0].AuthStyle = "login-bearer"
	badLogin.Scrapers[0].AuthRef = "example-key"
	badLogin.Scrapers[0].Login = &LoginSpec{Path: "login", SecretField: "apikey", TokenPath: "data.token"}
	wantBundleError(t, badLogin, "login declaration needs a /-rooted path, a secret field, and a token path")

	validLogin := scraperBundle()
	validLogin.Scrapers[0].AuthStyle = "login-bearer"
	validLogin.Scrapers[0].AuthRef = "example-key"
	validLogin.Scrapers[0].Login = &LoginSpec{Path: "/login", SecretField: "apikey", TokenPath: "data.token"}
	if err := ValidateBundle(validLogin); err != nil {
		t.Fatalf("ValidateBundle rejected a valid login-bearer scraper: %v", err)
	}
}

// TestValidateBundleScraperSignature: the "signature:<name>" style — mutual
// requirement with the Signature declaration, archetype vocabulary, the
// archetype/style name match, the header-name requirement, and the valid
// Podcast Index shape.
func TestValidateBundleScraperSignature(t *testing.T) {
	valid := scraperBundle()
	valid.Scrapers[0].AuthStyle = "signature:podcast-index-sha1"
	valid.Scrapers[0].AuthRef = "podcast-index"
	valid.Scrapers[0].Signature = &SignatureSpec{Archetype: "podcast-index-sha1", HeaderName: "Authorization"}
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected a valid signature scraper: %v", err)
	}

	styleWithoutSpec := scraperBundle()
	styleWithoutSpec.Scrapers[0].AuthStyle = "signature:podcast-index-sha1"
	styleWithoutSpec.Scrapers[0].AuthRef = "podcast-index"
	wantBundleError(t, styleWithoutSpec, "the signature:<name> style and the Signature declaration require each other")

	specWithoutStyle := scraperBundle()
	specWithoutStyle.Scrapers[0].Signature = &SignatureSpec{Archetype: "podcast-index-sha1", HeaderName: "Authorization"}
	wantBundleError(t, specWithoutStyle, "the signature:<name> style and the Signature declaration require each other")

	unknownArchetype := scraperBundle()
	unknownArchetype.Scrapers[0].AuthStyle = "signature:nope"
	unknownArchetype.Scrapers[0].AuthRef = "podcast-index"
	unknownArchetype.Scrapers[0].Signature = &SignatureSpec{Archetype: "nope", HeaderName: "Authorization"}
	wantBundleError(t, unknownArchetype, `uses unknown signature archetype "nope"`)

	mismatch := scraperBundle()
	mismatch.Scrapers[0].AuthStyle = "signature:other"
	mismatch.Scrapers[0].AuthRef = "podcast-index"
	mismatch.Scrapers[0].Signature = &SignatureSpec{Archetype: "podcast-index-sha1", HeaderName: "Authorization"}
	wantBundleError(t, mismatch, `signature archetype "podcast-index-sha1" does not match auth style "signature:other"`)

	noHeader := scraperBundle()
	noHeader.Scrapers[0].AuthStyle = "signature:podcast-index-sha1"
	noHeader.Scrapers[0].AuthRef = "podcast-index"
	noHeader.Scrapers[0].Signature = &SignatureSpec{Archetype: "podcast-index-sha1"}
	wantBundleError(t, noHeader, "signature declaration needs a header name")
}

// exifScraperBundle swaps scraperBundle's http-json scraper for the minimal
// valid local-file ("exif") scraper the local-file restriction tests mutate.
func exifScraperBundle() Bundle {
	b := scraperBundle()
	b.Scrapers = []ScraperSpec{{ID: "exif", Kind: "exif"}}
	return b
}

// TestValidateBundleLocalFileScraper: the exif kind carries none of the
// network-scraper machinery — endpoint, templates, auth, candidate paths,
// field maps, and entity maps are each rejected; the bare declaration passes.
func TestValidateBundleLocalFileScraper(t *testing.T) {
	if err := ValidateBundle(exifScraperBundle()); err != nil {
		t.Fatalf("ValidateBundle rejected a minimal exif scraper: %v", err)
	}

	cases := []struct {
		name     string
		mutate   func(s *ScraperSpec)
		fragment string
	}{
		{
			"endpoint",
			func(s *ScraperSpec) { s.Endpoint = "https://example.test" },
			`is a local-file kind "exif" and must not declare an endpoint`,
		},
		{
			"url templates",
			func(s *ScraperSpec) { s.LookupTemplate = "/lookup/{id}" },
			`is a local-file kind "exif" and must not declare URL templates`,
		},
		{
			"auth",
			func(s *ScraperSpec) { s.AuthRef = "example-key" },
			`is a local-file kind "exif" and must not declare auth`,
		},
		{
			"candidate paths",
			func(s *ScraperSpec) { s.CandidatesPath = "results" },
			`is a local-file kind "exif" and must not declare candidate paths`,
		},
		{
			"field maps",
			func(s *ScraperSpec) { s.FieldMap = map[string]string{"title": "name"} },
			"it emits a fixed generic field set, not document-path field maps",
		},
		{
			"entity maps",
			func(s *ScraperSpec) {
				s.EntityMaps = []EntityMap{{Kind: "genre", Relation: "genres", Path: "genres"}}
			},
			`is a local-file kind "exif" and must not declare entity maps`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := exifScraperBundle()
			tc.mutate(&b.Scrapers[0])
			wantBundleError(t, b, tc.fragment)
		})
	}
}

// TestValidateBundleLocalFileScraperImpliedMediaFields pins the closed gap
// where the local-file `continue` skipped ImpliedMediaFields validation
// entirely: local-file kinds DO declare implied fields (the photos plugin's
// {"media.class": ...}), so their keys must route to declared plugin text
// media fields like every other scraper's.
func TestValidateBundleLocalFileScraperImpliedMediaFields(t *testing.T) {
	undeclared := exifScraperBundle()
	undeclared.Scrapers[0].ImpliedMediaFields = map[string]string{"media.class": "photo"}
	wantBundleError(t, undeclared, `names media field "class", which the bundle does not declare`)

	builtin := exifScraperBundle()
	builtin.Scrapers[0].ImpliedMediaFields = map[string]string{"title": "photo"}
	wantBundleError(t, builtin, `implies built-in field "title"`)

	valid := exifScraperBundle()
	valid.MediaFields = []FieldSpec{{Name: "class", Type: "text", Label: L("Class")}}
	valid.Scrapers[0].ImpliedMediaFields = map[string]string{"media.class": "photo"}
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected a declared implied media field on a local-file scraper: %v", err)
	}
}

// TestValidateBundleLookupByIDTemplate: a bundle declaring LookupByIDTemplate
// validates, one omitting it still validates (the field is optional and
// additive), and the standalone shape check rejects malformed declarations.
func TestValidateBundleLookupByIDTemplate(t *testing.T) {
	omitted := scraperBundle()
	if err := ValidateBundle(omitted); err != nil {
		t.Fatalf("bundle without LookupByIDTemplate rejected: %v", err)
	}
	if err := ValidateLookupByIDTemplates(omitted.Scrapers[0]); err != nil {
		t.Fatalf("scraper without LookupByIDTemplate rejected: %v", err)
	}

	declared := scraperBundle()
	declared.Scrapers[0].LookupByIDTemplate = map[string]string{
		"tmdb": "/movie/{id}",
		"imdb": "/find/{id}?external_source=imdb_id",
	}
	if err := ValidateBundle(declared); err != nil {
		t.Fatalf("bundle with LookupByIDTemplate rejected: %v", err)
	}
	if err := ValidateLookupByIDTemplates(declared.Scrapers[0]); err != nil {
		t.Fatalf("well-formed LookupByIDTemplate rejected: %v", err)
	}

	bad := map[string]map[string]string{
		"unknown provider":    {"plex": "/x/{id}"},
		"missing id":          {"tmdb": "/movie/"},
		"blank template":      {"tmdb": "  "},
		"vocabulary sentinel": {"musicbrainz-<entity>": "/r/{id}"},
	}
	for name, tmpl := range bad {
		spec := scraperBundle().Scrapers[0]
		spec.LookupByIDTemplate = tmpl
		if err := ValidateLookupByIDTemplates(spec); err == nil {
			t.Errorf("%s: ValidateLookupByIDTemplates accepted %v", name, tmpl)
		}
	}

	local := exifScraperBundle().Scrapers[0]
	local.LookupByIDTemplate = map[string]string{"tmdb": "/movie/{id}"}
	if err := ValidateLookupByIDTemplates(local); err == nil {
		t.Error("local-file scraper with LookupByIDTemplate accepted")
	}
}
