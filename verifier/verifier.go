package verifier

import (
	"net/http"

	"github.com/pivotal-golang/lager"
)

type Verifier interface {
	Verify(lager.Logger, *http.Client) (bool, error)
}
