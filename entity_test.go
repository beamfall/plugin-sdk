package sdk

import "testing"

// TestClassifyMediaFieldKeyBuiltin: a bare key in BuiltinMediaFields resolves
// as a builtin route.
func TestClassifyMediaFieldKeyBuiltin(t *testing.T) {
	route, err := classifyMediaFieldKey("title", nil)
	if err != nil {
		t.Fatalf("ClassifyMediaFieldKey: %v", err)
	}
	if !route.Builtin || route.MediaField != "" || route.Type != "" {
		t.Errorf("route = %+v, want a bare builtin route", route)
	}
}

// TestClassifyMediaFieldKeyPluginField: a "media.<name>" key declared in the
// bundle's Media fields resolves to that field's name and declared type.
func TestClassifyMediaFieldKeyPluginField(t *testing.T) {
	fields := map[string]string{"runtime-minutes": "number"}
	route, err := classifyMediaFieldKey("media.runtime-minutes", fields)
	if err != nil {
		t.Fatalf("ClassifyMediaFieldKey: %v", err)
	}
	if route.Builtin || route.MediaField != "runtime-minutes" || route.Type != "number" {
		t.Errorf("route = %+v, want plugin field route", route)
	}
}

// TestClassifyMediaFieldKeyUndeclaredPluginField: a "media.<name>" key not
// present in the bundle's declared Media fields is a load-time error.
func TestClassifyMediaFieldKeyUndeclaredPluginField(t *testing.T) {
	_, err := classifyMediaFieldKey("media.nope", map[string]string{"other": "text"})
	if err == nil {
		t.Fatal("want an error for an undeclared media field, got nil")
	}
}

// TestClassifyMediaFieldKeyEntityNamespaceRejected: the "entity." namespace
// is always rejected — entity routing is EntityMap's job, not FieldMap's.
func TestClassifyMediaFieldKeyEntityNamespaceRejected(t *testing.T) {
	_, err := classifyMediaFieldKey("entity.name", nil)
	if err == nil {
		t.Fatal("want an error for the entity namespace, got nil")
	}
}

// TestClassifyMediaFieldKeyUnknownBareKey: a bare key that is neither a
// builtin nor namespaced is a load-time error (the typo trap).
func TestClassifyMediaFieldKeyUnknownBareKey(t *testing.T) {
	_, err := classifyMediaFieldKey("subtitle", nil)
	if err == nil {
		t.Fatal("want an error for an unrecognized bare key, got nil")
	}
}

// TestMediaFieldTypes: MediaFieldTypes indexes the bundle's declared Media
// fields by name -> type.
func TestMediaFieldTypes(t *testing.T) {
	b := Bundle{MediaFields: []FieldSpec{
		{Name: "runtime-minutes", Type: "number"},
		{Name: "class", Type: "text"},
	}}
	got := b.MediaFieldTypes()
	want := map[string]string{"runtime-minutes": "number", "class": "text"}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for name, typ := range want {
		if got[name] != typ {
			t.Errorf("field %q: got type %q, want %q", name, got[name], typ)
		}
	}
}

// TestMediaFieldTypesEmpty: a bundle with no declared Media fields yields an
// empty (non-nil) map.
func TestMediaFieldTypesEmpty(t *testing.T) {
	var b Bundle
	got := b.MediaFieldTypes()
	if len(got) != 0 {
		t.Errorf("got %+v, want empty map", got)
	}
}

// TestEntityFieldTypesFound: EntityFieldTypes indexes the named kind's
// declared fields by name -> type.
func TestEntityFieldTypesFound(t *testing.T) {
	b := Bundle{EntityKinds: []EntityKind{
		{Name: "person", Fields: []FieldSpec{{Name: "birth-year", Type: "number"}}},
		{Name: "container", Fields: []FieldSpec{{Name: "title", Type: "text"}}},
	}}
	got := b.EntityFieldTypes("person")
	if len(got) != 1 || got["birth-year"] != "number" {
		t.Errorf("got %+v, want {birth-year: number}", got)
	}
}

// TestEntityFieldTypesUndeclaredKind: an undeclared kind yields an empty map,
// not an error/panic.
func TestEntityFieldTypesUndeclaredKind(t *testing.T) {
	b := Bundle{EntityKinds: []EntityKind{{Name: "person"}}}
	got := b.EntityFieldTypes("nonexistent")
	if len(got) != 0 {
		t.Errorf("got %+v, want empty map", got)
	}
}
