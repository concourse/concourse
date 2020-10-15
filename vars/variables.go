package vars

import (
	"fmt"
	"strings"
)

//go:generate counterfeiter . Variables

type Variables interface {
	Get(VariableDefinition) (interface{}, bool, error)
	List() ([]VariableDefinition, error)
}

type VariableReference struct {
	Source string
	Path   string
	Fields []string
}

type VariableDefinition struct {
	Ref     VariableReference
	Type    string
	Options interface{}
}

func ParseReference(name string) (VariableReference, error) {
	var ref VariableReference

	input := name
	if i, ok := findUnquoted(input, ':'); ok {
		ref.Source = input[:i]
		if strings.ContainsAny(ref.Source, `"`) {
			return VariableReference{}, fmt.Errorf("invalid var '%s': source must not be quoted", name)
		}
		input = input[i+1:]
	}
	var fields []string
	hasNextSegment := true
	for hasNextSegment {
		var field string
		field, input, hasNextSegment = readPathSegment(input)
		if field == "" {
			return VariableReference{}, fmt.Errorf("invalid var '%s': empty field", name)
		}
		fields = append(fields, field)
	}

	if len(fields) == 0 {
		// Should be impossible (since we'd error that the var is empty), but better safe than sorry
		return VariableReference{}, fmt.Errorf("invalid var '%s': no fields", name)
	}

	ref.Path = fields[0]
	ref.Fields = fields[1:]

	return ref, nil
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
	return field, rest, hasNextSegment
}

func (r VariableReference) String() string {
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

func refSegmentString(seg string) string {
	if strings.ContainsAny(seg, ".:") {
		return fmt.Sprintf("%q", seg)
	}
	return seg
}
