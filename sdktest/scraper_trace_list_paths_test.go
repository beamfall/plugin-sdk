package sdktest

import (
	"encoding/json"
	"reflect"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
)

func TestScraperTraceListPaths(t *testing.T) {
	cases := []struct {
		name, path, doc string
		idPath, label   string
		want            []TraceCandidate
	}{
		{
			name: "bare search results", path: " $ ",
			doc:    `[{"score":0.9,"show":{"id":42,"name":"Solstice"}},{"score":0.5,"show":{"id":95,"name":"Equinox"}}]`,
			idPath: "show.id", label: "show.name",
			want: []TraceCandidate{
				{ID: "42", Fields: map[string]string{"title": "Solstice"}},
				{ID: "95", Fields: map[string]string{"title": "Equinox"}},
			},
		},
		{
			name: "TVmaze bare season listing", path: "$",
			doc:    `[{"id":1234,"number":1,"name":"Season 1","premiereDate":"2024-04-02","image":{"medium":"https://example.test/season.jpg"}}]`,
			idPath: "id", label: "name",
			want: []TraceCandidate{{ID: "1234", Fields: map[string]string{"title": "Season 1"}}},
		},
		{
			name: "TVDB official seasons", path: "data.seasons[?type.type=official]",
			doc: `{"data":{"seasons":[
				{"id":10,"number":0,"name":"Specials","type":{"type":"official"}},
				{"id":11,"number":1,"name":"Season 1","type":{"type":"official"}},
				{"id":21,"number":1,"name":"DVD Season 1","type":{"type":"dvd"}}
			]}}`,
			idPath: "id", label: "name",
			want: []TraceCandidate{
				{ID: "10", Fields: map[string]string{"title": "Specials"}},
				{ID: "11", Fields: map[string]string{"title": "Season 1"}},
			},
		},
		{
			name: "root filter", path: "$[?type=official]",
			doc:    `[{"id":10,"name":"Official","type":"official"},{"id":11,"name":"DVD","type":"dvd"}]`,
			idPath: "id", label: "name",
			want: []TraceCandidate{{ID: "10", Fields: map[string]string{"title": "Official"}}},
		},
		{
			name: "root filter and scalar comparisons", path: "$[?type=official,number=1,active=true,weight=1.5]",
			doc: `[
				{"id":10,"name":"Official","type":"official","number":1,"active":true,"weight":1.5},
				{"id":11,"name":"DVD","type":"dvd","number":1,"active":true,"weight":1.5},
				{"id":12,"name":"Inactive","type":"official","number":1,"active":false,"weight":1.5},
				{"id":13,"name":"Unknown","type":"official","number":null,"active":true,"weight":1.5},
				null,
				{"id":14,"name":"Also official","type":"official","number":"1","active":"true","weight":1.5}
			]`,
			idPath: "id", label: "name",
			want: []TraceCandidate{
				{ID: "10", Fields: map[string]string{"title": "Official"}},
				{ID: "14", Fields: map[string]string{"title": "Also official"}},
			},
		},
		{
			name: "zero matches", path: "rows[?type.type=null]",
			doc:    `{"rows":[{"type":{"type":null}},{"type":{}},{"type":{"type":[]}},{"type":{"type":{}}}]}`,
			idPath: "id", label: "name", want: []TraceCandidate{},
		},
		{
			name: "literal dotted key takes precedence", path: "rows[?type.type=official]",
			doc: `{"rows":[
				{"id":10,"name":"Literal","type.type":"official","type":{"type":"dvd"}},
				{"id":11,"name":"Shadowed","type.type":null,"type":{"type":"official"}},
				{"id":12,"name":"Nested","type":{"type":"official"}}
			]}`,
			idPath: "id", label: "name",
			want: []TraceCandidate{
				{ID: "10", Fields: map[string]string{"title": "Literal"}},
				{ID: "12", Fields: map[string]string{"title": "Nested"}},
			},
		},
		{
			name: "single predicate keeps first match", path: "groups[tag.kind=edition,version=1.0].items",
			doc: `{"groups":[
				{"tag.kind":"skip","tag":{"kind":"edition"},"version":"1.0","items":[]},
				{"tag":{"kind":"edition"},"version":"1.0","items":[{"id":10,"name":"First"}]},
				{"tag.kind":"edition","version":"1.0","items":[{"id":11,"name":"Second"}]}
			]}`,
			idPath: "id", label: "name",
			want: []TraceCandidate{{ID: "10", Fields: map[string]string{"title": "First"}}},
		},
		{
			name: "single predicate with bracketed value", path: "groups[tag=edition[1]].items",
			doc:    `{"groups":[{"tag":"edition[1]","items":[{"id":10,"name":"Bracketed"}]}]}`,
			idPath: "id", label: "name",
			want: []TraceCandidate{{ID: "10", Fields: map[string]string{"title": "Bracketed"}}},
		},
		{
			name: "single predicate with multiple conditions", path: "groups[tag=edition,version=1.0].items",
			doc:    `{"groups":[{"tag":"edition","version":"1.0","items":[{"id":10,"name":"Edition"}]}]}`,
			idPath: "id", label: "name",
			want: []TraceCandidate{{ID: "10", Fields: map[string]string{"title": "Edition"}}},
		},
		{
			name: "single predicate with empty value", path: "groups[tag=].items",
			doc:    `{"groups":[{"items":[]},{"tag":"","items":[{"id":10,"name":"Empty tag"}]}]}`,
			idPath: "id", label: "name",
			want: []TraceCandidate{{ID: "10", Fields: map[string]string{"title": "Empty tag"}}},
		},
		{
			name: "named array", path: "items", doc: `{"items":[{"id":10,"name":"Named"}]}`,
			idPath: "id", label: "name",
			want: []TraceCandidate{{ID: "10", Fields: map[string]string{"title": "Named"}}},
		},
		{
			name: "indexed array", path: "data.0.items", doc: `{"data":[{"items":[{"id":10,"name":"Indexed"}]}]}`,
			idPath: "id", label: "name",
			want: []TraceCandidate{{ID: "10", Fields: map[string]string{"title": "Indexed"}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			child := sdk.ChildListing{
				Level: "item", ListTemplate: "/list/{id}", ItemsPath: tc.path, Key: tc.idPath, Label: tc.label,
			}
			spec := sdk.ScraperSpec{
				ID: "example", Kind: "http-json", Endpoint: "https://example.test",
				SearchTemplate: "/search?q={query}", CandidatesPath: tc.path, CandidateIDPath: tc.idPath,
				SearchFieldMap: map[string]string{"title": tc.label}, Children: []sdk.ChildListing{child},
			}
			bundle := sdk.Bundle{
				ID: "example", Version: "0.1.0", SDKFloor: sdk.Version, SDKCeiling: sdk.Version,
				DisplayName: sdk.L("Example"), MediaLabel: sdk.L("Item"), MediaClasses: []string{"video"},
				Layouts: []sdk.LayoutSpec{{ID: "browse", Role: "browse"}}, Scrapers: []sdk.ScraperSpec{spec},
			}
			if err := sdk.ValidateBundle(bundle); err != nil {
				t.Fatalf("ValidateBundle rejected paths: %v", err)
			}

			var doc any
			if err := json.Unmarshal([]byte(tc.doc), &doc); err != nil {
				t.Fatal(err)
			}
			input := TraceInput{Query: "example"}
			switch doc := doc.(type) {
			case map[string]any:
				input.SearchDoc = doc
			case []any:
				input.SearchArray = doc
			}
			trace := CompileScraperTrace(spec, input)
			if !reflect.DeepEqual(trace.Candidates, tc.want) {
				t.Errorf("candidates = %#v, want %#v", trace.Candidates, tc.want)
			}

			rows, ok := resolveArray(doc, child.ItemsPath)
			if !ok || len(rows) != len(tc.want) {
				t.Fatalf("child rows = %#v, %t, want %d rows", rows, ok, len(tc.want))
			}
			for i, row := range rows {
				id, idOK := resolvePath(row, child.Key)
				label, labelOK := resolvePath(row, child.Label)
				if want := tc.want[i]; !idOK || id != want.ID || !labelOK || label != want.Fields["title"] {
					t.Errorf("child row %d: key = %q, %t, label = %q, %t, want %q and %q",
						i, id, idOK, label, labelOK, want.ID, want.Fields["title"])
				}
			}
		})
	}
}
