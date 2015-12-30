package auth

import "net/http"

func CopyRequest(r *http.Request) *http.Request {
	requestCopy := *r
	requestCopy.Header = make(http.Header)
	for k, s := range r.Header {
		requestCopy.Header[k] = s
	}
	return &requestCopy
}
