package accessor_test

import (
	"encoding/base64"
	"errors"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Batcher", func() {
	var (
		user            *dbfakes.FakeUser
		userTracker     accessor.UserTracker
		fakeUserFactory *dbfakes.FakeUserFactory
		logger          *lagertest.TestLogger
		err             error
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		user = new(dbfakes.FakeUser)
		user.NameReturns("some-user")
		user.ConnectorReturns("connector")
		user.SubReturns(base64.StdEncoding.EncodeToString([]byte("some-user" + "connector")))

		fakeUserFactory = new(dbfakes.FakeUserFactory)
		userTracker = accessor.NewBatcher(logger, fakeUserFactory, 0, 1)
	})

	JustBeforeEach(func() {
		err = userTracker.CreateOrUpdateUser(user.Name(), user.Connector(), user.Sub())
		Expect(err).ToNot(HaveOccurred())
	})

	It("upserts the user", func() {
		Eventually(fakeUserFactory.BatchUpsertUsersCallCount).Should(Equal(1))

		users := fakeUserFactory.BatchUpsertUsersArgsForCall(0)

		sub := base64.StdEncoding.EncodeToString([]byte("some-user" + "connector"))
		Expect(len(users)).To(Equal(1))
		Expect(users[sub].Name()).To(Equal("some-user"))
		Expect(users[sub].Connector()).To(Equal("connector"))
		Expect(users[sub].Sub()).To(Equal(sub))
	})

	Context("when batch upserting users fails", func() {
		BeforeEach(func() {
			fakeUserFactory.BatchUpsertUsersReturns(errors.New("nope"))
		})

		It("logs the error", func() {
			Eventually(logger.LogMessages).Should(ContainElement("test.failed-to-upsert-user"))
		})
	})
})
