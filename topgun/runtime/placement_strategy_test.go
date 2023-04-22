package topgun_test

import (
	"fmt"
	"math"
	"strings"
	"time"

	. "github.com/concourse/concourse/topgun/common"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = XDescribe("Fewest Build Containers Found Placement Strategy", func() {
	var firstWorkerName string
	var secondWorkerName string
	BeforeEach(func() {
		Deploy("deployments/concourse.yml", "-o", "operations/worker-instances.yml", "-v", "worker_instances=2", "-o", "operations/add-placement-strategy.yml")
	})

	Context("when there is a deployment the worker with the fewest containers is assigned the task to execute", func() {
		It("ensures the worker with the least build containers is assigned the task to execute", func() {
			By("stopping one worker instance")
			workers := JobInstances("worker")

			firstWorkerName = strings.Split(strings.TrimPrefix(workers[0].Name, "worker/"), "-")[0]
			secondWorkerName = strings.Split(strings.TrimPrefix(workers[1].Name, "worker/"), "-")[0]

			Bosh("stop", fmt.Sprintf("worker/%s", workers[0].ID))

			By("setting a pipeline with many containers")
			Fly.Run("set-pipeline", "-n", "main", "-c", "pipelines/lots-ata-time.yml", "-p", "many-containers-pipeline")

			By("unpausing the pipeline")
			Fly.Run("unpause-pipeline", "-p", "many-containers-pipeline")

			By("waiting a few minutes before re-starting the worker instance")
			time.Sleep(1 * time.Minute)
			Bosh("start", fmt.Sprintf("worker/%s", workers[0].ID))
			time.Sleep(2 * time.Minute)

			By("setting the second pipeline with many containers")
			Fly.Run("set-pipeline", "-n", "main", "-c", "pipelines/lots-ata-time-2.yml", "-p", "many-containers-pipeline-2")

			By("unpausing the second pipeline")
			Fly.Run("unpause-pipeline", "-p", "many-containers-pipeline-2")

			By("getting the container count on the workers")
			time.Sleep(1 * time.Minute)
			containersTable := FlyTable("containers")
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

			fmt.Println("Number of build containers on first worker: ", containersOnFirstWorker)
			fmt.Println("Number of build containers on second worker: ", containersOnSecondWorker)

			differenceInContainers := math.Abs(float64(containersOnFirstWorker - containersOnSecondWorker))
			totalContainers := float64(containersOnFirstWorker + containersOnSecondWorker)
			Expect(totalContainers).ToNot(BeZero())
			Expect(differenceInContainers).To(BeNumerically("~", 2)) //arbitrary tolerance of 2
		})

	})
})
