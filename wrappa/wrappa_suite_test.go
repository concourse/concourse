package wrappa_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestWrappa(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Wrappa Suite")
}

type stupidHandler struct{}

func (stupidHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
}

type descriptiveRoute struct {
	route   string
	handler http.Handler
}
