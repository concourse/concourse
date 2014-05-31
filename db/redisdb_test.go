package db_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	Builds "github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
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

		err = db.SaveBuildStatus("some-job", build.ID, Builds.StatusStarted)
		Ω(err).ShouldNot(HaveOccurred())

		build, err = db.GetBuild("some-job", build.ID)
		Ω(build.ID).Should(Equal(1))
		Ω(build.Status).Should(Equal(Builds.StatusStarted))

		build, err = db.GetCurrentBuild("some-job")
		Ω(build.ID).Should(Equal(1))
		Ω(build.Status).Should(Equal(Builds.StatusStarted))

		_, err = db.BuildLog("some-job", 1)
		Ω(err).Should(HaveOccurred())

		err = db.SaveBuildLog("some-job", 1, []byte("some log"))
		Ω(err).ShouldNot(HaveOccurred())

		log, err := db.BuildLog("some-job", 1)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(string(log)).Should(Equal("some log"))

		_, err = db.GetCurrentVersion("some-job", "some-input")
		Ω(err).Should(HaveOccurred())

		currentVersion := Builds.Version{"some": "version"}
		err = db.SaveCurrentVersion("some-job", "some-input", currentVersion)
		Ω(err).ShouldNot(HaveOccurred())

		gotCurrentVersion, err := db.GetCurrentVersion("some-job", "some-input")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(gotCurrentVersion).Should(Equal(currentVersion))

		output1 := Builds.Version{"ver": "1"}
		output2 := Builds.Version{"ver": "2"}
		output3 := Builds.Version{"ver": "3"}

		err = db.SaveOutputVersion("some-job", 1, "some-input", output1)
		Ω(err).ShouldNot(HaveOccurred())

		err = db.SaveOutputVersion("some-job", 2, "some-input", output2)
		Ω(err).ShouldNot(HaveOccurred())

		err = db.SaveOutputVersion("some-other-job", 1, "some-input", output1)
		Ω(err).ShouldNot(HaveOccurred())

		err = db.SaveOutputVersion("some-other-job", 2, "some-input", output2)
		Ω(err).ShouldNot(HaveOccurred())

		err = db.SaveOutputVersion("some-other-job", 3, "some-input", output3)
		Ω(err).ShouldNot(HaveOccurred())

		outputs, err := db.GetCommonOutputs([]string{"some-job", "some-other-job"}, "some-input")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(outputs).Should(Equal([]Builds.Version{output1, output2}))

		outputs, err = db.GetCommonOutputs([]string{"some-other-job"}, "some-input")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(outputs).Should(Equal([]Builds.Version{output1, output2, output3}))

		buildMetadata := []Builds.MetadataField{
			{
				Name:  "meta1",
				Value: "value1",
			},
			{
				Name:  "meta2",
				Value: "value2",
			},
		}

		input1 := Builds.Input{
			Name:     "some-input",
			Source:   config.Source{"some": "source"},
			Version:  Builds.Version{"ver": "1"},
			Metadata: buildMetadata,
		}

		err = db.SaveBuildInput("some-job", build.ID, input1)
		Ω(err).ShouldNot(HaveOccurred())

		input2 := Builds.Input{
			Name:    "some-other-input",
			Source:  config.Source{"some": "other-source"},
			Version: Builds.Version{"ver": "2"},
		}

		err = db.SaveBuildInput("some-job", build.ID, input2)
		Ω(err).ShouldNot(HaveOccurred())

		build, err = db.GetBuild("some-job", build.ID)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(build.Inputs).Should(Equal([]Builds.Input{input1, input2}))
	})
})
