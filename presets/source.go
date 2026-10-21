// Package presets builds a YAML preset source from Go structs. Each value is
// marshaled when a preset is requested, so the application never hand-writes
// YAML.
package presets

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Source supplies YAML preset snippets keyed by (field, preset name). The
// editor uses it to populate the preset picker and to seed the YAML editor
// when a block is opened.
type Source interface {
	// ListFields returns the field names that have at least one preset.
	ListFields() []string

	// ListPresets returns the preset names available for the given field,
	// or an empty slice if the field has no presets.
	ListPresets(field string) []string

	// PresetYAML returns the YAML snippet for (field, name) or an error if
	// either is unknown.
	PresetYAML(field, name string) (string, error)
}

// ForField wraps a map of Go structs as a single-field Source. Each value is
// YAML-marshaled under its field key when PresetYAML is called.
func ForField[T any](field string, m map[string]T) *FieldPresets[T] {
	return &FieldPresets[T]{field: field, m: m}
}

// FieldPresets is a single-field Source returned by ForField.
type FieldPresets[T any] struct {
	field string
	m     map[string]T
}

func (p *FieldPresets[T]) ListFields() []string { return []string{p.field} }

func (p *FieldPresets[T]) ListPresets(field string) []string {
	if field != p.field {
		return nil
	}
	keys := make([]string, 0, len(p.m))
	for k := range p.m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (p *FieldPresets[T]) PresetYAML(field, name string) (string, error) {
	if field != p.field {
		return "", fmt.Errorf("presets: unknown field %q", field)
	}
	val, ok := p.m[name]
	if !ok {
		return "", fmt.Errorf("presets: unknown preset %q for field %q", name, field)
	}
	// 2 spaces, as config files are written: examples get copied into them.
	var out strings.Builder
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(map[string]any{field: val}); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return out.String(), nil
}

// Combine merges Sources into one. Fields enumerate in declaration order;
// a lookup tries each source in turn and the first answer wins, so a
// non-enumerating source such as Func still resolves what the others did not.
func Combine(sources ...Source) Source {
	return &multiPresets{sources: sources}
}

type multiPresets struct {
	sources []Source
}

func (m *multiPresets) ListFields() []string {
	seen := make(map[string]bool)
	var fields []string
	for _, s := range m.sources {
		for _, f := range s.ListFields() {
			if !seen[f] {
				fields = append(fields, f)
				seen[f] = true
			}
		}
	}
	return fields
}

func (m *multiPresets) ListPresets(field string) []string {
	for _, s := range m.sources {
		if p := s.ListPresets(field); len(p) > 0 {
			return p
		}
	}
	return nil
}

func (m *multiPresets) PresetYAML(field, name string) (string, error) {
	// Try every source in order and return the first successful answer.
	// Sources such as Func do not enumerate presets, so routing by
	// ListPresets alone would make them unreachable.
	var fieldErr error
	for _, s := range m.sources {
		out, err := s.PresetYAML(field, name)
		if err == nil {
			return out, nil
		}
		if fieldErr == nil && len(s.ListPresets(field)) > 0 {
			fieldErr = err
		}
	}
	if fieldErr != nil {
		return "", fieldErr
	}
	return "", fmt.Errorf("presets: unknown field %q", field)
}

// Func adapts a plain function to the Source interface for dynamic preset
// lookup without enumeration. ListFields and ListPresets return nil, so the
// preset picker will not appear; only direct (field, name) lookups work.
type Func func(field, name string) (string, error)

func (f Func) ListFields() []string                          { return nil }
func (f Func) ListPresets(_ string) []string                 { return nil }
func (f Func) PresetYAML(field, name string) (string, error) { return f(field, name) }
