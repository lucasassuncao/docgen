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
| **One method, many readers** | `Metadata()` returns a plain `map[string]any`. Any number of tools read it, none imports another's types. |
| **Structure from the code** | Names, kinds and nesting come from the yaml tags. You describe what a field *means*, never what it *is*. |
| **Declared once** | Each type documents its own direct fields. A nested struct is picked up wherever it appears; a recursive one just works. |
| **Layouts and columns you choose** | One page per type, per field, or one file. Show allowed values and constraints, or keep the default five columns. |
| **Typos fail loudly** | A key matching no field is an error naming the type and the key, at startup. It never becomes dead documentation. |
| **One dependency** | `go.yaml.in/yaml/v3`. |

## Install

```bash
go get github.com/lucasassuncao/docgen
```

## Usage

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

A map literal is not compiler-checked. Declare your own struct with yaml tags
and it is again, while `Metadata()` stays a plain map:

```go
// your package - imports nothing
type meta struct {
    Description string `yaml:"description,omitempty"`
    Required    bool   `yaml:"required,omitempty"`
}

"name": meta{Description: "Job name.", Required: true},
```

Declare only the keys you use. A key matching no field fails at startup:

```
metadata for Config: line 3: field descriptoin not found in type docgen.Node
```

## What comes out

[`demo/`](demo) is a small config documented end to end: the types and their
`Metadata()` in [`demo/main.go`](demo/main.go), the committed output in
[`demo/docs/`](demo/docs). Run it with `go run ./demo`.

The table of a `Config` whose `server` and `logging` are split onto pages of
their own:

```markdown
| Name | Type | Description | Required | Default |
|------|------|-------------|----------|---------|
| name | string | Name of the service, used in logs and metrics. | Yes | - |
| [server](./server.md) | object | Where the HTTP server binds and who may call it. | No | - |
| [logging](./logging.md) | object | Log destination and severity. | No | - |
```

And the same `logging.level` in the schema, from `oneof`, `default` and
`required`:

```json
"level": {
  "default": "info",
  "description": "Minimum severity to emit.",
  "enum": ["debug", "info", "warn", "error"],
  "type": "string"
}
```

## Keys

| key | type | meaning |
| --- | --- | --- |
| `description` | string | prose shown in the docs |
| `type` | string | type label; derived from the field when empty |
| `required` | bool | field must be present |
| `default` | string | documented default |
| `example` | string | yaml snippet shown verbatim |
| `oneof`, `notoneof` | []string | allowed / denied values |
| `min`, `max` | string | numeric, duration or size bounds |
| `pattern` | string | RE2 regular expression |
| `mincount`, `maxcount` | int | collection size bounds |
| `minlength`, `maxlength` | int | string length in code points |
| `unique` | bool | list items must not repeat |
| `formats` | []string | accepted string formats, OR semantics |
| `deprecated` | string | migration hint; marks the field deprecated |
| `multiline` | bool | documents the field as a multiline string |
| `children` | map | metadata for nested fields the type cannot declare |
| `kind`, `scalar` | string | the shape of a type reflection cannot read |

`presentation`, `snippet`, `prechecked` and `enablepicker` are accepted and ignored: a key
another reader of the same `Metadata()` needs is not an error here.

A union type — a value that is a string *or* an object — has no readable struct
shape. Its entries declare one by carrying `kind`, one of `primitive`, `object`,
`list`, `dictionary`, `variant`, `any`:

```go
func (MountOrString) Metadata() map[string]any {
    return map[string]any{
        "source": map[string]any{"kind": "primitive", "scalar": "string"},
        "target": map[string]any{"kind": "primitive", "scalar": "string"},
    }
}
```

## Outputs

Every output is opt-in; calling `Generate` with none is an error.

| option | writes |
| --- | --- |
| `WithMarkdown(dir, layout)` | reference pages, laid out by `layout` |
| `WithJSONSchema(dir)` | `<type>.schema.json`, draft 2020-12, recursion as `$defs`/`$ref` |
| `WithExamples(src, dir, titles)` | one page per preset field, linked from the reference pages |
| `WithIndex(dir)` | `README.md` linking the generated pages |
| `WithMetadata(src)` | overrides the metadata of every entry |

## Layout

```go
docgen.WithMarkdown("docs/attributes", docgen.Layout{
    Folders: docgen.FolderPerEntry,  // or FolderFlat (default)
    Files:   docgen.FilePerField,    // or FilePerEntry (default), FileSingle
    Columns: []docgen.Column{docgen.ColName, docgen.ColDescription, docgen.ColValues},
})
```

`Entry.Name` names the page, `Entry.Slug` the directory. Set `Slug` to the yaml
key the struct answers to, so the directories read like the config file:

```
docs/attributes/
├── configuration/
│   └── configuration.md
└── categories/          ← Slug: "categories"
    ├── category.md      ← Name: "Category"
    ├── source.md        ← FilePerField
    └── hooks.md
```

`FolderFlat` drops the directories, `FilePerEntry` the per-field pages, and
`FileSingle` writes one `reference.md` holding everything.

Only object-shaped fields get a page, and the split stops one level down: an
object nested deeper stays inline. That is what keeps two same-named fields on
different branches from overwriting each other.

Columns are `ColName`, `ColType`, `ColFormat`, `ColDescription`, `ColRequired`,
`ColDefault`, `ColValues`, `ColConstraints`, `ColDeprecated`. The last four,
plus `ColFormat`, disappear from a table no field in it fills. `nil` is the
default five.

## The rest of the surface

```go
docgen.EntriesOf(models.Config{})   // one entry per top-level object field
docgen.Entry{Hidden: []string{"internal", "source.debug"}}  // out of every output
docgen.Entry{SplitStructs: &no}     // override the layout for one entry
docgen.Entry{Fields: …, Metadata: …} // supply what reflection cannot read
```

A hidden field stays valid in the file — the schema never emits
`additionalProperties: false` — it just stops being documented.

## Non-goals

HTML or PDF output, and deriving metadata from struct tags.

## License

MIT
