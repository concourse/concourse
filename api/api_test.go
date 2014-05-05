package api_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/winston-ci/winston/api"
	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/db"
	"github.com/winston-ci/winston/redisrunner"
)

var _ = Describe("API", func() {
	var redisRunner *redisrunner.Runner
	var redis db.DB

	var server *httptest.Server
	var client *http.Client

	BeforeEach(func() {
		redisRunner = redisrunner.NewRunner()
		redisRunner.Start()

		redis = db.NewRedis(redisRunner.Pool())

		handler, err := api.New(redis)
		Ω(err).ShouldNot(HaveOccurred())

		server = httptest.NewServer(handler)

		client = &http.Client{
			Transport: &http.Transport{},
		}
	})

	AfterEach(func() {
		server.Close()
		redisRunner.Stop()
	})

	Describe("PUT /builds/:job/:build/result", func() {
		var build builds.Build

		var response *http.Response

		BeforeEach(func() {
			var err error

			build, err = redis.CreateBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			reqPayload := bytes.NewBufferString(`{"status":"succeeded"}`)

			req, err := http.NewRequest("PUT", server.URL+"/builds/some-job/1/result", reqPayload)
			Ω(err).ShouldNot(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("updates the build's state", func() {
			Ω(response.StatusCode).Should(Equal(http.StatusOK))

			updatedBuild, err := redis.GetBuild("some-job", build.ID)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(updatedBuild.State).Should(Equal(builds.BuildStateSucceeded))
		})
	})

	Describe("/builds/:guid/log/input", func() {
	})
})
