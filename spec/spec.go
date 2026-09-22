// Package spec holds what every generator in docgen reads: a field's shape,
// its meaning, and the resolved entry pairing them. It imports nothing, so the
// generators depend on it without depending on each other.
package spec

import (
	"path/filepath"
	"strings"
)

// Entry is one documented type, resolved: its structure and its metadata
// settled once, for every generator to read.
type Entry struct {
	Name   string // display title, and the page's basename
	Slug   string // directory basename, for layouts that give each entry one
	Split  bool   // whether the object fields get pages of their own
	Fields []Field
	Meta   MetadataSource // nil means "no metadata declared"
}

// GeneratedFile records one file written. Path is the file as written, so
// consumers never have to re-derive the naming rule.
type GeneratedFile struct {
	Name string // display title, e.g. "Config"
	Path string // path as written, e.g. "docs/reference/config.md"
}

// MetaAt asks src about the field at sectionPath + fieldName, joined with dots.
// A nil source yields a zero FieldMeta, which every consumer reads as "declares
// nothing".
func MetaAt(src MetadataSource, sectionPath []string, fieldName string) FieldMeta {
	if src == nil {
		return FieldMeta{}
	}
	return src.FieldMeta(strings.Join(ChildPath(sectionPath, fieldName), "."))
}

// ChildPath appends name to path without aliasing path's backing array.
func ChildPath(path []string, name string) []string {
	return append(append([]string(nil), path...), name)
}

// PathWithin rejects a target that escapes baseDir, guarding against a type or
// title name that resolves outside the output directory.
func PathWithin(baseDir, targetPath string) (string, bool) {
	cleanBase := filepath.Clean(baseDir)
	cleanTarget := filepath.Clean(targetPath)
	rel, err := filepath.Rel(cleanBase, cleanTarget)
	if err != nil {
		return "", false
	}
	if rel == "." {
		return cleanBase, true
	}
	if strings.HasPrefix(rel, "..") {
		return "", false
	}
	return cleanTarget, true
}
