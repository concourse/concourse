package db_test

import (
	"time"

	"github.com/concourse/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("User Factory", func() {

	var (
		err  error
		user db.User
	)

	JustBeforeEach(func() {
		user, err = userFactory.CreateOrUpdateUser("test", "github")
		Expect(err).ToNot(HaveOccurred())

	})

	Context("when user doesn't exist", func() {
		It("Insert a user with last_login now()", func() {
			Expect(user.Name()).To(Equal("test"))
			duration, err := time.ParseDuration("20s")
			Expect(err).ToNot(HaveOccurred())
			Expect(user.LastLogin()).Should(BeTemporally("~", time.Now(), duration))
		})
	})

	Context("when username exists but with different connector", func() {
		var user2 db.User

		JustBeforeEach(func() {
			user2, err = userFactory.CreateOrUpdateUser("test", "basic")
			Expect(err).ToNot(HaveOccurred())
		})

		It("Creates a different user", func() {

			var users []db.User

			Expect(user.ID()).ToNot(Equal(user2.ID()))

			users, err = userFactory.GetAllUsers()
			Expect(err).ToNot(HaveOccurred())
			Expect(users).To(HaveLen(2))
		})
	})

	Context("when username exists and with the same connector", func() {
		var updatedUser db.User

		JustBeforeEach(func() {
			updatedUser, err = userFactory.CreateOrUpdateUser("test", "github")
			Expect(err).ToNot(HaveOccurred())

		})

		It("Doesn't create a different user", func() {
			var users []db.User
			users, err = userFactory.GetAllUsers()
			Expect(err).ToNot(HaveOccurred())
			Expect(users).To(HaveLen(1))
		})

		It("Doesn't create a new record", func() {
			Expect(updatedUser.ID()).To(Equal(user.ID()))

		})

		It("Update the last_login time", func() {
			Expect(updatedUser.LastLogin()).NotTo(Equal(user.LastLogin()))
		})
	})

})
