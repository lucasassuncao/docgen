package spec

import (
	"reflect"
	"strings"
	"testing"
)

type srcConfig struct {
	Path  string `yaml:"path"`
	Depth int    `yaml:"depth"`
}

func (srcConfig) Metadata() map[string]any {
	return map[string]any{
		"path": map[string]any{
			"description": "Directory to watch.",
			"required":    true,
			"formats":     []string{"directory"},
		},
		"depth": map[string]any{"default": "1"},
	}
}

type catConfig struct {
	Name   string    `yaml:"name"`
	Source srcConfig `yaml:"source"`
}

func (catConfig) Metadata() map[string]any {
	return map[string]any{
		"name":   map[string]any{"description": "Category name.", "required": true},
		"source": map[string]any{"description": "Where files come from."},
	}
}

func TestDescribe_readsMetadataOfNestedTypes(t *testing.T) {
	fields, meta, err := Describe(catConfig{})
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if len(fields) != 2 || fields[0].YAMLName != "name" || fields[1].YAMLName != "source" {
		t.Fatalf("fields = %+v", fields)
	}
	if got := meta.FieldMeta("name"); got.Description != "Category name." || !got.Required {
		t.Errorf("name meta = %+v", got)
	}
	// The nested type declares its own fields; the parent never repeats them.
	if got := meta.FieldMeta("source"); got.Description != "Where files come from." {
		t.Errorf("source meta = %+v", got)
	}
	got := meta.FieldMeta("source.path")
	if got.Description != "Directory to watch." || !got.Required {
		t.Errorf("source.path meta = %+v", got)
	}
	if len(got.Formats) != 1 || got.Formats[0] != "directory" {
		t.Errorf("source.path formats = %v", got.Formats)
	}
	if meta.FieldMeta("source.depth").Default != "1" {
		t.Errorf("source.depth default = %q", meta.FieldMeta("source.depth").Default)
	}
}

func TestDescribe_unknownPathDeclaresNothing(t *testing.T) {
	_, meta, err := Describe(catConfig{})
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if got := meta.FieldMeta("source.nope"); !reflect.DeepEqual(got, FieldMeta{}) {
		t.Errorf("want zero meta, got %+v", got)
	}
}

type typoConfig struct {
	Name string `yaml:"name"`
}

func (typoConfig) Metadata() map[string]any {
	return map[string]any{"name": map[string]any{"descriptoin": "typo"}} //nolint:misspell // intentional typo
}

func TestDescribe_unknownKeyIsAnError(t *testing.T) {
	_, _, err := Describe(typoConfig{})
	if err == nil {
		t.Fatal("want an error naming the offending key")
	}
	if !strings.Contains(err.Error(), "typoConfig") || !strings.Contains(err.Error(), "descriptoin") { //nolint:misspell // intentional typo
		t.Errorf("error should name the type and the key: %v", err)
	}
}

type editorKeysConfig struct {
	Mode string `yaml:"mode"`
}

func (editorKeysConfig) Metadata() map[string]any {
	return map[string]any{"mode": map[string]any{
		"oneof": []string{"a", "b"}, "presentation": "flat", "snippet": "mode: a", "prechecked": true, "enablepicker": true,
	}}
}

// Keys only an editor reads share the same Metadata(), so they are no error.
func TestDescribe_acceptsEditorOnlyKeys(t *testing.T) {
	_, meta, err := Describe(editorKeysConfig{})
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if got := meta.FieldMeta("mode").OneOf; len(got) != 2 {
		t.Errorf("mode oneof = %v", got)
	}
}

// mountOrString stands for a union: a mount is a string or an object, so
// reflection cannot read its shape and Metadata declares it instead.
type mountOrString struct {
	Str string
}

func (mountOrString) Metadata() map[string]any {
	return map[string]any{
		"source": map[string]any{"kind": "primitive", "scalar": "string", "description": "Host path."},
		"target": map[string]any{"kind": "primitive", "scalar": "string"},
	}
}

type mountHolder struct {
	Mounts []mountOrString `yaml:"mounts"`
}

func TestDescribe_metadataDeclaresUnionShape(t *testing.T) {
	fields, meta, err := Describe(mountHolder{})
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if len(fields) != 1 {
		t.Fatalf("fields = %+v", fields)
	}
	children := fields[0].Children
	if len(children) != 2 {
		t.Fatalf("children = %+v", children)
	}
	for _, c := range children {
		if c.Kind != KindPrimitive || c.Scalar != "string" {
			t.Errorf("child %q = %+v", c.YAMLName, c)
		}
	}
	if got := meta.FieldMeta("mounts.source"); got.Description != "Host path." {
		t.Errorf("mounts.source meta = %+v", got)
	}
}

type bareConfig struct {
	Name string `yaml:"name"`
	Skip string `yaml:"-"`
	Off  string
}

func TestDescribe_withoutMetadata(t *testing.T) {
	fields, meta, err := Describe(bareConfig{})
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if len(fields) != 1 || fields[0].YAMLName != "name" {
		t.Fatalf("fields = %+v", fields)
	}
	if got := meta.FieldMeta("name"); !reflect.DeepEqual(got, FieldMeta{}) {
		t.Errorf("want zero meta, got %+v", got)
	}
}

// selfConfig is recursive: the walk must stop, and so must the metadata tree
// built alongside it.
type selfConfig struct {
	Name string       `yaml:"name"`
	Sub  []selfConfig `yaml:"sub"`
}

func (selfConfig) Metadata() map[string]any {
	return map[string]any{"name": map[string]any{"description": "A name."}}
}

func TestDescribe_recursiveTypeTerminates(t *testing.T) {
	fields, meta, err := Describe(selfConfig{}, 1)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if len(fields) != 2 {
		t.Fatalf("fields = %+v", fields)
	}
	if meta.FieldMeta("sub.name").Description != "A name." {
		t.Error("nested level should carry the type's own metadata")
	}
	if meta.FieldMeta("sub.sub.sub.name").Description != "" {
		t.Error("tree should stop where the fields do")
	}
}
