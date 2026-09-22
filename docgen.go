// Package docgen turns a Go configuration type into reference documentation.
// It wires the generators in markdown, jsonschema, examples and index, which
// read the vocabulary in spec and can also be called directly. See README.md.
package docgen

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lucasassuncao/docgen/examples"
	"github.com/lucasassuncao/docgen/index"
	"github.com/lucasassuncao/docgen/jsonschema"
	"github.com/lucasassuncao/docgen/markdown"
	"github.com/lucasassuncao/docgen/spec"
)

// The vocabulary and the layout knobs, re-exported so a caller needs one
// import. They are aliases, not new types.
type (
	Field          = spec.Field
	Kind           = spec.Kind
	FieldMeta      = spec.FieldMeta
	Node           = spec.Node
	Provider       = spec.Provider
	MetadataSource = spec.MetadataSource
	MetaFunc       = spec.MetaFunc
	GeneratedFile  = spec.GeneratedFile

	Layout     = markdown.Layout
	FolderMode = markdown.FolderMode
	FileMode   = markdown.FileMode
	Column     = markdown.Column

	Examples = examples.Source
)

const (
	KindPrimitive  = spec.KindPrimitive
	KindObject     = spec.KindObject
	KindList       = spec.KindList
	KindDictionary = spec.KindDictionary
	KindVariant    = spec.KindVariant
	KindAny        = spec.KindAny

	FolderFlat     = markdown.FolderFlat
	FolderPerEntry = markdown.FolderPerEntry

	FilePerEntry = markdown.FilePerEntry
	FilePerField = markdown.FilePerField
	FileSingle   = markdown.FileSingle

	ColName        = markdown.ColName
	ColType        = markdown.ColType
	ColFormat      = markdown.ColFormat
	ColDescription = markdown.ColDescription
	ColRequired    = markdown.ColRequired
	ColDefault     = markdown.ColDefault
	ColValues      = markdown.ColValues
	ColConstraints = markdown.ColConstraints
	ColDeprecated  = markdown.ColDeprecated
)

// Describe and Reflect read a type's shape and metadata. See spec.
func Describe(v any, recursionLimit ...int) ([]Field, MetadataSource, error) {
	return spec.Describe(v, recursionLimit...)
}

func Reflect(v any, recursionLimit ...int) []Field { return spec.Reflect(v, recursionLimit...) }

// ErrNoOutput is returned by Generate when no output option was supplied.
var ErrNoOutput = errors.New("docgen: no output configured")

// ErrNoPresetSource is returned when WithExamples was given a nil source.
var ErrNoPresetSource = examples.ErrNoPresetSource

// Entry is one struct to document, with what decides how.
type Entry struct {
	Config   any
	Name     string
	Slug     string
	Fields   []Field
	Metadata MetadataSource
	// Hidden are dot-joined field paths left out of every output: the markdown,
	// the JSON Schema, all of it. The field stays valid in the file; it just
	// stops being documented.
	Hidden []string
	// SplitStructs overrides the layout's FileMode for this entry alone; nil
	// follows the layout.
	SplitStructs *bool
	// RecursionLimit is how many extra levels a self-referential type expands.
	// nil is 1; 0 stops at the second visit. JSON Schema is unaffected.
	RecursionLimit *int
}

// config holds a generation run's settings. Each output is enabled by its
// directory being non-empty.
type config struct {
	metadata MetadataSource

	markdownDir    string
	markdownLayout Layout
	jsonSchemaDir  string
	indexDir       string

	examplesSrc   Examples
	examplesDir   string
	exampleTitles map[string]string
}

// Option configures a generation run. Passing the same option twice keeps the
// last value.
type Option func(*config)

// WithMetadata uses src for every entry, in place of the metadata their types
// declare. Without it, each entry reads its own Metadata; a type that declares
// none is documented from structure alone.
func WithMetadata(src MetadataSource) Option {
	return func(c *config) { c.metadata = src }
}

// WithMarkdown writes markdown reference pages under dir. The optional layout
// decides how they are laid out there; omitted, it is one flat page per entry.
func WithMarkdown(dir string, layout ...Layout) Option {
	return func(c *config) {
		c.markdownDir = dir
		if len(layout) > 0 {
			c.markdownLayout = layout[0]
		}
	}
}

// WithJSONSchema writes a JSON Schema (draft 2020-12) per entry to dir, named
// "<lowercased name>.schema.json".
func WithJSONSchema(dir string) Option {
	return func(c *config) { c.jsonSchemaDir = dir }
}

// WithExamples writes one page per preset field to dir, linked from the
// reference pages. titles maps a source field name to its display title, whose
// lowercase is the page name and the link key. Untitled fields are skipped.
func WithExamples(src Examples, dir string, titles map[string]string) Option {
	return func(c *config) {
		c.examplesSrc = src
		c.examplesDir = dir
		c.exampleTitles = titles
	}
}

// WithIndex writes a README.md to dir linking to the generated markdown and
// example pages. Generated JSON Schema files are not listed: the index is a
// readable documentation index, not a manifest of build artifacts.
func WithIndex(dir string) Option {
	return func(c *config) { c.indexDir = dir }
}

// Generate writes every output selected by opts and returns the files written,
// ordered markdown, JSON Schema, examples, index.
func Generate(entries []Entry, opts ...Option) ([]GeneratedFile, error) {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.markdownDir == "" && cfg.jsonSchemaDir == "" && cfg.examplesDir == "" && cfg.indexDir == "" {
		return nil, ErrNoOutput
	}

	resolved, err := resolveEntries(cfg, entries)
	if err != nil {
		return nil, err
	}

	var mdFiles, schemaFiles, exampleFiles []GeneratedFile
	if cfg.markdownDir != "" {
		if mdFiles, err = markdown.Emit(markdown.Config{
			Dir:           cfg.markdownDir,
			Layout:        cfg.markdownLayout,
			ExamplesDir:   cfg.examplesDir,
			ExampleTitles: cfg.exampleTitles,
		}, resolved); err != nil {
			return nil, err
		}
	}
	if cfg.jsonSchemaDir != "" {
		if schemaFiles, err = jsonschema.Emit(cfg.jsonSchemaDir, resolved); err != nil {
			return nil, err
		}
	}
	if cfg.examplesDir != "" {
		if exampleFiles, err = examples.Emit(cfg.examplesSrc, cfg.examplesDir, cfg.exampleTitles); err != nil {
			return nil, err
		}
	}

	files := make([]GeneratedFile, 0, len(mdFiles)+len(schemaFiles)+len(exampleFiles)+1)
	files = append(files, mdFiles...)
	files = append(files, schemaFiles...)
	files = append(files, exampleFiles...)

	if cfg.indexDir != "" {
		indexFile, err := index.Emit(cfg.indexDir, mdFiles, exampleFiles)
		if err != nil {
			return nil, err
		}
		files = append(files, indexFile)
	}
	return files, nil
}

// resolveEntries settles each entry once: its structure, its metadata, and the
// names and split the generators read. None of them re-discovers anything.
func resolveEntries(cfg *config, entries []Entry) ([]spec.Entry, error) {
	out := make([]spec.Entry, 0, len(entries))
	for _, e := range entries {
		fields, meta, err := resolveStructure(e)
		if err != nil {
			return nil, err
		}
		if cfg.metadata != nil {
			meta = cfg.metadata
		}
		name := e.Name
		if name == "" {
			name = typeName(e.Config)
		}
		slug := e.Slug
		if slug == "" {
			slug = strings.ToLower(name)
		}
		out = append(out, spec.Entry{
			Name:   name,
			Slug:   slug,
			Split:  splitFor(cfg, e),
			Fields: fields,
			Meta:   meta,
		})
	}
	return out, nil
}

func resolveStructure(e Entry) ([]Field, MetadataSource, error) {
	fields, meta := e.Fields, e.Metadata
	if fields != nil && meta != nil {
		return fields, meta, nil
	}

	opts := spec.Options{Hidden: e.Hidden}
	if e.RecursionLimit != nil {
		opts.RecursionLimit = *e.RecursionLimit
		if *e.RecursionLimit == 0 {
			opts.RecursionLimit = -1
		}
	}
	describedFields, describedMeta, err := spec.DescribeWith(e.Config, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("describe %T: %w", e.Config, err)
	}
	if fields == nil {
		fields = describedFields
	}
	if meta == nil {
		meta = describedMeta
	}
	return fields, meta, nil
}

// splitFor reports whether an entry's object fields get pages of their own:
// what the layout says, unless the entry overrides it.
func splitFor(cfg *config, e Entry) bool {
	if e.SplitStructs != nil {
		return *e.SplitStructs
	}
	return cfg.markdownLayout.Files == markdown.FilePerField
}
