package concourse_test

import (
	"fmt"
	"net/http"

	"github.com/concourse/concourse/atc"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Components", func() {
	Describe("ListComponents", func() {
		var expectedComponents []atc.Component

		BeforeEach(func() {
			expectedComponents = []atc.Component{
				{Name: "scheduler", Paused: false},
				{Name: "tracker", Paused: true},
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/components"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedComponents),
				),
			)
		})

		It("returns all the components", func() {
			components, err := client.ListComponents()
			Expect(err).NotTo(HaveOccurred())
			Expect(components).To(Equal(expectedComponents))
		})

		Context("when the user is forbidden", func() {
			BeforeEach(func() {
				atcServer.Reset()
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/components"),
						ghttp.RespondWith(http.StatusForbidden, nil),
					),
				)
			})

			It("returns a forbidden error", func() {
				_, err := client.ListComponents()
				Expect(err).To(MatchError("must be an owner of the 'main' team to interact with components"))
			})
		})
	})

	Describe("PauseComponent", func() {
		var (
			expectedStatus int
			componentName  = "scheduler"
			expectedURL    = fmt.Sprintf("/api/v1/components/%s/pause", componentName)
		)

		JustBeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", expectedURL),
					ghttp.RespondWith(expectedStatus, nil),
				),
			)
		})

		Context("when the component exists", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusOK
			})

			It("returns no error", func() {
				err := client.PauseComponent(componentName)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the component does not exist", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusNotFound
			})

			It("returns an error", func() {
				err := client.PauseComponent(componentName)
				Expect(err).To(MatchError("component 'scheduler' not found"))
			})
		})

		Context("when the user is forbidden", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusForbidden
			})

			It("returns a forbidden error", func() {
				err := client.PauseComponent(componentName)
				Expect(err).To(MatchError("must be an owner of the 'main' team to interact with components"))
			})
		})
	})

	Describe("UnpauseComponent", func() {
		var (
			expectedStatus int
			componentName  = "scheduler"
			expectedURL    = fmt.Sprintf("/api/v1/components/%s/unpause", componentName)
		)

		JustBeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", expectedURL),
					ghttp.RespondWith(expectedStatus, nil),
				),
			)
		})

		Context("when the component exists", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusOK
			})

			It("returns no error", func() {
				err := client.UnpauseComponent(componentName)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the component does not exist", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusNotFound
			})

			It("returns an error", func() {
				err := client.UnpauseComponent(componentName)
				Expect(err).To(MatchError("component 'scheduler' not found"))
			})
		})

		Context("when the user is forbidden", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusForbidden
			})

			It("returns a forbidden error", func() {
				err := client.UnpauseComponent(componentName)
				Expect(err).To(MatchError("must be an owner of the 'main' team to interact with components"))
			})
		})
	})

	Describe("PauseAllComponents", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", "/api/v1/components/pause"),
					ghttp.RespondWith(http.StatusOK, nil),
				),
			)
		})

		It("returns no error", func() {
			err := client.PauseAllComponents()
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("UnpauseAllComponents", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", "/api/v1/components/unpause"),
					ghttp.RespondWith(http.StatusOK, nil),
				),
			)
		})

		It("returns no error", func() {
			err := client.UnpauseAllComponents()
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
