package vars

import (
	"fmt"
	"strings"
)

//go:generate counterfeiter . Variables

type Variables interface {
	Get(Reference) (interface{}, bool, error)
	List() ([]Reference, error)
}

type Reference struct {
	Source string
	Path   string
	Fields []string
}

func parseReference(name string) Reference {
	var pathPieces []string
	var fields []string

	var ref Reference

	if strings.Index(name, ":") > 0 {
		parts := strings.SplitN(name, ":", 2)
		ref.Source = parts[0]

		pathPieces = pathRegex.FindAllString(parts[1], -1)

	} else {
		pathPieces = pathRegex.FindAllString(name, -1)
	}

	ref.Path = strings.ReplaceAll(pathPieces[0], "\"", "")

	if len(pathPieces) >= 2 {
		for _, piece := range pathPieces[1:] {
			fields = append(fields, strings.ReplaceAll(piece, "\"", ""))
		}

		ref.Fields = fields
	}

	return ref
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

func refSegmentString(seg string) string {
	if strings.ContainsAny(seg, ".:") {
		return fmt.Sprintf("%q", seg)
	}
	return seg
}
