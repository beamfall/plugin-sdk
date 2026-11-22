package jsonpath

import "strings"

// Segment is an object key or array index, optionally with predicate conditions.
type Segment struct {
	Key        string
	Conditions [][2]string
}

// Parse parses a non-empty scraper JSON path. Dots inside predicate brackets
// are part of the predicate, and each comma-separated condition must match.
func Parse(path string) ([]Segment, bool) {
	parts, ok := splitPath(path)
	if !ok {
		return nil, false
	}
	segments := make([]Segment, 0, len(parts))
	for _, part := range parts {
		segment, ok := parseSegment(part)
		if !ok {
			return nil, false
		}
		segments = append(segments, segment)
	}
	return segments, true
}

// splitPath splits on dots outside predicate brackets.
func splitPath(path string) ([]string, bool) {
	if strings.TrimSpace(path) == "" {
		return nil, false
	}
	var segments []string
	start, depth := 0, 0
	for i, r := range path {
		switch r {
		case '[':
			depth++
		case ']':
			depth--
			if depth < 0 {
				return nil, false
			}
		case '.':
			if depth == 0 {
				segments = append(segments, path[start:i])
				start = i + 1
			}
		}
	}
	if depth != 0 {
		return nil, false
	}
	return append(segments, path[start:]), true
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
	for _, condition := range strings.Split(predicate, ",") {
		name, value, found := strings.Cut(condition, "=")
		if !found || name == "" {
			return Segment{}, false
		}
		segment.Conditions = append(segment.Conditions, [2]string{name, value})
	}
	return segment, true
}
