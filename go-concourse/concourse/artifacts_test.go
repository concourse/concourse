package concourse_test

import (
	"bytes"
	"io/ioutil"
	"net/http"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ArtifactRepository", func() {

	Describe("CreateArtifact", func() {
		Context("when creating the artifact fails", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/api/v1/teams/some-team/artifacts"),
						ghttp.VerifyHeader(http.Header{"Content-Type": {"application/octet-stream"}}),
						ghttp.VerifyBody([]byte("some-contents")),
						ghttp.RespondWith(http.StatusInternalServerError, nil),
					),
				)
			})

			It("errors", func() {
				_, err := team.CreateArtifact(bytes.NewBufferString("some-contents"), "some-platform")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when creating the artifact succeeds", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/api/v1/teams/some-team/artifacts"),
						ghttp.VerifyHeader(http.Header{"Content-Type": {"application/octet-stream"}}),
						ghttp.VerifyBody([]byte("some-contents")),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, atc.WorkerArtifact{ID: 17}),
					),
				)
			})

			It("returns json", func() {
				artifact, err := team.CreateArtifact(bytes.NewBufferString("some-contents"), "some-platform")
				Expect(err).NotTo(HaveOccurred())
				Expect(artifact.ID).To(Equal(17))
			})
		})
	})

	Describe("GetArtifact", func() {
		Context("when getting the artifact fails", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/some-team/artifacts/17"),
						ghttp.RespondWith(http.StatusInternalServerError, ""),
					),
				)
			})

			It("errors", func() {
				_, err := team.GetArtifact(17)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the artifact exsits", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/some-team/artifacts/17"),
						ghttp.RespondWith(http.StatusOK, "some-other-contents"),
					),
				)
			})

			It("returns the contents", func() {
				contents, err := team.GetArtifact(17)
				Expect(err).NotTo(HaveOccurred())
				Expect(ioutil.ReadAll(contents)).To(Equal([]byte("some-other-contents")))
			})
		})
	})
})
