<!-- markdownlint-disable MD033 -->
<p align="center">
  <img src="logo.png" alt="docgen logo" width="300" height="300">
</p>

<p align="center">
  <strong>Reference documentation generated from your Go config types.</strong><br>
  Markdown pages, a JSON Schema, example pages and an index, from the code that already exists.
</p>

<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white">
  <img alt="Dependencies" src="https://img.shields.io/badge/dependencies-1-brightgreen">
  <img alt="JSON Schema" src="https://img.shields.io/badge/JSON%20Schema-draft%202020--12-blue">
  <a href="https://pkg.go.dev/github.com/lucasassuncao/docgen"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/lucasassuncao/docgen.svg"></a>
</p>
<!-- markdownlint-enable MD033 -->

## Highlights

| | |
| --- | --- |
| 🪞 **One method, many readers** | `Metadata()` returns a plain `map[string]any`, not a package type. Any number of tools can read the same method, and none of them has to import another's types. |
| 🧬 **Structure from the code** | Field names, kinds and nesting come from reflection over yaml tags. You describe what a field *means*, never what it *is*. |
| 🔁 **Declared once** | Each type documents its own direct fields. A nested struct is picked up wherever it appears, so the declaration is never repeated, and a self-referential type just works. |
| 📐 **Layouts you choose** | One page per type, one per field, or everything in one file. Flat, or a directory per entry named after the config key. |
| 📊 **Columns you choose** | Nine of them. Show allowed values and constraints, or keep the five-column table you already have. |
| 🧾 **A schema a language server can use** | Draft 2020-12, with recursion as `$defs`/`$ref` instead of expansion, and constraints that have no keyword surfaced in `description` rather than dropped. |
| 🚨 **Typos fail loudly** | A metadata key that matches no field is an error naming the type and the key, at startup. It never becomes silently dead documentation. |
| 🧩 **Union types welcome** | A value that is a string *or* an object has no readable shape. Its metadata declares one by carrying `kind`. |
| 🙈 **Hide what is not public** | `Entry.Hidden` drops a field from every output at once. It stays valid in the file; it just stops being documented. |
| 🪶 **One dependency** | `gopkg.in/yaml.v3`. Nothing else. |

## Install

```bash
go get github.com/lucasassuncao/docgen
```

## Quick start

```go
type Config struct {
    Name    string   `yaml:"name"`
    Retries int      `yaml:"retries"`
    Tags    []string `yaml:"tags"`
}

func (Config) Metadata() map[string]any {
    return map[string]any{
        "name":    map[string]any{"description": "Job name.", "required": true},
        "retries": map[string]any{"description": "Attempts before giving up.", "default": "3", "max": "10"},
        "tags":    map[string]any{"description": "Free-form labels.", "unique": true},
    }
}

docgen.Generate(
    []docgen.Entry{{Config: Config{}}},
    docgen.WithMarkdown("docs/reference"),
    docgen.WithJSONSchema("docs/schema"),
)
```

Each type declares only its own direct fields. A nested struct that has its own `Metadata` is picked up wherever it appears, so the declaration is never repeated.

## Why a map

The method returns `map[string]any` rather than a package type so the same `Metadata()` can serve more than one consumer without any of them importing another's types. A Go method has one return type; a package type would force that choice on every reader.

The cost is that the compiler does not check the keys. It is checked at startup instead: an unknown key fails `Describe` with the type and the key named.

```
metadata for Config: line 3: field descriptoin not found in type docgen.Node
```

To keep the compiler in the loop, declare your own struct with yaml tags. The map is still plain, so nothing changes on this side:

```go
// your package - imports nothing
type meta struct {
    Description string   `yaml:"description,omitempty"`
    Required    bool     `yaml:"required,omitempty"`
    OneOf       []string `yaml:"oneof,omitempty"`
}

func (Config) Metadata() map[string]any {
    return map[string]any{
        "name": meta{Description: "Job name.", Required: true},
    }
}
```

Declare only the keys you use.

## Keys

| key | type | meaning |
| --- | --- | --- |
| `description` | string | prose shown in the docs |
| `type` | string | type label; derived from the field when empty |
| `required` | bool | field must be present |
| `default` | string | documented default |
| `oneof`, `notoneof` | []string | allowed / denied values |
| `min`, `max` | string | numeric, duration or size bounds |
| `pattern` | string | RE2 regular expression |
| `mincount`, `maxcount` | int | collection size bounds |
| `minlength`, `maxlength` | int | string length in code points |
| `unique` | bool | list items must not repeat |
| `formats` | []string | accepted string formats, OR semantics |
| `deprecated` | string | migration hint; marks the field deprecated |
| `multiline` | bool | documents the field as a multiline string |
| `children` | map | metadata for nested fields, when the type cannot declare it |

### Declaring a shape reflection cannot read

A union type, a value that is a string or an object, has no readable struct shape. Its `Metadata` entries declare one by carrying `kind`:

```go
func (MountOrString) Metadata() map[string]any {
    return map[string]any{
        "source": map[string]any{"kind": "primitive", "scalar": "string"},
        "target": map[string]any{"kind": "primitive", "scalar": "string"},
    }
}
```

`kind` is one of `primitive`, `object`, `list`, `dictionary`, `variant`, `any`.
`scalar` is one of `string`, `int`, `uint`, `bool`, `float`, `duration`.

### Keys docgen ignores

`presentation`, `snippet` and `prechecked` are accepted and ignored. A key another reader of the same `Metadata()` needs is not an error here.

## Outputs

Every output is opt-in; calling `Generate` with none is an error.

| option | writes |
| --- | --- |
| `WithMarkdown(dir, layout)` | reference pages, laid out by `layout` |
| `WithJSONSchema(dir)` | `<type>.schema.json`, draft 2020-12, recursion as `$defs`/`$ref` |
| `WithExamples(src, dir, titles)` | one page per preset field, linked from the reference pages |
| `WithIndex(dir)` | `README.md` linking the generated pages |
| `WithMetadata(src)` | overrides the metadata of every entry |

## Markdown layout

```go
docgen.WithMarkdown("docs/attributes", docgen.Layout{
    Folders: docgen.FolderPerEntry,  // or FolderFlat (default)
    Files:   docgen.FilePerField,    // or FilePerEntry (default), FileSingle
})
```

`Entry.Name` names the page, `Entry.Slug` the directory. Set `Slug` to the yaml key the struct answers to, so the directories read like the config file.

**`FolderFlat` + `FilePerEntry`**, the default:

```
docs/attributes/
├── configuration.md
└── category.md
```

**`FolderFlat` + `FilePerField`**:

```
docs/attributes/
├── configuration.md   ← links to its object fields
├── logging.md
├── watch.md
├── category.md
├── source.md
└── hooks.md
```

**`FolderPerEntry` + `FilePerField`**:

```
docs/attributes/
├── configuration/
│   └── configuration.md
└── categories/          ← Slug: "categories"
    ├── category.md      ← Name: "Category"
    ├── source.md
    ├── destination.md
    └── hooks.md
```

**`FileSingle`** ignores `Folders` and writes one `reference.md` holding every entry.

Only object-shaped fields get a page, and the split stops one level down: an object nested deeper stays inline in its parent's page. That is what keeps two same-named fields on different branches from overwriting each other.

`Entry.SplitStructs` overrides the file mode for one entry: `&false` keeps it on a single page while its neighbours split.

Every page the layout writes is listed in `WithIndex`, with links relative to where it landed.

### Columns

```go
docgen.Layout{Columns: []docgen.Column{
    docgen.ColName, docgen.ColDescription, docgen.ColValues, docgen.ColConstraints,
}}
```

| column | shows |
| --- | --- |
| `ColName`, `ColType`, `ColDescription`, `ColRequired`, `ColDefault` | the default five |
| `ColFormat` | the declared format names |
| `ColValues` | the allowed values, from `oneof` |
| `ColConstraints` | `min 0, max 10`, `min items 1, unique`, `` pattern `^x$` `` |
| `ColDeprecated` | the migration hint |

`ColFormat`, `ColValues`, `ColConstraints` and `ColDeprecated` disappear from a table no field in it fills.

### Deriving the entries

`EntriesOf` makes one entry per object-shaped top-level field, slugged by its yaml name:

```go
docgen.Generate(docgen.EntriesOf(models.Config{}),
    docgen.WithMarkdown(dir, docgen.Layout{Folders: docgen.FolderPerEntry}),
)
```

## Hiding a field

```go
docgen.Entry{Config: Config{}, Hidden: []string{"internal", "source.debug"}}
```

Dropped from the markdown *and* the schema. The field stays valid in the file, the schema never emits `additionalProperties: false`, it just loses its autocomplete.

## Supplying structure or metadata directly

`Entry.Fields` and `Entry.Metadata` replace what reflection would read:

```go
docgen.Entry{
    Name:     "Config",
    Fields:   []docgen.Field{{YAMLName: "name", Kind: docgen.KindPrimitive, Scalar: "string"}},
    Metadata: docgen.MetaFunc(func(path string) docgen.FieldMeta { ... }),
}
```

## How it is built

Each generator is its own package and can be called without the others. They share the vocabulary in `spec` and nothing else.

| package | does |
| --- | --- |
| [`spec`](spec) | `Field`, `FieldMeta`, `Node`, `Provider`, `Describe` |
| [`markdown`](markdown) | the reference pages, `Layout`, `Column` |
| [`jsonschema`](jsonschema) | the JSON Schema |
| [`examples`](examples) | the preset pages |
| [`index`](index) | the README linking them |
| [`presets`](presets) | building a preset source from Go structs |

```
spec        → (nothing)
markdown    → spec
jsonschema  → spec
examples    → spec
index       → spec
docgen      → all of them
```

## Non-goals

- **Rendering HTML or PDF.** Markdown and a JSON Schema are what a repository and a language server read.
- **Deriving metadata from struct tags.** A description belongs in prose, not in a tag; and tags cannot express what a nested type already declared.
- **Splitting recursively.** A page per object at any depth collides the moment two branches name a field the same. The split stops one level down.

## License

MIT
