package robotshandler

import (
	"fmt"
	"net/http"
)

func NewHandler() http.Handler {
	return &handler{}
}

type handler struct{}

func (self *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "User-agent: *")
	fmt.Fprintln(w, "Disallow: /")
}
