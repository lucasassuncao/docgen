package spec

import "strings"

// Provider is implemented by a type that describes its own direct fields, keyed
// by yaml name. A plain map, so the same method serves any consumer without
// either side importing the other's types. FieldMeta lists the keys.
type Provider interface {
	Metadata() map[string]any
}

// FieldMeta is one field's declared meaning, decoded from the map a Provider
// returns. Every key is optional; an unknown one is an error naming it, so a
// typo cannot become silently dead metadata.
type FieldMeta struct {
	Description string   `yaml:"description"`
	Type        string   `yaml:"type"` // human-readable type label; derived from the field when empty
	Required    bool     `yaml:"required"`
	Default     string   `yaml:"default"`
	OneOf       []string `yaml:"oneof"`
	NotOneOf    []string `yaml:"notoneof"`
	Example     string   `yaml:"example"`
	Min         string   `yaml:"min"`
	Max         string   `yaml:"max"`
	Pattern     string   `yaml:"pattern"`
	MinCount    int      `yaml:"mincount"`
	MaxCount    int      `yaml:"maxcount"`
	MinLength   int      `yaml:"minlength"`
	MaxLength   int      `yaml:"maxlength"`
	Unique      bool     `yaml:"unique"`
	Deprecated  string   `yaml:"deprecated"`
	Formats     []string `yaml:"formats"`
	Multiline   bool     `yaml:"multiline"`

	// Structure, declared only by types whose shape reflection cannot read -
	// a union that is a string or an object, for instance. When Kind is set on
	// a Provider's entries, those entries become the type's fields.
	Kind   string `yaml:"kind"`   // "primitive", "object", "list", "dictionary", "variant", "any"
	Scalar string `yaml:"scalar"` // "string", "int", "bool", "float", "uint", "duration"

	// Accepted and ignored: a key another reader of the same Metadata() needs
	// must not be an error here.
	Presentation string `yaml:"presentation"`
	Snippet      string `yaml:"snippet"`
	PreChecked   bool   `yaml:"prechecked"`
}

// Node is one field's metadata plus the metadata of its children.
type Node struct {
	FieldMeta `yaml:",inline"`
	Children  map[string]*Node `yaml:"children"`
}

// MetadataSource answers metadata for a field by its dot-joined path from the
// documented type's root ("source", "source.path"). The root itself is "".
// A zero FieldMeta means "nothing declared".
type MetadataSource interface {
	FieldMeta(path string) FieldMeta
}

// MetaFunc adapts a plain function to MetadataSource.
type MetaFunc func(path string) FieldMeta

// FieldMeta calls f.
func (f MetaFunc) FieldMeta(path string) FieldMeta { return f(path) }

// tree is a decoded metadata tree acting as a MetadataSource.
type tree map[string]*Node

// FieldMeta walks the tree by path segments.
func (t tree) FieldMeta(path string) FieldMeta {
	if path == "" {
		return FieldMeta{}
	}
	nodes := map[string]*Node(t)
	var meta FieldMeta
	for _, seg := range strings.Split(path, ".") {
		n := nodes[seg]
		if n == nil {
			return FieldMeta{}
		}
		meta, nodes = n.FieldMeta, n.Children
	}
	return meta
}
