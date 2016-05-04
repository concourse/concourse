package algorithm

import "fmt"

type MissingInputReasons map[string]string

const (
	NoVerionsSatisfiedPassedConstraints string = "no versions satisfy passed constraints"
	NoVersionsAvailable                 string = "no versions available"
	PinnedVersionUnavailable            string = "pinned version %s is not available"
)

func (mir MissingInputReasons) RegisterPassedConstraint(inputName string) {
	mir[inputName] = NoVerionsSatisfiedPassedConstraints
}

func (mir MissingInputReasons) RegisterNoVersions(inputName string) {
	mir[inputName] = NoVersionsAvailable
}

func (mir MissingInputReasons) RegisterPinnedVersionUnavailable(inputName string, version string) {
	mir[inputName] = fmt.Sprintf(PinnedVersionUnavailable, version)
}

func (mir MissingInputReasons) Append(otherReasons MissingInputReasons) {
	for k, v := range otherReasons {
		mir[k] = v
	}
}
