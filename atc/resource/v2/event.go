package v2

import "github.com/concourse/concourse/atc"

type EventHandler interface {
	SaveVersion(atc.SpaceVersion) error
	SaveDefaultSpace(atc.Space) error
	SaveSpaces([]atc.Space) error
}
