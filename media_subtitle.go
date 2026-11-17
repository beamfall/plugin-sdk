package sdk

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// MediaSubtitleSeparator joins the rendered segments of a MediaSubtitle.
const MediaSubtitleSeparator = " · "

var mediaSubtitleOrdinals = []string{"sequence", "index", "index-end"}

// MediaSubtitleOrdinals returns the structural ordinal names a MediaSubtitle
// placeholder may use besides declared Media fields. They mirror ParsedMedia's
// Sequence, Index and IndexEnd.
func MediaSubtitleOrdinals() []string { return copySlice(mediaSubtitleOrdinals) }

var mediaSubtitlePlaceholderPattern = regexp.MustCompile(`\{([^{}]*)\}`)

func validateMediaSubtitle(segments []LocaleString, mediaFields map[string]string) error {
	for i, segment := range segments {
		if strings.TrimSpace(segment["en"]) == "" {
			return fmt.Errorf("media subtitle segment %d requires an English fallback", i)
		}
		if err := validateLocaleString(segment); err != nil {
			return fmt.Errorf("media subtitle segment %d: %w", i, err)
		}
		for tag, template := range segment {
			matches := mediaSubtitlePlaceholderPattern.FindAllStringSubmatch(template, -1)
			if len(matches) == 0 {
				return fmt.Errorf("media subtitle segment %d (%s) %q names no placeholder", i, tag, template)
			}
			if strings.ContainsAny(mediaSubtitlePlaceholderPattern.ReplaceAllString(template, ""), "{}") {
				return fmt.Errorf("media subtitle segment %d (%s) %q has an unbalanced brace", i, tag, template)
			}
			for _, match := range matches {
				name := match[1]
				if slices.Contains(mediaSubtitleOrdinals, name) {
					continue
				}
				switch mediaFields[name] {
				case "text", "number":
				case "":
					return fmt.Errorf("media subtitle segment %d (%s) names undeclared media field %q", i, tag, name)
				default:
					return fmt.Errorf("media subtitle segment %d (%s) names media field %q of type %q, only text and number fields render", i, tag, name, mediaFields[name])
				}
			}
		}
	}
	return nil
}

// RenderMediaSubtitle renders a bundle's MediaSubtitle for one Media item in
// one locale. lookup returns the display text of a placeholder name (a
// declared Media field or a structural ordinal) and whether the item has a
// value; formatting numbers is the host's job. A segment whose placeholders
// are not all present is dropped; the rest join with MediaSubtitleSeparator.
// The result is empty when no segment renders.
func RenderMediaSubtitle(segments []LocaleString, locale string, lookup func(name string) (string, bool)) string {
	parts := make([]string, 0, len(segments))
	for _, segment := range segments {
		complete := true
		rendered := mediaSubtitlePlaceholderPattern.ReplaceAllStringFunc(segment.Resolve(locale), func(placeholder string) string {
			value, ok := lookup(placeholder[1 : len(placeholder)-1])
			value = strings.TrimSpace(value)
			if !ok || value == "" {
				complete = false
			}
			return value
		})
		if complete && strings.TrimSpace(rendered) != "" {
			parts = append(parts, strings.TrimSpace(rendered))
		}
	}
	return strings.Join(parts, MediaSubtitleSeparator)
}

// MediaSubtitleLocales returns the sorted locale tags any segment declares,
// "en" always included when segments exist: the locales a host renders a
// locale-mapped subtitle for.
func MediaSubtitleLocales(segments []LocaleString) []string {
	seen := map[string]bool{}
	for _, segment := range segments {
		for tag := range segment {
			seen[tag] = true
		}
	}
	if len(segments) > 0 {
		seen["en"] = true
	}
	tags := make([]string, 0, len(seen))
	for tag := range seen {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

// MediaSubtitleFields returns the sorted, de-duplicated placeholder names the
// segments use, so a host reads only the values it renders.
func MediaSubtitleFields(segments []LocaleString) []string {
	seen := map[string]bool{}
	for _, segment := range segments {
		for _, template := range segment {
			for _, match := range mediaSubtitlePlaceholderPattern.FindAllStringSubmatch(template, -1) {
				seen[match[1]] = true
			}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
