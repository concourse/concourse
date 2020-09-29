package wrappa_test

import (
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/wrappa"

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
			inputHandlers map[string]http.Handler

			expectedHandlers map[string]http.Handler

			wrappedHandlers map[string]http.Handler
		)

		BeforeEach(func() {
			inputHandlers = map[string]http.Handler{}

			for _, routeName := range atc.RouteNames() {
				inputHandlers[routeName] = &stupidHandler{}
			}

			expectedHandlers = map[string]http.Handler{}

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
