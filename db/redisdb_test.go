package db_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ProleBuilds "github.com/winston-ci/prole/api/builds"

	Builds "github.com/winston-ci/winston/builds"
	. "github.com/winston-ci/winston/db"
	"github.com/winston-ci/winston/redisrunner"
)

var _ = Describe("RedisDB", func() {
	var redisRunner *redisrunner.Runner

	var db DB

	BeforeEach(func() {
		redisRunner = redisrunner.NewRunner()
		redisRunner.Start()

		db = NewRedis(redisRunner.Pool())
	})

	AfterEach(func() {
		redisRunner.Stop()
	})

	It("works", func() {
		builds, err := db.Builds("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(builds).Should(BeEmpty())

		build, err := db.CreateBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(build.ID).Should(Equal(1))
		Ω(build.Status).Should(Equal(Builds.StatusPending))

		builds, err = db.Builds("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(builds).Should(HaveLen(1))
		Ω(builds[0].ID).Should(Equal(1))
		Ω(builds[0].Status).Should(Equal(Builds.StatusPending))

		build, err = db.SaveBuildStatus("some-job", build.ID, Builds.StatusStarted)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(build.ID).Should(Equal(1))
		Ω(build.Status).Should(Equal(Builds.StatusStarted))

		build, err = db.GetBuild("some-job", build.ID)
		Ω(build.ID).Should(Equal(1))
		Ω(build.Status).Should(Equal(Builds.StatusStarted))

		_, err = db.BuildLog("some-job", 1)
		Ω(err).Should(HaveOccurred())

		err = db.SaveBuildLog("some-job", 1, []byte("some log"))
		Ω(err).ShouldNot(HaveOccurred())

		log, err := db.BuildLog("some-job", 1)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(string(log)).Should(Equal("some log"))

		_, err = db.GetCurrentSource("some-resource")
		Ω(err).Should(HaveOccurred())

		source := ProleBuilds.Source("some source")
		err = db.SaveCurrentSource("some-resource", source)
		Ω(err).ShouldNot(HaveOccurred())

		currentSource, err := db.GetCurrentSource("some-resource")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(currentSource).Should(Equal(source))
	})
})
