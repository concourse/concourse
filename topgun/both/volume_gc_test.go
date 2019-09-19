package topgun_test

import (
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	. "github.com/concourse/concourse/topgun/common"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Garbage-collecting volumes", func() {
	BeforeEach(func() {
		Deploy("deployments/concourse.yml")
	})

	Describe("A volume that belonged to a container that is now gone", func() {
		It("is removed from the database and worker [#129726011]", func() {
			By("running a task with container having a rootfs, input, and output volume")
			Fly.Run("execute", "-c", "tasks/input-output.yml", "-i", "some-input=./tasks")

			By("collecting the build containers")
			rows, err := Psql.Select("id, handle").From("containers").Where(sq.Eq{"build_id": 1}).RunWith(DbConn).Query()
			Expect(err).ToNot(HaveOccurred())

			buildContainers := map[int]string{}
			containerIDs := []int{}
			for rows.Next() {
				var id int
				var handle string
				err := rows.Scan(&id, &handle)
				Expect(err).ToNot(HaveOccurred())

				buildContainers[id] = handle
				containerIDs = append(containerIDs, id)
			}

			By("collecting the container volumes")
			rows, err = Psql.Select("id, handle").From("volumes").Where(sq.Eq{"container_id": containerIDs}).RunWith(DbConn).Query()
			Expect(err).ToNot(HaveOccurred())

			containerVolumes := map[int]string{}
			volumeIDs := []int{}
			for rows.Next() {
				var id int
				var handle string
				err := rows.Scan(&id, &handle)
				Expect(err).ToNot(HaveOccurred())

				containerVolumes[id] = handle
				volumeIDs = append(volumeIDs, id)
			}

			By(fmt.Sprintf("eventually expiring the build containers: %v", containerIDs))
			Eventually(func() int {
				var volNum int
				err := Psql.Select("COUNT(id)").From("volumes").Where(sq.Eq{"id": volumeIDs}).RunWith(DbConn).QueryRow().Scan(&volNum)
				Expect(err).ToNot(HaveOccurred())

				var containerNum int
				err = Psql.Select("COUNT(id)").From("containers").Where(sq.Eq{"id": containerIDs}).RunWith(DbConn).QueryRow().Scan(&containerNum)
				Expect(err).ToNot(HaveOccurred())

				if containerNum == len(containerIDs) {
					By(fmt.Sprintf("not expiring volumes so long as their containers are there (%d remaining)", containerNum))
					Expect(volNum).To(Equal(len(volumeIDs)))
				}

				return containerNum
			}, 10*time.Minute, time.Second).Should(BeZero())

			By("having removed the containers from the worker")
			containers, err := WorkerGardenClient.Containers(nil)
			Expect(err).ToNot(HaveOccurred())

			existingHandles := []string{}
			for _, c := range containers {
				existingHandles = append(existingHandles, c.Handle())
			}

			for _, handle := range buildContainers {
				Expect(existingHandles).ToNot(ContainElement(handle))
			}

			By(fmt.Sprintf("eventually expiring the container volumes: %v", volumeIDs))
			Eventually(func() int {
				var num int
				err := Psql.Select("COUNT(id)").From("volumes").Where(sq.Eq{"id": volumeIDs}).RunWith(DbConn).QueryRow().Scan(&num)
				Expect(err).ToNot(HaveOccurred())
				return num
			}, 10*time.Minute, time.Second).Should(BeZero())

			By("having removed the volumes from the worker")
			volumes, err := WorkerBaggageclaimClient.ListVolumes(Logger, nil)
			Expect(err).ToNot(HaveOccurred())

			existingHandles = []string{}
			for _, v := range volumes {
				existingHandles = append(existingHandles, v.Handle())
			}

			for _, handle := range containerVolumes {
				Expect(existingHandles).ToNot(ContainElement(handle))
			}
		})
	})

	Describe("A volume that belonged to a resource cache that is no longer in use", func() {
		It("is removed from the database and worker [#129726933]", func() {
			By("setting pipeline that creates resource cache")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task-changing-resource.yml", "-p", "volume-gc-test")

			By("unpausing the pipeline")
			Fly.Run("unpause-pipeline", "-p", "volume-gc-test")

			By("triggering job")
			Fly.Run("trigger-job", "-w", "-j", "volume-gc-test/simple-job")

			By("getting resource cache")
			var resourceCacheID int
			err := Psql.Select("rch.id").
				From("resource_caches rch").
				LeftJoin("resource_configs rcf ON rch.resource_config_id = rcf.id").
				LeftJoin("base_resource_types brt ON rcf.base_resource_type_id = brt.id").
				Where("brt.name = 'time'").
				RunWith(DbConn).
				QueryRow().Scan(&resourceCacheID)
			Expect(err).ToNot(HaveOccurred())

			By("getting volume for resource cache")
			var volumeID int
			var volumeHandle string
			err = Psql.Select("v.id, v.handle").
				From("volumes v").
				LeftJoin("worker_resource_caches wrc ON wrc.id = v.worker_resource_cache_id").
				Where(sq.Eq{"wrc.resource_cache_id": resourceCacheID}).
				RunWith(DbConn).
				QueryRow().
				Scan(&volumeID, &volumeHandle)
			Expect(err).ToNot(HaveOccurred())

			By("creating a new version of resource")
			Fly.Run("check-resource", "-r", "volume-gc-test/tick-tock")

			By(fmt.Sprintf("eventually expiring the resource cache: %d", resourceCacheID))
			Eventually(func() int {
				var resourceCacheNum int
				err := Psql.Select("COUNT(id)").From("resource_caches").Where(sq.Eq{"id": resourceCacheID}).RunWith(DbConn).QueryRow().Scan(&resourceCacheNum)
				Expect(err).ToNot(HaveOccurred())

				var volumeNum int
				err = Psql.Select("COUNT(id)").From("volumes").Where(sq.Eq{"id": volumeID}).RunWith(DbConn).QueryRow().Scan(&volumeNum)
				Expect(err).ToNot(HaveOccurred())

				if resourceCacheNum == 1 {
					By(fmt.Sprintf("not expiring volume so long as its resource cache is there"))
					Expect(volumeNum).To(Equal(1))
				}

				return resourceCacheNum
			}, 10*time.Minute, time.Second).Should(BeZero())

			By(fmt.Sprintf("eventually expiring the resource cache volumes: %d", resourceCacheID))
			Eventually(func() int {
				var volumeNum int
				err := Psql.Select("COUNT(id)").From("volumes").Where(sq.Eq{"id": volumeID}).RunWith(DbConn).QueryRow().Scan(&volumeNum)
				Expect(err).ToNot(HaveOccurred())
				return volumeNum
			}, 10*time.Minute, time.Second).Should(BeZero())

			By("having removed the volumes from the worker")
			volumes, err := WorkerBaggageclaimClient.ListVolumes(Logger, nil)
			Expect(err).ToNot(HaveOccurred())

			existingHandles := []string{}
			for _, v := range volumes {
				existingHandles = append(existingHandles, v.Handle())
			}

			Expect(existingHandles).ToNot(ContainElement(volumeHandle))
		})
	})
})
