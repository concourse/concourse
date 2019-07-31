package concourse_test

import (
	"net/http"
	"time"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

const inputDateLayout = "2006-01-02"

var _ = Describe("ATC Users Handler", func() {

	Describe("ListActiveUsersSince", func() {

		expectedURL := "/api/v1/users"
		expectedTime := time.Now().AddDate(0, -2, 0)
		expectedUsers := []atc.User{
			{ID: 1, Username: "test", Connector: "github", LastLogin: time.Now().AddDate(0, -1, 0).UTC().Unix()},
		}
		Context("users exist", func() {

			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL, "since="+expectedTime.Format(inputDateLayout)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedUsers),
					),
				)
			})
			It("returns the correct users", func() {
				users, err := client.ListActiveUsersSince(expectedTime)

				Expect(err).NotTo(HaveOccurred())
				Expect(len(users)).To(Equal(1))
				Expect(users[0]).To(Equal(expectedUsers[0]))
			})
		})
	})
})
