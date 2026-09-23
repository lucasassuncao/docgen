package examples

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasassuncao/docgen/presets"
	"github.com/lucasassuncao/docgen/spec"
)

type logConfig struct {
	Output string `yaml:"output"`
	Level  string `yaml:"level"`
}

type categoryConfig struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

func mustEmit(t *testing.T, dir string, src Source, titles map[string]string) []spec.GeneratedFile {
	t.Helper()
	files, err := Emit(src, dir, titles)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return files
}

func TestEmit_onePagePerField(t *testing.T) {
	src := presets.Combine(
		presets.ForField("configuration", map[string]*logConfig{
			"console": {Output: "console", Level: "info"},
			"file":    {Output: "file", Level: "warn"},
		}),
		presets.ForField("categories", map[string]*categoryConfig{
			"images": {Name: "photos", Path: "~/Downloads"},
		}),
	)
	titles := map[string]string{
		"configuration": "Configuration",
		"categories":    "Category",
	}

	dir := t.TempDir()
	files := mustEmit(t, dir, src, titles)

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	for _, name := range []string{"configuration.md", "category.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected file %s to exist: %v", name, err)
		}
	}
}

func TestEmit_pageHoldsEveryPreset(t *testing.T) {
	src := presets.ForField("configuration", map[string]*logConfig{
		"console": {Output: "console", Level: "info"},
		"file":    {Output: "file", Level: "warn"},
	})

	dir := t.TempDir()
	mustEmit(t, dir, src, map[string]string{"configuration": "Configuration"})

	data, err := os.ReadFile(filepath.Join(dir, "configuration.md")) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatalf("read configuration.md: %v", err)
	}
	body := string(data)

	for _, want := range []string{"# Configuration Examples", "## Preset: console", "## Preset: file", "```yaml"} {
		if !strings.Contains(body, want) {
			t.Errorf("configuration.md missing %q", want)
		}
	}
}

func TestEmit_skipsFieldNotInTitles(t *testing.T) {
	src := presets.Combine(
		presets.ForField("configuration", map[string]*logConfig{
			"console": {Output: "console", Level: "info"},
		}),
		presets.ForField("categories", map[string]*categoryConfig{
			"images": {Name: "photos", Path: "~/Downloads"},
		}),
	)

	dir := t.TempDir()
	files := mustEmit(t, dir, src, map[string]string{"configuration": "Configuration"})

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if _, err := os.Stat(filepath.Join(dir, "category.md")); err == nil {
		t.Error("category.md should not have been generated")
	}
}

func TestEmit_skipsFieldWithNoPresets(t *testing.T) {
	src := presets.ForField("configuration", map[string]*logConfig{})

	files := mustEmit(t, t.TempDir(), src, map[string]string{"configuration": "Configuration"})

	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(files))
	}
}

func TestEmit_writesNoReadme(t *testing.T) {
	src := presets.ForField("configuration", map[string]*logConfig{
		"console": {Output: "console", Level: "info"},
	})

	dir := t.TempDir()
	mustEmit(t, dir, src, map[string]string{"configuration": "Configuration"})

	if _, err := os.Stat(filepath.Join(dir, "README.md")); err == nil {
		t.Error("README.md should not be generated inside the examples dir")
	}
}

func TestEmit_filenameIsLowercasedTitle(t *testing.T) {
	src := presets.ForField("categories", map[string]*categoryConfig{
		"images": {Name: "photos", Path: "~/Downloads"},
	})

	dir := t.TempDir()
	files := mustEmit(t, dir, src, map[string]string{"categories": "Category"})

	if len(files) != 1 || files[0].Name != "Category" {
		t.Fatalf("expected 1 file named 'Category', got %v", files)
	}
	if _, err := os.Stat(filepath.Join(dir, "category.md")); err != nil {
		t.Error("expected file to be named category.md (lowercased title)")
	}
}

func TestEmit_nilSourceIsAnError(t *testing.T) {
	_, err := Emit(nil, t.TempDir(), map[string]string{"a": "A"})
	if !errors.Is(err, ErrNoPresetSource) {
		t.Fatalf("want ErrNoPresetSource, got %v", err)
	}
}
