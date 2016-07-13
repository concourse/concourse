package containerkeepaliver_test

import (
	"errors"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/containerkeepaliver"
	"github.com/concourse/atc/containerkeepaliver/containerkeepaliverfakes"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/worker/workerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("ContainerKeepAliver", func() {
	var (
		containerKeepAliver       containerkeepaliver.ContainerKeepAliver
		fakeContainerKeepAliverDB *containerkeepaliverfakes.FakeContainerKeepAliverDB
		fakePipelineDBFactory     *dbfakes.FakePipelineDBFactory
		fakeWorkerClient          *workerfakes.FakeClient
		fakeWorkerContainer       *workerfakes.FakeContainer
		fakePipelineDB            *dbfakes.FakePipelineDB

		failedContainers []db.SavedContainer
		jobIDMap         map[int]int
	)

	BeforeEach(func() {
		fakeContainerKeepAliverDB = new(containerkeepaliverfakes.FakeContainerKeepAliverDB)
		containerKeepAliverLogger := lagertest.NewTestLogger("test")
		fakePipelineDBFactory = new(dbfakes.FakePipelineDBFactory)
		fakeWorkerClient = new(workerfakes.FakeClient)
		fakeWorkerContainer = new(workerfakes.FakeContainer)
		fakePipelineDB = new(dbfakes.FakePipelineDB)

		containerKeepAliver = containerkeepaliver.NewContainerKeepAliver(
			containerKeepAliverLogger,
			fakeWorkerClient,
			fakeContainerKeepAliverDB,
			fakePipelineDBFactory,
		)

		failedContainers = []db.SavedContainer{
			createSavedContainer(1111, "some-job", "some-handle-0", 11),
			createSavedContainer(1110, "some-job", "some-handle-1", 11),
			createSavedContainer(1114, "another-other-job", "some-handle-4", 22),
			createSavedContainer(1112, "some-job", "some-handle-2", 11),
			createSavedContainer(1113, "some-other-job", "some-handle-3", 33),
			createSavedContainer(1115, "another-other-job", "some-handle-5", 22),
		}

		fakeContainerKeepAliverDB.FindLatestSuccessfulBuildsPerJobReturns(map[int]int{1: 2002, 3: 999}, nil)

		jobIDMap = map[int]int{
			1110: 1, // failed, but latest is success 2002
			1111: 1, // failed, but latest is success 2002
			1112: 1, // failed, but latest is success 2002
			1113: 2, // latest failed, no success builds
			1114: 3, // non-latest failed
			1115: 3, // latest failed, success build before 999
		}

		fakeContainerKeepAliverDB.FindJobContainersFromUnsuccessfulBuildsReturns(failedContainers, nil)

		fakeContainerKeepAliverDB.FindJobIDForBuildStub = func(buildID int) (int, bool, error) {
			return jobIDMap[buildID], true, nil
		}

		fakeWorkerClient.LookupContainerReturns(fakeWorkerContainer, true, nil)
		fakePipelineDBFactory.BuildReturns(fakePipelineDB)
		fakePipelineDB.GetConfigReturns(atc.Config{
			Jobs: atc.JobConfigs{
				atc.JobConfig{
					Name: "some-job",
				},
				atc.JobConfig{
					Name: "some-other-job",
				},
				atc.JobConfig{
					Name: "another-other-job",
				},
			},
		}, 0, true, nil)
	})

	JustBeforeEach(func() {
		containerKeepAliver.Run()
	})

	It("keeps alive containers for latest failed builds", func() {
		Expect(fakeWorkerClient.LookupContainerCallCount()).To(Equal(2))

		lookedUpHandles := []string{}
		for i := 0; i < 2; i++ {
			_, handle := fakeWorkerClient.LookupContainerArgsForCall(i)
			lookedUpHandles = append(lookedUpHandles, handle)
		}
		Expect(lookedUpHandles).To(ConsistOf("some-handle-3", "some-handle-5"))
		Expect(fakeWorkerContainer.ReleaseCallCount()).To(Equal(2))
	})

	Context("when pipeline is not found", func() {
		BeforeEach(func() {
			fakeContainerKeepAliverDB.FindJobContainersFromUnsuccessfulBuildsReturns(
				[]db.SavedContainer{failedContainers[0], failedContainers[1]},
				nil,
			)
			fakeContainerKeepAliverDB.GetPipelineByIDReturns(db.SavedPipeline{}, errors.New("some-error"))
		})

		It("does not heartbeat its containers", func() {
			Expect(fakeWorkerClient.LookupContainerCallCount()).To(Equal(0))
		})
	})

	Context("when container is not found on worker", func() {
		BeforeEach(func() {
			fakeContainerKeepAliverDB.FindJobContainersFromUnsuccessfulBuildsReturns(
				[]db.SavedContainer{failedContainers[0], failedContainers[1]},
				nil,
			)
			fakeWorkerClient.LookupContainerReturns(nil, false, nil)
		})

		It("does not heartbeat its containers", func() {
			Expect(fakeWorkerClient.LookupContainerCallCount()).To(Equal(0))
		})
	})

	Context("when looking up container on worker fails", func() {
		BeforeEach(func() {
			fakeContainerKeepAliverDB.FindJobContainersFromUnsuccessfulBuildsReturns(
				[]db.SavedContainer{failedContainers[0]},
				nil,
			)
			fakeWorkerClient.LookupContainerReturns(nil, false, errors.New("some-error"))
		})

		It("does not heartbeat its containers", func() {
			Expect(fakeWorkerClient.LookupContainerCallCount()).To(Equal(0))
		})
	})

	Context("when a pipeline config doesn't exist", func() {
		BeforeEach(func() {
			fakeContainerKeepAliverDB.FindJobContainersFromUnsuccessfulBuildsReturns(
				[]db.SavedContainer{failedContainers[0], failedContainers[1]},
				nil,
			)
			fakePipelineDB.GetConfigReturns(atc.Config{}, 0, false, nil)
		})

		It("does not heartbeat its containers", func() {
			Expect(fakeWorkerClient.LookupContainerCallCount()).To(Equal(0))
		})
	})

	Context("when a job is no longer in the pipeline config", func() {
		BeforeEach(func() {
			fakeContainerKeepAliverDB.FindJobContainersFromUnsuccessfulBuildsReturns(
				[]db.SavedContainer{failedContainers[0], failedContainers[1]},
				nil,
			)
			fakePipelineDB.GetConfigReturns(atc.Config{
				Jobs: atc.JobConfigs{
					atc.JobConfig{Name: "another-other-job"},
				},
			}, 0, true, nil)
		})

		It("does not heartbeat its containers", func() {
			Expect(fakeWorkerClient.LookupContainerCallCount()).To(Equal(0))
		})
	})
})

func createSavedContainer(buildID int, jobName string, handle string, pipelineID int) db.SavedContainer {
	return db.SavedContainer{
		Container: db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				BuildID: buildID,
			},
			ContainerMetadata: db.ContainerMetadata{
				JobName:    jobName,
				Handle:     handle,
				PipelineID: pipelineID,
			},
		},
		TTL: time.Duration(0),
	}
}
