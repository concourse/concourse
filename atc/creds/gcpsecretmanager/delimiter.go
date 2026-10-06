package gcpsecretmanager

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"text/template/parse"

	"github.com/concourse/concourse/atc/creds"
)

// DefaultSegmentDelimiter separates the segments of a secret ID. Secret IDs
// allow no slashes, so a double hyphen stands in for the path separator.
//
// Secret IDs stay unambiguous only while no segment contains the delimiter or
// begins or ends with one of its characters. Otherwise ((other-team--secret))
// or a team named "a--b" could resolve to a secret belonging to another team
// or pipeline. If a segment's first and last characters are not delimiter
// characters, no delimiter can form across a segment boundary.
const DefaultSegmentDelimiter = "--"

// Delimiters are limited to the special characters Google Secret Manager permits in IDs.
var delimiterPattern = regexp.MustCompile(`^[-_]+$`)

var ErrAmbiguousSegment = errors.New("ambiguous Google Secret Manager secret ID segment")

// placeholderMarker wraps placeholders while tokenizing a template. It cannot
// occur in template text, which is limited to the secret ID character set.
const placeholderMarker = "\x00"

func validateDelimiter(delimiter string) error {
	if !delimiterPattern.MatchString(delimiter) {
		return fmt.Errorf("invalid segment delimiter %q: only hyphens and underscores are permitted", delimiter)
	}
	return nil
}

// segmentIsAmbiguous reports whether a non-empty value could be confused
// with, or merge into, an adjacent delimiter.
func segmentIsAmbiguous(value, delimiter string) bool {
	return strings.Contains(value, delimiter) ||
		strings.IndexByte(delimiter, value[0]) >= 0 ||
		strings.IndexByte(delimiter, value[len(value)-1]) >= 0
}

// delimitedLookupPath refuses to build a secret ID from any segment that
// could be mistaken for a delimiter.
type delimitedLookupPath struct {
	creds.SecretLookupPath
	teamName     string
	pipelineName string
	delimiter    string
}

func (p delimitedLookupPath) VariableToSecretPath(secret string) (string, error) {
	segments := []struct{ kind, value string }{
		{"team name", p.teamName},
		{"pipeline name", p.pipelineName},
		{"var name", secret},
	}
	for _, segment := range segments {
		if segment.value == "" {
			continue
		}
		if segmentIsAmbiguous(segment.value, p.delimiter) {
			return "", fmt.Errorf("%w: %s %q must not contain the delimiter %q or begin or end with any of its characters", ErrAmbiguousSegment, segment.kind, segment.value, p.delimiter)
		}
	}

	return p.SecretLookupPath.VariableToSecretPath(secret)
}

// templateToken is one delimiter-separated segment of a secret template:
// either a placeholder field or a literal.
type templateToken struct {
	field   string
	literal string
}

// tokenizeTemplate splits a template on the delimiter. It requires every
// placeholder to fill a whole segment, so values can be recovered unambiguously.
func tokenizeTemplate(name string, tmpl *creds.SecretTemplate, delimiter string) ([]templateToken, error) {
	var marked strings.Builder
	for _, node := range tmpl.Tree.Root.Nodes {
		switch n := node.(type) {
		case *parse.TextNode:
			marked.Write(n.Text)
		case *parse.ActionNode:
			field, ok := placeholderField(n)
			if !ok {
				return nil, fmt.Errorf("%s: unsupported template action %s: only {{.Team}}, {{.Pipeline}} and {{.Secret}} are permitted", name, n)
			}
			marked.WriteString(placeholderMarker + field + placeholderMarker)
		default:
			return nil, fmt.Errorf("%s: unsupported template construct %s: only {{.Team}}, {{.Pipeline}} and {{.Secret}} are permitted", name, n)
		}
	}

	var tokens []templateToken
	seen := map[string]bool{}
	for part := range strings.SplitSeq(marked.String(), delimiter) {
		if !strings.Contains(part, placeholderMarker) {
			if part == "" || segmentIsAmbiguous(part, delimiter) {
				return nil, fmt.Errorf("%s: literal segments must be non-empty and must not begin or end with a character of the delimiter %q", name, delimiter)
			}
			tokens = append(tokens, templateToken{literal: part})
			continue
		}

		field := strings.TrimSuffix(strings.TrimPrefix(part, placeholderMarker), placeholderMarker)
		if strings.Contains(field, placeholderMarker) || len(field)+2*len(placeholderMarker) != len(part) {
			return nil, fmt.Errorf("%s: every placeholder must be separated from the rest of the template by the delimiter %q", name, delimiter)
		}
		if seen[field] {
			return nil, fmt.Errorf("%s: {{.%s}} must appear at most once", name, field)
		}
		seen[field] = true
		tokens = append(tokens, templateToken{field: field})
	}

	if !seen["Secret"] {
		return nil, fmt.Errorf("%s: must contain {{.Secret}}", name)
	}
	// Pipeline names are only unique within a team.
	if seen["Pipeline"] && !seen["Team"] {
		return nil, fmt.Errorf("%s: {{.Pipeline}} requires {{.Team}}", name)
	}

	return tokens, nil
}

func placeholderField(n *parse.ActionNode) (string, bool) {
	pipe := n.Pipe
	if pipe == nil || len(pipe.Decl) != 0 || len(pipe.Cmds) != 1 || len(pipe.Cmds[0].Args) != 1 {
		return "", false
	}
	field, ok := pipe.Cmds[0].Args[0].(*parse.FieldNode)
	if !ok || len(field.Ident) != 1 {
		return "", false
	}
	switch field.Ident[0] {
	case "Team", "Pipeline", "Secret":
		return field.Ident[0], true
	}
	return "", false
}

// tokensOverlap reports whether two templates can render the same secret ID.
// A placeholder can take the value of any literal in the same position.
func tokensOverlap(a, b []templateToken) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].field == "" && b[i].field == "" && a[i].literal != b[i].literal {
			return false
		}
	}
	return true
}
