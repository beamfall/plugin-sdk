package sdktest

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	sdk "github.com/beamfall/plugin-sdk"
)

// This file is the scraper REFERENCE-TRACE VALIDATOR, NOT the
// production scraper executor. Core owns the executor — the code that actually
// fetches documents over the network, follows pagination, and applies mappings;
// this harness never fetches anything. CompileScraperTrace re-derives, purely and
// offline, what a declarative ScraperSpec SHOULD produce for a given input: the
// request plan (implicit method, path/query, auth placement, pagination) and the
// mapping outputs (response-path resolution, transforms, artwork composition,
// emitted values). A plugin freezes golden traces and RunScraperTraceConformance
// grades its specs against them, so a spec drift or an executor divergence reds
// CI. The mapping logic here is a deliberate second implementation of the
// documented ScraperSpec semantics (scraper.go doc comments) — the independent
// oracle the executor is checked against — never a copy of Core's engine.

// TraceInput is the offline stimulus a reference trace is compiled against: the
// search/lookup identifiers to substitute and the decoded JSON documents the
// mappings resolve against. No field triggers a network call.
type TraceInput struct {
	// Query is the free-text {query} substitution for the search request; empty
	// compiles no search request.
	Query string
	// ID is the {id} substitution for the lookup request; empty compiles no
	// lookup request (unless fingerprints alone drive it — see LookupTemplate).
	ID string
	// Fingerprints supplies the content-identity variables {phash}, {oshash},
	// {duration}. An unknown/absent fingerprint substitutes as the empty string,
	// matching the executor contract (ScraperSpec.LookupTemplate).
	Fingerprints map[string]string
	// LookupDoc is the decoded JSON lookup document FieldMap/ArtworkMap/Transforms
	// resolve against. Nil resolves nothing (every mapped value is omitted).
	LookupDoc map[string]any
	// SearchDoc is the decoded JSON search response CandidatesPath resolves the
	// candidate array from. Nil compiles no candidates.
	SearchDoc map[string]any
}

// AuthPlacement is where a scraper's AuthRef secret rides on a request — the
// placement the trace predicts from AuthStyle, never the secret value itself.
type AuthPlacement struct {
	// Location is "header", "query", or "" (no auth / no AuthRef).
	Location string
	// Name is the header or query-parameter name carrying the secret.
	Name string
	// Scheme is a header value prefix ("Bearer"); empty for non-bearer forms.
	Scheme string
	// InURL is true when the secret rides in the request URL (the "query:<param>"
	// form) — the placement the SDK doc warns leaks secrets to logs and proxies.
	InURL bool
}

// TraceRequest is one outbound request the reference trace predicts.
type TraceRequest struct {
	// Role is "search", "lookup", or "login".
	Role string
	// Page is the 1-based search page; 0 for lookup/login.
	Page int
	// Method is the implicit HTTP method the scraper kind uses.
	Method string
	// URL is the composed target: Endpoint plus the substituted path/query
	// template (GET kinds) or bare Endpoint (POST kinds carry the body instead).
	URL string
	// Body is the request body — the substituted GraphQL query for http-graphql,
	// or the login exchange payload; empty for GET requests.
	Body string
	// Auth is where the AuthRef secret rides on this request; the login exchange
	// carries no Auth (it mints the token).
	Auth AuthPlacement
}

// TraceCandidate is one search-result element's emitted values, resolved through
// SearchFieldMap/SearchArtworkMap (falling back to FieldMap/ArtworkMap).
type TraceCandidate struct {
	// ID is the value at CandidateIDPath — what a chosen candidate feeds into the
	// lookup {id}. Empty when no CandidateIDPath is declared or it is unresolved.
	ID      string
	Fields  map[string]string
	Artwork map[string]string
}

// ScraperTrace is the compiled reference trace for one input: the predicted
// request plan plus the emitted mapping outputs.
type ScraperTrace struct {
	// Requests is the ordered request plan (login, then search pages, then lookup).
	Requests []TraceRequest
	// Fields is the emitted lookup-document core-field → value map, after
	// transforms; an unresolved or unparseable value is omitted (never present as
	// an empty string).
	Fields map[string]string
	// Artwork is the emitted lookup-document artwork-slot → absolute URL map.
	Artwork map[string]string
	// Candidates is the emitted search-result elements, in document order.
	Candidates []TraceCandidate
}

// ScraperTraceVector is one golden reference-trace case: a spec + input, and the
// trace the spec must compile to.
type ScraperTraceVector struct {
	Name  string
	Spec  sdk.ScraperSpec
	Input TraceInput
	Want  ScraperTrace
}

// RunScraperTraceConformance grades a plugin's declarative ScraperSpecs against
// golden reference traces. It is diagnostic by construction: a method, URL,
// auth-placement, pagination, response-path, transform, or artwork divergence
// each reds with a message naming the dimension and showing got vs want. Like the
// other runners it runs RequireAllowedImports first.
func RunScraperTraceConformance(t *testing.T, vectors []ScraperTraceVector) {
	t.Helper()
	RequireAllowedImports(t)
	if len(vectors) == 0 {
		t.Fatal("sdktest.RunScraperTraceConformance: no scraper trace vectors supplied")
	}
	for _, v := range vectors {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			got := CompileScraperTrace(v.Spec, v.Input)
			for _, reason := range scraperTraceViolations(v.Want, got) {
				t.Error(reason)
			}
		})
	}
}

// CompileScraperTrace re-derives the reference trace for spec against input,
// purely and offline. It is the oracle RunScraperTraceConformance grades against
// and the executor is checked against — not a fetch.
func CompileScraperTrace(spec sdk.ScraperSpec, input TraceInput) ScraperTrace {
	vars := substitutionVars(input)
	auth := authPlacement(spec)
	return ScraperTrace{
		Requests:   compileRequests(spec, input, vars, auth),
		Fields:     resolveFieldMap(spec.FieldMap, spec.Transforms, input.LookupDoc),
		Artwork:    resolveArtworkMap(spec, spec.ArtworkMap, input.LookupDoc),
		Candidates: compileCandidates(spec, input),
	}
}

// implicitMethod is the HTTP method a scraper kind uses without any spec field —
// GET fetches, GraphQL posts. The local-file kind issues no request.
func implicitMethod(kind string) string {
	switch kind {
	case "http-graphql":
		return "POST"
	case "http-json", "rss":
		return "GET"
	default:
		return ""
	}
}

func substitutionVars(input TraceInput) map[string]string {
	vars := map[string]string{
		"id":       input.ID,
		"query":    input.Query,
		"phash":    input.Fingerprints["phash"],
		"oshash":   input.Fingerprints["oshash"],
		"duration": input.Fingerprints["duration"],
	}
	return vars
}

// substitute replaces every recognized {var} in tmpl. {page} is filled per call.
// Unrecognized placeholders are left verbatim (they are not scraper variables).
func substitute(tmpl string, vars map[string]string, page int) string {
	out := tmpl
	for name, value := range vars {
		out = strings.ReplaceAll(out, "{"+name+"}", value)
	}
	return strings.ReplaceAll(out, "{page}", strconv.Itoa(page))
}

func compileRequests(spec sdk.ScraperSpec, input TraceInput, vars map[string]string, auth AuthPlacement) []TraceRequest {
	method := implicitMethod(spec.Kind)
	if method == "" {
		return nil
	}
	requests := loginRequests(spec)
	requests = append(requests, searchRequests(spec, input, vars, method, auth)...)
	requests = append(requests, lookupRequest(spec, input, vars, method, auth)...)
	return requests
}

// loginRequests prepends the login-bearer token exchange when the spec declares
// it: a POST to Endpoint+Login.Path carrying the SecretField, no Auth of its own.
func loginRequests(spec sdk.ScraperSpec) []TraceRequest {
	if spec.AuthStyle != "login-bearer" || spec.Login == nil {
		return nil
	}
	return []TraceRequest{{
		Role:   "login",
		Method: "POST",
		URL:    spec.Endpoint + spec.Login.Path,
		Body:   fmt.Sprintf("{%q:<secret>}", spec.Login.SecretField),
	}}
}

func searchRequests(spec sdk.ScraperSpec, input TraceInput, vars map[string]string, method string, auth AuthPlacement) []TraceRequest {
	if spec.SearchTemplate == "" || input.Query == "" {
		return nil
	}
	pages := spec.MaxPages
	if pages < 1 {
		pages = 1
	}
	requests := make([]TraceRequest, 0, pages)
	for page := 1; page <= pages; page++ {
		requests = append(requests, buildRequest("search", page, spec, substitute(spec.SearchTemplate, vars, page), method, auth))
	}
	return requests
}

func lookupRequest(spec sdk.ScraperSpec, input TraceInput, vars map[string]string, method string, auth AuthPlacement) []TraceRequest {
	if spec.LookupTemplate == "" || (input.ID == "" && !hasFingerprint(input)) {
		return nil
	}
	return []TraceRequest{buildRequest("lookup", 0, spec, substitute(spec.LookupTemplate, vars, 1), method, auth)}
}

func hasFingerprint(input TraceInput) bool {
	for _, key := range []string{"phash", "oshash", "duration"} {
		if input.Fingerprints[key] != "" {
			return true
		}
	}
	return false
}

// buildRequest composes one request from a substituted template: a POST kind
// carries the template as its body against the bare Endpoint, a GET kind appends
// the template to the Endpoint as its path/query.
func buildRequest(role string, page int, spec sdk.ScraperSpec, rendered, method string, auth AuthPlacement) TraceRequest {
	if method == "POST" {
		return TraceRequest{Role: role, Page: page, Method: method, URL: spec.Endpoint, Body: rendered, Auth: auth}
	}
	return TraceRequest{Role: role, Page: page, Method: method, URL: spec.Endpoint + rendered, Auth: auth}
}

// authPlacement derives where the AuthRef secret rides from the spec's AuthStyle.
// A spec with no AuthRef carries no auth even if it names a style.
func authPlacement(spec sdk.ScraperSpec) AuthPlacement {
	if spec.AuthRef == "" {
		return AuthPlacement{}
	}
	switch {
	case spec.AuthStyle == "" || spec.AuthStyle == "bearer" || spec.AuthStyle == "login-bearer":
		return AuthPlacement{Location: "header", Name: "Authorization", Scheme: "Bearer"}
	case strings.HasPrefix(spec.AuthStyle, "header:"):
		return AuthPlacement{Location: "header", Name: strings.TrimPrefix(spec.AuthStyle, "header:")}
	case strings.HasPrefix(spec.AuthStyle, "query:"):
		return AuthPlacement{Location: "query", Name: strings.TrimPrefix(spec.AuthStyle, "query:"), InURL: true}
	case strings.HasPrefix(spec.AuthStyle, "signature:"):
		return AuthPlacement{Location: "header", Name: signatureHeaderName(spec)}
	default:
		return AuthPlacement{}
	}
}

func signatureHeaderName(spec sdk.ScraperSpec) string {
	if spec.Signature == nil {
		return ""
	}
	return spec.Signature.HeaderName
}

// resolveFieldMap resolves each mapped core field against the document and applies
// its transform. An unresolved path or an unparseable transform omits the field —
// the emitted map never carries a present-but-empty value.
func resolveFieldMap(fieldMap, transforms map[string]string, doc map[string]any) map[string]string {
	out := map[string]string{}
	for field, path := range fieldMap {
		raw, ok := resolvePath(doc, path)
		if !ok {
			continue
		}
		value, ok := applyTransform(transforms[field], raw)
		if !ok {
			continue
		}
		out[field] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// resolveArtworkMap resolves each artwork slot and composes it into an absolute
// URL via ArtworkURLTemplates ({value} render) or ArtworkBaseURL (site-relative
// prefix); absolute values pass through.
func resolveArtworkMap(spec sdk.ScraperSpec, artworkMap map[string]string, doc map[string]any) map[string]string {
	out := map[string]string{}
	for slot, path := range artworkMap {
		raw, ok := resolvePath(doc, path)
		if !ok {
			continue
		}
		out[slot] = composeArtworkURL(spec, slot, raw)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func composeArtworkURL(spec sdk.ScraperSpec, slot, raw string) string {
	if tmpl := spec.ArtworkURLTemplates[slot]; tmpl != "" {
		return strings.ReplaceAll(tmpl, "{value}", raw)
	}
	if strings.HasPrefix(raw, "/") && spec.ArtworkBaseURL != "" {
		return spec.ArtworkBaseURL + raw
	}
	return raw
}

// compileCandidates resolves the search-result array and maps each element with
// the search-scoped maps, falling back to the lookup maps when none are declared.
func compileCandidates(spec sdk.ScraperSpec, input TraceInput) []TraceCandidate {
	if spec.SearchTemplate == "" || spec.CandidatesPath == "" {
		return nil
	}
	elements, ok := resolveArray(input.SearchDoc, spec.CandidatesPath)
	if !ok {
		return nil
	}
	fieldMap := fallbackMap(spec.SearchFieldMap, spec.FieldMap)
	artworkMap := fallbackMap(spec.SearchArtworkMap, spec.ArtworkMap)
	candidates := make([]TraceCandidate, 0, len(elements))
	for _, element := range elements {
		id, _ := resolvePath(element, spec.CandidateIDPath)
		candidates = append(candidates, TraceCandidate{
			ID:      id,
			Fields:  resolveFieldMap(fieldMap, spec.Transforms, element),
			Artwork: resolveArtworkMap(spec, artworkMap, element),
		})
	}
	return candidates
}

func fallbackMap(primary, secondary map[string]string) map[string]string {
	if primary != nil {
		return primary
	}
	return secondary
}

// resolvePath walks a dotted object path and stringifies the scalar leaf. A path
// that hits a missing key, a non-object mid-segment, or a non-scalar leaf is
// unresolved (ok=false) — the FieldMap "omit, never error" posture.
func resolvePath(doc map[string]any, path string) (string, bool) {
	if path == "" || doc == nil {
		return "", false
	}
	var cur any = doc
	for _, segment := range strings.Split(path, ".") {
		object, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		value, ok := object[segment]
		if !ok {
			return "", false
		}
		cur = value
	}
	return stringifyScalar(cur)
}

// resolveArray walks a dotted object path to an array of JSON objects.
func resolveArray(doc map[string]any, path string) ([]map[string]any, bool) {
	if path == "" || doc == nil {
		return nil, false
	}
	var cur any = doc
	for _, segment := range strings.Split(path, ".") {
		object, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		value, ok := object[segment]
		if !ok {
			return nil, false
		}
		cur = value
	}
	raw, ok := cur.([]any)
	if !ok {
		return nil, false
	}
	elements := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		object, ok := item.(map[string]any)
		if !ok {
			return nil, false
		}
		elements = append(elements, object)
	}
	return elements, true
}

// stringifyScalar renders a JSON scalar as its string value. An integral number
// renders without a decimal point (a year is "2020", not "2020"); arrays and
// objects are not scalars and do not resolve.
func stringifyScalar(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case bool:
		return strconv.FormatBool(t), true
	case float64:
		if !math.IsInf(t, 0) && !math.IsNaN(t) && t == math.Trunc(t) {
			return strconv.FormatInt(int64(t), 10), true
		}
		return strconv.FormatFloat(t, 'g', -1, 64), true
	default:
		return "", false
	}
}

// applyTransform applies a fixed TransformKind to a raw value. An empty kind
// passes the value through. An unparseable value omits the field (ok=false),
// matching the executor's "omit, like an unresolved path" contract.
func applyTransform(kind, raw string) (string, bool) {
	switch kind {
	case "":
		return raw, true
	case "year-from-date":
		return yearFromDate(raw)
	case "date-from-rfc822":
		return dateFromRFC822(raw)
	case "seconds-from-clock":
		return secondsFromClock(raw)
	default:
		return "", false
	}
}

// yearFromDate extracts a 4-digit year from an ISO or RFC-822 date string.
func yearFromDate(raw string) (string, bool) {
	if len(raw) >= 4 && isDigits(raw[:4]) {
		return raw[:4], true
	}
	if t, ok := parseRFC822(raw); ok {
		return strconv.Itoa(t.Year()), true
	}
	return "", false
}

// dateFromRFC822 reformats an RFC-822 date (RSS pubDate) as YYYY-MM-DD.
func dateFromRFC822(raw string) (string, bool) {
	t, ok := parseRFC822(raw)
	if !ok {
		return "", false
	}
	return t.Format("2006-01-02"), true
}

func parseRFC822(raw string) (time.Time, bool) {
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// secondsFromClock parses "HH:MM:SS", "MM:SS", or plain seconds into a seconds
// count. A non-numeric or over-segmented value omits the field.
func secondsFromClock(raw string) (string, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) == 0 || len(parts) > 3 {
		return "", false
	}
	total := 0
	for _, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return "", false
		}
		total = total*60 + n
	}
	return strconv.Itoa(total), true
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// scraperTraceViolations reports every dimension in which got diverges from want,
// each message naming the dimension so a plugin sees exactly what drifted.
func scraperTraceViolations(want, got ScraperTrace) []string {
	var violations []string
	violations = append(violations, requestViolations(want.Requests, got.Requests)...)
	violations = append(violations, mapViolations("field", want.Fields, got.Fields)...)
	violations = append(violations, mapViolations("artwork", want.Artwork, got.Artwork)...)
	violations = append(violations, candidateViolations(want.Candidates, got.Candidates)...)
	return violations
}

func requestViolations(want, got []TraceRequest) []string {
	if len(want) != len(got) {
		return []string{fmt.Sprintf("pagination/requests: got %d request(s), want %d\n  got:  %+v\n  want: %+v", len(got), len(want), got, want)}
	}
	var violations []string
	for i := range want {
		violations = append(violations, requestFieldViolations(i, want[i], got[i])...)
	}
	return violations
}

// requestFieldViolations names each diverging request field individually so a
// method, URL, pagination-page, body, or auth-placement drift is unambiguous.
func requestFieldViolations(i int, want, got TraceRequest) []string {
	var violations []string
	check := func(dimension, wantStr, gotStr string) {
		if wantStr != gotStr {
			violations = append(violations, fmt.Sprintf("request[%d] %s: got %q, want %q (role %s)", i, dimension, gotStr, wantStr, want.Role))
		}
	}
	check("role", want.Role, got.Role)
	check("method", want.Method, got.Method)
	check("url", want.URL, got.URL)
	check("body", want.Body, got.Body)
	check("page", strconv.Itoa(want.Page), strconv.Itoa(got.Page))
	if want.Auth != got.Auth {
		violations = append(violations, fmt.Sprintf("request[%d] auth-placement: got %+v, want %+v", i, got.Auth, want.Auth))
	}
	return violations
}

func mapViolations(dimension string, want, got map[string]string) []string {
	var violations []string
	for key, wantValue := range want {
		gotValue, ok := got[key]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf("%s %q: missing, want %q", dimension, key, wantValue))
		case gotValue != wantValue:
			violations = append(violations, fmt.Sprintf("%s %q: got %q, want %q", dimension, key, gotValue, wantValue))
		}
	}
	for _, key := range sortedStringMapKeys(got) {
		if _, ok := want[key]; !ok {
			violations = append(violations, fmt.Sprintf("%s %q: unexpected %q (no such value was expected)", dimension, key, got[key]))
		}
	}
	return violations
}

func candidateViolations(want, got []TraceCandidate) []string {
	if len(want) != len(got) {
		return []string{fmt.Sprintf("candidates: got %d, want %d", len(got), len(want))}
	}
	var violations []string
	for i := range want {
		if want[i].ID != got[i].ID {
			violations = append(violations, fmt.Sprintf("candidate[%d] id: got %q, want %q", i, got[i].ID, want[i].ID))
		}
		violations = append(violations, mapViolations(fmt.Sprintf("candidate[%d] field", i), want[i].Fields, got[i].Fields)...)
		violations = append(violations, mapViolations(fmt.Sprintf("candidate[%d] artwork", i), want[i].Artwork, got[i].Artwork)...)
	}
	return violations
}

func sortedStringMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
