package wrappa_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/wrappa"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/rata"
)

var _ = Describe("APIAuthWrappa", func() {
	var (
		fakeCheckPipelineAccessHandlerFactory   auth.CheckPipelineAccessHandlerFactory
		fakeCheckBuildReadAccessHandlerFactory  auth.CheckBuildReadAccessHandlerFactory
		fakeCheckBuildWriteAccessHandlerFactory auth.CheckBuildWriteAccessHandlerFactory
		fakeCheckWorkerTeamAccessHandlerFactory auth.CheckWorkerTeamAccessHandlerFactory
		fakeBuildFactory                        *dbfakes.FakeBuildFactory
	)

	BeforeEach(func() {
		fakeTeamFactory := new(dbfakes.FakeTeamFactory)
		workerFactory := new(dbfakes.FakeWorkerFactory)
		fakeBuildFactory = new(dbfakes.FakeBuildFactory)
		fakeCheckPipelineAccessHandlerFactory = auth.NewCheckPipelineAccessHandlerFactory(
			fakeTeamFactory,
		)

		fakeCheckBuildReadAccessHandlerFactory = auth.NewCheckBuildReadAccessHandlerFactory(fakeBuildFactory)
		fakeCheckBuildWriteAccessHandlerFactory = auth.NewCheckBuildWriteAccessHandlerFactory(fakeBuildFactory)
		fakeCheckWorkerTeamAccessHandlerFactory = auth.NewCheckWorkerTeamAccessHandlerFactory(workerFactory)
	})

	Describe("Wrap", func() {
		It("handles each route", func() {
			inputHandlers := rata.Handlers{}

			for _, route := range atc.Routes {
				inputHandlers[route.Name] = &stupidHandler{}
			}
			Expect(func() {
				wrappa.NewAPIAuthWrappa(
					fakeCheckPipelineAccessHandlerFactory,
					fakeCheckBuildReadAccessHandlerFactory,
					fakeCheckBuildWriteAccessHandlerFactory,
					fakeCheckWorkerTeamAccessHandlerFactory,
				).Wrap(inputHandlers)
			}).NotTo(Panic())
		})
	})
})
