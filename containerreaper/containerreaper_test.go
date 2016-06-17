package containerreaper_test

import (
	"time"

	"github.com/concourse/atc/containerreaper"
	"github.com/concourse/atc/containerreaper/fakes"
	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	"github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("ContainerReaper", func() {
	var (
		containerReaper       containerreaper.ContainerReaper
		fakeContainerReaperDB *fakes.FakeContainerReaperDB
		fakePipelineDBFactory *dbfakes.FakePipelineDBFactory
		fakeWorkerClient      *wfakes.FakeClient
		fakeWorkerContainer   *wfakes.FakeContainer
		failedContainers      []db.SavedContainer
		successfulContainers  []db.SavedContainer
	)

	BeforeEach(func() {
		fakeContainerReaperDB = new(fakes.FakeContainerReaperDB)
		containerReaperLogger := lagertest.NewTestLogger("test")
		fakePipelineDBFactory = new(dbfakes.FakePipelineDBFactory)
		fakeWorkerClient = new(wfakes.FakeClient)
		fakeWorkerContainer = new(wfakes.FakeContainer)
		containerReaper = containerreaper.NewContainerReaper(containerReaperLogger, fakeWorkerClient, fakeContainerReaperDB, fakePipelineDBFactory)

		failedContainers = []db.SavedContainer{
			createSavedContainer(1111, "some-job", "some-handle-0"),
			createSavedContainer(1110, "some-job", "some-handle-1"),
			createSavedContainer(1114, "another-other-job", "some-handle-4"),
			createSavedContainer(1112, "some-job", "some-handle-2"),
			createSavedContainer(1113, "some-other-job", "some-handle-3"),
			createSavedContainer(1115, "another-other-job", "some-handle-5"),
		}

		successfulContainers = []db.SavedContainer{
			createSavedContainer(2001, "some-job", "some-handle-6"),
			createSavedContainer(2002, "some-job", "some-handle-7"),
			createSavedContainer(999, "another-other-job", "some-handle-8"),
		}

		jobIDMap := map[int]int{
			1110: 1,
			1111: 1,
			2001: 1,
			2002: 1,
			1112: 1,
			1113: 2,
			1114: 3,
			999:  3,
			1115: 3,
		}

		fakeContainerReaperDB.FindContainersFromUnsuccessfulBuildsWithInfiniteTTLReturns(failedContainers, nil)
		fakeContainerReaperDB.FindContainersFromSuccessfulBuildsWithInfiniteTTLReturns(successfulContainers, nil)

		fakeContainerReaperDB.FindJobIDForBuildStub = func(buildID int) (int, bool, error) {
			return jobIDMap[buildID], true, nil
		}

		fakeWorkerClient.LookupContainerReturns(fakeWorkerContainer, true, nil)
	})

	JustBeforeEach(func() {
		containerReaper.Run()
	})

	It("sets TTL to finite for finished builds for the same job that are not the latest build", func() {
		Expect(fakeContainerReaperDB.UpdateExpiresAtOnContainerCallCount()).To(Equal(6))
		Expect(fakeWorkerClient.LookupContainerCallCount()).To(Equal(6))

		expiredHandles := []string{"some-handle-0", "some-handle-1", "some-handle-4", "some-handle-6", "some-handle-7", "some-handle-8"}

		for i := 0; i < 6; i++ {
			verifyLookupContainerCalls(fakeWorkerClient, expiredHandles, i)
			verifyTTLWasSet(fakeContainerReaperDB, expiredHandles, i)
		}

		Expect(fakeWorkerContainer.ReleaseCallCount()).To(Equal(6))
	})
})

func verifyLookupContainerCalls(fakeWorkerClient *wfakes.FakeClient, expiredHandles []string, callIndex int) {
	_, handle := fakeWorkerClient.LookupContainerArgsForCall(callIndex)
	Expect(expiredHandles).To(ContainElement(handle))
}

func verifyTTLWasSet(fakeContainerReaperDB *fakes.FakeContainerReaperDB, expiredHandles []string, callIndex int) {
	handle, ttl := fakeContainerReaperDB.UpdateExpiresAtOnContainerArgsForCall(callIndex)
	Expect(expiredHandles).To(ContainElement(handle))
	Expect(ttl).To(Equal(worker.ContainerTTL))
}

func createSavedContainer(buildID int, jobName string, handle string) db.SavedContainer {
	return db.SavedContainer{
		Container: db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				BuildID: buildID,
			},
			ContainerMetadata: db.ContainerMetadata{
				JobName: jobName,
				Handle:  handle,
			},
		},
		TTL: time.Duration(0),
	}
}
