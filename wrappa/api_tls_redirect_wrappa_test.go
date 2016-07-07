package wrappa_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/wrappa"
	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("APITLSRedirectWrappa", func() {
	var expectedHandlers rata.Handlers

	Describe("Wrap", func() {
		var (
			inputHandlers rata.Handlers

			wrappedHandlers rata.Handlers
		)

		BeforeEach(func() {
			inputHandlers = rata.Handlers{}
			expectedHandlers = rata.Handlers{}

			for _, route := range atc.Routes {
				inputHandlers[route.Name] = &stupidHandler{}
				if route.Name == atc.ReadPipe || (route.Method != "GET" && route.Method != "HEAD") {
					expectedHandlers[route.Name] = &stupidHandler{}
				} else {
					expectedHandlers[route.Name] = wrappa.RedirectingAPIHandler("redirected-external-host")
				}
			}

		})

		JustBeforeEach(func() {
			wrappedHandlers = wrappa.NewAPITLSRedirectWrappa("redirected-external-host").Wrap(inputHandlers)
		})

		It("redirects everything except ReadPipe endpoint", func() {
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
