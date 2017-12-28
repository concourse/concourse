package buildserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/atc/api/auth"
	. "github.com/concourse/atc/api/buildserver"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ScopedHandlerFactory", func() {
	var (
		response *http.Response
		server   *httptest.Server
		delegate *delegateHandler
		handler  http.Handler
	)

	BeforeEach(func() {
		delegate = &delegateHandler{}
		logger := lagertest.NewTestLogger("test")
		handlerFactory := NewScopedHandlerFactory(logger)
		handler = handlerFactory.HandlerFor(delegate.GetHandler)
	})

	JustBeforeEach(func() {
		server = httptest.NewServer(handler)

		request, err := http.NewRequest("POST", server.URL, nil)
		Expect(err).NotTo(HaveOccurred())

		response, err = new(http.Client).Do(request)
		Expect(err).NotTo(HaveOccurred())
	})

	var _ = AfterEach(func() {
		server.Close()
	})

	Context("build is in the context", func() {
		var contextBuild *dbfakes.FakeBuild

		BeforeEach(func() {
			contextBuild = new(dbfakes.FakeBuild)
			handler = &wrapHandler{handler, contextBuild}
		})

		It("calls scoped handler with build from context", func() {
			Expect(delegate.IsCalled).To(BeTrue())
			Expect(delegate.Build).To(BeIdenticalTo(contextBuild))
		})
	})

	Context("build not found in the context", func() {
		It("returns 500", func() {
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
		})

		It("does not call the scoped handler", func() {
			Expect(delegate.IsCalled).To(BeFalse())
		})
	})
})

type delegateHandler struct {
	IsCalled bool
	Build    db.Build
}

func (handler *delegateHandler) GetHandler(build db.Build) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.IsCalled = true
		handler.Build = build
	})
}

type wrapHandler struct {
	delegate     http.Handler
	contextBuild db.Build
}

func (h *wrapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.WithValue(r.Context(), auth.BuildContextKey, h.contextBuild)
	h.delegate.ServeHTTP(w, r.WithContext(ctx))
}
