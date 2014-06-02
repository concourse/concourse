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

		pending, err := db.CreateBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(pending.ID).Should(Equal(2))
		Ω(pending.Status).Should(Equal(Builds.StatusPending))

		build, err = db.GetCurrentBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(build.ID).Should(Equal(1))
		Ω(build.Status).Should(Equal(Builds.StatusPending))

		build, err = db.StartBuild("some-job", build.ID, false)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(build.Status).Should(Equal(Builds.StatusStarted))

		build, err = db.GetCurrentBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(build.ID).Should(Equal(1))
		Ω(build.Status).Should(Equal(Builds.StatusStarted))

		builds, err = db.Builds("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(builds).Should(HaveLen(2))
		Ω(builds[0].ID).Should(Equal(build.ID))
		Ω(builds[0].Status).Should(Equal(Builds.StatusStarted))
		Ω(builds[1].ID).Should(Equal(pending.ID))
		Ω(builds[1].Status).Should(Equal(Builds.StatusPending))

		err = db.SaveBuildStatus("some-job", build.ID, Builds.StatusSucceeded)
		Ω(err).ShouldNot(HaveOccurred())

		build, err = db.GetBuild("some-job", build.ID)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(build.Status).Should(Equal(Builds.StatusSucceeded))

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

	Context("when the first build is created", func() {
		var firstBuild Builds.Build

		var job string

		BeforeEach(func() {
			var err error

			job = "some-job"

			firstBuild, err = db.CreateBuild(job)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(firstBuild.ID).Should(Equal(1))
			Ω(firstBuild.Status).Should(Equal(Builds.StatusPending))
		})

		Describe("starting the build", func() {
			It("updates the status", func() {
				started, err := db.StartBuild(job, firstBuild.ID, false)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(started.ID).Should(Equal(firstBuild.ID))
				Ω(started.Status).Should(Equal(Builds.StatusStarted))
			})

			Context("serially", func() {
				It("updates the status", func() {
					started, err := db.StartBuild(job, firstBuild.ID, true)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(started.ID).Should(Equal(firstBuild.ID))
					Ω(started.Status).Should(Equal(Builds.StatusStarted))
				})
			})
		})

		Context("and second build is created", func() {
			var secondBuild Builds.Build

			BeforeEach(func() {
				var err error

				secondBuild, err = db.CreateBuild(job)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(secondBuild.ID).Should(Equal(2))
				Ω(secondBuild.Status).Should(Equal(Builds.StatusPending))
			})

			Describe("starting the second build", func() {
				It("updates the status to started", func() {
					started, err := db.StartBuild(job, secondBuild.ID, false)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(started.ID).Should(Equal(secondBuild.ID))
					Ω(started.Status).Should(Equal(Builds.StatusStarted))
				})

				Context("with serial true", func() {
					It("does not update the status", func() {
						started, err := db.StartBuild(job, secondBuild.ID, true)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(started.ID).Should(Equal(secondBuild.ID))
						Ω(started.Status).Should(Equal(Builds.StatusPending))
					})
				})
			})

			Describe("after the first build starts", func() {
				BeforeEach(func() {
					var err error

					firstBuild, err = db.StartBuild(job, firstBuild.ID, false)
					Ω(err).ShouldNot(HaveOccurred())
				})

				Context("when the second build is started serially", func() {
					It("does not update the status", func() {
						started, err := db.StartBuild(job, secondBuild.ID, true)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(started.ID).Should(Equal(secondBuild.ID))
						Ω(started.Status).Should(Equal(Builds.StatusPending))
					})
				})

				for _, s := range []Builds.Status{Builds.StatusSucceeded, Builds.StatusFailed, Builds.StatusErrored} {
					status := s

					Context("and the first build's status changes to "+string(status), func() {
						BeforeEach(func() {
							err := db.SaveBuildStatus(job, firstBuild.ID, status)
							Ω(err).ShouldNot(HaveOccurred())
						})

						Context("and the second build is started serially", func() {
							It("updates the status", func() {
								started, err := db.StartBuild(job, secondBuild.ID, true)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(started.ID).Should(Equal(secondBuild.ID))
								Ω(started.Status).Should(Equal(Builds.StatusStarted))
							})
						})
					})
				}
			})

			Context("and a third build is created", func() {
				var thirdBuild Builds.Build

				BeforeEach(func() {
					var err error

					thirdBuild, err = db.CreateBuild(job)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(thirdBuild.ID).Should(Equal(3))
					Ω(thirdBuild.Status).Should(Equal(Builds.StatusPending))
				})

				Context("and the first build finishes", func() {
					BeforeEach(func() {
						err := db.SaveBuildStatus(job, firstBuild.ID, Builds.StatusSucceeded)
						Ω(err).ShouldNot(HaveOccurred())
					})

					Context("and the third build is started serially", func() {
						It("does not update the status, as it would have jumped the queue", func() {
							started, err := db.StartBuild(job, thirdBuild.ID, true)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(started.ID).Should(Equal(thirdBuild.ID))
							Ω(started.Status).Should(Equal(Builds.StatusPending))
						})
					})
				})

				Context("and then started", func() {
					It("updates the status to started", func() {
						started, err := db.StartBuild(job, thirdBuild.ID, false)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(started.Status).Should(Equal(Builds.StatusStarted))
					})

					Context("with serial true", func() {
						It("does not update the status", func() {
							started, err := db.StartBuild(job, thirdBuild.ID, true)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(started.ID).Should(Equal(thirdBuild.ID))
							Ω(started.Status).Should(Equal(Builds.StatusPending))
						})
					})
				})
			})
		})
	})
})
