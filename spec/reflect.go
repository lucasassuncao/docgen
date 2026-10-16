package spec

import (
	"bytes"
	"encoding"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// defaultSkip lists yaml keys never documented.
var defaultSkip = map[string]bool{"$schema": true}

var (
	yamlMarshalerType = reflect.TypeOf((*yaml.Marshaler)(nil)).Elem()
	textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
	providerType      = reflect.TypeOf((*Provider)(nil)).Elem()
)

// Options tunes a walk.
type Options struct {
	// RecursionLimit is how many extra times a type may re-enter the walk.
	// Zero is the default of 1; use -1 for strict, stopping at the second visit.
	RecursionLimit int
	// Hidden are dot-joined field paths to leave out entirely - of the fields,
	// of the metadata, and so of every generator downstream.
	Hidden []string
}

// Describe returns v's documented fields and the metadata every type along the
// way declares, so a nested struct declares its fields once. recursionLimit is
// how many extra times a type may re-enter the walk; omitted it is 1.
func Describe(v any, recursionLimit ...int) ([]Field, MetadataSource, error) {
	opts := Options{}
	if len(recursionLimit) > 0 {
		opts.RecursionLimit = recursionLimit[0]
		if recursionLimit[0] == 0 {
			opts.RecursionLimit = -1
		}
	}
	return DescribeWith(v, opts)
}

// DescribeWith is Describe with the walk's options named.
func DescribeWith(v any, opts Options) ([]Field, MetadataSource, error) {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil, nil, nil
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	limit := opts.RecursionLimit
	switch {
	case limit == 0:
		limit = 1
	case limit < 0:
		limit = 0
	}
	hidden := make(map[string]bool, len(opts.Hidden))
	for _, p := range opts.Hidden {
		hidden[p] = true
	}
	w := &walker{
		limit:  limit,
		hidden: hidden,
		seen:   map[reflect.Type]int{},
		cache:  map[reflect.Type]map[string]*Node{},
	}
	fields, nodes := w.walk(t, 0, nil)
	if w.err != nil {
		return nil, nil, w.err
	}
	return fields, tree(nodes), nil
}

// Reflect is Describe without its metadata, for callers that supply their own.
func Reflect(v any, recursionLimit ...int) []Field {
	fields, _, _ := Describe(v, recursionLimit...)
	return fields
}

// walker carries one Describe run: the recursion budget, what to leave out, the
// types currently on the stack, and each type's decoded metadata.
type walker struct {
	limit  int
	hidden map[string]bool
	seen   map[reflect.Type]int
	cache  map[reflect.Type]map[string]*Node
	err    error
}

// walk returns the fields of a struct type and the metadata tree that mirrors
// them. The two are built together so the tree is never deeper than the fields
// a recursion limit allowed.
func (w *walker) walk(t reflect.Type, depth int, path []string) ([]Field, map[string]*Node) {
	if w.seen[t] > w.limit || depth > 20 || t.Kind() != reflect.Struct {
		return nil, nil
	}
	w.seen[t]++
	defer func() { w.seen[t]-- }()

	declared := w.metaOf(t)
	var fields []Field
	nodes := map[string]*Node{}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		yamlTag := f.Tag.Get("yaml")
		name := strings.Split(yamlTag, ",")[0]
		inline := name == "" && strings.Contains(yamlTag, "inline")

		if !f.IsExported() {
			// yaml.v3 serialises an unexported embed only through yaml:",inline".
			if f.Anonymous && inline {
				w.promote(f, depth, path, &fields, nodes)
			}
			continue
		}
		if name == "-" {
			continue
		}
		if inline {
			w.promote(f, depth, path, &fields, nodes)
			continue
		}
		if name == "" {
			if yamlTag == "" && !f.Anonymous {
				continue // untagged plain fields are not documented
			}
			name = strings.ToLower(f.Name)
		}
		if defaultSkip[name] {
			continue
		}

		field := buildField(f, name, yamlTag)
		full := ChildPath(path, name)
		if w.hidden[strings.Join(full, ".")] {
			continue
		}
		childFields, childNodes := w.children(f.Type, depth, full)
		if childFields != nil {
			field.Children = childFields
		}
		fields = append(fields, field)
		nodes[name] = merge(declared[name], childNodes)
	}
	return fields, nodes
}

// promote folds an inline embed's fields and metadata into its parent.
func (w *walker) promote(f reflect.StructField, depth int, path []string, fields *[]Field, nodes map[string]*Node) {
	ft := f.Type
	for ft.Kind() == reflect.Pointer {
		ft = ft.Elem()
	}
	if ft.Kind() != reflect.Struct {
		return
	}
	sub, subNodes := w.walk(ft, depth+1, path)
	*fields = append(*fields, sub...)
	for k, v := range subNodes {
		nodes[k] = v
	}
}

// children resolves what nests under a field: the shape a Provider declares,
// or the struct reflection finds. A type that serialises as a scalar has none.
func (w *walker) children(ft reflect.Type, depth int, path []string) ([]Field, map[string]*Node) {
	if variant, nodes := w.variantOf(ft); variant != nil {
		return variant, nodes
	}
	if isMarshalerType(ft) {
		return nil, nil
	}
	nested := unwrap(ft)
	if nested.Kind() != reflect.Struct {
		return nil, nil
	}
	return w.walk(nested, depth+1, path)
}

// variantOf builds the fields of a union type: one whose Metadata entries
// declare a "kind", because reflection cannot read its shape.
func (w *walker) variantOf(ft reflect.Type) ([]Field, map[string]*Node) {
	t := unwrapAll(ft)
	if t.Kind() == reflect.Interface {
		return nil, nil
	}
	declared := w.metaOf(t)
	fields := make([]Field, 0, len(declared))
	for _, name := range sortedKeys(declared) {
		n := declared[name]
		kind, ok := kindNames[n.Kind]
		if !ok {
			return nil, nil // not a shape declaration; ordinary metadata
		}
		fields = append(fields, Field{YAMLName: name, Kind: kind, Scalar: n.Scalar})
	}
	if len(fields) == 0 {
		return nil, nil
	}
	return fields, declared
}

// metaOf decodes a type's Metadata once and remembers it. A key that matches
// no FieldMeta field is an error: a typo must not become dead metadata.
func (w *walker) metaOf(t reflect.Type) map[string]*Node {
	if nodes, ok := w.cache[t]; ok {
		return nodes
	}
	w.cache[t] = nil // guard against a recursive type re-entering

	raw := providerMap(t)
	if raw == nil {
		return nil
	}
	nodes, err := decode(raw)
	if err != nil {
		w.err = fmt.Errorf("metadata for %s: %w", t.Name(), err)
		return nil
	}
	w.cache[t] = nodes
	return nodes
}

// providerMap calls Metadata on a zero value of t, or returns nil when t does
// not implement Provider.
func providerMap(t reflect.Type) map[string]any {
	switch {
	case t.Implements(providerType):
		return reflect.New(t).Elem().Interface().(Provider).Metadata()
	case reflect.PointerTo(t).Implements(providerType):
		return reflect.New(t).Interface().(Provider).Metadata()
	}
	return nil
}

// decode turns the plain map a Provider returns into a metadata tree. It goes
// through yaml so the FieldMeta tags do the mapping, and rejects unknown keys.
func decode(raw map[string]any) (map[string]*Node, error) {
	b, err := yaml.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var nodes map[string]*Node
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

// merge lays a field's own declaration over the metadata its type declares.
// The field's entry wins; the type fills in what the field left unsaid.
func merge(own *Node, fromType map[string]*Node) *Node {
	out := &Node{}
	if own != nil {
		out.FieldMeta = own.FieldMeta
	}
	if len(fromType) == 0 && (own == nil || len(own.Children) == 0) {
		return out
	}
	out.Children = map[string]*Node{}
	for k, v := range fromType {
		out.Children[k] = v
	}
	if own != nil {
		for k, v := range own.Children {
			out.Children[k] = v
		}
	}
	return out
}

// buildField reads a struct field's yaml tag into a Field.
func buildField(f reflect.StructField, name, yamlTag string) Field {
	out := Field{YAMLName: name, Kind: kindOf(f.Type), Scalar: scalarLabel(f.Type)}
	for _, opt := range strings.Split(yamlTag, ",")[1:] {
		switch strings.TrimSpace(opt) {
		case "omitempty":
			out.OmitEmpty = true
		case "flow":
			out.Flow = true
		}
	}
	if out.Kind == KindDictionary || out.Kind == KindList {
		ft := f.Type
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		switch ft.Kind() {
		case reflect.Map:
			out.MapKeyScalar = scalarLabel(ft.Key())
			out.ElemScalar = scalarLabel(ft.Elem())
		case reflect.Slice, reflect.Array:
			out.ElemScalar = scalarLabel(ft.Elem())
		}
	}
	// TypeName is set even when the recursion limit leaves Children empty, so a
	// consumer can tell a recursive type from a truncated one.
	if nested := unwrap(f.Type); nested.Kind() == reflect.Struct && !isMarshalerType(f.Type) {
		out.TypeName = nested.Name()
	}
	return out
}

// yamlNameOf returns the key a field serialises under, or "" when it has none
// that this package documents.
func yamlNameOf(f reflect.StructField) string {
	tag := f.Tag.Get("yaml")
	name := strings.Split(tag, ",")[0]
	if name == "-" {
		return ""
	}
	if name == "" {
		if tag == "" && !f.Anonymous {
			return ""
		}
		name = strings.ToLower(f.Name)
	}
	return name
}

// isMarshalerType reports whether t serialises as a scalar, in which case its
// struct fields are not documented.
func isMarshalerType(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Implements(yamlMarshalerType) ||
		reflect.PointerTo(t).Implements(yamlMarshalerType) ||
		t.Implements(textMarshalerType) ||
		reflect.PointerTo(t).Implements(textMarshalerType)
}

// kindOf classifies a Go type.
func kindOf(t reflect.Type) Kind {
	if t.Kind() == reflect.Pointer {
		return kindOf(t.Elem())
	}
	if t.Kind() == reflect.Interface {
		return KindAny
	}
	if isMarshalerType(t) {
		return KindPrimitive
	}
	switch t.Kind() {
	case reflect.Struct:
		return KindObject
	case reflect.Slice, reflect.Array:
		return KindList
	case reflect.Map:
		return KindDictionary
	default:
		return KindPrimitive
	}
}

// scalarLabel names a scalar Go type, or returns "" when t is not one. Named
// types with their own meaning (time.Duration) win over their underlying kind.
func scalarLabel(t reflect.Type) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.PkgPath() == "time" && t.Name() == "Duration" {
		return "duration"
	}
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "bool"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "int"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "uint"
	case reflect.Float32, reflect.Float64:
		return "float"
	default:
		return ""
	}
}

// unwrap strips pointer, slice and map wrappers to reach a struct worth
// descending into.
func unwrap(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() == reflect.Map {
		return unwrap(t.Elem())
	}
	return t
}

// unwrapAll strips every wrapper, including maps, in one pass.
func unwrapAll(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array || t.Kind() == reflect.Map {
		t = t.Elem()
	}
	return t
}

// sortedKeys returns a map's keys in declaration-stable order.
func sortedKeys(m map[string]*Node) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// Object is one object-shaped top-level field of a struct: what it is called in
// the file, what its Go type is called, and a zero value of that type.
type Object struct {
	YAMLName string
	TypeName string
	Zero     any
}

// TopLevelObjects returns the object-shaped top-level fields of root. A scalar
// field, or one whose element is not a struct, is skipped: there is no type to
// document on its own.
func TopLevelObjects(root any) []Object {
	t := reflect.TypeOf(root)
	if t == nil {
		return nil
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	var out []Object
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := ""
		if f.IsExported() {
			name = yamlNameOf(f)
		}
		if name == "" || defaultSkip[name] {
			continue
		}
		elem := unwrap(f.Type)
		if elem.Kind() != reflect.Struct || isMarshalerType(f.Type) {
			continue
		}
		out = append(out, Object{
			YAMLName: name,
			TypeName: elem.Name(),
			Zero:     reflect.New(elem).Elem().Interface(),
		})
	}
	return out
}
