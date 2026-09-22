package markdown

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lucasassuncao/docgen/spec"
)

// Column is one column of the arguments table.
type Column int

const (
	ColName Column = iota
	ColType
	ColFormat
	ColDescription
	ColRequired
	ColDefault
	ColValues      // the allowed values, from oneof
	ColConstraints // min, max, lengths, counts, pattern, unique
	ColDeprecated  // the migration hint
)

// DefaultColumns is the table when a Layout names none: Format appears only
// when some field in that table declares one.
var DefaultColumns = []Column{ColName, ColType, ColFormat, ColDescription, ColRequired, ColDefault}

// header is the column's title.
func (c Column) header() string {
	switch c {
	case ColName:
		return "Name"
	case ColType:
		return "Type"
	case ColFormat:
		return "Format"
	case ColDescription:
		return "Description"
	case ColRequired:
		return "Required"
	case ColDefault:
		return "Default"
	case ColValues:
		return "Valid values"
	case ColConstraints:
		return "Constraints"
	case ColDeprecated:
		return "Deprecated"
	}
	return ""
}

// rule is the separator row's dashes, as wide as the padded header cell.
func (c Column) rule() string { return strings.Repeat("-", len(c.header())+2) }

// optional reports whether the column is dropped from a table where no field
// fills it. Only Format behaves this way, to keep today's output.
func (c Column) optional() bool {
	switch c {
	case ColFormat, ColValues, ColConstraints, ColDeprecated:
		return true
	}
	return false
}

// cell renders one column for one field. displayName carries the linked name
// used by a split page's summary table.
func (c Column) cell(f spec.Field, m spec.FieldMeta, displayName string) string {
	switch c {
	case ColName:
		return displayName
	case ColType:
		return cellText(docTypeLabel(f, m))
	case ColFormat:
		return formatLabels(m)
	case ColDescription:
		return cellText(m.Description)
	case ColRequired:
		if m.Required {
			return "Yes"
		}
		return "No"
	case ColDefault:
		if m.Default == "" {
			return "-"
		}
		return cellText(m.Default)
	case ColValues:
		return joinCells(m.OneOf)
	case ColConstraints:
		return joinCells(constraints(m))
	case ColDeprecated:
		if m.Deprecated == "" {
			return "-"
		}
		return cellText(m.Deprecated)
	}
	return "-"
}

// filled reports whether this field gives the column anything to show.
func (c Column) filled(m spec.FieldMeta) bool {
	switch c {
	case ColFormat:
		return len(m.Formats) > 0
	case ColValues:
		return len(m.OneOf) > 0
	case ColConstraints:
		return len(constraints(m)) > 0
	case ColDeprecated:
		return m.Deprecated != ""
	}
	return true
}

// constraints lists the bounds a field declares, in a fixed order.
func constraints(m spec.FieldMeta) []string {
	var out []string
	add := func(label, v string) {
		if v != "" {
			out = append(out, label+" "+v)
		}
	}
	addN := func(label string, v int) {
		if v > 0 {
			out = append(out, label+" "+strconv.Itoa(v))
		}
	}
	add("min", m.Min)
	add("max", m.Max)
	addN("min length", m.MinLength)
	addN("max length", m.MaxLength)
	addN("min items", m.MinCount)
	addN("max items", m.MaxCount)
	if m.Unique {
		out = append(out, "unique")
	}
	add("pattern", codeSpan(m.Pattern))
	if len(m.NotOneOf) > 0 {
		out = append(out, "not "+strings.Join(m.NotOneOf, ", "))
	}
	return out
}

func codeSpan(s string) string {
	if s == "" {
		return ""
	}
	return "`" + cellText(s) + "`"
}

// joinCells renders a list as one cell, or "-" when empty.
func joinCells(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = cellText(v)
	}
	return strings.Join(out, ", ")
}

// visibleColumns drops the optional columns no field in this table fills.
func visibleColumns(cols []Column, metas []spec.FieldMeta) []Column {
	if len(cols) == 0 {
		cols = DefaultColumns
	}
	out := make([]Column, 0, len(cols))
	for _, c := range cols {
		if !c.optional() {
			out = append(out, c)
			continue
		}
		for _, m := range metas {
			if c.filled(m) {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

// writeHeader writes the header and separator rows.
func writeHeader(sb *strings.Builder, cols []Column) {
	titles := make([]string, len(cols))
	rules := make([]string, len(cols))
	for i, c := range cols {
		titles[i] = c.header()
		rules[i] = c.rule()
	}
	fmt.Fprintf(sb, "| %s |\n", strings.Join(titles, " | "))
	fmt.Fprintf(sb, "|%s|\n", strings.Join(rules, "|"))
}

// writeRow writes one field's row.
func writeRow(sb *strings.Builder, cols []Column, f spec.Field, m spec.FieldMeta, displayName string) {
	cells := make([]string, len(cols))
	for i, c := range cols {
		cells[i] = c.cell(f, m, displayName)
	}
	fmt.Fprintf(sb, "| %s |\n", strings.Join(cells, " | "))
}
