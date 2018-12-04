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

var _ = FDescribe("Least Containers Found Placement Strategy", func() {
	var firstWorkerName string
	var secondWorkerName string
	BeforeEach(func() {
		Deploy("deployments/concourse.yml", "-v", "stemcell_version=97", "-o", "operations/worker-instances.yml", "-v", "worker_instances=2", "-o", "operations/add-placement-strategy.yml")
		// fmt.Println("about to wait for running worker")
		// firstWorkerName = waitForRunningWorker()
		// fmt.Println("=== running worker first name: ", firstWorkerName)
	})

	Context("when there is a deployment the worker with the least containers is assigned the task to execute", func() {
		It("ensures the worker with the least containers is assigned the task to execute", func() {
			By("ignoring one worker instance")
			workers := JobInstances("worker")

			firstWorkerName = strings.Split(strings.TrimPrefix(workers[0].Name, "worker/"), "-")[0]
			secondWorkerName = strings.Split(strings.TrimPrefix(workers[1].Name, "worker/"), "-")[0]

			fmt.Println("== first worker name: ", firstWorkerName)
			fmt.Println("== second worker name: ", secondWorkerName)

			bosh("stop", fmt.Sprintf("worker/%s", workers[0].ID))

			By("setting a pipeline with many containers")
			fly.Run("set-pipeline", "-n", "main", "-c", "pipelines/lots-ata-time.yml", "-p", "many-containers-pipeline")

			By("unpausing the pipeline")
			fly.Run("unpause-pipeline", "-p", "many-containers-pipeline")

			By("waiting a few minutes before un-ignoring the worker instance")
			time.Sleep(5 * time.Minute)
			bosh("start", fmt.Sprintf("worker/%s", workers[0].ID))
			time.Sleep(3 * time.Minute)

			By("setting the second pipeline with many containers")
			fly.Run("set-pipeline", "-n", "main", "-c", "pipelines/lots-ata-time-2.yml", "-p", "many-containers-pipeline-2")

			By("unpausing the second pipeline")
			fly.Run("unpause-pipeline", "-p", "many-containers-pipeline-2")

			By("getting the container count on the workers")
			time.Sleep(3 * time.Minute)
			containersTable := flyTable("containers")
			containersOnFirstWorker := 0
			containersOnSecondWorker := 0
			for _, c := range containersTable {
				fmt.Println("------------container worker: ", c["worker"])
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

		// BeforeEach(func() {
		// 	By("setting a pipeline with many containers")
		// 	fly.Run("set-pipeline", "-n", "main", "-c", "pipelines/lots-ata-time.yml", "-p", "many-containers-pipeline")

		// 	By("unpausing the pipeline")
		// 	fly.Run("unpause-pipeline", "-p", "many-containers-pipeline")
		// })

		// It("ensures the worker with the least containers is assigned the task to execute", func() {
		// 	By("deploying a second worker")
		// 	Deploy("deployments/concourse.yml", "-o", "operations/add-other-worker.yml", "-v", "worker_instances=2")
		// 	waitForWorkersToBeRunning(2)

		// 	workers := runningWorkers()
		// 	for _, w := range workers {
		// 		if w["name"] != firstWorkerName {
		// 			secondWorkerName = w["name"]
		// 		}
		// 	}

		// 	By("setting a pipeline with many containers")
		// 	fly.Run("set-pipeline", "-n", "main", "-c", "pipelines/lots-ata-time.yml", "-p", "many-containers-pipeline-2")

		// 	By("unpausing the pipeline")
		// 	fly.Run("unpause-pipeline", "-p", "many-containers-pipeline-2")

		// 	By("getting the container count on the workers")
		// 	time.Sleep(5 * time.Minute)
		// 	containersTable := flyTable("containers")
		// 	containersOnFirstWorker := 0
		// 	containersOnSecondWorker := 0
		// 	for _, c := range containersTable {
		// 		if c["worker"] == firstWorkerName {
		// 			containersOnFirstWorker++
		// 		} else if c["worker"] == secondWorkerName {
		// 			containersOnSecondWorker++
		// 		}
		// 	}

		// 	fmt.Println("first worker: ", containersOnFirstWorker)
		// 	fmt.Println("second worker: ", containersOnSecondWorker)

		// 	differenceInContainers := math.Abs(float64(containersOnFirstWorker - containersOnSecondWorker))
		// 	totalContainers := float64(containersOnFirstWorker + containersOnSecondWorker)
		// 	Expect(totalContainers).ToNot(BeZero())
		// 	tolerance := differenceInContainers / totalContainers
		// 	Expect(tolerance <= 0.2).To(BeTrue())
		// })
	})
})
