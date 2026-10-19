package sdktest

import (
	"strings"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
)

// tmdbLikeSpec is an http-json scraper exercising search+lookup, query-parameter
// auth, pagination, a year-from-date transform, and site-relative artwork — the
// happy fixture the reference-trace tests build from.
func tmdbLikeSpec() sdk.ScraperSpec {
	return sdk.ScraperSpec{
		ID:              "example-tmdb",
		Kind:            "http-json",
		Endpoint:        "https://api.example.test",
		AuthRef:         "tmdb",
		AuthStyle:       "query:api_key",
		SearchTemplate:  "/search?query={query}&page={page}",
		MaxPages:        2,
		CandidatesPath:  "results",
		CandidateIDPath: "id",
		LookupTemplate:  "/movie/{id}",
		FieldMap:        map[string]string{"title": "title", "year": "release_date"},
		Transforms:      map[string]string{"year": "year-from-date"},
		ArtworkMap:      map[string]string{"poster": "poster_path"},
		ArtworkBaseURL:  "https://img.example.test",
	}
}

func tmdbLikeInput() TraceInput {
	return TraceInput{
		Query:     "dune",
		ID:        "438631",
		LookupDoc: map[string]any{"title": "Dune", "release_date": "2021-10-22", "poster_path": "/dune.jpg"},
		SearchDoc: map[string]any{"results": []any{
			map[string]any{"id": float64(438631), "title": "Dune", "release_date": "2021-10-22", "poster_path": "/dune.jpg"},
		}},
	}
}

func tmdbLikeWant() ScraperTrace {
	auth := AuthPlacement{Location: "query", Name: "api_key", InURL: true}
	fields := map[string]string{"title": "Dune", "year": "2021"}
	artwork := map[string]string{"poster": "https://img.example.test/dune.jpg"}
	return ScraperTrace{
		Requests: []TraceRequest{
			{Role: "search", Page: 1, Method: "GET", URL: "https://api.example.test/search?query=dune&page=1", Auth: auth},
			{Role: "search", Page: 2, Method: "GET", URL: "https://api.example.test/search?query=dune&page=2", Auth: auth},
			{Role: "lookup", Method: "GET", URL: "https://api.example.test/movie/438631", Auth: auth},
		},
		Fields:     fields,
		Artwork:    artwork,
		Candidates: []TraceCandidate{{ID: "438631", Fields: fields, Artwork: artwork}},
	}
}

// TestScraperTraceHappyPath: the reference compiler reproduces the frozen trace
// for a realistic search+lookup spec, and RunScraperTraceConformance is green.
func TestScraperTraceHappyPath(t *testing.T) {
	got := CompileScraperTrace(tmdbLikeSpec(), tmdbLikeInput())
	if reasons := scraperTraceViolations(tmdbLikeWant(), got); len(reasons) != 0 {
		t.Fatalf("happy trace diverged:\n%s", strings.Join(reasons, "\n"))
	}
	RunScraperTraceConformance(t, []ScraperTraceVector{
		{Name: "tmdb-like", Spec: tmdbLikeSpec(), Input: tmdbLikeInput(), Want: tmdbLikeWant()},
	})
}

// TestScraperTraceCompilerIsDeterministic: compiling twice yields identical
// traces — the reference oracle must be stable input → output.
func TestScraperTraceCompilerIsDeterministic(t *testing.T) {
	first := CompileScraperTrace(tmdbLikeSpec(), tmdbLikeInput())
	if reasons := scraperTraceViolations(first, CompileScraperTrace(tmdbLikeSpec(), tmdbLikeInput())); len(reasons) != 0 {
		t.Fatalf("compiler not deterministic:\n%s", strings.Join(reasons, "\n"))
	}
}

// TestScraperTraceGraphQLPostsBearer: an http-graphql spec posts its substituted
// query as the body against the bare Endpoint and rides a bearer header — the
// implicit-method and auth-placement dimensions for the POST/bearer shape.
func TestScraperTraceGraphQLPostsBearer(t *testing.T) {
	spec := sdk.ScraperSpec{
		ID: "gql", Kind: "http-graphql", Endpoint: "https://gql.example.test",
		AuthRef: "stashdb", AuthStyle: "bearer",
		LookupTemplate: "{scene(id:\"{id}\"){title}}",
	}
	got := CompileScraperTrace(spec, TraceInput{ID: "s-1"})
	if len(got.Requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(got.Requests))
	}
	req := got.Requests[0]
	if req.Method != "POST" || req.URL != "https://gql.example.test" {
		t.Errorf("method/url = %s %q, want POST bare endpoint", req.Method, req.URL)
	}
	if req.Body != "{scene(id:\"s-1\"){title}}" {
		t.Errorf("body = %q, want substituted query as POST body", req.Body)
	}
	want := AuthPlacement{Location: "header", Name: "Authorization", Scheme: "Bearer"}
	if req.Auth != want {
		t.Errorf("auth = %+v, want %+v", req.Auth, want)
	}
}

// TestScraperTraceLoginBearerPrependsExchange: a login-bearer spec compiles the
// token-exchange POST before the lookup, and the lookup then rides a bearer header.
func TestScraperTraceLoginBearerPrependsExchange(t *testing.T) {
	spec := sdk.ScraperSpec{
		ID: "tvdb", Kind: "http-json", Endpoint: "https://tvdb.example.test",
		AuthRef: "tvdb", AuthStyle: "login-bearer",
		Login:          &sdk.LoginSpec{Path: "/login", SecretField: "apikey", TokenPath: "data.token"},
		LookupTemplate: "/series/{id}",
	}
	got := CompileScraperTrace(spec, TraceInput{ID: "42"})
	if len(got.Requests) != 2 {
		t.Fatalf("got %d requests, want login+lookup", len(got.Requests))
	}
	if got.Requests[0].Role != "login" || got.Requests[0].Method != "POST" || got.Requests[0].URL != "https://tvdb.example.test/login" {
		t.Errorf("login request = %+v", got.Requests[0])
	}
	if got.Requests[0].Auth != (AuthPlacement{}) {
		t.Errorf("login exchange must carry no auth, got %+v", got.Requests[0].Auth)
	}
	if got.Requests[1].Role != "lookup" || got.Requests[1].Auth.Scheme != "Bearer" {
		t.Errorf("lookup request = %+v, want bearer", got.Requests[1])
	}
}

// TestScraperTraceHostileDiagnostics: each single-dimension divergence reds with
// a message naming that dimension — pagination, path/query, auth placement, field
// mapping, transform result, and artwork composition (assert on message quality,
// not just non-empty).
func TestScraperTraceHostileDiagnostics(t *testing.T) {
	ref := CompileScraperTrace(tmdbLikeSpec(), tmdbLikeInput())
	cases := []struct {
		name    string
		mutate  func(w *ScraperTrace)
		wantSub string
	}{
		{"pagination request count", func(w *ScraperTrace) { w.Requests = w.Requests[:2] }, "pagination/requests"},
		{"path/query url", func(w *ScraperTrace) { w.Requests[0].URL = "https://api.example.test/search?query=dune&page=9" }, "request[0] url"},
		{"auth placement", func(w *ScraperTrace) {
			w.Requests[2].Auth = AuthPlacement{Location: "header", Name: "Authorization", Scheme: "Bearer"}
		}, "request[2] auth-placement"},
		{"field mapping", func(w *ScraperTrace) { w.Fields = map[string]string{"title": "Wrong", "year": "2021"} }, `field "title"`},
		{"transform result", func(w *ScraperTrace) { w.Fields = map[string]string{"title": "Dune", "year": "2020"} }, `field "year"`},
		{"artwork composition", func(w *ScraperTrace) { w.Artwork = map[string]string{"poster": "https://wrong.example.test/dune.jpg"} }, `artwork "poster"`},
		{"candidate id", func(w *ScraperTrace) { w.Candidates[0].ID = "999" }, "candidate[0] id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := tmdbLikeWant()
			tc.mutate(&want)
			reasons := scraperTraceViolations(want, ref)
			if !containsSubstring(reasons, tc.wantSub) {
				t.Fatalf("violations %v do not name %q", reasons, tc.wantSub)
			}
		})
	}
}

// TestScraperTraceOmitsUnresolvedAndUnparseable: an unresolved response path and
// a value the transform cannot parse are both omitted — never emitted as an empty
// string (the executor's "omit, like an unresolved path" posture).
func TestScraperTraceOmitsUnresolvedAndUnparseable(t *testing.T) {
	spec := sdk.ScraperSpec{
		ID: "omit", Kind: "http-json", Endpoint: "https://api.example.test",
		LookupTemplate: "/x/{id}",
		FieldMap:       map[string]string{"title": "missing", "year": "not-a-date"},
		Transforms:     map[string]string{"year": "year-from-date"},
	}
	doc := map[string]any{"not-a-date": "hello"}
	got := CompileScraperTrace(spec, TraceInput{ID: "1", LookupDoc: doc})
	if got.Fields != nil {
		t.Errorf("fields = %+v, want nil (unresolved title, unparseable year both omitted)", got.Fields)
	}
}

// TestScraperTraceTransforms exercises the three transform kinds directly through
// the compiler, including their omit-on-unparseable posture.
func TestScraperTraceTransforms(t *testing.T) {
	cases := []struct {
		kind, raw, want string
		omit            bool
	}{
		{"year-from-date", "2021-10-22", "2021", false},
		{"year-from-date", "Fri, 22 Oct 2021 00:00:00 +0000", "2021", false},
		{"year-from-date", "nope", "", true},
		{"date-from-rfc822", "Fri, 22 Oct 2021 08:30:00 +0000", "2021-10-22", false},
		{"date-from-rfc822", "2021-10-22", "", true},
		{"seconds-from-clock", "01:02:03", "3723", false},
		{"seconds-from-clock", "02:03", "123", false},
		{"seconds-from-clock", "90", "90", false},
		{"seconds-from-clock", "aa:bb", "", true},
	}
	for _, tc := range cases {
		spec := sdk.ScraperSpec{
			ID: "tr", Kind: "http-json", Endpoint: "https://api.example.test",
			LookupTemplate: "/x/{id}",
			FieldMap:       map[string]string{"f": "v"},
			Transforms:     map[string]string{"f": tc.kind},
		}
		got := CompileScraperTrace(spec, TraceInput{ID: "1", LookupDoc: map[string]any{"v": tc.raw}})
		value, present := got.Fields["f"]
		if tc.omit {
			if present {
				t.Errorf("%s(%q) = %q, want omitted", tc.kind, tc.raw, value)
			}
			continue
		}
		if value != tc.want {
			t.Errorf("%s(%q) = %q, want %q", tc.kind, tc.raw, value, tc.want)
		}
	}
}

// TestScraperTraceArtworkURLTemplate: a non-URL artwork id renders through the
// per-slot ArtworkURLTemplate rather than the base-URL composition.
func TestScraperTraceArtworkURLTemplate(t *testing.T) {
	spec := sdk.ScraperSpec{
		ID: "art", Kind: "http-json", Endpoint: "https://api.example.test",
		LookupTemplate:      "/x/{id}",
		ArtworkMap:          map[string]string{"cover": "coverId"},
		ArtworkURLTemplates: map[string]string{"cover": "https://cdn.example.test/{value}/full.jpg"},
		ArtworkBaseURL:      "https://ignored.example.test",
	}
	got := CompileScraperTrace(spec, TraceInput{ID: "1", LookupDoc: map[string]any{"coverId": "abc123"}})
	if got.Artwork["cover"] != "https://cdn.example.test/abc123/full.jpg" {
		t.Errorf("artwork = %+v, want template render", got.Artwork)
	}
}

// TestScraperTraceLocalFileKindIssuesNoRequest: the exif local-file kind reads
// the Media's own file and compiles no request plan.
func TestScraperTraceLocalFileKindIssuesNoRequest(t *testing.T) {
	spec := sdk.ScraperSpec{ID: "exif", Kind: "exif"}
	if got := CompileScraperTrace(spec, TraceInput{ID: "1"}); len(got.Requests) != 0 {
		t.Errorf("exif compiled %d requests, want 0 (local-file kind)", len(got.Requests))
	}
}

func containsSubstring(reasons []string, sub string) bool {
	for _, r := range reasons {
		if strings.Contains(r, sub) {
			return true
		}
	}
	return false
}
