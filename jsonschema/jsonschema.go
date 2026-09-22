// Package jsonschema writes a JSON Schema (draft 2020-12) per documented type.
package jsonschema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/lucasassuncao/docgen/spec"
)

// jsonSchemaDialect is the JSON Schema draft the generated documents declare.
const jsonSchemaDialect = "https://json-schema.org/draft/2020-12/schema"

// jsonSchemaBuilder turns discovered fields into a JSON Schema document.
// Recursion is represented, not expanded: a type already on the ancestor chain
// becomes a $ref and is hoisted into $defs, independent of any recursion limit.
type jsonSchemaBuilder struct {
	meta spec.MetadataSource
	// defs holds the hoisted schema of each recursive type.
	defs map[string]any
	// needDefs records type names a $ref was emitted for. A type is only known
	// to be recursive after its own expansion asks to reference it, so the hoist
	// happens on the way back up.
	needDefs map[string]bool
}

func newJSONSchemaBuilder(meta spec.MetadataSource) *jsonSchemaBuilder {
	return &jsonSchemaBuilder{
		meta:     meta,
		defs:     map[string]any{},
		needDefs: map[string]bool{},
	}
}

// root builds the complete document for an entry's top-level fields.
func (b *jsonSchemaBuilder) root(fields []spec.Field) map[string]any {
	doc := b.object(fields, nil, nil)
	doc["$schema"] = jsonSchemaDialect
	if len(b.defs) > 0 {
		doc["$defs"] = b.defs
	}
	return doc
}

// object builds an object schema. sectionPath is the metadata coordinate of the
// object itself; ancestors is the chain of struct type names being expanded.
func (b *jsonSchemaBuilder) object(fields []spec.Field, sectionPath, ancestors []string) map[string]any {
	props := map[string]any{}
	var required []string

	for _, f := range fields {
		name := f.YAMLName
		if name == "" || name == "-" {
			continue
		}
		m := spec.MetaAt(b.meta, sectionPath, name)
		if m.Required {
			required = append(required, name)
		}
		node := b.shape(f, spec.ChildPath(sectionPath, name), ancestors)
		applyFieldMeta(node, m)
		props[name] = node
	}

	// additionalProperties is deliberately not emitted: a field hidden from the
	// schema, or passed through untouched, would become a false error.
	out := map[string]any{"type": "object"}
	if len(props) > 0 {
		out["properties"] = props
	}
	if len(required) > 0 {
		slices.Sort(required)
		out["required"] = required
	}
	return out
}

// shape builds the structural half of a field's schema, before metadata.
func (b *jsonSchemaBuilder) shape(f spec.Field, childPath, ancestors []string) map[string]any {
	switch f.Kind {
	case spec.KindObject:
		return b.structNode(f, childPath, ancestors)

	case spec.KindList:
		node := map[string]any{"type": "array"}
		if items := b.elementNode(f, childPath, ancestors); items != nil {
			node["items"] = items
		}
		return node

	case spec.KindDictionary:
		node := map[string]any{"type": "object"}
		if values := b.elementNode(f, childPath, ancestors); values != nil {
			node["additionalProperties"] = values
		}
		if f.MapKeyScalar == "int" || f.MapKeyScalar == "uint" {
			node["propertyNames"] = map[string]any{"type": "integer"}
		}
		return node

	case spec.KindVariant, spec.KindAny:
		// A union or an interface constrains nothing that can be expressed here.
		return map[string]any{}

	default:
		if t := scalarNode(f.Scalar); t != nil {
			return t
		}
		return map[string]any{}
	}
}

// elementNode builds the schema of a collection's element: a struct when the
// field names one, a scalar when it does not, nil when neither is known.
func (b *jsonSchemaBuilder) elementNode(f spec.Field, childPath, ancestors []string) map[string]any {
	if f.TypeName != "" || len(f.Children) > 0 {
		return b.structNode(f, childPath, ancestors)
	}
	return scalarNode(f.ElemScalar)
}

// structNode expands a struct-shaped field, or references it when recursive.
func (b *jsonSchemaBuilder) structNode(f spec.Field, childPath, ancestors []string) map[string]any {
	name := f.TypeName
	if name == "" {
		// Anonymous struct: nothing to name, so nothing to reference.
		return b.object(f.Children, childPath, ancestors)
	}
	if slices.Contains(ancestors, name) {
		b.needDefs[name] = true
		return refNode(name)
	}

	node := b.object(f.Children, childPath, append(slices.Clone(ancestors), name))
	if b.needDefs[name] {
		if _, done := b.defs[name]; !done {
			b.defs[name] = node
		}
		return refNode(name)
	}
	return node
}

func refNode(name string) map[string]any {
	return map[string]any{"$ref": "#/$defs/" + name}
}

// scalarNode maps a FieldDef.Scalar label to its JSON Schema type. Durations are
// strings ("30s"), not numbers.
func scalarNode(scalar string) map[string]any {
	switch scalar {
	case "string", "duration":
		return map[string]any{"type": "string"}
	case "bool":
		return map[string]any{"type": "boolean"}
	case "int":
		return map[string]any{"type": "integer"}
	case "uint":
		return map[string]any{"type": "integer", "minimum": 0}
	case "float":
		return map[string]any{"type": "number"}
	}
	return nil
}

// jsonFormats maps a format name to the standard JSON Schema "format" value.
// A name with no standard equivalent goes to "description" instead of being
// dropped.
var jsonFormats = map[string]string{
	"url":      "uri",
	"uuid":     "uuid",
	"date":     "date",
	"email":    "email",
	"ipv4":     "ipv4",
	"ipv6":     "ipv6",
	"duration": "duration",
	"host":     "hostname",
	"fqdn":     "hostname",
}

// applyFieldMeta layers FieldMeta keywords onto an already-shaped node. What
// has no JSON Schema equivalent goes to "description" rather than being
// dropped. Default and Example stay strings: coercing them would be a guess.
func applyFieldMeta(node map[string]any, m spec.FieldMeta) {
	var notes []string

	if len(m.OneOf) > 0 {
		node["enum"] = m.OneOf
	}
	if len(m.NotOneOf) > 0 {
		node["not"] = map[string]any{"enum": m.NotOneOf}
	}
	if m.Pattern != "" {
		node["pattern"] = m.Pattern
	}
	if m.MinLength > 0 {
		node["minLength"] = m.MinLength
	}
	if m.MaxLength > 0 {
		node["maxLength"] = m.MaxLength
	}
	if m.MinCount > 0 {
		node["minItems"] = m.MinCount
	}
	if m.MaxCount > 0 {
		node["maxItems"] = m.MaxCount
	}
	if m.Unique {
		node["uniqueItems"] = true
	}
	if m.Default != "" {
		node["default"] = m.Default
	}
	if m.Example != "" {
		node["examples"] = []string{m.Example}
	}
	if m.Deprecated != "" {
		node["deprecated"] = true
		node["$comment"] = m.Deprecated
	}

	// Min/Max accept numbers, durations and size strings; JSON Schema bounds are
	// numeric only.
	if v, ok := numericBound(m.Min); ok {
		node["minimum"] = v
	} else if m.Min != "" {
		notes = append(notes, "Min: "+m.Min)
	}
	if v, ok := numericBound(m.Max); ok {
		node["maximum"] = v
	} else if m.Max != "" {
		notes = append(notes, "Max: "+m.Max)
	}

	mapped, unmapped := splitFormats(m.Formats)
	switch len(mapped) {
	case 0:
	case 1:
		node["format"] = mapped[0]
	default:
		// FieldMeta.Formats has OR semantics, which anyOf expresses directly.
		anyOf := make([]any, 0, len(mapped))
		for _, name := range mapped {
			anyOf = append(anyOf, map[string]any{"format": name})
		}
		node["anyOf"] = anyOf
	}
	for _, label := range unmapped {
		notes = append(notes, "Format: "+label)
	}

	description := m.Description
	if len(notes) > 0 {
		if description != "" {
			description += " "
		}
		description += strings.Join(notes, " ")
	}
	if description != "" {
		node["description"] = description
	}
}

// numericBound parses a FieldMeta Min/Max value as a number.
func numericBound(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// splitFormats separates formats with a standard JSON Schema keyword from those
// without. Zero formats are ignored.
func splitFormats(formats []string) (mapped, unmapped []string) {
	for _, f := range formats {
		if f == "" {
			continue
		}
		if name, ok := jsonFormats[f]; ok {
			mapped = append(mapped, name)
			continue
		}
		unmapped = append(unmapped, f)
	}
	return mapped, unmapped
}

// Emit writes one JSON Schema per entry to dir, named after the entry.
func Emit(dir string, entries []spec.Entry) ([]spec.GeneratedFile, error) {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("create json schema dir: %w", err)
	}

	var files []spec.GeneratedFile
	written := map[string]bool{}
	for _, c := range entries {
		doc := newJSONSchemaBuilder(c.Meta).root(c.Fields)
		data, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal json schema for %s: %w", c.Name, err)
		}
		data = append(data, '\n')

		path := filepath.Join(filepath.Clean(dir), strings.ToLower(c.Name)+".schema.json")
		if written[path] {
			return nil, fmt.Errorf("duplicate json schema %q: %s was already generated by an earlier entry", c.Name, path)
		}
		written[path] = true

		valid, ok := spec.PathWithin(dir, path)
		if !ok {
			return nil, fmt.Errorf("invalid json schema path: %s", path)
		}
		if err := os.WriteFile(valid, data, 0600); err != nil {
			return nil, fmt.Errorf("write json schema %s: %w", valid, err)
		}
		files = append(files, spec.GeneratedFile{Name: c.Name, Path: valid})
	}
	return files, nil
}
