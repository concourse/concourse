package concourse_test

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Build Plans", func() {
	Describe("BuildPlan", func() {
		Context("when build exists and has a plan", func() {
			// var plan json.RawMessage = json.RawMessage(`{}`)
			expectedPlanJson := json.RawMessage(`{"do":"stuff"}`)
			expectedBuildPlan := atc.PublicBuildPlan{
				Schema: "exec.v2",
				Plan:   &expectedPlanJson,
			}
			expectedURL := "/api/v1/builds/1234/plan"

			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuildPlan),
					),
				)
			})

			It("returns the given build", func() {
				build, found, err := client.BuildPlan(1234)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build).To(Equal(expectedBuildPlan))
			})
		})

		Context("when build does not exist or has no plan", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/builds/1234/plan"

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusNotFound, nil),
					),
				)
			})

			It("returns false and no error", func() {
				_, found, err := client.BuildPlan(1234)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})
})
