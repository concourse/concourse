package policychecker

import (
	"fmt"
	"net/http"
)

//go:generate counterfeiter . Rejector
type Rejector interface {
	Fail(http.ResponseWriter, *http.Request)
	NotPass(http.ResponseWriter, *http.Request)
}

type NotPassRejector struct{
	msg string
}

func (n NotPassRejector) Fail(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprintf(w, fmt.Sprintf("policy check failed: %s", n.msg))
}


func (NotPassRejector) NotPass(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprintf(w, "policy check not pass")
}
