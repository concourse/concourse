package vars

import (
	"fmt"
	"path"
	"strings"
)

type Reference struct {
	Source string
	Path   string
	Fields []string
}

func (r Reference) WithoutSource() Reference {
	return Reference{
		Path:   r.Path,
		Fields: r.Fields,
	}
}

func (r Reference) String() string {
	var s strings.Builder
	if r.Source != "" {
		s.WriteString(r.Source + ":")
	}
	s.WriteString(refSegmentString(r.Path))
	fields := r.Fields
	for len(fields) > 0 {
		s.WriteRune('.')
		s.WriteString(refSegmentString(fields[0]))
		fields = fields[1:]
	}
	return s.String()
}

func ParseReference(name string) (Reference, error) {
	var ref Reference

	input := name
	if unclosedQuotes(input) {
		return Reference{}, fmt.Errorf("invalid var '%s': contains unclosed quotes", name)
	}

	if i, ok := findUnquoted(input, ':'); ok {
		ref.Source = input[:i]
		ref.Source = strings.ReplaceAll(ref.Source, `"`, "")
		ref.Source = strings.TrimSpace(ref.Source)
		input = input[i+1:]
	}

	var fields []string
	hasNextSegment := true
	for hasNextSegment {
		var field string
		field, input, hasNextSegment = readPathSegment(input)
		if field == "" {
			return Reference{}, fmt.Errorf("invalid var '%s': empty field", name)
		}
		fields = append(fields, field)
	}

	if len(fields) == 0 {
		// Should be impossible (since we'd error that the var is empty), but better safe than sorry
		return Reference{}, fmt.Errorf("invalid var '%s': no fields", name)
	}

	ref.Path = fields[0]
	ref.Fields = fields[1:]

	return ref, nil
}

func cleanPath(p string) string {
	if path.IsAbs(p) {
		return path.Clean(p)
	} else {
		p = path.Clean("/" + p)
		return p[1:]
	}
}

func unclosedQuotes(s string) bool {
	quotes := strings.Count(s, `"`)
	return quotes%2 != 0
}

func findUnquoted(s string, r rune) (int, bool) {
	quoted := false
	for i, c := range s {
		switch c {
		case r:
			if !quoted {
				return i, true
			}
		case '"':
			quoted = !quoted
		}
	}
	return 0, false
}

func readPathSegment(raw string) (string, string, bool) {
	var field string
	var rest string
	i, hasNextSegment := findUnquoted(raw, '.')
	if hasNextSegment {
		field = raw[:i]
		rest = raw[i+1:]
	} else {
		field = raw
	}
	field = strings.TrimSpace(field)
	field = strings.ReplaceAll(field, `"`, "")
	field = cleanPath(field)
	return field, rest, hasNextSegment
}

func refSegmentString(seg string) string {
	if strings.ContainsAny(seg, ",.:/ ") {
		return fmt.Sprintf("%q", seg)
	}
	return seg
}
