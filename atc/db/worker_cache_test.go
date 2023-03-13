package db_test

import (
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbtest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkerCache", func() {
	var (
		workerCache *db.WorkerCache
		scenario    *dbtest.Scenario
	)

	BeforeEach(func() {
		_, err := dbConn.Exec("DELETE FROM workers")
		Expect(err).ToNot(HaveOccurred())

		workerCache, err = db.NewWorkerCache(logger, dbConn, 5*time.Second)
		Expect(err).ToNot(HaveOccurred())

		scenario = dbtest.Setup()
	})

	getWorkers := func() []db.Worker {
		defer GinkgoRecover()

		workers, err := workerCache.Workers()
		Expect(err).ToNot(HaveOccurred())
		return workers
	}

	getContainerCounts := func() map[string]int {
		defer GinkgoRecover()

		containerCounts, err := workerCache.WorkerContainerCounts()
		Expect(err).ToNot(HaveOccurred())
		return containerCounts
	}

	getWorkerNames := func() []string {
		workers := getWorkers()
		names := make([]string, len(workers))
		for i, worker := range workers {
			names[i] = worker.Name()
		}
		return names
	}

	getWorkerActiveVolumes := func() []int {
		workers := getWorkers()
		volumes := make([]int, len(workers))
		for i, worker := range workers {
			volumes[i] = worker.ActiveVolumes()
		}
		return volumes
	}

	It("keeps its data up-to-date by listening to triggers", func() {
		By("ensuring the data starts out as empty")
		Expect(getWorkers()).To(BeEmpty())
		Expect(getContainerCounts()).To(BeEmpty())

		By("adding a new worker")
		atcWorker := dbtest.BaseWorker("some-worker")
		scenario.Run(builder.WithWorker(atcWorker))

		By("ensuring the cache eventually adds the new worker")
		Eventually(getWorkerNames).Should(ConsistOf("some-worker"))
		Eventually(getContainerCounts).Should(Equal(map[string]int{
			"some-worker": 0,
		}))

		By("updating the worker")
		atcWorker.ActiveVolumes = 50
		scenario.Run(builder.WithWorker(atcWorker))
		Eventually(getWorkerActiveVolumes).Should(ConsistOf(50))

		By("adding a build container")
		var build db.Build
		scenario.Run(
			builder.WithPipeline(atc.Config{
				Resources: atc.ResourceConfigs{
					{
						Name:   "some-resource",
						Type:   dbtest.BaseResourceType,
						Source: atc.Source{"some": "source"},
					},
				},
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
				},
			}),
			builder.WithJobBuild(&build, "some-job", nil, nil),
		)
		_, err := scenario.Workers[0].CreateContainer(db.NewBuildStepContainerOwner(build.ID(), "123", scenario.Team.ID()), db.ContainerMetadata{})
		Expect(err).ToNot(HaveOccurred())
		Eventually(getContainerCounts).Should(Equal(map[string]int{
			"some-worker": 1,
		}))

		By("adding a check container")
		scenario.Run(
			builder.WithResourceVersions("some-resource"),
			builder.WithCheckContainer("some-resource", "some-worker"),
		)
		// Only counts build containers
		Consistently(getContainerCounts).Should(Equal(map[string]int{
			"some-worker": 1,
		}))

		By("removing the container")
		_, err = dbConn.Exec("DELETE FROM containers")
		Expect(err).ToNot(HaveOccurred())
		Eventually(getContainerCounts).Should(Equal(map[string]int{
			"some-worker": 0,
		}))

		By("removing the worker")
		_, err = dbConn.Exec("DELETE FROM workers")
		Expect(err).ToNot(HaveOccurred())

		Eventually(getWorkers).Should(BeEmpty())
		Eventually(getContainerCounts).Should(BeEmpty())
	})
})
