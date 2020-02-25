package accessor_test

import (
	"encoding/base64"

	"code.cloudfoundry.org/lager"
	"github.com/MasterOfBinary/gobatch/batch"
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
		logger          lager.Logger
		err             error
	)

	BeforeEach(func() {
		logger = lager.NewLogger("test")

		user = new(dbfakes.FakeUser)
		user.NameReturns("some-user")
		user.ConnectorReturns("connector")
		user.SubReturns(base64.StdEncoding.EncodeToString([]byte("some-user" + "connector")))

		fakeUserFactory = new(dbfakes.FakeUserFactory)
		userTracker = accessor.NewBatcher(logger, fakeUserFactory, &batch.ConfigValues{
			MaxItems: 1,
		})
	})

	JustBeforeEach(func() {
		err = userTracker.CreateOrUpdateUser(user.Name(), user.Connector(), user.Sub())
		Expect(err).ToNot(HaveOccurred())
	})

	Context("when batcher is config to flush immediately", func() {
		BeforeEach(func() {
			userTracker = accessor.NewBatcher(logger, fakeUserFactory, &batch.ConfigValues{
				MaxItems: 1,
			})
		})

		It("upsert the user", func() {
			Eventually(func() int {
				return fakeUserFactory.BatchUpsertUsersCallCount()
			}).Should(Equal(1))

			users := fakeUserFactory.BatchUpsertUsersArgsForCall(0)

			sub := base64.StdEncoding.EncodeToString([]byte("some-user" + "connector"))
			Expect(len(users)).To(Equal(1))
			Expect(users[sub].Name()).To(Equal("some-user"))
			Expect(users[sub].Connector()).To(Equal("connector"))
			Expect(users[sub].Sub()).To(Equal(base64.StdEncoding.EncodeToString([]byte("some-user" + "connector"))))
		})
	})
})
