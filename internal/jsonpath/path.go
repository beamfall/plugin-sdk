package jsonpath

import "strings"

// Segment is an object key or array index, optionally with predicate conditions.
type Segment struct {
	Key        string
	Conditions [][2]string
	FilterAll  bool
}

// Parse parses a non-empty scraper JSON path. The explicit root "$" returns
// no segments. Dots inside predicate brackets are part of the predicate, and
// filter-all predicates must end the path.
func Parse(path string) ([]Segment, bool) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "$" {
		return nil, true
	}
	if strings.HasPrefix(trimmed, "$[") {
		path = trimmed
	}
	parts, ok := splitPath(path, ".")
	if !ok {
		return nil, false
	}
	segments := make([]Segment, 0, len(parts))
	for i, part := range parts {
		segment, ok := parseSegment(part)
		if !ok || segment.FilterAll && i != len(parts)-1 {
			return nil, false
		}
		if strings.TrimSpace(segment.Key) == "$" && (segment.Key != "$" || len(parts) != 1 || !segment.FilterAll) {
			return nil, false
		}
		segments = append(segments, segment)
	}
	return segments, true
}

// UsesListSyntax recognizes list-only forms even when they are misplaced.
// Scalar map paths may also have alternatives or use a legacy path dialect.
func UsesListSyntax(path string) bool {
	if !strings.ContainsAny(path, "$?") {
		return false
	}
	alternatives, _ := splitPath(path, "||")
	for _, alternative := range alternatives {
		parts, _ := splitPath(alternative, ".")
		for _, part := range parts {
			key, predicate, _ := strings.Cut(part, "[")
			if strings.TrimSpace(key) == "$" || strings.HasPrefix(predicate, "?") {
				return true
			}
		}
	}
	return false
}

// splitPath splits on the separator outside predicate brackets.
func splitPath(path, separator string) ([]string, bool) {
	if strings.TrimSpace(path) == "" {
		return nil, false
	}
	var segments []string
	start, depth := 0, 0
	balanced := true
	for i, r := range path {
		if i < start {
			continue
		}
		switch r {
		case '[':
			depth++
		case ']':
			if depth == 0 {
				balanced = false
			} else {
				depth--
			}
		default:
			if depth == 0 && strings.HasPrefix(path[i:], separator) {
				segments = append(segments, path[start:i])
				start = i + len(separator)
			}
		}
	}
	return append(segments, path[start:]), balanced && depth == 0
}

func parseSegment(part string) (Segment, bool) {
	key, predicate, found := strings.Cut(part, "[")
	segment := Segment{Key: key}
	if !found {
		return segment, key != ""
	}
	predicate, closed := strings.CutSuffix(predicate, "]")
	if key == "" || !closed {
		return Segment{}, false
	}
	predicate, segment.FilterAll = strings.CutPrefix(predicate, "?")
	if segment.FilterAll {
		if _, balanced := splitPath(predicate, "."); !balanced {
			return Segment{}, false
		}
	}
	for _, condition := range strings.Split(predicate, ",") {
		name, value, found := strings.Cut(condition, "=")
		if !found || name == "" {
			return Segment{}, false
		}
		segment.Conditions = append(segment.Conditions, [2]string{name, value})
	}
	return segment, true
}
