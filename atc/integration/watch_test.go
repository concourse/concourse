package integration_test

import (
	"errors"
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/watch"
	concourse "github.com/concourse/concourse/go-concourse/concourse"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type jobsEventsVisitor struct {
	OnInitialEvent func([]atc.Job) error
	OnPatchEvent   func([]watch.JobSummaryEvent) error
}

func (j jobsEventsVisitor) VisitInitialEvent(jobs []atc.Job) error {
	if j.OnInitialEvent == nil {
		return errors.New("unexpected initial event")
	}
	return j.OnInitialEvent(jobs)
}

func (j jobsEventsVisitor) VisitPatchEvent(events []watch.JobSummaryEvent) error {
	if j.OnPatchEvent == nil {
		return errors.New("unexpected patch event")
	}
	return j.OnPatchEvent(events)
}

var _ = Describe("Watch Test", func() {
	var (
		client concourse.Client
	)

	BeforeEach(func() {
		cmd.FeatureFlags.EnableWatchEndpoints = true
	})

	JustBeforeEach(func() {
		client = login(atcURL, "test", "test")
	})

	It("can watch for changes to ListAllJobs", func() {
		By("initiating WatchListAllJobs")
		givenAPipeline(client, atc.PipelineRef{Name: "pipeline"})

		events := whenIWatchListAllJobs(client)
		defer events.Close()

		thenIReceiveInitialJobs(events, []atc.Job{getJob(client, atc.PipelineRef{Name: "pipeline"}, "simple")})

		By("triggering a job build")
		whenITriggerJobBuild(client, atc.PipelineRef{Name: "pipeline"}, "simple")
		job := getJob(client, atc.PipelineRef{Name: "pipeline"}, "simple")
		fmt.Printf("%#v\n", *job.NextBuild)
		thenIReceivePatchEvents(events, []watch.JobSummaryEvent{putEvent(toJobSummary(job))})

		By("deleting the pipeline")
		whenIDeletePipeline(client, atc.PipelineRef{Name: "pipeline"})
		thenIReceivePatchEvents(events, []watch.JobSummaryEvent{deleteEvent(job.ID)})
	})

	Context("when watch endpoints are not enabled", func() {
		BeforeEach(func() {
			cmd.FeatureFlags.EnableWatchEndpoints = false
		})

		It("errors when trying to watch ListAllJobs", func() {
			_, err := client.WatchListAllJobs()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("406 Not Acceptable"))
		})
	})
})

func whenIWatchListAllJobs(client concourse.Client) concourse.JobsEvents {
	events, err := client.WatchListAllJobs()
	Expect(err).ToNot(HaveOccurred())
	return events
}

func thenIReceiveInitialJobs(events concourse.JobsEvents, initialJobs []atc.Job) {
	err := events.Accept(jobsEventsVisitor{
		OnInitialEvent: func(jobs []atc.Job) error {
			defer GinkgoRecover()
			Expect(jobs).To(Equal(initialJobs))
			return nil
		},
	})
	Expect(err).ToNot(HaveOccurred())
}

func thenIReceivePatchEvents(events concourse.JobsEvents, patchEvents []watch.JobSummaryEvent) {
	err := events.Accept(jobsEventsVisitor{
		OnPatchEvent: func(events []watch.JobSummaryEvent) error {
			defer GinkgoRecover()
			Expect(events).To(Equal(patchEvents))
			return nil
		},
	})
	Expect(err).ToNot(HaveOccurred())
}

func whenITriggerJobBuild(client concourse.Client, pipelineRef atc.PipelineRef, jobName string) {
	_, err := client.Team("main").CreateJobBuild(pipelineRef, jobName)
	Expect(err).ToNot(HaveOccurred())
}

func whenIDeletePipeline(client concourse.Client, pipelineRef atc.PipelineRef) {
	_, err := client.Team("main").DeletePipeline(pipelineRef)
	Expect(err).ToNot(HaveOccurred())
}

func getJob(client concourse.Client, pipelineRef atc.PipelineRef, jobName string) atc.Job {
	job, found, err := client.Team("main").Job(pipelineRef, jobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(found).To(BeTrue())
	return job
}

func putEvent(job atc.JobSummary) watch.JobSummaryEvent {
	return watch.JobSummaryEvent{ID: job.ID, Type: watch.Put, Job: &job}
}

func deleteEvent(id int) watch.JobSummaryEvent {
	return watch.JobSummaryEvent{ID: id, Type: watch.Delete}
}

func toJobSummary(job atc.Job) atc.JobSummary {
	return atc.JobSummary{
		ID:              job.ID,
		Name:            job.Name,
		PipelineID:      job.PipelineID,
		PipelineName:    job.PipelineName,
		TeamName:        job.TeamName,
		Paused:          job.Paused,
		HasNewInputs:    job.HasNewInputs,
		FinishedBuild:   toBuildSummary(job.FinishedBuild),
		TransitionBuild: toBuildSummary(job.TransitionBuild),
		NextBuild:       toBuildSummary(job.NextBuild),
	}
}

func toBuildSummary(build *atc.Build) *atc.BuildSummary {
	if build == nil {
		return nil
	}
	return &atc.BuildSummary{
		ID:           build.ID,
		Name:         build.Name,
		JobName:      build.JobName,
		PipelineName: build.PipelineName,
		PipelineID:   build.PipelineID,
		TeamName:     build.TeamName,
		Status:       build.Status,
		StartTime:    build.StartTime,
		EndTime:      build.EndTime,
	}
}
