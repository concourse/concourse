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

type stupidHandler struct{}

func (stupidHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
}

type descriptiveRoute struct {
	route   string
	handler http.Handler
}

func noopAuth(handler http.Handler) http.Handler {
	return auth.WrapHandler(
		handler,
		auth.NoopValidator{},
		auth.UnauthorizedRejector{},
	)
}

var _ = Describe("WebAuthWrappa", func() {
	var (
		publiclyViewable bool
		fakeValidator    *fakes.FakeValidator
	)

	BeforeEach(func() {
		publiclyViewable = true
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

				redirectRejector := auth.RedirectRejector{
					Location: "/login",
				}

				// do not rename
				expectedHandlers = rata.Handlers{
					web.Index:    noopAuth(inputHandlers[web.Index]),
					web.Pipeline: noopAuth(inputHandlers[web.Pipeline]),
					web.TriggerBuild: auth.WrapHandler(
						inputHandlers[web.TriggerBuild],
						fakeValidator,
						redirectRejector,
					),
					web.GetBuild:        noopAuth(inputHandlers[web.GetBuild]),
					web.GetBuilds:       noopAuth(inputHandlers[web.GetBuilds]),
					web.GetJoblessBuild: noopAuth(inputHandlers[web.GetJoblessBuild]),
					web.Public:          noopAuth(inputHandlers[web.Public]),
					web.GetResource:     noopAuth(inputHandlers[web.GetResource]),
					web.GetJob:          noopAuth(inputHandlers[web.GetJob]),
					web.LogIn:           noopAuth(inputHandlers[web.LogIn]),
					web.BasicAuth: auth.WrapHandler(
						inputHandlers[web.BasicAuth],
						fakeValidator,
						auth.BasicAuthRejector{},
					),
					web.Debug: auth.WrapHandler(
						inputHandlers[web.Debug],
						fakeValidator,
						redirectRejector,
					),
				}
			})

			It("validates sensitive routes, and noop validates public routes", func() {
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

				redirectRejector := auth.RedirectRejector{
					Location: "/login",
				}

				// do not rename
				parappa := func(handler http.Handler) http.Handler {
					return auth.WrapHandler(handler, fakeValidator, redirectRejector)
				}

				expectedHandlers = rata.Handlers{
					web.Index:           parappa(inputHandlers[web.Index]),
					web.Pipeline:        parappa(inputHandlers[web.Pipeline]),
					web.TriggerBuild:    parappa(inputHandlers[web.TriggerBuild]),
					web.GetBuild:        parappa(inputHandlers[web.GetBuild]),
					web.GetBuilds:       parappa(inputHandlers[web.GetBuilds]),
					web.GetJoblessBuild: parappa(inputHandlers[web.GetJoblessBuild]),
					web.Public:          parappa(inputHandlers[web.Public]),
					web.GetResource:     parappa(inputHandlers[web.GetResource]),
					web.GetJob:          parappa(inputHandlers[web.GetJob]),
					web.LogIn:           noopAuth(inputHandlers[web.LogIn]),
					web.BasicAuth: auth.WrapHandler(
						inputHandlers[web.BasicAuth],
						fakeValidator,
						auth.BasicAuthRejector{},
					),
					web.Debug: parappa(inputHandlers[web.Debug]),
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
