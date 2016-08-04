package verifier

import (
	"net/http"

	"code.cloudfoundry.org/lager"
)

type Verifier interface {
	Verify(lager.Logger, *http.Client) (bool, error)
}
