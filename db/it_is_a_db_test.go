package db_test

import (
	"fmt"
	"time"

	Builds "github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	. "github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func itIsADB() {
	BeforeEach(func() {
		err := db.RegisterJob("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		err = db.RegisterJob("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		err = db.RegisterResource("some-resource")
		Ω(err).ShouldNot(HaveOccurred())

		err = db.RegisterResource("some-resource")
		Ω(err).ShouldNot(HaveOccurred())

		err = db.RegisterJob("some-other-job")
		Ω(err).ShouldNot(HaveOccurred())

		err = db.RegisterResource("some-other-resource")
		Ω(err).ShouldNot(HaveOccurred())
	})

	It("works", func() {
		builds, err := db.GetAllJobBuilds("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(builds).Should(BeEmpty())

		_, err = db.GetCurrentBuild("some-job")
		Ω(err).Should(HaveOccurred())

		build, err := db.CreateJobBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(build.ID).ShouldNot(BeZero())
		Ω(build.JobName).Should(Equal("some-job"))
		Ω(build.Name).Should(Equal("1"))
		Ω(build.Status).Should(Equal(Builds.StatusPending))

		gotBuild, err := db.GetBuild(build.ID)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(gotBuild).Should(Equal(build))

		pending, err := db.CreateJobBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(pending.ID).ShouldNot(BeZero())
		Ω(pending.ID).ShouldNot(Equal(build.ID))
		Ω(pending.Name).Should(Equal("2"))
		Ω(pending.Status).Should(Equal(Builds.StatusPending))

		nextPending, _, err := db.GetNextPendingBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(nextPending).Should(Equal(build))

		currentBuild, err := db.GetCurrentBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(currentBuild).Should(Equal(build))

		scheduled, err := db.ScheduleBuild(build.ID, false)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(scheduled).Should(BeTrue())

		nextPending, _, err = db.GetNextPendingBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(nextPending).Should(Equal(pending))

		build, err = db.GetCurrentBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(build.Name).Should(Equal("1"))
		Ω(build.Status).Should(Equal(Builds.StatusPending))

		started, err := db.StartBuild(build.ID, "some-abort-url", "some-hijack-url")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(started).Should(BeTrue())

		build, err = db.GetCurrentBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(build.Name).Should(Equal("1"))
		Ω(build.Status).Should(Equal(Builds.StatusStarted))
		Ω(build.AbortURL).Should(Equal("some-abort-url"))
		Ω(build.HijackURL).Should(Equal("some-hijack-url"))

		allJobBuilds, err := db.GetAllJobBuilds("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(allJobBuilds).Should(HaveLen(2))
		Ω(allJobBuilds[0].Name).Should(Equal(pending.Name))
		Ω(allJobBuilds[0].JobName).Should(Equal("some-job"))
		Ω(allJobBuilds[0].Status).Should(Equal(Builds.StatusPending))
		Ω(allJobBuilds[1].Name).Should(Equal(build.Name))
		Ω(allJobBuilds[1].JobName).Should(Equal("some-job"))
		Ω(allJobBuilds[1].Status).Should(Equal(Builds.StatusStarted))

		allBuilds, err := db.GetAllBuilds()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(allBuilds).Should(Equal(allJobBuilds))

		err = db.SaveBuildStatus(build.ID, Builds.StatusSucceeded)
		Ω(err).ShouldNot(HaveOccurred())

		build, err = db.GetJobBuild("some-job", build.Name)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(build.ID).ShouldNot(BeZero())
		Ω(build.Name).Should(Equal("1"))
		Ω(build.JobName).Should(Equal("some-job"))
		Ω(build.Status).Should(Equal(Builds.StatusSucceeded))

		otherBuild, err := db.CreateJobBuild("some-other-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(otherBuild.ID).ShouldNot(BeZero())
		Ω(otherBuild.Name).Should(Equal("1"))
		Ω(otherBuild.Status).Should(Equal(Builds.StatusPending))

		build, err = db.GetJobBuild("some-other-job", otherBuild.Name)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(build.Name).Should(Equal("1"))
		Ω(build.Status).Should(Equal(Builds.StatusPending))

		started, err = db.StartBuild(build.ID, "some-other-abort-url", "some-other-hijack-url")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(started).Should(BeTrue())

		abortURL, err := db.AbortBuild(build.ID)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(abortURL).Should(Equal("some-other-abort-url"))

		log, err := db.BuildLog(build.ID)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(string(log)).Should(Equal(""))

		err = db.AppendBuildLog(build.ID, []byte("some "))
		Ω(err).ShouldNot(HaveOccurred())

		err = db.AppendBuildLog(build.ID, []byte("log"))
		Ω(err).ShouldNot(HaveOccurred())

		log, err = db.BuildLog(build.ID)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(string(log)).Should(Equal("some log"))
	})

	It("can create one-off builds with increasing names", func() {
		oneOff, err := db.CreateOneOffBuild()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(oneOff.ID).ShouldNot(BeZero())
		Ω(oneOff.JobName).Should(BeZero())
		Ω(oneOff.Name).Should(Equal("1"))
		Ω(oneOff.Status).Should(Equal(Builds.StatusPending))

		oneOffGot, err := db.GetBuild(oneOff.ID)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(oneOffGot).Should(Equal(oneOff))

		jobBuild, err := db.CreateJobBuild("some-other-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(jobBuild.Name).Should(Equal("1"))

		nextOneOff, err := db.CreateOneOffBuild()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(nextOneOff.ID).ShouldNot(BeZero())
		Ω(nextOneOff.ID).ShouldNot(Equal(oneOff.ID))
		Ω(nextOneOff.JobName).Should(BeZero())
		Ω(nextOneOff.Name).Should(Equal("2"))
		Ω(nextOneOff.Status).Should(Equal(Builds.StatusPending))

		allBuilds, err := db.GetAllBuilds()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(allBuilds).Should(Equal([]Builds.Build{nextOneOff, jobBuild, oneOff}))
	})

	It("can save a build's start/end timestamps", func() {
		build, err := db.CreateJobBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(build.Name).Should(Equal("1"))

		startTime := time.Now()
		endTime := startTime.Add(time.Second)

		err = db.SaveBuildStartTime(build.ID, startTime)
		Ω(err).ShouldNot(HaveOccurred())

		err = db.SaveBuildEndTime(build.ID, endTime)
		Ω(err).ShouldNot(HaveOccurred())

		build, err = db.GetBuild(build.ID)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(build.StartTime.Unix()).Should(Equal(startTime.Unix()))
		Ω(build.EndTime.Unix()).Should(Equal(endTime.Unix()))
	})

	It("can report a job's latest running and finished builds", func() {
		finished, next, err := db.GetJobFinishedAndNextBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		Ω(next).Should(BeNil())
		Ω(finished).Should(BeNil())

		finishedBuild, err := db.CreateJobBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		err = db.SaveBuildStatus(finishedBuild.ID, Builds.StatusSucceeded)
		Ω(err).ShouldNot(HaveOccurred())

		finished, next, err = db.GetJobFinishedAndNextBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		Ω(next).Should(BeNil())
		Ω(finished.ID).Should(Equal(finishedBuild.ID))

		nextBuild, err := db.CreateJobBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		started, err := db.StartBuild(nextBuild.ID, "abort-url", "hijack-url")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(started).Should(BeTrue())

		finished, next, err = db.GetJobFinishedAndNextBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		Ω(next.ID).Should(Equal(nextBuild.ID))
		Ω(finished.ID).Should(Equal(finishedBuild.ID))

		anotherRunningBuild, err := db.CreateJobBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		finished, next, err = db.GetJobFinishedAndNextBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		Ω(next.ID).Should(Equal(nextBuild.ID)) // not anotherRunningBuild
		Ω(finished.ID).Should(Equal(finishedBuild.ID))

		started, err = db.StartBuild(anotherRunningBuild.ID, "abort-url", "hijack-url")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(started).Should(BeTrue())

		finished, next, err = db.GetJobFinishedAndNextBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		Ω(next.ID).Should(Equal(nextBuild.ID)) // not anotherRunningBuild
		Ω(finished.ID).Should(Equal(finishedBuild.ID))

		err = db.SaveBuildStatus(nextBuild.ID, Builds.StatusSucceeded)
		Ω(err).ShouldNot(HaveOccurred())

		finished, next, err = db.GetJobFinishedAndNextBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		Ω(next.ID).Should(Equal(anotherRunningBuild.ID))
		Ω(finished.ID).Should(Equal(nextBuild.ID))
	})

	Describe("saving build inputs", func() {
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

		vr1 := Builds.VersionedResource{
			Name:     "some-resource",
			Type:     "some-type",
			Source:   Builds.Source{"some": "source"},
			Version:  Builds.Version{"ver": "1"},
			Metadata: buildMetadata,
		}

		vr2 := Builds.VersionedResource{
			Name:    "some-other-resource",
			Type:    "some-type",
			Source:  Builds.Source{"some": "other-source"},
			Version: Builds.Version{"ver": "2"},
		}

		It("saves build's inputs and outputs as versioned resources", func() {
			build, err := db.CreateJobBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildInput(build.ID, vr1)
			Ω(err).ShouldNot(HaveOccurred())

			_, err = db.GetJobBuildForInputs("some-job", Builds.VersionedResources{vr1, vr2})
			Ω(err).Should(HaveOccurred())

			err = db.SaveBuildInput(build.ID, vr2)
			Ω(err).ShouldNot(HaveOccurred())

			foundBuild, err := db.GetJobBuildForInputs("some-job", Builds.VersionedResources{vr1, vr2})
			Ω(err).ShouldNot(HaveOccurred())
			Ω(foundBuild).Should(Equal(build))

			err = db.SaveBuildOutput(build.ID, vr1)
			Ω(err).ShouldNot(HaveOccurred())

			modifiedVR2 := vr2
			modifiedVR2.Version = Builds.Version{"ver": "3"}

			err = db.SaveBuildOutput(build.ID, modifiedVR2)
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildOutput(build.ID, vr2)
			Ω(err).ShouldNot(HaveOccurred())

			inputs, outputs, err := db.GetBuildResources(build.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(inputs).Should(ConsistOf([]BuildInput{
				{VersionedResource: vr1, FirstOccurrence: true},
				{VersionedResource: vr2, FirstOccurrence: true},
			}))
			Ω(outputs).Should(ConsistOf([]BuildOutput{
				{VersionedResource: modifiedVR2},
			}))

			duplicateBuild, err := db.CreateJobBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildInput(duplicateBuild.ID, vr1)
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildInput(duplicateBuild.ID, vr2)
			Ω(err).ShouldNot(HaveOccurred())

			inputs, _, err = db.GetBuildResources(duplicateBuild.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(inputs).Should(ConsistOf([]BuildInput{
				{VersionedResource: vr1, FirstOccurrence: false},
				{VersionedResource: vr2, FirstOccurrence: false},
			}))

			newBuildInOtherJob, err := db.CreateJobBuild("some-other-job")
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildInput(newBuildInOtherJob.ID, vr1)
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildInput(newBuildInOtherJob.ID, vr2)
			Ω(err).ShouldNot(HaveOccurred())

			inputs, _, err = db.GetBuildResources(newBuildInOtherJob.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(inputs).Should(ConsistOf([]BuildInput{
				{VersionedResource: vr1, FirstOccurrence: true},
				{VersionedResource: vr2, FirstOccurrence: true},
			}))
		})

		It("updates metadata of existing inputs resources", func() {
			build, err := db.CreateJobBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildInput(build.ID, vr2)
			Ω(err).ShouldNot(HaveOccurred())

			inputs, _, err := db.GetBuildResources(build.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(inputs).Should(ConsistOf([]BuildInput{
				{VersionedResource: vr2, FirstOccurrence: true},
			}))

			withMetadata := vr2
			withMetadata.Metadata = buildMetadata

			err = db.SaveBuildInput(build.ID, withMetadata)
			Ω(err).ShouldNot(HaveOccurred())

			inputs, _, err = db.GetBuildResources(build.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(inputs).Should(ConsistOf([]BuildInput{
				{VersionedResource: withMetadata, FirstOccurrence: true},
			}))
		})

		It("can be done on build creation", func() {
			inputs := Builds.VersionedResources{vr1, vr2}

			pending, err := db.CreateJobBuildWithInputs("some-job", inputs)
			Ω(err).ShouldNot(HaveOccurred())

			foundBuild, err := db.GetJobBuildForInputs("some-job", inputs)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(foundBuild).Should(Equal(pending))

			nextPending, pendingInputs, err := db.GetNextPendingBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(nextPending).Should(Equal(pending))
			Ω(pendingInputs).Should(Equal(inputs))
		})
	})

	Describe("saving versioned resources", func() {
		It("updates the latest versioned resource", func() {
			vr1 := Builds.VersionedResource{
				Name:    "some-resource",
				Type:    "some-type",
				Source:  Builds.Source{"some": "source"},
				Version: Builds.Version{"version": "1"},
				Metadata: []Builds.MetadataField{
					{Name: "meta1", Value: "value1"},
				},
			}

			vr2 := Builds.VersionedResource{
				Name:    "some-resource",
				Type:    "some-type",
				Source:  Builds.Source{"some": "source"},
				Version: Builds.Version{"version": "2"},
				Metadata: []Builds.MetadataField{
					{Name: "meta2", Value: "value2"},
				},
			}

			vr3 := Builds.VersionedResource{
				Name:    "some-resource",
				Type:    "some-type",
				Source:  Builds.Source{"some": "source"},
				Version: Builds.Version{"version": "3"},
				Metadata: []Builds.MetadataField{
					{Name: "meta3", Value: "value3"},
				},
			}

			err := db.SaveVersionedResource(vr1)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(db.GetLatestVersionedResource("some-resource")).Should(Equal(vr1))

			err = db.SaveVersionedResource(vr2)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(db.GetLatestVersionedResource("some-resource")).Should(Equal(vr2))

			err = db.SaveVersionedResource(vr3)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(db.GetLatestVersionedResource("some-resource")).Should(Equal(vr3))
		})

		It("overwrites the existing source and metadata for the same version", func() {
			vr := Builds.VersionedResource{
				Name:    "some-resource",
				Type:    "some-type",
				Source:  Builds.Source{"some": "source"},
				Version: Builds.Version{"version": "1"},
				Metadata: []Builds.MetadataField{
					{Name: "meta1", Value: "value1"},
				},
			}

			err := db.SaveVersionedResource(vr)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(db.GetLatestVersionedResource("some-resource")).Should(Equal(vr))

			modified := vr
			modified.Source["additional"] = "data"
			modified.Metadata[0].Value = "modified-value1"

			err = db.SaveVersionedResource(modified)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(db.GetLatestVersionedResource("some-resource")).Should(Equal(modified))
		})
	})

	Describe("determining the inputs for a job", func() {
		It("ensures that versions from jobs mentioned in two input's 'passed' sections came from the same builds", func() {
			err := db.RegisterJob("job-1")
			Ω(err).ShouldNot(HaveOccurred())

			err = db.RegisterJob("job-2")
			Ω(err).ShouldNot(HaveOccurred())

			err = db.RegisterJob("shared-job")
			Ω(err).ShouldNot(HaveOccurred())

			err = db.RegisterResource("resource-1")
			Ω(err).ShouldNot(HaveOccurred())

			err = db.RegisterResource("resource-2")
			Ω(err).ShouldNot(HaveOccurred())

			j1b1, err := db.CreateJobBuild("job-1")
			Ω(err).ShouldNot(HaveOccurred())

			j2b1, err := db.CreateJobBuild("job-2")
			Ω(err).ShouldNot(HaveOccurred())

			sb1, err := db.CreateJobBuild("shared-job")
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildOutput(sb1.ID, Builds.VersionedResource{
				Name:    "resource-1",
				Type:    "some-type",
				Version: Builds.Version{"v": "r1-common-to-shared-and-j1"},
			})
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildOutput(sb1.ID, Builds.VersionedResource{
				Name:    "resource-2",
				Type:    "some-type",
				Version: Builds.Version{"v": "r2-common-to-shared-and-j2"},
			})
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildOutput(j1b1.ID, Builds.VersionedResource{
				Name:    "resource-1",
				Type:    "some-type",
				Version: Builds.Version{"v": "r1-common-to-shared-and-j1"},
			})
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildOutput(j2b1.ID, Builds.VersionedResource{
				Name:    "resource-2",
				Type:    "some-type",
				Version: Builds.Version{"v": "r2-common-to-shared-and-j2"},
			})
			Ω(err).ShouldNot(HaveOccurred())

			Ω(db.GetLatestInputVersions([]config.Input{
				{
					Resource: "resource-1",
					Passed:   []string{"shared-job", "job-1"},
				},
				{
					Resource: "resource-2",
					Passed:   []string{"shared-job", "job-2"},
				},
			})).Should(Equal(Builds.VersionedResources{
				{
					Name:    "resource-1",
					Type:    "some-type",
					Version: Builds.Version{"v": "r1-common-to-shared-and-j1"},
				},
				{
					Name:    "resource-2",
					Type:    "some-type",
					Version: Builds.Version{"v": "r2-common-to-shared-and-j2"},
				},
			}))

			sb2, err := db.CreateJobBuild("shared-job")
			Ω(err).ShouldNot(HaveOccurred())

			j1b2, err := db.CreateJobBuild("job-1")
			Ω(err).ShouldNot(HaveOccurred())

			j2b2, err := db.CreateJobBuild("job-2")
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildOutput(sb2.ID, Builds.VersionedResource{
				Name:    "resource-1",
				Type:    "some-type",
				Version: Builds.Version{"v": "new-r1-common-to-shared-and-j1"},
			})
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildOutput(sb2.ID, Builds.VersionedResource{
				Name:    "resource-2",
				Type:    "some-type",
				Version: Builds.Version{"v": "new-r2-common-to-shared-and-j2"},
			})
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildOutput(j1b2.ID, Builds.VersionedResource{
				Name:    "resource-1",
				Type:    "some-type",
				Version: Builds.Version{"v": "new-r1-common-to-shared-and-j1"},
			})
			Ω(err).ShouldNot(HaveOccurred())

			// do NOT save resource-2 as an output of job-2

			Ω(db.GetLatestInputVersions([]config.Input{
				{
					Resource: "resource-1",
					Passed:   []string{"shared-job", "job-1"},
				},
				{
					Resource: "resource-2",
					Passed:   []string{"shared-job", "job-2"},
				},
			})).Should(Equal(Builds.VersionedResources{
				{
					Name:    "resource-1",
					Type:    "some-type",
					Version: Builds.Version{"v": "r1-common-to-shared-and-j1"},
				},
				{
					Name:    "resource-2",
					Type:    "some-type",
					Version: Builds.Version{"v": "r2-common-to-shared-and-j2"},
				},
			}))

			// now save the output of resource-2 job-2
			err = db.SaveBuildOutput(j2b2.ID, Builds.VersionedResource{
				Name:    "resource-2",
				Type:    "some-type",
				Version: Builds.Version{"v": "new-r2-common-to-shared-and-j2"},
			})
			Ω(err).ShouldNot(HaveOccurred())

			Ω(db.GetLatestInputVersions([]config.Input{
				{
					Resource: "resource-1",
					Passed:   []string{"shared-job", "job-1"},
				},
				{
					Resource: "resource-2",
					Passed:   []string{"shared-job", "job-2"},
				},
			})).Should(Equal(Builds.VersionedResources{
				{
					Name:    "resource-1",
					Type:    "some-type",
					Version: Builds.Version{"v": "new-r1-common-to-shared-and-j1"},
				},
				{
					Name:    "resource-2",
					Type:    "some-type",
					Version: Builds.Version{"v": "new-r2-common-to-shared-and-j2"},
				},
			}))

			// save newer versions; should be new latest
			for i := 0; i < 10; i++ {
				version := fmt.Sprintf("version-%d", i+1)

				err = db.SaveBuildOutput(sb1.ID, Builds.VersionedResource{
					Name:    "resource-1",
					Type:    "some-type",
					Version: Builds.Version{"v": version + "-r1-common-to-shared-and-j1"},
				})
				Ω(err).ShouldNot(HaveOccurred())

				err = db.SaveBuildOutput(sb1.ID, Builds.VersionedResource{
					Name:    "resource-2",
					Type:    "some-type",
					Version: Builds.Version{"v": version + "-r2-common-to-shared-and-j2"},
				})
				Ω(err).ShouldNot(HaveOccurred())

				err = db.SaveBuildOutput(j1b1.ID, Builds.VersionedResource{
					Name:    "resource-1",
					Type:    "some-type",
					Version: Builds.Version{"v": version + "-r1-common-to-shared-and-j1"},
				})
				Ω(err).ShouldNot(HaveOccurred())

				err = db.SaveBuildOutput(j2b1.ID, Builds.VersionedResource{
					Name:    "resource-2",
					Type:    "some-type",
					Version: Builds.Version{"v": version + "-r2-common-to-shared-and-j2"},
				})
				Ω(err).ShouldNot(HaveOccurred())

				Ω(db.GetLatestInputVersions([]config.Input{
					{
						Resource: "resource-1",
						Passed:   []string{"shared-job", "job-1"},
					},
					{
						Resource: "resource-2",
						Passed:   []string{"shared-job", "job-2"},
					},
				})).Should(Equal(Builds.VersionedResources{
					{
						Name:    "resource-1",
						Type:    "some-type",
						Version: Builds.Version{"v": version + "-r1-common-to-shared-and-j1"},
					},
					{
						Name:    "resource-2",
						Type:    "some-type",
						Version: Builds.Version{"v": version + "-r2-common-to-shared-and-j2"},
					},
				}))
			}
		})
	})

	Context("when the first build is created", func() {
		var firstBuild Builds.Build

		var job string

		BeforeEach(func() {
			var err error

			job = "some-job"

			firstBuild, err = db.CreateJobBuild(job)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(firstBuild.Name).Should(Equal("1"))
			Ω(firstBuild.Status).Should(Equal(Builds.StatusPending))
		})

		Context("and then aborted", func() {
			BeforeEach(func() {
				_, err := db.AbortBuild(firstBuild.ID)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("changes the state to aborted", func() {
				build, err := db.GetJobBuild(job, firstBuild.Name)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(build.Status).Should(Equal(Builds.StatusAborted))
			})

			Describe("scheduling the build", func() {
				It("fails", func() {
					scheduled, err := db.ScheduleBuild(firstBuild.ID, false)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeFalse())
				})
			})
		})

		Context("and then scheduled", func() {
			BeforeEach(func() {
				scheduled, err := db.ScheduleBuild(firstBuild.ID, false)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(scheduled).Should(BeTrue())
			})

			Context("and then aborted", func() {
				BeforeEach(func() {
					_, err := db.AbortBuild(firstBuild.ID)
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("changes the state to aborted", func() {
					build, err := db.GetJobBuild(job, firstBuild.Name)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(build.Status).Should(Equal(Builds.StatusAborted))
				})

				Describe("starting the build", func() {
					It("fails", func() {
						started, err := db.StartBuild(firstBuild.ID, "abort-url", "hijack-url")
						Ω(err).ShouldNot(HaveOccurred())
						Ω(started).Should(BeFalse())
					})
				})
			})
		})

		Describe("scheduling the build", func() {
			It("succeeds", func() {
				scheduled, err := db.ScheduleBuild(firstBuild.ID, false)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(scheduled).Should(BeTrue())
			})

			Context("serially", func() {
				It("succeeds", func() {
					scheduled, err := db.ScheduleBuild(firstBuild.ID, true)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())
				})
			})
		})

		Context("and a second build is created", func() {
			var secondBuild Builds.Build

			Context("for a different job", func() {
				BeforeEach(func() {
					var err error

					secondBuild, err = db.CreateJobBuild("some-other-job")
					Ω(err).ShouldNot(HaveOccurred())
					Ω(secondBuild.Name).Should(Equal("1"))
					Ω(secondBuild.Status).Should(Equal(Builds.StatusPending))
				})

				Describe("scheduling the second build", func() {
					It("succeeds", func() {
						scheduled, err := db.ScheduleBuild(secondBuild.ID, false)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeTrue())
					})

					Describe("serially", func() {
						It("succeeds", func() {
							scheduled, err := db.ScheduleBuild(secondBuild.ID, true)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(scheduled).Should(BeTrue())
						})
					})
				})
			})

			Context("for the same job", func() {
				BeforeEach(func() {
					var err error

					secondBuild, err = db.CreateJobBuild(job)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(secondBuild.Name).Should(Equal("2"))
					Ω(secondBuild.Status).Should(Equal(Builds.StatusPending))
				})

				Describe("scheduling the second build", func() {
					It("succeeds", func() {
						scheduled, err := db.ScheduleBuild(secondBuild.ID, false)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeTrue())
					})

					Describe("serially", func() {
						It("fails", func() {
							scheduled, err := db.ScheduleBuild(secondBuild.ID, true)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(scheduled).Should(BeFalse())
						})
					})
				})

				Describe("after the first build schedules", func() {
					BeforeEach(func() {
						scheduled, err := db.ScheduleBuild(firstBuild.ID, false)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeTrue())
					})

					Context("when the second build is scheduled serially", func() {
						It("fails", func() {
							scheduled, err := db.ScheduleBuild(secondBuild.ID, true)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(scheduled).Should(BeFalse())
						})
					})

					for _, s := range []Builds.Status{Builds.StatusSucceeded, Builds.StatusFailed, Builds.StatusErrored} {
						status := s

						Context("and the first build's status changes to "+string(status), func() {
							BeforeEach(func() {
								err := db.SaveBuildStatus(firstBuild.ID, status)
								Ω(err).ShouldNot(HaveOccurred())
							})

							Context("and the second build is scheduled serially", func() {
								It("succeeds", func() {
									scheduled, err := db.ScheduleBuild(secondBuild.ID, true)
									Ω(err).ShouldNot(HaveOccurred())
									Ω(scheduled).Should(BeTrue())
								})
							})
						})
					}
				})

				Describe("after the first build is aborted", func() {
					BeforeEach(func() {
						_, err := db.AbortBuild(firstBuild.ID)
						Ω(err).ShouldNot(HaveOccurred())
					})

					Context("when the second build is scheduled serially", func() {
						It("succeeds", func() {
							scheduled, err := db.ScheduleBuild(secondBuild.ID, true)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(scheduled).Should(BeTrue())
						})
					})
				})

				Context("and a third build is created", func() {
					var thirdBuild Builds.Build

					BeforeEach(func() {
						var err error

						thirdBuild, err = db.CreateJobBuild(job)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(thirdBuild.Name).Should(Equal("3"))
						Ω(thirdBuild.Status).Should(Equal(Builds.StatusPending))
					})

					Context("and the first build finishes", func() {
						BeforeEach(func() {
							err := db.SaveBuildStatus(firstBuild.ID, Builds.StatusSucceeded)
							Ω(err).ShouldNot(HaveOccurred())
						})

						Context("and the third build is scheduled serially", func() {
							It("fails, as it would have jumped the queue", func() {
								scheduled, err := db.ScheduleBuild(thirdBuild.ID, true)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(scheduled).Should(BeFalse())
							})
						})
					})

					Context("and then scheduled", func() {
						It("succeeds", func() {
							scheduled, err := db.ScheduleBuild(thirdBuild.ID, false)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(scheduled).Should(BeTrue())
						})

						Describe("serially", func() {
							It("fails", func() {
								scheduled, err := db.ScheduleBuild(thirdBuild.ID, true)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(scheduled).Should(BeFalse())
							})
						})
					})
				})
			})
		})
	})
}
