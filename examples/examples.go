// Package examples writes one page of preset YAML per documented field.
package examples

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasassuncao/docgen/spec"
)

// ErrNoPresetSource is returned when WithExamples was given a nil Examples.
var ErrNoPresetSource = errors.New("docgen: WithExamples requires a non-nil Examples")

// Emit writes one page per preset field listed in titles, holding the YAML of
// every preset that field exposes. The page is named after the lowercased
// title, so it matches the reference page for the same type.
func Emit(src Source, dir string, titles map[string]string) ([]spec.GeneratedFile, error) {
	if src == nil {
		return nil, ErrNoPresetSource
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("create examples dir: %w", err)
	}

	var files []spec.GeneratedFile
	for _, field := range src.ListFields() {
		title, ok := titles[field]
		if !ok {
			continue
		}
		names := src.ListPresets(field)
		if len(names) == 0 {
			continue
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "# %s Examples\n\n", title)
		for _, name := range names {
			y, err := src.PresetYAML(field, name)
			if err != nil {
				return nil, fmt.Errorf("preset yaml for %s/%s: %w", field, name, err)
			}
			fmt.Fprintf(&sb, "## Preset: %s\n\n", name)
			sb.WriteString("```yaml\n")
			sb.WriteString(y)
			if !strings.HasSuffix(y, "\n") {
				sb.WriteString("\n")
			}
			sb.WriteString("```\n\n")
		}

		out := filepath.Join(filepath.Clean(dir), strings.ToLower(title)+".md")
		valid, ok := spec.PathWithin(dir, out)
		if !ok {
			return nil, fmt.Errorf("invalid examples path: %s", out)
		}
		if err := os.WriteFile(valid, []byte(sb.String()), 0600); err != nil {
			return nil, fmt.Errorf("write example doc %s: %w", valid, err)
		}
		files = append(files, spec.GeneratedFile{Name: title, Path: valid})
	}
	return files, nil
}

// Source supplies the preset snippets an example page is built from. Declared
// here rather than imported, so any package whose preset source has this shape
// satisfies it without either side knowing the other.
type Source interface {
	ListFields() []string
	ListPresets(field string) []string
	PresetYAML(field, name string) (string, error)
}
