package publichandler

import (
	"net/http"

	"github.com/gobuffalo/packr"
)

func NewHandler() (http.Handler, error) {
	return CacheNearlyForever(http.FileServer(packr.NewBox("../public"))), nil
}
