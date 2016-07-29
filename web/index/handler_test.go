package index_test

import (
	"html/template"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/web/index"
	"github.com/concourse/atc/web/pipeline"
	"github.com/concourse/atc/web/webfakes"
	cfakes "github.com/concourse/go-concourse/concourse/concoursefakes"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handler", func() {
	var server *httptest.Server
	var request *http.Request
	var response *http.Response
	var fakeClient *cfakes.FakeClient
	var handler *Handler

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("index")

		clientFactory := &webfakes.FakeClientFactory{}
		fakeClient = new(cfakes.FakeClient)
		clientFactory.BuildReturns(fakeClient)

		fakeTeam := new(cfakes.FakeTeam)
		fakeClient.TeamReturns(fakeTeam)

		firstPipeline := atc.Pipeline{Name: "first-pipeline", TeamName: "team-a"}
		secondPipeline := atc.Pipeline{Name: "other-pipeline", TeamName: "team-a"}

		fakeClient.ListPipelinesReturns([]atc.Pipeline{firstPipeline, secondPipeline}, nil)

		fakeTeam.PipelineStub = func(pipelineName string) (atc.Pipeline, bool, error) {
			switch pipelineName {
			case "first-pipeline":
				return firstPipeline, true, nil
			case "other-pipeline":
				return secondPipeline, true, nil
			default:
				return atc.Pipeline{}, false, nil
			}
		}

		pipelineTemplate, err := template.New("test").Parse("{{.PipelineName}}")
		Expect(err).NotTo(HaveOccurred())
		pipelineHandler := pipeline.NewHandler(logger, clientFactory, pipelineTemplate)

		handler = NewHandler(logger, clientFactory, pipelineHandler, &template.Template{})
	})

	JustBeforeEach(func() {
		var err error
		client := &http.Client{
			Transport: &http.Transport{},
		}

		response, err = client.Do(request)
		Expect(err).NotTo(HaveOccurred())
	})

	var _ = AfterEach(func() {
		server.Close()
	})

	Context("when team is not in user context", func() {
		BeforeEach(func() {
			server = httptest.NewServer(wrapHandler{delegate: handler, authenticated: false})

			var err error
			request, err = http.NewRequest("GET", server.URL+"/", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200 OK", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("returns first pipeline", func() {
			body, err := ioutil.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())

			Expect(string(body)).To(Equal("first-pipeline"))
		})

		It("returns builds for the team on first pipeline", func() {
			Expect(fakeClient.TeamCallCount()).To(Equal(1))
			Expect(fakeClient.TeamArgsForCall(0)).To(Equal("team-a"))
		})
	})
})

type wrapHandler struct {
	delegate      *Handler
	authenticated bool
}

func (handler wrapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := handler.delegate.ServeHTTP(w, r)
	Expect(err).NotTo(HaveOccurred())
}
