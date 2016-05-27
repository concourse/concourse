package wrappa_test

import (
	"github.com/concourse/atc/auth/fakes"
	"github.com/concourse/atc/web"
	"github.com/concourse/atc/wrappa"
	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WebAuthWrappa", func() {
	var (
		fakeValidator         *fakes.FakeValidator
		fakeUserContextReader *fakes.FakeUserContextReader

		expectedHandlers rata.Handlers
	)

	BeforeEach(func() {
		fakeValidator = new(fakes.FakeValidator)
	})

	Describe("Wrap", func() {
		var (
			inputHandlers rata.Handlers

			wrappedHandlers rata.Handlers
		)

		BeforeEach(func() {
			inputHandlers = rata.Handlers{}

			for _, route := range web.Routes {
				inputHandlers[route.Name] = &stupidHandler{}
			}

			expectedHandlers = rata.Handlers{
				web.Index:                 inputHandlers[web.Index],
				web.Pipeline:              inputHandlers[web.Pipeline],
				web.TriggerBuild:          inputHandlers[web.TriggerBuild],
				web.GetBuild:              inputHandlers[web.GetBuild],
				web.GetBuilds:             inputHandlers[web.GetBuilds],
				web.GetJoblessBuild:       inputHandlers[web.GetJoblessBuild],
				web.Public:                inputHandlers[web.Public],
				web.GetResource:           inputHandlers[web.GetResource],
				web.GetJob:                inputHandlers[web.GetJob],
				web.LogIn:                 inputHandlers[web.LogIn],
				web.TeamLogIn:             inputHandlers[web.TeamLogIn],
				web.ProcessBasicAuthLogIn: inputHandlers[web.ProcessBasicAuthLogIn],
				web.GetBasicAuthLogIn:     inputHandlers[web.GetBasicAuthLogIn],
			}
		})

		JustBeforeEach(func() {
			wrappedHandlers = wrappa.NewWebAuthWrappa(
				fakeValidator,
				fakeUserContextReader,
			).Wrap(inputHandlers)
		})

		It("requires validation for the basic auth route", func() {
			for name, _ := range inputHandlers {
				Expect(descriptiveRoute{
					route:   name,
					handler: wrappedHandlers[name],
				}).To(Equal(descriptiveRoute{
					route:   name,
					handler: expectedHandlers[name],
				}))
			}
		})
	})
})
