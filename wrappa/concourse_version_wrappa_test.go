package wrappa_test

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/wrappa"
	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConcourseVersionWrappa", func() {
	var (
		version string
	)

	BeforeEach(func() {
		version = "1.2.3-test"
	})

	versioned := func(handler http.Handler) http.Handler {
		return wrappa.VersionedHandler{
			Version: version,
			Handler: handler,
		}
	}

	Describe("Wrap", func() {
		var (
			inputHandlers rata.Handlers

			expectedHandlers rata.Handlers

			wrappedHandlers rata.Handlers
		)

		BeforeEach(func() {
			inputHandlers = rata.Handlers{}

			for _, route := range atc.Routes {
				inputHandlers[route.Name] = &stupidHandler{}
			}

			expectedHandlers = rata.Handlers{}

			// wrap everything
			for route, handler := range inputHandlers {
				expectedHandlers[route] = versioned(handler)
			}
		})

		JustBeforeEach(func() {
			wrappedHandlers = wrappa.NewConcourseVersionWrappa(
				version,
			).Wrap(inputHandlers)
		})

		It("wraps every single handler with a version reporting handler", func() {
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
