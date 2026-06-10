package binder

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// This file renders help output in the format of go-flags v1.6.1, which
// the concourse command used before the cobra migration: a single global
// description column, `--flag=VALUE[choice|choice]` option lines,
// `(default: x) [$ENV_VAR]` description suffixes, and group headings.
// The one deliberate difference: on terminals too narrow for the
// description column, descriptions drop onto their own wrapped lines
// instead of go-flags' unclamped column, which rendered as garbage.

const (
	paddingBeforeOption                 = 2
	commandOptionIndent                 = 4
	distanceBetweenOptionAndDescription = 2

	// below this much room for descriptions, switch to the narrow layout
	minDescriptionWidth = 20
	narrowIndent        = 8
)

// RootOption is a flag living on the root command (-v/--version,
// -h/--help), rendered above the command options under its own section
// heading but sharing the global description column.
type RootOption struct {
	Section     string
	Short       string
	Long        string
	Description string
}

// UsageOptions configures WriteUsage.
type UsageOptions struct {
	// CommandName names the `[<name> command options]` heading.
	CommandName string

	// Width is the terminal width to wrap at; <= 0 falls back to 80,
	// like go-flags.
	Width int

	// RootOptions are rendered before the command options, grouped by
	// their Section in order of first appearance.
	RootOptions []RootOption
}

type alignment struct {
	descStart int
	hasShort  bool
	narrow    bool
	width     int
}

// WriteUsage writes the flag portion of the help text: the root option
// sections, the `[<name> command options]` heading, and each group
// section with its heading, all sharing one description column.
func (b *Binder) WriteUsage(w io.Writer, opts UsageOptions) {
	width := opts.Width
	if width <= 0 {
		width = 80
	}

	a := b.alignmentInfo(width, opts.RootOptions)

	var lastSection string
	for _, opt := range opts.RootOptions {
		if opt.Section != lastSection {
			if lastSection != "" {
				fmt.Fprintln(w)
			}
			fmt.Fprintf(w, "%s:\n", opt.Section)
			lastSection = opt.Section
		}
		b.writeOptionLine(w, a, false, opt.Short, opt.Long, "", opt.Description)
	}

	if len(b.ordered) == 0 {
		return
	}

	fmt.Fprintf(w, "\n[%s command options]\n", opts.CommandName)

	for _, sec := range b.sections {
		flags := sec.visibleFlags(b)
		if len(flags) == 0 {
			continue
		}

		if sec.title != "" {
			fmt.Fprintf(w, "\n    %s:\n", sec.title)
		}

		for _, bf := range flags {
			left := bf.name + argumentSuffix(bf)
			b.writeOptionLine(w, a, true, b.fs.Lookup(bf.name).Shorthand, left, descriptionSuffix(bf), b.fs.Lookup(bf.name).Usage)
		}
	}
}

func (sec *usageSection) visibleFlags(b *Binder) []*boundFlag {
	visible := make([]*boundFlag, 0, len(sec.flags))
	for _, bf := range sec.flags {
		if !b.fs.Lookup(bf.name).Hidden {
			visible = append(visible, bf)
		}
	}
	return visible
}

// argumentSuffix renders the value part of the option: `=`, the
// value-name, and the choice list, exactly as go-flags did. Boolean
// flags take no argument and get no suffix.
func argumentSuffix(bf *boundFlag) string {
	if bf.value.boolish {
		return ""
	}

	suffix := "=" + bf.value.valueName
	if len(bf.value.choices) > 0 {
		suffix += "[" + strings.Join(bf.value.choices, "|") + "]"
	}
	return suffix
}

// descriptionSuffix renders ` (default: a, b) [$ENV_KEY]`.
func descriptionSuffix(bf *boundFlag) string {
	var suffix string

	if len(bf.defaults) > 0 {
		quoted := make([]string, len(bf.defaults))
		for i, d := range bf.defaults {
			quoted[i] = quoteIfNeeded(d)
		}
		suffix += fmt.Sprintf(" (default: %s)", strings.Join(quoted, ", "))
	}

	if bf.envKey != "" {
		suffix += fmt.Sprintf(" [$%s]", bf.envKey)
	}

	return suffix
}

func quoteIfNeeded(s string) string {
	for _, c := range s {
		if !strconv.IsPrint(c) {
			return strconv.Quote(s)
		}
	}
	return s
}

func (b *Binder) alignmentInfo(width int, rootOpts []RootOption) alignment {
	a := alignment{width: width}

	maxLongLen := 0
	hasValueName := false

	update := func(left string, indent bool) {
		l := utf8.RuneCountInString(left)
		if indent {
			l += commandOptionIndent
		}
		if l > maxLongLen {
			maxLongLen = l
		}
	}

	for _, opt := range rootOpts {
		if opt.Short != "" {
			a.hasShort = true
		}
		update(opt.Long, false)
	}

	for _, bf := range b.ordered {
		if b.fs.Lookup(bf.name).Hidden {
			continue
		}
		if b.fs.Lookup(bf.name).Shorthand != "" {
			a.hasShort = true
		}
		if bf.value.valueName != "" {
			hasValueName = true
		}

		left := bf.name + bf.value.valueName
		if len(bf.value.choices) > 0 {
			left += "[" + strings.Join(bf.value.choices, "|") + "]"
		}
		update(left, true)
	}

	a.descStart = maxLongLen + distanceBetweenOptionAndDescription
	if a.hasShort {
		a.descStart += 2
	}
	if maxLongLen > 0 {
		a.descStart += 4
	}
	if hasValueName {
		a.descStart += 3
	}
	a.descStart += paddingBeforeOption

	// the narrow-terminal fix go-flags lacked
	a.narrow = a.descStart > width-minDescriptionWidth

	return a
}

// writeOptionLine renders one option: the left column (indentation,
// short, long with argument suffix) and the description with its
// suffixes, wrapped to the terminal width.
func (b *Binder) writeOptionLine(w io.Writer, a alignment, indent bool, short, left, descSuffix, description string) {
	var line strings.Builder

	prefix := paddingBeforeOption
	if indent {
		prefix += commandOptionIndent
	}
	line.WriteString(strings.Repeat(" ", prefix))

	if short != "" {
		line.WriteString("-" + short)
	} else if a.hasShort {
		line.WriteString("  ")
	}

	if short != "" {
		line.WriteString(", ")
	} else if a.hasShort {
		line.WriteString("  ")
	}

	line.WriteString("--" + left)

	// go-flags only rendered the default/env suffixes when the flag had
	// a description
	if description == "" {
		fmt.Fprintln(w, line.String())
		return
	}

	desc := description + descSuffix

	if a.narrow {
		fmt.Fprintln(w, line.String())
		indentStr := strings.Repeat(" ", narrowIndent)
		fmt.Fprintln(w, indentStr+wrapText(desc, a.width-narrowIndent, indentStr))
		return
	}

	descStart := a.descStart + paddingBeforeOption
	fmt.Fprint(w, line.String())
	fmt.Fprint(w, strings.Repeat(" ", descStart-line.Len()))
	fmt.Fprintln(w, wrapText(desc, a.width-descStart, strings.Repeat(" ", descStart)))
}

// wrapText is go-flags' basic space-splitting wrapper: each continuation
// line is prefixed, words longer than the line are hyphenated.
func wrapText(s string, l int, prefix string) string {
	var ret string

	if l < 10 {
		l = 10
	}

	for line := range strings.SplitSeq(s, "\n") {
		var retline string

		line = strings.TrimSpace(line)

		for len(line) > l {
			suffix := ""

			pos := strings.LastIndex(line[:l], " ")
			if pos < 0 {
				pos = l - 1
				suffix = "-\n"
			}

			if len(retline) != 0 {
				retline += "\n" + prefix
			}

			retline += strings.TrimSpace(line[:pos]) + suffix
			line = strings.TrimSpace(line[pos:])
		}

		if len(line) > 0 {
			if len(retline) != 0 {
				retline += "\n" + prefix
			}

			retline += line
		}

		if len(ret) > 0 {
			ret += "\n"

			if len(retline) > 0 {
				ret += prefix
			}
		}

		ret += retline
	}

	return ret
}
