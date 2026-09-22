package docgen_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasassuncao/docgen"
	"github.com/lucasassuncao/docgen/presets"
)

type dockerSettings struct {
	Image string `yaml:"image"`
}

type dockerConfig struct {
	Settings dockerSettings `yaml:"settings"`
}

type podmanSettings struct {
	Runtime string `yaml:"runtime"`
}

type podmanConfig struct {
	Settings podmanSettings `yaml:"settings"`
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func lsNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestGenerate_noOutputOptionIsAnError(t *testing.T) {
	_, err := docgen.Generate([]docgen.Entry{{Config: dockerConfig{}}})
	if !errors.Is(err, docgen.ErrNoOutput) {
		t.Fatalf("want ErrNoOutput, got %v", err)
	}
}

func TestGenerate_markdownOnlyWritesNoOtherOutput(t *testing.T) {
	dir := t.TempDir()
	files, err := docgen.Generate(
		[]docgen.Entry{{Config: dockerConfig{}}},
		docgen.WithMarkdown(dir),
	)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("want 1 file, got %d: %v", len(files), files)
	}
	if got := filepath.Base(files[0].Path); got != "dockerconfig.md" {
		t.Fatalf("want dockerconfig.md, got %s", got)
	}
	if files[0].Name != "dockerConfig" {
		t.Fatalf("want name dockerConfig, got %s", files[0].Name)
	}
	for _, name := range lsNames(t, dir) {
		if name == "README.md" || strings.HasSuffix(name, ".json") {
			t.Fatalf("unexpected file written: %s", name)
		}
	}
}

func TestGenerate_errorsOnDuplicateMarkdownPage(t *testing.T) {
	dir := t.TempDir()
	_, err := docgen.Generate(
		[]docgen.Entry{
			{Config: dockerConfig{}},
			{Config: podmanConfig{}},
		},
		docgen.WithMarkdown(dir, docgen.Layout{Files: docgen.FilePerField}),
	)
	if err == nil {
		t.Fatal("want duplicate page error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate docs page") {
		t.Fatalf("want duplicate docs page error, got %v", err)
	}
}

func TestGenerate_examplesCrossLinkUsesOneDeclaration(t *testing.T) {
	base := t.TempDir()
	docsDir := filepath.Join(base, "reference")
	examplesDir := filepath.Join(base, "examples")

	src := presets.Combine(
		presets.ForField("dockerconfig", map[string]*dockerSettings{
			"minimal": {Image: "alpine"},
		}),
	)

	_, err := docgen.Generate(
		[]docgen.Entry{{Config: dockerConfig{}}},
		docgen.WithMarkdown(docsDir),
		docgen.WithExamples(src, examplesDir, map[string]string{"dockerconfig": "dockerConfig"}),
	)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	page := readFile(t, filepath.Join(docsDir, "dockerconfig.md"))
	want := "[dockerConfig presets](../examples/dockerconfig.md)"
	if !strings.Contains(page, want) {
		t.Fatalf("want cross-link %q in page:\n%s", want, page)
	}
	if _, err := os.Stat(filepath.Join(examplesDir, "dockerconfig.md")); err != nil {
		t.Fatalf("example page not written: %v", err)
	}
}

func TestGenerate_markdownWithoutExamplesHasNoExamplesSection(t *testing.T) {
	dir := t.TempDir()
	if _, err := docgen.Generate(
		[]docgen.Entry{{Config: dockerConfig{}}},
		docgen.WithMarkdown(dir),
	); err != nil {
		t.Fatalf("generate: %v", err)
	}
	page := readFile(t, filepath.Join(dir, "dockerconfig.md"))
	if strings.Contains(page, "## Examples") {
		t.Fatalf("unexpected Examples section:\n%s", page)
	}
}

func TestGenerate_indexListsPagesAndExamplesOnly(t *testing.T) {
	base := t.TempDir()
	docsDir := filepath.Join(base, "reference")
	examplesDir := filepath.Join(base, "examples")
	schemaDir := filepath.Join(base, "schema")

	src := presets.Combine(
		presets.ForField("dockerconfig", map[string]*dockerSettings{
			"minimal": {Image: "alpine"},
		}),
	)

	if _, err := docgen.Generate(
		[]docgen.Entry{{Config: dockerConfig{}}},
		docgen.WithMarkdown(docsDir),
		docgen.WithJSONSchema(schemaDir),
		docgen.WithExamples(src, examplesDir, map[string]string{"dockerconfig": "dockerConfig"}),
		docgen.WithIndex(base),
	); err != nil {
		t.Fatalf("generate: %v", err)
	}

	index := readFile(t, filepath.Join(base, "README.md"))
	for _, want := range []string{
		"## Available Configurations",
		"[dockerConfig](./reference/dockerconfig.md)",
		"## Examples",
		"[dockerConfig](./examples/dockerconfig.md)",
	} {
		if !strings.Contains(index, want) {
			t.Fatalf("want %q in index:\n%s", want, index)
		}
	}
	if strings.Contains(index, ".schema.json") {
		t.Fatalf("index must not list schema files:\n%s", index)
	}
}

func TestGenerate_indexOmitsEmptySections(t *testing.T) {
	base := t.TempDir()
	if _, err := docgen.Generate(
		[]docgen.Entry{{Config: dockerConfig{}}},
		docgen.WithMarkdown(filepath.Join(base, "reference")),
		docgen.WithIndex(base),
	); err != nil {
		t.Fatalf("generate: %v", err)
	}
	index := readFile(t, filepath.Join(base, "README.md"))
	if strings.Contains(index, "## Examples") {
		t.Fatalf("empty Examples section must be omitted:\n%s", index)
	}
}

func TestGenerate_examplesWithoutSourceIsAnError(t *testing.T) {
	_, err := docgen.Generate(
		[]docgen.Entry{{Config: dockerConfig{}}},
		docgen.WithExamples(nil, t.TempDir(), map[string]string{"a": "A"}),
	)
	if !errors.Is(err, docgen.ErrNoPresetSource) {
		t.Fatalf("want ErrNoPresetSource, got %v", err)
	}
}

func TestGenerate_jsonSchemaOnlyWritesNoMarkdown(t *testing.T) {
	dir := t.TempDir()
	files, err := docgen.Generate(
		[]docgen.Entry{{Config: dockerConfig{}}},
		docgen.WithJSONSchema(dir),
	)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("want 1 file, got %d: %v", len(files), files)
	}
	if got := filepath.Base(files[0].Path); got != "dockerconfig.schema.json" {
		t.Fatalf("want dockerconfig.schema.json, got %s", got)
	}
	for _, name := range lsNames(t, dir) {
		if strings.HasSuffix(name, ".md") {
			t.Fatalf("unexpected markdown written: %s", name)
		}
	}

	content := readFile(t, files[0].Path)
	if !strings.HasSuffix(content, "\n") {
		t.Fatal("generated schema must end with a newline")
	}
	if !strings.Contains(content, `"$schema": "https://json-schema.org/draft/2020-12/schema"`) {
		t.Fatalf("missing dialect declaration:\n%s", content)
	}
	if strings.Contains(content, `"additionalProperties": false`) {
		t.Fatalf("additionalProperties must not be emitted:\n%s", content)
	}
}

func TestGenerate_errorsOnDuplicateJSONSchema(t *testing.T) {
	dir := t.TempDir()
	_, err := docgen.Generate(
		[]docgen.Entry{
			{Config: dockerConfig{}},
			{Config: dockerConfig{}},
		},
		docgen.WithJSONSchema(dir),
	)
	if err == nil || !strings.Contains(err.Error(), "duplicate json schema") {
		t.Fatalf("want duplicate json schema error, got %v", err)
	}
}

// TestLayout_folderPerEntry pins the layout movelooper depends on: each entry
// in a directory named after its slug, with the index above them and the
// example cross-links resolved relative to each entry's own directory.
func TestLayout_folderPerEntry(t *testing.T) {
	base := t.TempDir()
	attributesDir := filepath.Join(base, "attributes")
	examplesDir := filepath.Join(base, "examples")

	src := presets.Combine(
		presets.ForField("dockerconfig", map[string]*dockerSettings{
			"minimal": {Image: "alpine"},
		}),
	)

	files, err := docgen.Generate(
		[]docgen.Entry{
			{Config: dockerConfig{}, Slug: "docker"},
			{Config: podmanConfig{}, Slug: "podman"},
		},
		docgen.WithMarkdown(attributesDir, docgen.Layout{
			Folders: docgen.FolderPerEntry,
			Files:   docgen.FilePerField,
		}),
		docgen.WithExamples(src, examplesDir, map[string]string{"dockerconfig": "dockerConfig"}),
		docgen.WithIndex(base),
	)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	for _, want := range []string{
		filepath.Join(attributesDir, "docker", "dockerconfig.md"),
		filepath.Join(attributesDir, "podman", "podmanconfig.md"),
		filepath.Join(attributesDir, "podman", "settings.md"),
	} {
		if _, err := os.Stat(want); err != nil {
			t.Fatalf("expected %s: %v (got %v)", want, err, files)
		}
	}
	if _, err := os.Stat(filepath.Join(attributesDir, "dockerconfig.md")); err == nil {
		t.Fatal("FolderPerEntry must not also write to the markdown dir")
	}

	// Both entries sit one level below attributesDir, so each link back to the
	// examples directory must climb two levels.
	page := readFile(t, filepath.Join(attributesDir, "docker", "dockerconfig.md"))
	want := "[dockerConfig presets](../../examples/dockerconfig.md)"
	if !strings.Contains(page, want) {
		t.Fatalf("want cross-link %q relative to the entry's own dir:\n%s", want, page)
	}

	index := readFile(t, filepath.Join(base, "README.md"))
	for _, link := range []string{
		"(./attributes/docker/dockerconfig.md)",
		"(./attributes/podman/podmanconfig.md)",
	} {
		if !strings.Contains(index, link) {
			t.Fatalf("want %q in index:\n%s", link, index)
		}
	}
}

// TestLayout_flatIsTheDefault pins that without a Layout every entry writes one
// page straight into the markdown directory.
func TestLayout_flatIsTheDefault(t *testing.T) {
	dir := t.TempDir()
	if _, err := docgen.Generate(
		[]docgen.Entry{{Config: dockerConfig{}}, {Config: podmanConfig{}}},
		docgen.WithMarkdown(dir),
	); err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, want := range []string{"dockerconfig.md", "podmanconfig.md"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Fatalf("expected %s: %v", want, err)
		}
	}
	// FilePerEntry inlines nested structs rather than splitting them out.
	if _, err := os.Stat(filepath.Join(dir, "settings.md")); err == nil {
		t.Fatal("FilePerEntry must not split nested structs into their own page")
	}
}

// TestLayout_fileSingle pins that every entry lands in one page.
func TestLayout_fileSingle(t *testing.T) {
	dir := t.TempDir()
	files, err := docgen.Generate(
		[]docgen.Entry{{Config: dockerConfig{}}, {Config: podmanConfig{}}},
		docgen.WithMarkdown(dir, docgen.Layout{Files: docgen.FileSingle}),
	)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("want one page, got %v", files)
	}
	page := readFile(t, filepath.Join(dir, "reference.md"))
	for _, want := range []string{"# dockerConfig", "# podmanConfig"} {
		if !strings.Contains(page, want) {
			t.Fatalf("want %q in the single page:\n%s", want, page)
		}
	}
}

// TestEntriesOf derives one entry per object-shaped top-level field, named the
// way the config file names it.
func TestEntriesOf(t *testing.T) {
	type root struct {
		Name    string         `yaml:"name"`
		Docker  dockerConfig   `yaml:"docker-settings"`
		Podmans []podmanConfig `yaml:"podman-settings"`
	}
	entries := docgen.EntriesOf(root{})
	if len(entries) != 2 {
		t.Fatalf("want two entries, got %+v", entries)
	}
	if entries[0].Slug != "docker-settings" || entries[0].Name != "dockerConfig" {
		t.Errorf("entry 0 = %+v", entries[0])
	}
	if entries[1].Slug != "podman-settings" || entries[1].Name != "podmanConfig" {
		t.Errorf("entry 1 = %+v", entries[1])
	}
}

// metaConfig declares its fields through Metadata, the way an application does.
type metaConfig struct {
	Name    string   `yaml:"name"`
	Retries int      `yaml:"retries"`
	Tags    []string `yaml:"tags"`
}

func (metaConfig) Metadata() map[string]any {
	return map[string]any{
		"name":    map[string]any{"description": "Job name.", "required": true, "example": "name: nightly"},
		"retries": map[string]any{"description": "Attempts before giving up.", "default": "3", "max": "10"},
		"tags":    map[string]any{"description": "Free-form labels.", "unique": true},
	}
}

func TestGenerate_usesMetadataFromTheType(t *testing.T) {
	dir := t.TempDir()
	files, err := docgen.Generate(
		[]docgen.Entry{{Config: metaConfig{}}},
		docgen.WithMarkdown(dir),
		docgen.WithJSONSchema(dir),
	)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("want a markdown page and a schema, got %d: %+v", len(files), files)
	}

	md, err := os.ReadFile(filepath.Join(dir, "metaconfig.md"))
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	for _, want := range []string{"Job name.", "Attempts before giving up.", "Free-form labels."} {
		if !strings.Contains(string(md), want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}

	schema, err := os.ReadFile(filepath.Join(dir, "metaconfig.schema.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	for _, want := range []string{`"required"`, `"Job name."`, `"maximum": 10`, `"uniqueItems": true`} {
		if !strings.Contains(string(schema), want) {
			t.Errorf("schema missing %s:\n%s", want, schema)
		}
	}
}

// TestLayout_entryOverridesSplit pins that one entry can opt out of the
// layout's split while its neighbour keeps it.
func TestLayout_entryOverridesSplit(t *testing.T) {
	base := t.TempDir()
	no := false
	if _, err := docgen.Generate(
		[]docgen.Entry{
			{Config: dockerConfig{}, Slug: "docker", SplitStructs: &no},
			{Config: podmanConfig{}, Slug: "podman"},
		},
		docgen.WithMarkdown(base, docgen.Layout{
			Folders: docgen.FolderPerEntry,
			Files:   docgen.FilePerField,
		}),
	); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "docker", "settings.md")); err == nil {
		t.Error("SplitStructs=false must keep nested structs inline")
	}
	if _, err := os.Stat(filepath.Join(base, "podman", "settings.md")); err != nil {
		t.Errorf("the other entry must still follow the layout: %v", err)
	}
}

// constrained declares the keys that never reached the markdown before columns
// were configurable.
type constrained struct {
	Level   string   `yaml:"level"`
	Retries int      `yaml:"retries"`
	Tags    []string `yaml:"tags"`
	Secret  string   `yaml:"secret"`
}

func (constrained) Metadata() map[string]any {
	return map[string]any{
		"level": map[string]any{
			"description": "Log level.",
			"oneof":       []string{"debug", "info", "warn"},
			"example":     "level: info",
		},
		"retries": map[string]any{"description": "Attempts.", "min": "0", "max": "10"},
		"tags":    map[string]any{"description": "Labels.", "mincount": 1, "unique": true},
		"secret":  map[string]any{"description": "Internal."},
	}
}

func TestColumns_defaultKeepsTheFiveColumns(t *testing.T) {
	dir := t.TempDir()
	if _, err := docgen.Generate(
		[]docgen.Entry{{Config: constrained{}}},
		docgen.WithMarkdown(dir),
	); err != nil {
		t.Fatalf("generate: %v", err)
	}
	page := readFile(t, filepath.Join(dir, "constrained.md"))
	if !strings.Contains(page, "| Name | Type | Description | Required | Default |") {
		t.Fatalf("default header changed:\n%s", page)
	}
	for _, unwanted := range []string{"debug, info, warn", "min 0", "min items 1"} {
		if strings.Contains(page, unwanted) {
			t.Errorf("default table must not show %q:\n%s", unwanted, page)
		}
	}
}

func TestColumns_valuesAndConstraints(t *testing.T) {
	dir := t.TempDir()
	if _, err := docgen.Generate(
		[]docgen.Entry{{Config: constrained{}}},
		docgen.WithMarkdown(dir, docgen.Layout{Columns: []docgen.Column{
			docgen.ColName, docgen.ColDescription, docgen.ColValues, docgen.ColConstraints,
		}}),
	); err != nil {
		t.Fatalf("generate: %v", err)
	}
	page := readFile(t, filepath.Join(dir, "constrained.md"))
	for _, want := range []string{
		"| Name | Description | Valid values | Constraints |",
		"debug, info, warn",
		"min 0, max 10",
		"min items 1, unique",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("want %q:\n%s", want, page)
		}
	}
}

func TestEntry_hiddenLeavesEveryOutput(t *testing.T) {
	dir := t.TempDir()
	if _, err := docgen.Generate(
		[]docgen.Entry{{Config: constrained{}, Hidden: []string{"secret"}}},
		docgen.WithMarkdown(dir),
		docgen.WithJSONSchema(dir),
	); err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, name := range []string{"constrained.md", "constrained.schema.json"} {
		if got := readFile(t, filepath.Join(dir, name)); strings.Contains(got, "secret") {
			t.Errorf("%s still mentions the hidden field:\n%s", name, got)
		}
	}
	// The rest is untouched.
	if got := readFile(t, filepath.Join(dir, "constrained.md")); !strings.Contains(got, "level") {
		t.Error("hiding one field must not drop the others")
	}
}
