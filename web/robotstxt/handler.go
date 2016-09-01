package robotstxt

import (
	"fmt"
	"net/http"
)

type Handler struct{}

func (Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "User-agent: *")
	fmt.Fprintln(w, "Disallow: /")
}
