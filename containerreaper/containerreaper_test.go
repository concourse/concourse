package containerreaper_test

import (
	"time"

	"github.com/concourse/atc/containerreaper"
	"github.com/concourse/atc/containerreaper/fakes"
	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	"github.com/concourse/atc/worker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("ContainerReaper", func() {
	var (
		containerReaper       containerreaper.ContainerReaper
		fakeContainerReaperDB *fakes.FakeContainerReaperDB
		fakePipelineDBFactory *dbfakes.FakePipelineDBFactory
		batchSize             int
		containers            []db.SavedContainer
	)

	BeforeEach(func() {
		fakeContainerReaperDB = new(fakes.FakeContainerReaperDB)
		containerReaperLogger := lagertest.NewTestLogger("test")
		fakePipelineDBFactory = new(dbfakes.FakePipelineDBFactory)
		batchSize = 5
		containerReaper = containerreaper.NewContainerReaper(containerReaperLogger, fakeContainerReaperDB, fakePipelineDBFactory, batchSize)
		containers = []db.SavedContainer{
			createSavedContainer(1111, "some-job", "some-handle-0"),
			createSavedContainer(1110, "some-job", "some-handle-1"),
			createSavedContainer(1114, "another-other-job", "some-handle-4"),
			createSavedContainer(1112, "some-job", "some-handle-2"),
			createSavedContainer(1113, "some-other-job", "some-handle-3"),
			createSavedContainer(1115, "another-other-job", "some-handle-5"),
		}

		jobIDMap := map[int]int{
			1110: 1,
			1111: 1,
			1112: 1,
			1113: 2,
			1114: 3,
			1115: 3,
		}
		fakeContainerReaperDB.GetContainersWithInfiniteTTLReturns(containers, nil)
		fakeContainerReaperDB.FindJobIDForBuildStub = func(buildID int) (int, bool, error) {
			return jobIDMap[buildID], true, nil
		}
	})

	It("sets TTL to finite for finished builds for the same job that are not the latest build", func() {
		containerReaper.Run()

		Expect(fakeContainerReaperDB.UpdateExpiresAtOnContainerCallCount()).To(Equal(3))

		expiredHandles := []string{"some-handle-0", "some-handle-1", "some-handle-4"}

		handle, ttl := fakeContainerReaperDB.UpdateExpiresAtOnContainerArgsForCall(0)
		Expect(expiredHandles).To(ContainElement(handle))
		Expect(ttl).To(Equal(worker.ContainerTTL))

		handle, ttl = fakeContainerReaperDB.UpdateExpiresAtOnContainerArgsForCall(1)
		Expect(expiredHandles).To(ContainElement(handle))
		Expect(ttl).To(Equal(worker.ContainerTTL))

		handle, ttl = fakeContainerReaperDB.UpdateExpiresAtOnContainerArgsForCall(2)
		Expect(expiredHandles).To(ContainElement(handle))
		Expect(ttl).To(Equal(worker.ContainerTTL))
	})
})

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
