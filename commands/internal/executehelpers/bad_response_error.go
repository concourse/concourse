package executehelpers

import (
	"fmt"
	"net/http"
)

func badResponseError(doing string, response *http.Response) error {
	return fmt.Errorf("bad response %s (%s)", doing, response.Status)
}
