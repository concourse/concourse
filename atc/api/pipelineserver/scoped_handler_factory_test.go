package pipelineserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/api/pipelineserver"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handler", func() {
	var (
		server   *httptest.Server
		delegate *delegateHandler

		fakePipeline *dbfakes.FakePipeline

		handler http.Handler
	)

	BeforeEach(func() {
		delegate = &delegateHandler{}

		fakePipeline = new(dbfakes.FakePipeline)

		handlerFactory := pipelineserver.ScopedHandlerFactory{}
		handler = wrapHandler{
			delegate:        handlerFactory.HandlerFor(delegate.GetHandler),
			contextPipeline: fakePipeline,
		}
	})

	JustBeforeEach(func() {
		server = httptest.NewServer(handler)

		request, err := http.NewRequest("POST", server.URL, nil)
		Expect(err).NotTo(HaveOccurred())

		_, err = new(http.Client).Do(request)
		Expect(err).NotTo(HaveOccurred())
	})

	var _ = AfterEach(func() {
		server.Close()
	})

	It("calls the scoped handler with the pipeline from context", func() {
		Expect(delegate.IsCalled).To(BeTrue())
		Expect(delegate.Pipeline).To(BeIdenticalTo(fakePipeline))
	})
})

type delegateHandler struct {
	IsCalled bool
	Pipeline db.Pipeline
}

func (handler *delegateHandler) GetHandler(dbPipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.IsCalled = true
		handler.Pipeline = dbPipeline
	})
}

type wrapHandler struct {
	delegate        http.Handler
	contextPipeline db.Pipeline
}

func (h wrapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.WithValue(r.Context(), auth.PipelineContextKey, h.contextPipeline)
	h.delegate.ServeHTTP(w, r.WithContext(ctx))
}
