package spec

// Kind classifies a discovered field's shape.
type Kind int

const (
	KindPrimitive  Kind = iota // scalar: string, int, bool, …
	KindObject                 // struct with typed fields
	KindList                   // slice or array
	KindDictionary             // map[K]V
	KindVariant                // union type declared through Metadata
	KindAny                    // interface{}/any
)

// kindNames maps the metadata spelling of a kind to its value. Declaring
// "kind" in a Metadata entry is how a union type describes a shape reflection
// cannot read.
var kindNames = map[string]Kind{
	"primitive":  KindPrimitive,
	"object":     KindObject,
	"list":       KindList,
	"dictionary": KindDictionary,
	"variant":    KindVariant,
	"any":        KindAny,
}

// Field describes one documented field: its name, its shape, and the fields
// nested under it. It carries structure only - what the field means lives in
// FieldMeta.
type Field struct {
	YAMLName string
	Kind     Kind
	// TypeName is the struct's name, or a collection's element type. Set even
	// when the recursion limit leaves Children empty, which is what tells a
	// recursive type from a truncated one.
	TypeName     string
	Scalar       string // scalar type for primitives ("string", "int", "bool", "float", "duration", "uint")
	Children     []Field
	OmitEmpty    bool   // yaml:",omitempty"
	Flow         bool   // yaml:",flow"
	MapKeyScalar string // KindDictionary only: scalar type of the map key
	ElemScalar   string // KindList/KindDictionary: scalar type of the element, empty when not a scalar
}

// TopLevelOrder returns the yaml names of fields in declaration order.
func TopLevelOrder(fields []Field) []string {
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		out = append(out, f.YAMLName)
	}
	return out
}
