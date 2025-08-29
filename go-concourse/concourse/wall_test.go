package concourse_test

import (
	"net/http"
	"time"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Wall functions", func() {
	Describe("GetWall function", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/wall"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Wall{Message: "test message", TTL: time.Minute}),
				),
			)
		})

		It("returns the wall message", func() {
			wall, err := client.GetWall()
			Expect(err).NotTo(HaveOccurred())
			Expect(wall.Message).To(Equal("test message"))
			Expect(wall.TTL).To(Equal(time.Minute))
		})
	})

	Describe("SetWall function", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", "/api/v1/wall"),
					ghttp.RespondWith(http.StatusOK, nil),
				),
			)
		})

		It("sends the message and ttl", func() {
			err := client.SetWall(atc.Wall{Message: "test message", TTL: time.Hour})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("ClearWall function", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("DELETE", "/api/v1/wall"),
					ghttp.RespondWith(http.StatusOK, nil),
				),
			)
		})

		It("clears the wall", func() {
			err := client.ClearWall()
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
