// Package markdown writes the reference pages: one table of arguments per
// documented section, cross-linked to the example pages.
package markdown

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasassuncao/docgen/spec"
)

// FolderMode decides whether an entry's pages get a directory of their own.
type FolderMode int

const (
	// FolderFlat writes every page straight into the markdown directory.
	FolderFlat FolderMode = iota
	// FolderPerEntry gives each entry a subdirectory named after its slug.
	FolderPerEntry
)

// FileMode decides how many pages an entry is split into.
type FileMode int

const (
	// FilePerEntry writes one page per entry, nested sections inlined.
	FilePerEntry FileMode = iota
	// FilePerField also gives each object-shaped direct field its own page,
	// which the entry's page links to. Scalars are never split, and the split
	// stops one level down: deeper objects stay inline in their parent's page.
	FilePerField
	// FileSingle writes one page holding every entry, one after another.
	FileSingle
)

// Layout is where pages land and what the tables show. The zero value is one
// flat page per entry with DefaultColumns.
type Layout struct {
	Folders FolderMode
	Files   FileMode
	// Columns are the table's columns, in order. nil is DefaultColumns.
	Columns []Column
}

// Config is one markdown run: where the pages go, how they are laid out, and
// where the example pages they link to live.
type Config struct {
	Dir           string
	Layout        Layout
	ExamplesDir   string
	ExampleTitles map[string]string
}

// columns is the layout's columns, or the default when it names none.
func (c Config) columns() []Column {
	if len(c.Layout.Columns) > 0 {
		return c.Layout.Columns
	}
	return DefaultColumns
}

// mdRenderer holds everything the rendering functions read. It is built per
// entry by Emit.
type mdRenderer struct {
	metadata       spec.MetadataSource
	columns        []Column
	examplesRelDir string
	examplePages   map[string]bool
}

// fieldMeta resolves metadata for a field within sectionPath.
func (g *mdRenderer) fieldMeta(sectionPath []string, fieldName string) spec.FieldMeta {
	return spec.MetaAt(g.metadata, sectionPath, fieldName)
}

// Emit writes the reference pages the layout asks for.
func Emit(cfg Config, entries []spec.Entry) ([]spec.GeneratedFile, error) {
	if cfg.Layout.Files == FileSingle {
		return emitSingle(cfg, entries)
	}

	var files []spec.GeneratedFile
	written := map[string]bool{}
	for _, c := range entries {
		dir := cfg.dirFor(c)
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("create markdown dir: %w", err)
		}
		relDir, pages := cfg.exampleLinks(dir)
		g := &mdRenderer{metadata: c.Meta, columns: cfg.columns(), examplesRelDir: relDir, examplePages: pages}

		if !c.Split {
			f, err := writeMarkdownOnce(written, dir, c.Name, c.Name, g.generateMarkdown(c.Name, c.Fields, nil))
			if err != nil {
				return nil, err
			}
			files = append(files, f)
			continue
		}

		f, err := writeMarkdownOnce(written, dir, c.Name, c.Name, g.generateRootMarkdown(c.Name, c.Fields))
		if err != nil {
			return nil, err
		}
		files = append(files, f)

		// One level only: an object nested deeper stays inline in its parent's
		// page, which is what keeps two same-named fields from colliding.
		for _, fd := range c.Fields {
			if fd.YAMLName == "" || fd.YAMLName == "-" || len(fd.Children) == 0 {
				continue
			}
			md := g.generateMarkdown(fd.YAMLName, fd.Children, []string{fd.YAMLName})
			child, err := writeMarkdownOnce(written, dir, fd.YAMLName, fd.YAMLName, md)
			if err != nil {
				return nil, err
			}
			files = append(files, child)
		}
	}
	return files, nil
}

// emitSingle writes every entry into one page, in the order given.
func emitSingle(cfg Config, entries []spec.Entry) ([]spec.GeneratedFile, error) {
	if err := os.MkdirAll(cfg.Dir, 0750); err != nil {
		return nil, fmt.Errorf("create markdown dir: %w", err)
	}
	relDir, pages := cfg.exampleLinks(cfg.Dir)

	var sb strings.Builder
	for i, c := range entries {
		if i > 0 {
			sb.WriteString("\n---\n\n")
		}
		g := &mdRenderer{metadata: c.Meta, columns: cfg.columns(), examplesRelDir: relDir, examplePages: pages}
		sb.WriteString(g.generateMarkdown(c.Name, c.Fields, nil))
	}

	f, err := writeMarkdownOnce(map[string]bool{}, cfg.Dir, "Reference", "reference", sb.String())
	if err != nil {
		return nil, err
	}
	return []spec.GeneratedFile{f}, nil
}

// dirFor returns where an entry's pages go: a subdirectory named after its
// slug, or the configured directory.
func (c Config) dirFor(e spec.Entry) string {
	if c.Layout.Folders == FolderPerEntry {
		return filepath.Join(c.Dir, e.Slug)
	}
	return c.Dir
}

// exampleLinks derives the cross-link inputs: the examples directory relative
// to fromDir, and which example pages exist. Each entry's link is computed from
// its own directory, since entries may sit at different depths.
func (c Config) exampleLinks(fromDir string) (relDir string, pages map[string]bool) {
	if fromDir == "" || c.ExamplesDir == "" || len(c.ExampleTitles) == 0 {
		return "", nil
	}
	rel, err := filepath.Rel(fromDir, c.ExamplesDir)
	if err != nil {
		return "", nil
	}
	pages = make(map[string]bool, len(c.ExampleTitles))
	for _, title := range c.ExampleTitles {
		pages[strings.ToLower(title)] = true
	}
	return filepath.ToSlash(rel), pages
}

// writeMarkdownOnce errors when the same output file was already written in this
// run: two entries with a same-named page would otherwise silently overwrite
// each other.
func writeMarkdownOnce(written map[string]bool, dir, name, slug, md string) (spec.GeneratedFile, error) {
	path := filepath.Join(filepath.Clean(dir), strings.ToLower(slug)+".md")
	if written[path] {
		return spec.GeneratedFile{}, fmt.Errorf("duplicate docs page %q: %s was already generated by an earlier entry", name, path)
	}
	written[path] = true

	valid, ok := spec.PathWithin(dir, path)
	if !ok {
		return spec.GeneratedFile{}, fmt.Errorf("invalid docs path: %s", path)
	}
	if err := os.WriteFile(valid, []byte(md), 0600); err != nil {
		return spec.GeneratedFile{}, fmt.Errorf("write docs page %s: %w", valid, err)
	}
	return spec.GeneratedFile{Name: name, Path: valid}, nil
}

// generateRootMarkdown builds the summary page for a split root type: it links
// to per-field files instead of inlining nested sections.
func (g *mdRenderer) generateRootMarkdown(title string, fields []spec.Field) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n", title)
	g.writeExamplesLink(&sb, title)
	sb.WriteString("## Arguments\n\n")
	sb.WriteString("The following arguments are supported:\n\n")
	g.writeFieldsTableLinked(&sb, fields)
	return sb.String()
}

// writeFieldsTableLinked is the summary table of a split page: a field with its
// own page is linked instead of inlined.
func (g *mdRenderer) writeFieldsTableLinked(sb *strings.Builder, fields []spec.Field) {
	g.writeTable(sb, fields, nil, func(f spec.Field, name string) string {
		if len(f.Children) > 0 {
			return fmt.Sprintf("[%s](./%s.md)", cellText(name), strings.ToLower(name))
		}
		return cellText(name)
	})
}

// cellText collapses whitespace (including newlines) and escapes pipes so an
// interpolated value cannot break a markdown table row.
func cellText(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.Join(strings.Fields(s), " ")
}

func (g *mdRenderer) generateMarkdown(typeName string, fields []spec.Field, sectionPath []string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "# %s\n\n", typeName)
	g.writeExamplesLink(&sb, typeName)
	sb.WriteString("## Arguments\n\n")
	sb.WriteString("The following arguments are supported:\n\n")
	g.writeFieldsTable(&sb, fields, sectionPath)
	g.writeNestedSections(&sb, fields, sectionPath, 3)

	return sb.String()
}

// writeExamplesLink emits an "Examples" section when WithExamples was set and an
// example page exists for title. The lookup is by lowercased title so root and
// split-child pages alike resolve to "<title>.md" in the examples directory.
func (g *mdRenderer) writeExamplesLink(sb *strings.Builder, title string) {
	if g.examplesRelDir == "" {
		return
	}
	key := strings.ToLower(title)
	if !g.examplePages[key] {
		return
	}
	sb.WriteString("## Examples\n\n")
	fmt.Fprintf(sb, "For usage examples, see [%s presets](%s/%s.md).\n\n", title, g.examplesRelDir, key)
}

func (g *mdRenderer) writeFieldsTable(sb *strings.Builder, fields []spec.Field, sectionPath []string) {
	g.writeTable(sb, fields, sectionPath, func(_ spec.Field, name string) string { return cellText(name) })
}

// writeTable renders the arguments table in the configured columns. An optional
// column no field fills is dropped, so a table without formats keeps its five.
func (g *mdRenderer) writeTable(sb *strings.Builder, fields []spec.Field, sectionPath []string, name func(spec.Field, string) string) {
	type row struct {
		field spec.Field
		meta  spec.FieldMeta
		label string
	}

	rows := make([]row, 0, len(fields))
	metas := make([]spec.FieldMeta, 0, len(fields))
	for _, f := range fields {
		if f.YAMLName == "" || f.YAMLName == "-" {
			continue
		}
		m := g.fieldMeta(sectionPath, f.YAMLName)
		rows = append(rows, row{f, m, name(f, f.YAMLName)})
		metas = append(metas, m)
	}

	cols := visibleColumns(g.columns, metas)
	writeHeader(sb, cols)
	for _, r := range rows {
		writeRow(sb, cols, r.field, r.meta, r.label)
	}
	sb.WriteString("\n")
}

func (g *mdRenderer) writeNestedSections(sb *strings.Builder, fields []spec.Field, sectionPath []string, level int) {
	hashes := strings.Repeat("#", level)

	for _, f := range fields {
		if len(f.Children) == 0 {
			continue
		}
		name := f.YAMLName
		childPath := append(append([]string(nil), sectionPath...), name)

		fmt.Fprintf(sb, "%s %s\n\n", hashes, name)
		sb.WriteString("The following arguments are supported:\n\n")
		g.writeFieldsTable(sb, f.Children, childPath)
		g.writeNestedSections(sb, f.Children, childPath, level+1)
	}
}

// formatLabels joins the field's format labels, or "-" when none. Labels are
// pipe-escaped so they cannot split the table row into extra columns.
func formatLabels(meta spec.FieldMeta) string {
	var labels []string
	for _, f := range meta.Formats {
		if f != "" {
			labels = append(labels, cellText(f))
		}
	}
	if len(labels) == 0 {
		return "-"
	}
	return strings.Join(labels, ", ")
}

// docTypeLabel returns the table type label, honouring the FieldMeta.Multiline
// and FieldMeta.Type overrides.
func docTypeLabel(f spec.Field, meta spec.FieldMeta) string {
	if meta.Multiline && meta.Type == "" {
		return "multiline string"
	}
	if meta.Type != "" {
		return meta.Type
	}
	return typeLabel(f)
}

func typeLabel(f spec.Field) string {
	switch f.Kind {
	case spec.KindPrimitive:
		if f.Scalar != "" {
			return f.Scalar
		}
		return "any"
	case spec.KindObject, spec.KindVariant:
		return "object"
	case spec.KindList:
		if f.ElemScalar != "" {
			return "[]" + f.ElemScalar
		}
		return "[]object"
	case spec.KindDictionary:
		key := f.MapKeyScalar
		if key == "" {
			key = "string"
		}
		if f.ElemScalar != "" {
			return "map[" + key + "]" + f.ElemScalar
		}
		return "map[" + key + "]object"
	case spec.KindAny:
		return "any"
	default:
		return "-"
	}
}
