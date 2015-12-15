package wrappa_test

import (
	"net/http"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/fakes"
	"github.com/concourse/atc/web"
	"github.com/concourse/atc/wrappa"
	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WebAuthWrappa", func() {
	var (
		publiclyViewable bool
		fakeValidator    *fakes.FakeValidator
	)

	BeforeEach(func() {
		publiclyViewable = true
		fakeValidator = new(fakes.FakeValidator)
	})

	unauthed := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			handler,
			fakeValidator,
		)
	}

	authed := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			auth.CheckAuthHandler(
				handler,
				auth.RedirectRejector{
					Location: "/login",
				},
			),
			fakeValidator,
		)
	}

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
		})

		JustBeforeEach(func() {
			wrappedHandlers = wrappa.NewWebAuthWrappa(
				publiclyViewable,
				fakeValidator,
			).Wrap(inputHandlers)
		})

		Context("when publicly viewable", func() {
			var expectedHandlers rata.Handlers

			BeforeEach(func() {
				publiclyViewable = true

				expectedHandlers = rata.Handlers{
					web.Index:           unauthed(inputHandlers[web.Index]),
					web.Pipeline:        unauthed(inputHandlers[web.Pipeline]),
					web.TriggerBuild:    authed(inputHandlers[web.TriggerBuild]),
					web.GetBuild:        unauthed(inputHandlers[web.GetBuild]),
					web.GetBuilds:       unauthed(inputHandlers[web.GetBuilds]),
					web.GetJoblessBuild: unauthed(inputHandlers[web.GetJoblessBuild]),
					web.Public:          unauthed(inputHandlers[web.Public]),
					web.GetResource:     unauthed(inputHandlers[web.GetResource]),
					web.GetJob:          unauthed(inputHandlers[web.GetJob]),
					web.LogIn:           unauthed(inputHandlers[web.LogIn]),
					web.BasicAuth: auth.WrapHandler(
						auth.CheckAuthHandler(
							inputHandlers[web.BasicAuth],
							auth.BasicAuthRejector{},
						),
						fakeValidator,
					),
				}
			})

			It("requires validation for sensitive routes", func() {
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

		Context("when not publicly viewable", func() {
			var expectedHandlers rata.Handlers

			BeforeEach(func() {
				publiclyViewable = false

				expectedHandlers = rata.Handlers{
					web.Index:           authed(inputHandlers[web.Index]),
					web.Pipeline:        authed(inputHandlers[web.Pipeline]),
					web.TriggerBuild:    authed(inputHandlers[web.TriggerBuild]),
					web.GetBuild:        authed(inputHandlers[web.GetBuild]),
					web.GetBuilds:       authed(inputHandlers[web.GetBuilds]),
					web.GetJoblessBuild: authed(inputHandlers[web.GetJoblessBuild]),
					web.Public:          unauthed(inputHandlers[web.Public]),
					web.GetResource:     authed(inputHandlers[web.GetResource]),
					web.GetJob:          authed(inputHandlers[web.GetJob]),
					web.LogIn:           unauthed(inputHandlers[web.LogIn]),
					web.BasicAuth: auth.WrapHandler(
						auth.CheckAuthHandler(
							inputHandlers[web.BasicAuth],
							auth.BasicAuthRejector{},
						),
						fakeValidator,
					),
				}
			})

			It("wraps all endpoints except for login", func() {
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
})
