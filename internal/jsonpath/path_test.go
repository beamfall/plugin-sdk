package jsonpath

import (
	"reflect"
	"testing"
)

func TestParseListPaths(t *testing.T) {
	cases := []struct {
		path string
		want []Segment
		ok   bool
	}{
		{"$", nil, true},
		{" \t$\n", nil, true},
		{"$[?type=a]", []Segment{{Key: "$", Conditions: [][2]string{{"type", "a"}}, FilterAll: true}}, true},
		{"a[?b.c=d]", []Segment{{Key: "a", Conditions: [][2]string{{"b.c", "d"}}, FilterAll: true}}, true},
		{"a[?x=1,y=2]", []Segment{{Key: "a", Conditions: [][2]string{{"x", "1"}, {"y", "2"}}, FilterAll: true}}, true},
		{"a[b.c=edition[1]].d", []Segment{{Key: "a", Conditions: [][2]string{{"b.c", "edition[1]"}}}, {Key: "d"}}, true},
		{"a[?b=edition[1][2]]", []Segment{{Key: "a", Conditions: [][2]string{{"b", "edition[1][2]"}}, FilterAll: true}}, true},
		{"", nil, false},
		{"$.a", nil, false},
		{"a.$", nil, false},
		{"$[type=a]", nil, false},
		{"$[?type=a].b", nil, false},
		{"$[?type=a][kind=b]", nil, false},
		{"a.$[?type=a]", nil, false},
		{"a[?]", nil, false},
		{"a[?=x]", nil, false},
		{"a[?b=c].d", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			got, ok := Parse(tc.path)
			if ok != tc.ok || !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Parse(%q) = %#v, %t, want %#v, %t", tc.path, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestUsesListSyntax(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"a|| $ ", true},
		{"a||rows[?type=official].name", true},
		{"rows[?type=official]]", true},
		{"rows[tag=edition[?draft]].name", false},
		{"rows[tag=alt||$].name", false},
		{"data[0].series.id", false},
	}
	for _, tc := range cases {
		if got := UsesListSyntax(tc.path); got != tc.want {
			t.Errorf("UsesListSyntax(%q) = %t, want %t", tc.path, got, tc.want)
		}
	}
}
