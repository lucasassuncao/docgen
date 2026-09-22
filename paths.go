package docgen

import "reflect"

// typeName returns the Go type name of v with pointers dereferenced. It is an
// entry's display title when Name is empty.
func typeName(v any) string {
	t := reflect.TypeOf(v)
	if t == nil {
		return ""
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name()
}
