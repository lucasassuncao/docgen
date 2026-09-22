// Command demo generates the documentation under demo/docs from the config
// types below. It is what the README points at: the input on this side, the
// committed output on the other.
//
// Run it from the repository root: go run ./demo
package main

import (
	"fmt"
	"os"

	"github.com/lucasassuncao/docgen"
)

// ── The config types ──────────────────────────────────────────────────────────

type Config struct {
	Name    string  `yaml:"name"`
	Server  Server  `yaml:"server"`
	Logging Logging `yaml:"logging"`
}

type Server struct {
	Host    string   `yaml:"host"`
	Port    int      `yaml:"port"`
	Origins []string `yaml:"origins,omitempty"`
}

type Logging struct {
	Level string `yaml:"level"`
	File  string `yaml:"file,omitempty"`
}

// ── What the fields mean ──────────────────────────────────────────────────────

// meta keeps the entries compiler-checked. It declares only the keys this
// config uses; the yaml names are the contract, not the type.
type meta struct {
	Description string   `yaml:"description,omitempty"`
	Required    bool     `yaml:"required,omitempty"`
	Default     string   `yaml:"default,omitempty"`
	Example     string   `yaml:"example,omitempty"`
	OneOf       []string `yaml:"oneof,omitempty"`
	Min         string   `yaml:"min,omitempty"`
	Max         string   `yaml:"max,omitempty"`
	Unique      bool     `yaml:"unique,omitempty"`
	Formats     []string `yaml:"formats,omitempty"`
}

// Each type declares only its own direct fields. Server and Logging are picked
// up wherever they appear, so nothing here repeats them.
func (Config) Metadata() map[string]any {
	return map[string]any{
		"name": meta{
			Description: "Name of the service, used in logs and metrics.",
			Required:    true,
			Example:     "name: billing-api",
		},
		"server":  meta{Description: "Where the HTTP server binds and who may call it."},
		"logging": meta{Description: "Log destination and severity."},
	}
}

func (Server) Metadata() map[string]any {
	return map[string]any{
		"host": meta{
			Description: "Address to bind. Use 0.0.0.0 to accept connections from outside the host.",
			Default:     "localhost",
			Example:     "host: 0.0.0.0",
		},
		"port": meta{
			Description: "TCP port to listen on. Ports below 1024 need elevated privileges.",
			Default:     "8080",
			Min:         "1",
			Max:         "65535",
			Example:     "port: 8080",
		},
		"origins": meta{
			Description: "Origins allowed by CORS. Empty allows none.",
			Unique:      true,
			Formats:     []string{"url"},
			Example:     "origins:\n  - https://app.example.com",
		},
	}
}

func (Logging) Metadata() map[string]any {
	return map[string]any{
		"level": meta{
			Description: "Minimum severity to emit.",
			Required:    true,
			OneOf:       []string{"debug", "info", "warn", "error"},
			Default:     "info",
			Example:     "level: info",
		},
		"file": meta{
			Description: "Path to the log file. Empty writes to stdout.",
			Formats:     []string{"directory"},
			Example:     "file: /var/log/billing-api.log",
		},
	}
}

// ── Generating ────────────────────────────────────────────────────────────────

func main() {
	files, err := docgen.Generate(
		[]docgen.Entry{{Config: Config{}}},
		docgen.WithMarkdown("demo/docs/reference", docgen.Layout{
			Files: docgen.FilePerField,
		}),
		docgen.WithJSONSchema("demo/docs/schema"),
		docgen.WithIndex("demo/docs"),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		os.Exit(1)
	}
	for _, f := range files {
		fmt.Println(f.Path)
	}
}
