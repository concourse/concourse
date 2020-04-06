package concourse_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Skymarshal Handler User", func() {
	Describe("UserInfo", func() {
		var expectedUserInfo map[string]interface{}

		BeforeEach(func() {
			expectedURL := "/api/v1/user"

			expectedUserInfo = map[string]interface{}{
				"email":     "test@test.com",
				"teams":     map[string][]string{"test_team": {"owner", "viewer"}},
				"user_id":   "test_user_id",
				"user_name": "test_user_name",
				"name":      "test_name",
				"is_admin":  false,
				"exp":       1123123,
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
			Expect(result["user_id"]).To(Equal("test_user_id"))
			Expect(result["user_name"]).To(Equal("test_user_name"))
			Expect(result["name"]).To(Equal("test_name"))
			Expect(result["is_admin"]).To(Equal(false))
			Expect(result["exp"]).To(BeNumerically("==", 1123123))
			Expect(result["email"]).To(Equal(expectedUserInfo["email"]))
			Expect(result["teams"]).To(HaveKeyWithValue("test_team", ContainElement("owner")))
			Expect(result["teams"]).To(HaveKeyWithValue("test_team", ContainElement("viewer")))
		})
	})
})
