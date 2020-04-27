package db_test

import (
	"encoding/base64"
	"time"

	"github.com/concourse/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("User Factory", func() {

	var (
		err   error
		users []db.User
	)

	JustBeforeEach(func() {
		err = userFactory.CreateOrUpdateUser("test", "github",
			base64.StdEncoding.EncodeToString([]byte("test"+"github")))
		Expect(err).ToNot(HaveOccurred())

		users, err = userFactory.GetAllUsers()
		Expect(err).ToNot(HaveOccurred())
	})

	Context("when user doesn't exist", func() {
		It("Insert a user with last_login now()", func() {
			Expect(users).To(HaveLen(1))
			Expect(users[0].Name()).To(Equal("test"))
			Expect(users[0].LastLogin()).Should(BeTemporally("~", time.Now(), time.Second*20))
		})
	})

	Context("when username exists but with different connector", func() {
		BeforeEach(func() {
			err = userFactory.CreateOrUpdateUser("test", "basic",
				base64.StdEncoding.EncodeToString([]byte("test"+"basic")))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Creates a different user", func() {
			Expect(users).To(HaveLen(2))
			Expect(users[0].ID()).ToNot(Equal(users[1].ID()))
		})
	})

	Context("when username exists and with the same connector", func() {
		var previousLastLogin time.Time

		BeforeEach(func() {
			err = userFactory.CreateOrUpdateUser("test", "github",
				base64.StdEncoding.EncodeToString([]byte("test"+"github")))
			Expect(err).ToNot(HaveOccurred())

			users, err = userFactory.GetAllUsers()
			Expect(err).ToNot(HaveOccurred())
			previousLastLogin = users[0].LastLogin()
		})

		It("Doesn't create a different user", func() {
			Expect(users).To(HaveLen(1))
		})

		It("Doesn't create a new record", func() {
			Expect(users[0].Name()).To(Equal("test"))
		})

		It("Update the last_login time", func() {
			Expect(users[0].LastLogin()).NotTo(Equal(previousLastLogin))
		})
	})
})
