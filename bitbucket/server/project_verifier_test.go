package server_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	"errors"
	"github.com/concourse/skymarshal/bitbucket/bitbucketfakes"
	"github.com/concourse/skymarshal/bitbucket/server"
	"github.com/concourse/skymarshal/verifier"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
)

var _ = Describe("ProjectVerifier", func() {
	var (
		projects   []string
		fakeClient *bitbucketfakes.FakeClient

		verifier verifier.Verifier
	)

	BeforeEach(func() {
		projects = []string{
			"some-project",
			"some-project-two",
		}
		fakeClient = new(bitbucketfakes.FakeClient)

		verifier = server.NewProjectVerifier(projects, fakeClient)
	})

	Describe("Verifiy", func() {
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

		Context("when the client yields projects", func() {
			Context("including the desired project", func() {
				BeforeEach(func() {
					fakeClient.ProjectsReturns(
						[]string{
							"some-other-project",
							"some-project",
						},
						nil,
					)
				})

				It("succeeds", func() {
					Expect(verifyErr).ToNot(HaveOccurred())
				})

				It("returns true", func() {
					Expect(verified).To(BeTrue())
				})
			})

			Context("not including the desired project", func() {
				BeforeEach(func() {
					fakeClient.ProjectsReturns(
						[]string{
							"some-other-project",
						},
						nil,
					)
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
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeClient.ProjectsReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(verifyErr).To(Equal(disaster))
			})
		})
	})
})
