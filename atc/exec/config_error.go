package exec

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/vars"
)

// identifyingSourceKeys are source fields that tell a user which image or
// resource failed. Credential-like fields are intentionally absent so error
// text can be shown in the build log.
var identifyingSourceKeys = []string{
	"repository",
	"tag",
	"tag_regex",
	"uri",
	"url",
	"branch",
	"path",
	"module",
	"package",
	"digest",
}

type missingVersionError struct {
	msg string
}

func (e missingVersionError) Error() string { return e.msg }

func (e missingVersionError) Unwrap() error { return ErrResultMissing }

func missingVersionErrorFor(plan atc.GetPlan) error {
	if plan.Resource == "" {
		return missingVersionError{
			msg: fmt.Sprintf(
				"unable to fetch %s: no versions found. The image may not exist, or the repository or tag may be invalid",
				imageSubject(plan.Type, plan.Source),
			),
		}
	}

	return missingVersionError{
		msg: fmt.Sprintf(
			"version is missing from the previous step for %s",
			namedConfig("resource", plan.Resource, plan.Type, plan.Source),
		),
	}
}

// ImageCheckFailed is returned when an image check exits unsuccessfully.
// The resource's own output is already in the build log.
func ImageCheckFailed(resourceType string, source atc.Source) error {
	return fmt.Errorf("image check failed for %s. See the image check output for details", imageSubject(resourceType, source))
}

// ImageFetchFailed is returned when an image get exits unsuccessfully.
func ImageFetchFailed(resourceType string, source atc.Source) error {
	return fmt.Errorf("fetching %s failed. See the image get output for details", imageSubject(resourceType, source))
}

func wrapEvalError(state RunState, what, subject string, err error) error {
	return withRedaction(state, fmt.Errorf("evaluating %s for %s: %w", what, subject, err))
}

func withRedaction(state RunState, err error) error {
	if err == nil || state == nil {
		return err
	}

	msg := redactInterpolated(state, err.Error())
	if msg == err.Error() {
		return err
	}

	return redactedError{msg: msg, cause: err}
}

type redactedError struct {
	msg   string
	cause error
}

func (e redactedError) Error() string { return e.msg }

func (e redactedError) Unwrap() error { return e.cause }

func redactInterpolated(state RunState, msg string) string {
	it := &lineRedactor{line: msg}
	state.IterateInterpolatedCreds(it)
	return it.line
}

// lineRedactor matches the build log redactor: interpolated credential values
// longer than one character are replaced, so error text does not leak them.
type lineRedactor struct {
	line string
}

func (it *lineRedactor) YieldCred(_, value string) {
	for lineValue := range strings.SplitSeq(value, "\n") {
		lineValue = strings.TrimSpace(lineValue)
		if len(lineValue) > 1 {
			it.line = strings.ReplaceAll(it.line, lineValue, "((redacted))")
		}
	}
}

func CheckSubject(plan atc.CheckPlan) string {
	switch {
	case plan.Resource != "":
		return namedConfig("resource", plan.Resource, plan.Type, plan.Source)
	case plan.ResourceType != "":
		return namedConfig("resource type", plan.ResourceType, plan.Type, plan.Source)
	case plan.Prototype != "":
		return namedConfig("prototype", plan.Prototype, plan.Type, plan.Source)
	default:
		return imageSubject(plan.Type, plan.Source)
	}
}

func GetSubject(plan atc.GetPlan) string {
	if plan.Resource != "" {
		return namedConfig("resource", plan.Resource, plan.Type, plan.Source)
	}
	return imageSubject(plan.Type, plan.Source)
}

func PutSubject(plan atc.PutPlan) string {
	name := plan.Resource
	if name == "" {
		name = plan.Name
	}
	return namedConfig("resource", name, plan.Type, plan.Source)
}

func imageSubject(resourceType string, source atc.Source) string {
	repo, hasRepo := sourceField(source, "repository")
	rest := sourceIdentifier(source, "repository")

	switch {
	case hasRepo && rest != "":
		return fmt.Sprintf("image %q (type %q, %s)", repo, resourceType, rest)
	case hasRepo:
		return fmt.Sprintf("image %q (type %q)", repo, resourceType)
	default:
		return namedConfig("image", "", resourceType, source)
	}
}

func namedConfig(kind, name, resourceType string, source atc.Source) string {
	ident := sourceIdentifier(source)
	switch {
	case name != "" && ident != "":
		return fmt.Sprintf("%s %q (type %q, %s)", kind, name, resourceType, ident)
	case name != "":
		return fmt.Sprintf("%s %q (type %q)", kind, name, resourceType)
	case ident != "":
		return fmt.Sprintf("%s (type %q, %s)", kind, resourceType, ident)
	default:
		return fmt.Sprintf("%s (type %q)", kind, resourceType)
	}
}

func sourceIdentifier(source atc.Source, skip ...string) string {
	skipSet := map[string]struct{}{}
	for _, key := range skip {
		skipSet[key] = struct{}{}
	}

	var parts []string
	for _, key := range identifyingSourceKeys {
		if _, skipped := skipSet[key]; skipped {
			continue
		}
		val, ok := sourceField(source, key)
		if !ok {
			continue
		}
		parts = append(parts, key+": "+val)
	}
	return strings.Join(parts, ", ")
}

func sourceField(source atc.Source, key string) (string, bool) {
	raw, ok := source[key]
	if !ok || raw == nil {
		return "", false
	}

	switch v := raw.(type) {
	case string:
		if v == "" {
			return "", false
		}
		return v, true
	case json.Number:
		if v.String() == "" {
			return "", false
		}
		return v.String(), true
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return "", false
		}
		if v == math.Trunc(v) && v < 1e15 && v > -1e15 {
			return strconv.FormatInt(int64(v), 10), true
		}
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case int:
		return strconv.Itoa(v), true
	case int64:
		return strconv.FormatInt(v, 10), true
	default:
		return "", false
	}
}

var _ vars.TrackedVarsIterator = (*lineRedactor)(nil)
