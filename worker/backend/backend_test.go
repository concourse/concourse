package backend_test

import (
	"errors"

	"github.com/concourse/concourse/worker/backend"
	"github.com/concourse/concourse/worker/backend/libcontainerd/libcontainerdfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Backend", func() {

	Describe("Ping", func() {

		var (
			err    error
			be     backend.Backend
			client *libcontainerdfakes.FakeClient
		)

		BeforeEach(func() {
			client = new(libcontainerdfakes.FakeClient)
			be = backend.New(client)
		})

		JustBeforeEach(func() {
			err = be.Ping()
		})

		It("calls containerd's Version", func() {
			Expect(client.VersionCallCount()).To(Equal(1))
		})

		Context("failing to call `version`", func() {
			BeforeEach(func() {
				client.VersionReturns(errors.New("errrr"))
			})

			It("errors", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
