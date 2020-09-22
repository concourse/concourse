package wrappa_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/pipelineserver"
	"github.com/concourse/concourse/atc/wrappa"
	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RejectArchivedWrappa", func() {
	var (
		raWrappa         *wrappa.RejectArchivedWrappa
		raHandlerFactory pipelineserver.RejectArchivedHandlerFactory
	)

	BeforeEach(func() {
		raHandlerFactory = pipelineserver.RejectArchivedHandlerFactory{}
		raWrappa = wrappa.NewRejectArchivedWrappa(raHandlerFactory)
	})

	It("wraps endpoints", func() {
		inputHandlers := rata.Handlers{}

		for _, route := range atc.Routes {
			inputHandlers[route.Name] = &stupidHandler{}
		}

		rejectArchivedRoutes := []string{
			atc.PausePipeline,
			atc.UnpausePipeline,
			atc.CreateJobBuild,
			atc.ScheduleJob,
			atc.CheckResource,
			atc.CheckResourceType,
			atc.DisableResourceVersion,
			atc.EnableResourceVersion,
			atc.PinResourceVersion,
			atc.UnpinResource,
			atc.SetPinCommentOnResource,
			atc.RerunJobBuild,
		}

		rejectArchivedLookup := make(map[string]bool)
		for _, name := range rejectArchivedRoutes {
			rejectArchivedLookup[name] = true
		}

		wrappedHandlers := raWrappa.Wrap(inputHandlers)

		for name, handler := range inputHandlers {
			expectedHandler := handler
			if rejectArchivedLookup[name] {
				expectedHandler = raHandlerFactory.RejectArchived(expectedHandler)
			}
			Expect(wrappedHandlers[name]).To(BeIdenticalTo(expectedHandler), "handler is "+name)
		}
	})

	It("panics on unknown handlers", func() {
		inputHandlers := rata.Handlers{
			"unknownHandler": &stupidHandler{},
		}
		Expect(func() { raWrappa.Wrap(inputHandlers) }).To(PanicWith("how do archived pipelines affect your endpoint?"))
	})
})
