package gcpsecretmanager

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"text/template/parse"

	"github.com/concourse/concourse/atc/creds"
)

// Secret templates are written as slash-delimited paths, like every other
// credential manager. Secret IDs allow no slashes, so a path maps to an ID by
// dropping the leading slash and joining the segments with idSeparator:
//
//	/concourse/main/my-pipeline/my-secret -> concourse--main--my-pipeline--my-secret
//
// Concourse has no per-secret access control: teams are kept apart only
// because each (scope, team, pipeline, var) maps to its own ID. That holds
// while every segment is valid (see segmentPattern) and no name contains a
// slash. A valid segment cannot contain idSeparator or merge into an adjacent
// one, so every "--" in an ID is a segment boundary and the mapping is
// reversible.
//
// Team and pipeline names are checked once per scope, by scopeIsMappable.
// Var names are checked on every lookup, by secretIDLookupPath.
const (
	pathSeparator = "/"
	idSeparator   = "--"
)

// segmentPattern allows letters, numerals, hyphens and underscores, with no
// hyphen at either end. idSeparator is rejected separately.
var segmentPattern = regexp.MustCompile(`^[A-Za-z0-9_]([A-Za-z0-9_-]*[A-Za-z0-9_])?$`)

var ErrInvalidSegment = errors.New("name cannot be mapped to a Google Secret Manager secret ID")

// placeholderMarker wraps placeholders while tokenizing a template. It cannot
// occur in template text, which is limited to the secret ID character set.
const placeholderMarker = "\x00"

func validSegment(segment string) bool {
	return segmentPattern.MatchString(segment) && !strings.Contains(segment, idSeparator)
}

// secretPathToID maps a rendered secret path to a secret ID. Callers must
// have validated every segment.
func secretPathToID(path string) string {
	return strings.ReplaceAll(strings.TrimPrefix(path, pathSeparator), pathSeparator, idSeparator)
}

// probeSecret stands in for the var name when checking whether a scope's team
// and pipeline names can be mapped to a secret ID.
const probeSecret = "secret"

// scopeIsMappable reports whether a lookup path's team and pipeline names can
// be mapped to a secret ID. Templates give every placeholder a whole segment,
// so a name is mappable when rendering the template with it yields the same
// number of segments as rendering it with placeholder values, all of them
// valid. A slash in a name adds a segment; an empty name leaves one empty.
func scopeIsMappable(tmpl *creds.SecretTemplate, lookupPath creds.SecretLookupPath) bool {
	want, err := creds.NewSecretLookupWithTemplate(tmpl, "team", "pipeline").VariableToSecretPath(probeSecret)
	if err != nil {
		return false
	}
	got, err := lookupPath.VariableToSecretPath(probeSecret)
	if err != nil {
		return false
	}

	segments := pathSegments(got)
	if len(segments) != len(pathSegments(want)) {
		return false
	}
	for _, segment := range segments {
		if !validSegment(segment) {
			return false
		}
	}
	return true
}

func pathSegments(path string) []string {
	return strings.Split(strings.TrimPrefix(path, pathSeparator), pathSeparator)
}

// secretIDLookupPath maps a rendered secret path to a secret ID. Its scope
// must have passed scopeIsMappable.
type secretIDLookupPath struct {
	creds.SecretLookupPath
}

// VariableToSecretPath fails, rather than skipping the scope, for a var name
// that cannot be mapped: a slash or "--" in it would reach another scope.
func (p secretIDLookupPath) VariableToSecretPath(secret string) (string, error) {
	if !validSegment(secret) {
		return "", fmt.Errorf("%w: var name %q may contain only letters, numerals, hyphens and underscores, must not contain %q, and must not begin or end with a hyphen", ErrInvalidSegment, secret, idSeparator)
	}

	path, err := p.SecretLookupPath.VariableToSecretPath(secret)
	if err != nil {
		return "", err
	}

	return secretPathToID(path), nil
}

// unmappableLookupPath fails every lookup for a team and pipeline that no
// scope can map to a secret ID.
type unmappableLookupPath struct {
	teamName     string
	pipelineName string
}

func (p unmappableLookupPath) VariableToSecretPath(string) (string, error) {
	return "", fmt.Errorf("%w: no secret scope can map team %q and pipeline %q", ErrInvalidSegment, p.teamName, p.pipelineName)
}

// templateToken is one slash-separated segment of a secret template: either
// a placeholder field or a literal.
type templateToken struct {
	field   string
	literal string
}

// tokenizeTemplate splits a template into segments. It requires every
// placeholder to fill a whole segment, so values can be recovered unambiguously.
func tokenizeTemplate(name string, tmpl *creds.SecretTemplate) ([]templateToken, error) {
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
	for part := range strings.SplitSeq(strings.TrimPrefix(marked.String(), pathSeparator), pathSeparator) {
		if !strings.Contains(part, placeholderMarker) {
			if !validSegment(part) {
				return nil, fmt.Errorf("%s: literal segment %q must be non-empty, may contain only letters, numerals, hyphens and underscores, must not contain %q, and must not begin or end with a hyphen", name, part, idSeparator)
			}
			tokens = append(tokens, templateToken{literal: part})
			continue
		}

		field := strings.TrimSuffix(strings.TrimPrefix(part, placeholderMarker), placeholderMarker)
		if strings.Contains(field, placeholderMarker) || len(field)+2*len(placeholderMarker) != len(part) {
			return nil, fmt.Errorf("%s: every placeholder must fill a whole segment between slashes", name)
		}
		if seen[field] {
			return nil, fmt.Errorf("%s: {{.%s}} must appear at most once", name, field)
		}
		seen[field] = true
		tokens = append(tokens, templateToken{field: field})
	}

	// A literal root keeps Concourse's secrets apart from the project's other
	// secrets, and lets shared secrets have a root of their own.
	if tokens[0].field != "" {
		return nil, fmt.Errorf("%s: must begin with a literal segment, such as /concourse/", name)
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
