package concourse_test

import (
	"net/http"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Skymarshal Handler User", func() {
	Describe("UserInfo", func() {
		var expectedUserInfo atc.UserInfo

		BeforeEach(func() {
			expectedURL := "/api/v1/user"

			expectedUserInfo = atc.UserInfo{
				Email:    "test@test.com",
				Teams:    map[string][]string{"test_team": {"owner", "viewer"}},
				UserId:   "test_user_id",
				UserName: "test_user_name",
				Name:     "test_name",
				IsAdmin:  false,
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedUserInfo),
				),
			)
		})

		It("returns user info", func() {
			result, err := client.UserInfo()
			Expect(err).NotTo(HaveOccurred())
			Expect(result.UserId).To(Equal("test_user_id"))
			Expect(result.UserName).To(Equal("test_user_name"))
			Expect(result.Name).To(Equal("test_name"))
			Expect(result.IsAdmin).To(Equal(false))
			Expect(result.Email).To(Equal(expectedUserInfo.Email))
			Expect(result.Teams).To(HaveKeyWithValue("test_team", ContainElement("owner")))
			Expect(result.Teams).To(HaveKeyWithValue("test_team", ContainElement("viewer")))
		})
	})
})
