package wrappa

import "net/http"

type Wrappa interface {
	Wrap(map[string]http.Handler) map[string]http.Handler
}

//go:generate counterfeiter net/http.Handler
