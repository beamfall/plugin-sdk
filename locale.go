package sdk

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// LocaleString is one display string in every language it ships: BCP-47
// locale tag (lowercase: "en", "fr", "fr-ca") → translated value. English
// ("en") is the required fallback anchor on any non-empty map. A nil or
// empty map means "no string declared" (where an empty string was legal
// before).
type LocaleString map[string]string

// L is the shorthand constructor: L("…") ≡ LocaleString{"en": "…"}, the
// in-Go equivalent of the contract's plain-string shorthand. L("") is the
// absent string (nil).
func L(value string) LocaleString {
	if value == "" {
		return nil
	}
	return LocaleString{"en": value}
}

// Resolve returns the best value for a BCP-47 locale: exact tag, then
// progressively truncated tags (fr-ca → fr), then the English fallback.
// The empty locale means English. Resolution is deterministic: a map with
// no usable match (no "en" entry) yields the value of the lexicographically
// smallest tag, and an absent map yields "".
func (s LocaleString) Resolve(locale string) string {
	if len(s) == 0 {
		return ""
	}
	tag := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(locale), "_", "-"))
	for tag != "" {
		if value, ok := s[tag]; ok {
			return value
		}
		if i := strings.LastIndex(tag, "-"); i >= 0 {
			tag = tag[:i]
		} else {
			tag = ""
		}
	}
	if value, ok := s["en"]; ok {
		return value
	}
	tags := make([]string, 0, len(s))
	for t := range s {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return s[tags[0]]
}

// MarshalJSON keeps the wire shorthand: an absent map is "", a map carrying
// only English is the plain string, anything else is the full locale-map
// object (keys sorted by encoding/json — deterministic).
func (s LocaleString) MarshalJSON() ([]byte, error) {
	if len(s) == 0 {
		return json.Marshal("")
	}
	if len(s) == 1 {
		if value, ok := s["en"]; ok {
			return json.Marshal(value)
		}
	}
	return json.Marshal(map[string]string(s))
}

// UnmarshalJSON accepts both contract forms: the plain-string shorthand
// (≡ {"en": …}) and the full locale-map object.
func (s *LocaleString) UnmarshalJSON(data []byte) error {
	var shorthand string
	if err := json.Unmarshal(data, &shorthand); err == nil {
		*s = L(shorthand)
		return nil
	}
	var full map[string]string
	if err := json.Unmarshal(data, &full); err != nil {
		return fmt.Errorf("locale string must be a plain string or a locale map: %w", err)
	}
	*s = LocaleString(full)
	return nil
}
