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

		scheduled, err := db.ScheduleBuild("some-job", build.ID, false)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(scheduled).Should(BeTrue())

		build, err = db.GetCurrentBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(build.ID).Should(Equal(1))
		Ω(build.Status).Should(Equal(Builds.StatusPending))

		started, err := db.StartBuild("some-job", build.ID, "some-abort-url")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(started).Should(BeTrue())

		build, err = db.GetCurrentBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(build.ID).Should(Equal(1))
		Ω(build.Status).Should(Equal(Builds.StatusStarted))
		Ω(build.AbortURL).Should(Equal("some-abort-url"))

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

		err = db.AppendBuildLog("some-job", 1, []byte("some "))
		Ω(err).ShouldNot(HaveOccurred())

		err = db.AppendBuildLog("some-job", 1, []byte("log"))
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

		Context("and then aborted", func() {
			BeforeEach(func() {
				err := db.AbortBuild(job, firstBuild.ID)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("changes the state to aborted", func() {
				build, err := db.GetBuild(job, firstBuild.ID)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(build.Status).Should(Equal(Builds.StatusAborted))
			})

			Describe("scheduling the build", func() {
				It("fails", func() {
					scheduled, err := db.ScheduleBuild(job, firstBuild.ID, false)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeFalse())
				})
			})
		})

		Context("and then scheduled", func() {
			BeforeEach(func() {
				scheduled, err := db.ScheduleBuild(job, firstBuild.ID, false)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(scheduled).Should(BeTrue())
			})

			Context("and then aborted", func() {
				BeforeEach(func() {
					err := db.AbortBuild(job, firstBuild.ID)
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("changes the state to aborted", func() {
					build, err := db.GetBuild(job, firstBuild.ID)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(build.Status).Should(Equal(Builds.StatusAborted))
				})

				Describe("starting the build", func() {
					It("fails", func() {
						started, err := db.StartBuild(job, firstBuild.ID, "abort-url")
						Ω(err).ShouldNot(HaveOccurred())
						Ω(started).Should(BeFalse())
					})
				})
			})
		})

		Describe("scheduling the build", func() {
			It("succeeds", func() {
				scheduled, err := db.ScheduleBuild(job, firstBuild.ID, false)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(scheduled).Should(BeTrue())
			})

			Context("serially", func() {
				It("succeeds", func() {
					scheduled, err := db.ScheduleBuild(job, firstBuild.ID, true)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())
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

			Describe("scheduling the second build", func() {
				It("succeeds", func() {
					scheduled, err := db.ScheduleBuild(job, secondBuild.ID, false)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())
				})

				Context("with serial true", func() {
					It("fails", func() {
						scheduled, err := db.ScheduleBuild(job, secondBuild.ID, true)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeFalse())
					})
				})
			})

			Describe("after the first build schedules", func() {
				BeforeEach(func() {
					scheduled, err := db.ScheduleBuild(job, firstBuild.ID, false)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())
				})

				Context("when the second build is scheduled serially", func() {
					It("fails", func() {
						scheduled, err := db.ScheduleBuild(job, secondBuild.ID, true)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeFalse())
					})
				})

				for _, s := range []Builds.Status{Builds.StatusSucceeded, Builds.StatusFailed, Builds.StatusErrored} {
					status := s

					Context("and the first build's status changes to "+string(status), func() {
						BeforeEach(func() {
							err := db.SaveBuildStatus(job, firstBuild.ID, status)
							Ω(err).ShouldNot(HaveOccurred())
						})

						Context("and the second build is scheduled serially", func() {
							It("succeeds", func() {
								scheduled, err := db.ScheduleBuild(job, secondBuild.ID, true)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(scheduled).Should(BeTrue())
							})
						})
					})
				}
			})

			Describe("after the first build is aborted", func() {
				BeforeEach(func() {
					err := db.AbortBuild(job, firstBuild.ID)
					Ω(err).ShouldNot(HaveOccurred())
				})

				Context("when the second build is scheduled serially", func() {
					It("succeeds", func() {
						scheduled, err := db.ScheduleBuild(job, secondBuild.ID, true)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeTrue())
					})
				})
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

					Context("and the third build is scheduled serially", func() {
						It("fails, as it would have jumped the queue", func() {
							scheduled, err := db.ScheduleBuild(job, thirdBuild.ID, true)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(scheduled).Should(BeFalse())
						})
					})
				})

				Context("and then scheduled", func() {
					It("succeeds", func() {
						scheduled, err := db.ScheduleBuild(job, thirdBuild.ID, false)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeTrue())
					})

					Context("with serial true", func() {
						It("fails", func() {
							scheduled, err := db.ScheduleBuild(job, thirdBuild.ID, true)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(scheduled).Should(BeFalse())
						})
					})
				})
			})
		})
	})

	Describe("attempting to initiate a build", func() {
		Context("when a build is already attempted", func() {
			BeforeEach(func() {
				build, err := db.AttemptBuild("some-job", "some-resource", Builds.Version{"version": "1"}, false)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(build.ID).Should(Equal(1))
			})

			Context("but with a different version", func() {
				It("succeeds", func() {
					build, err := db.AttemptBuild("some-job", "some-resource", Builds.Version{"version": "2"}, false)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(build.ID).Should(Equal(2))
				})
			})

			Context("with the same version", func() {
				It("fails", func() {
					_, err := db.AttemptBuild("some-job", "some-resource", Builds.Version{"version": "1"}, false)
					Ω(err).Should(Equal(ErrInputRedundant))
				})
			})
		})

		Context("when a build is already started", func() {
			var startedBuild Builds.Build

			BeforeEach(func() {
				var err error

				startedBuild, err = db.CreateBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())

				scheduled, err := db.ScheduleBuild("some-job", startedBuild.ID, true)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(scheduled).Should(BeTrue())

				started, err := db.StartBuild("some-job", startedBuild.ID, "some-abort-url")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(started).Should(BeTrue())
			})

			Context("and its inputs have not been determined", func() {
				It("fails, regardless of serial", func() {
					_, err := db.AttemptBuild("some-job", "some-resource", Builds.Version{}, false)
					Ω(err).Should(Equal(ErrInputNotDetermined))

					_, err = db.AttemptBuild("some-job", "some-resource", Builds.Version{}, true)
					Ω(err).Should(Equal(ErrInputNotDetermined))
				})
			})

			Context("and its inputs have been determined", func() {
				BeforeEach(func() {
					err := db.SaveBuildInput("some-job", startedBuild.ID, Builds.Input{
						Name:    "some-resource",
						Version: Builds.Version{"version": "1"},
					})
					Ω(err).ShouldNot(HaveOccurred())
				})

				Context("and its input resource is a different version", func() {
					It("succeeds", func() {
						attemptedBuild, err := db.AttemptBuild(
							"some-job",
							"some-resource",
							Builds.Version{"version": "2"},
							false,
						)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(attemptedBuild.ID).Should(Equal(2))
					})

					Context("after the attempt succeeds", func() {
						BeforeEach(func() {
							attemptedBuild, err := db.AttemptBuild(
								"some-job",
								"some-resource",
								Builds.Version{"version": "2"},
								false,
							)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(attemptedBuild.ID).Should(Equal(2))
						})

						Describe("attempting another build", func() {
							Context("with a different version", func() {
								It("succeeds", func() {
									build, err := db.AttemptBuild("some-job", "some-resource", Builds.Version{"version": "3"}, false)
									Ω(err).ShouldNot(HaveOccurred())
									Ω(build.ID).Should(Equal(3))
								})
							})

							Context("with the same version", func() {
								It("fails", func() {
									_, err := db.AttemptBuild("some-job", "some-resource", Builds.Version{"version": "2"}, false)
									Ω(err).Should(Equal(ErrInputRedundant))
								})
							})
						})
					})

					Context("with serial true", func() {
						It("fails, in case its eventual output is the same version", func() {
							_, err := db.AttemptBuild(
								"some-job",
								"some-resource",
								Builds.Version{"version": "2"},
								true,
							)
							Ω(err).Should(Equal(ErrOutputNotDetermined))
						})
					})
				})

				Context("and its input resource is the same version", func() {
					It("fails", func() {
						_, err := db.AttemptBuild(
							"some-job",
							"some-resource",
							Builds.Version{"version": "1"},
							true,
						)
						Ω(err).Should(Equal(ErrInputRedundant))
					})
				})

				Context("and its outputs have been determined", func() {
					BeforeEach(func() {
						err := db.SaveOutputVersion("some-job", startedBuild.ID, "some-resource", Builds.Version{"version": "2"})
						Ω(err).ShouldNot(HaveOccurred())
					})

					Context("and its output resource is a different version", func() {
						It("succeeds", func() {
							attemptedBuild, err := db.AttemptBuild(
								"some-job",
								"some-resource",
								Builds.Version{"version": "3"},
								false,
							)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(attemptedBuild.ID).Should(Equal(2))
						})
					})

					Context("and its output resource is the same version", func() {
						It("fails", func() {
							_, err := db.AttemptBuild(
								"some-job",
								"some-resource",
								Builds.Version{"version": "2"},
								true,
							)
							Ω(err).Should(Equal(ErrOutputRedundant))
						})
					})
				})
			})
		})
	})
})
