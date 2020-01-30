package policychecker

import (
	"fmt"
	"net/http"
)

//go:generate counterfeiter . Rejector
type Rejector interface {
	Error(http.ResponseWriter, *http.Request)
	Reject(http.ResponseWriter, *http.Request)
}

type NotPassRejector struct{
	msg string
}

func (n NotPassRejector) Error(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(w, fmt.Sprintf("policy check error: %s", n.msg))
}

func (NotPassRejector) Reject(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprintf(w, "policy check not pass")
}
