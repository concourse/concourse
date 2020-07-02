package integration_test

import (
	"errors"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/jobserver"
	"github.com/concourse/concourse/atc/db/watch"
	concourse "github.com/concourse/concourse/go-concourse/concourse"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type jobsEventsVisitor struct {
	OnInitialEvent func([]atc.Job) error
	OnPatchEvent   func([]jobserver.JobWatchEvent) error
}

func (j jobsEventsVisitor) VisitInitialEvent(jobs []atc.Job) error {
	if j.OnInitialEvent == nil {
		return errors.New("unexpected initial event")
	}
	return j.OnInitialEvent(jobs)
}

func (j jobsEventsVisitor) VisitPatchEvent(events []jobserver.JobWatchEvent) error {
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
		cmd.EnableWatchEndpoints = true
	})

	JustBeforeEach(func() {
		client = login(atcURL, "test", "test")
	})

	It("can watch for changes to ListAllJobs", func() {
		By("initiating WatchListAllJobs")
		givenAPipeline(client, "pipeline")

		events := whenIWatchListAllJobs(client)
		defer events.Close()

		thenIReceiveInitialJobs(events, []atc.Job{getJob(client, "pipeline", "simple")})

		By("triggering a job build")
		whenITriggerJobBuild(client, "pipeline", "simple")
		job := getJob(client, "pipeline", "simple")
		thenIReceivePatchEvents(events, []jobserver.JobWatchEvent{putEvent(job)})

		By("deleting the pipeline")
		whenIDeletePipeline(client, "pipeline")
		thenIReceivePatchEvents(events, []jobserver.JobWatchEvent{deleteEvent(job.ID)})
	})

	Context("when watch endpoints are not enabled", func() {
		BeforeEach(func() {
			cmd.EnableWatchEndpoints = false
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

func thenIReceivePatchEvents(events concourse.JobsEvents, patchEvents []jobserver.JobWatchEvent) {
	err := events.Accept(jobsEventsVisitor{
		OnPatchEvent: func(events []jobserver.JobWatchEvent) error {
			defer GinkgoRecover()
			Expect(events).To(Equal(patchEvents))
			return nil
		},
	})
	Expect(err).ToNot(HaveOccurred())
}

func whenITriggerJobBuild(client concourse.Client, pipelineName string, jobName string) {
	_, err := client.Team("main").CreateJobBuild(pipelineName, jobName)
	Expect(err).ToNot(HaveOccurred())
}

func whenIDeletePipeline(client concourse.Client, pipelineName string) {
	_, err := client.Team("main").DeletePipeline(pipelineName)
	Expect(err).ToNot(HaveOccurred())
}

func getJob(client concourse.Client, pipelineName string, jobName string) atc.Job {
	job, found, err := client.Team("main").Job(pipelineName, jobName)
	Expect(err).ToNot(HaveOccurred())
	Expect(found).To(BeTrue())
	return job
}

func putEvent(job atc.Job) jobserver.JobWatchEvent {
	return jobserver.JobWatchEvent{ID: job.ID, Type: watch.Put, Job: &job}
}

func deleteEvent(id int) jobserver.JobWatchEvent {
	return jobserver.JobWatchEvent{ID: id, Type: watch.Delete}
}
