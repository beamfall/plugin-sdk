package sdktest

import (
	"fmt"
	"sort"
	"strings"

	sdk "github.com/beamfall/plugin-sdk"
)

// TraceContainer is one Container EntityMap's emitted result against the
// lookup document: the fields, artwork slots (ArtworkPath as "poster" plus
// ArtworkMap), display title and the links its ContainerRelations resolve to.
type TraceContainer struct {
	Kind    string
	Fields  map[string]string
	Artwork map[string]string
	Title   string
	Links   []TraceLink
}

// TraceLink is one container → sibling-hydrated Entity link: the container
// relation and the linked Entity's kind and name.
type TraceLink struct {
	Relation string
	Kind     string
	Name     string
}

// traceEntity is one Entity a non-container EntityMap hydrates offline.
type traceEntity struct {
	relation, kind, name string
}

// compileContainers resolves each Container EntityMap against the lookup
// document. A map whose Path does not resolve (an ordinal {sequence} path has
// no offline value) emits nothing. Sibling maps with a LookupTemplate are
// traced offline: an element without a LookupIDPath value is skipped, at most
// MaxLookups elements count, and NamePath resolves against the element's
// referenced document in entityDocs.
func compileContainers(spec sdk.ScraperSpec, doc map[string]any, entityDocs map[string]map[string]any) []TraceContainer {
	if doc == nil {
		return nil
	}
	var siblings []traceEntity
	for _, entityMap := range spec.EntityMaps {
		if !entityMap.Container {
			siblings = append(siblings, traceEntities(entityMap, doc, entityDocs)...)
		}
	}
	var containers []TraceContainer
	for _, entityMap := range spec.EntityMaps {
		if !entityMap.Container {
			continue
		}
		var node any = doc
		if entityMap.Path != "" {
			var ok bool
			if node, ok = resolveNode(doc, entityMap.Path); !ok {
				continue
			}
		}
		if elements, isList := node.([]any); isList && len(elements) > 0 {
			node = elements[0]
		}
		element, isObject := node.(map[string]any)
		if !isObject {
			continue
		}
		container := TraceContainer{
			Kind:    entityMap.Kind,
			Fields:  nonBlank(resolveFieldMap(entityMap.FieldMap, nil, element)),
			Artwork: containerArtwork(spec, entityMap, element),
		}
		if title, ok := resolvePath(element, entityMap.TitlePath); ok {
			container.Title = strings.TrimSpace(title)
		}
		if container.Fields == nil && container.Artwork == nil && container.Title == "" && len(entityMap.ContainerRelations) == 0 {
			continue
		}
		relations := make([]string, 0, len(entityMap.ContainerRelations))
		for relation := range entityMap.ContainerRelations {
			relations = append(relations, relation)
		}
		sort.Strings(relations)
		for _, relation := range relations {
			for _, sibling := range siblings {
				if sibling.relation == entityMap.ContainerRelations[relation] {
					container.Links = append(container.Links, TraceLink{Relation: relation, Kind: sibling.kind, Name: sibling.name})
				}
			}
		}
		containers = append(containers, container)
	}
	return containers
}

// containerArtwork resolves a Container map's ArtworkPath into the "poster"
// slot, rendered through the map's own ArtworkURLTemplate, and each
// ArtworkMap slot through the scraper's ArtworkURLTemplates; blank values are
// omitted.
func containerArtwork(spec sdk.ScraperSpec, entityMap sdk.EntityMap, element map[string]any) map[string]string {
	out := map[string]string{}
	if raw, ok := resolvePath(element, entityMap.ArtworkPath); ok && strings.TrimSpace(raw) != "" {
		if entityMap.ArtworkURLTemplate != "" {
			out["poster"] = strings.ReplaceAll(entityMap.ArtworkURLTemplate, "{value}", raw)
		} else {
			out["poster"] = composeArtworkURL(sdk.ScraperSpec{ArtworkBaseURL: spec.ArtworkBaseURL}, "poster", raw)
		}
	}
	for slot, path := range entityMap.ArtworkMap {
		if raw, ok := resolvePath(element, path); ok && strings.TrimSpace(raw) != "" {
			out[slot] = composeArtworkURL(spec, slot, raw)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// nonBlank drops blank values, as the executor never writes a blank field.
func nonBlank(fields map[string]string) map[string]string {
	for key, value := range fields {
		if strings.TrimSpace(value) == "" {
			delete(fields, key)
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}

func traceEntities(entityMap sdk.EntityMap, doc map[string]any, entityDocs map[string]map[string]any) []traceEntity {
	node, ok := resolveNode(doc, entityMap.Path)
	if entityMap.Path == "" || !ok {
		return nil
	}
	elements, isList := node.([]any)
	if !isList {
		elements = []any{node}
	}
	var entities []traceEntity
	lookups := 0
	for _, element := range elements {
		object, _ := element.(map[string]any)
		if entityMap.LookupTemplate != "" {
			id, ok := resolvePath(object, entityMap.LookupIDPath)
			if !ok || strings.TrimSpace(id) == "" {
				continue
			}
			if entityMap.MaxLookups > 0 && lookups >= entityMap.MaxLookups {
				break
			}
			lookups++
			referenced, found := entityDocs[strings.ReplaceAll(entityMap.LookupTemplate, "{id}", id)]
			if !found {
				continue
			}
			element, object = referenced, referenced
		}
		var name string
		if entityMap.NamePath == "" {
			name, ok = stringifyScalar(element)
		} else {
			name, ok = resolvePath(object, entityMap.NamePath)
		}
		if name = strings.TrimSpace(name); ok && name != "" {
			entities = append(entities, traceEntity{relation: entityMap.Relation, kind: entityMap.Kind, name: name})
		}
	}
	return entities
}

func containerViolations(want, got []TraceContainer) []string {
	if len(want) != len(got) {
		return []string{fmt.Sprintf("containers: got %d, want %d\n  got:  %+v\n  want: %+v", len(got), len(want), got, want)}
	}
	var violations []string
	for i := range want {
		prefix := fmt.Sprintf("container[%d]", i)
		if want[i].Kind != got[i].Kind {
			violations = append(violations, fmt.Sprintf("%s kind: got %q, want %q", prefix, got[i].Kind, want[i].Kind))
		}
		if want[i].Title != got[i].Title {
			violations = append(violations, fmt.Sprintf("%s title: got %q, want %q", prefix, got[i].Title, want[i].Title))
		}
		violations = append(violations, mapViolations(prefix+" field", want[i].Fields, got[i].Fields)...)
		violations = append(violations, mapViolations(prefix+" artwork", want[i].Artwork, got[i].Artwork)...)
		if fmt.Sprint(want[i].Links) != fmt.Sprint(got[i].Links) {
			violations = append(violations, fmt.Sprintf("%s links: got %v, want %v", prefix, got[i].Links, want[i].Links))
		}
	}
	return violations
}
