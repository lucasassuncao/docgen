package docgen

import "github.com/lucasassuncao/docgen/spec"

// EntriesOf makes one Entry per object-shaped top-level field of root, so a
// config file documents itself: the slug is the field's yaml name, the title
// its Go type name. root itself is never an entry.
func EntriesOf(root any) []Entry {
	objects := spec.TopLevelObjects(root)
	out := make([]Entry, 0, len(objects))
	for _, o := range objects {
		out = append(out, Entry{Config: o.Zero, Name: o.TypeName, Slug: o.YAMLName})
	}
	return out
}
