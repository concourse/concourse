package topgun_test

import (
	"fmt"
	"math"
	"strings"
	"time"

	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Least Containers Found Placement Strategy", func() {
	var firstWorkerName string
	var secondWorkerName string
	BeforeEach(func() {
		Deploy("deployments/concourse.yml", "-o", "operations/worker-instances.yml", "-v", "worker_instances=2", "-o", "operations/add-placement-strategy.yml")
	})

	Context("when there is a deployment the worker with the least containers is assigned the task to execute", func() {
		It("ensures the worker with the least containers is assigned the task to execute", func() {
			By("stopping one worker instance")
			workers := JobInstances("worker")

			firstWorkerName = strings.Split(strings.TrimPrefix(workers[0].Name, "worker/"), "-")[0]
			secondWorkerName = strings.Split(strings.TrimPrefix(workers[1].Name, "worker/"), "-")[0]

			bosh("stop", fmt.Sprintf("worker/%s", workers[0].ID))

			By("setting a pipeline with many containers")
			fly.Run("set-pipeline", "-n", "main", "-c", "pipelines/lots-ata-time.yml", "-p", "many-containers-pipeline")

			By("unpausing the pipeline")
			fly.Run("unpause-pipeline", "-p", "many-containers-pipeline")

			By("waiting a few minutes before re-starting the worker instance")
			time.Sleep(1 * time.Minute)
			bosh("start", fmt.Sprintf("worker/%s", workers[0].ID))
			time.Sleep(2 * time.Minute)

			By("setting the second pipeline with many containers")
			fly.Run("set-pipeline", "-n", "main", "-c", "pipelines/lots-ata-time-2.yml", "-p", "many-containers-pipeline-2")

			By("unpausing the second pipeline")
			fly.Run("unpause-pipeline", "-p", "many-containers-pipeline-2")

			By("getting the container count on the workers")
			time.Sleep(1 * time.Minute)
			containersTable := flyTable("containers")
			containersOnFirstWorker := 0
			containersOnSecondWorker := 0
			for _, c := range containersTable {
				if c["type"] == "check" {
					continue
				}

				if strings.HasPrefix(c["worker"], firstWorkerName) {
					containersOnFirstWorker++
				} else if strings.HasPrefix(c["worker"], secondWorkerName) {
					containersOnSecondWorker++
				}
			}

			fmt.Println("first worker: ", containersOnFirstWorker)
			fmt.Println("second worker: ", containersOnSecondWorker)

			differenceInContainers := math.Abs(float64(containersOnFirstWorker - containersOnSecondWorker))
			totalContainers := float64(containersOnFirstWorker + containersOnSecondWorker)
			Expect(totalContainers).ToNot(BeZero())
			tolerance := differenceInContainers / totalContainers
			Expect(tolerance <= 0.2).To(BeTrue())
		})

	})
})
