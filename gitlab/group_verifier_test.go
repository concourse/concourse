package gitlab_test

import (
	"errors"
	"net/http"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/concourse/skymarshal/gitlab"
	"github.com/concourse/skymarshal/gitlab/gitlabfakes"
	"github.com/concourse/skymarshal/verifier"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GroupVerifier", func() {

	var (
		groups     []string
		fakeClient *gitlabfakes.FakeClient

		verifier verifier.Verifier
	)

	BeforeEach(func() {
		groups = []string{"some-group", "another-group"}
		fakeClient = new(gitlabfakes.FakeClient)

		verifier = NewGroupVerifier(groups, fakeClient)
	})

	Describe("Verify", func() {
		var (
			httpClient *http.Client

			verified  bool
			verifyErr error
		)

		BeforeEach(func() {
			httpClient = &http.Client{}
		})

		JustBeforeEach(func() {
			verified, verifyErr = verifier.Verify(lagertest.NewTestLogger("test"), httpClient)
		})

		Context("when the client has groups", func() {
			Context("including one of the desired groups", func() {
				BeforeEach(func() {
					fakeClient.GroupsReturns([]string{groups[0], "bogus-group"}, nil)
				})

				It("succeeds", func() {
					Expect(verifyErr).ToNot(HaveOccurred())
				})

				It("returns true", func() {
					Expect(verified).To(BeTrue())
				})
			})

			Context("not including the desired groups", func() {
				BeforeEach(func() {
					fakeClient.GroupsReturns([]string{"bogus-group"}, nil)
				})

				It("succeeds", func() {
					Expect(verifyErr).ToNot(HaveOccurred())
				})

				It("returns false", func() {
					Expect(verified).To(BeFalse())
				})
			})
		})

		Context("when the client fails", func() {
			unexpected := errors.New("fake err")

			BeforeEach(func() {
				fakeClient.GroupsReturns(nil, unexpected)
			})

			It("returns the error", func() {
				Expect(verifyErr).To(Equal(unexpected))
			})
		})
	})
})
