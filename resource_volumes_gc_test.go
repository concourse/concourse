package topgun_test

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe(":life Garbage collecting resource cache volumes", func() {
	var dbConn *sql.DB

	BeforeEach(func() {
		var err error
		dbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://atc:dummy-password@%s:5432/atc?sslmode=disable", atcIP))
		Expect(err).ToNot(HaveOccurred())

		Deploy("deployments/single-vm.yml")
	})

	Describe("A resource that was removed from pipeline", func() {
		It("has its resource cache, resource cache uses and resource cache volumes cleared out", func() {
			By("setting pipeline that creates resource cache")
			fly("set-pipeline", "-n", "-c", "pipelines/get-task-changing-resource.yml", "-p", "volume-gc-test")

			By("unpausing the pipeline")
			fly("unpause-pipeline", "-p", "volume-gc-test")

			By("triggering the job")
			fly("trigger-job", "-w", "-j", "volume-gc-test/simple-job")

			By("getting the resource config container")
			volumes := flyTable("volumes")
			resourceVolumeHandles := []string{}
			for _, volume := range volumes {
				// there is going to be one for image resource
				if volume["type"] == "resource" && strings.HasPrefix(volume["identifier"], "time:") {
					resourceVolumeHandles = append(resourceVolumeHandles, volume["handle"])
				}
			}
			Expect(resourceVolumeHandles).To(HaveLen(1))

			By("getting the resource caches")
			var resourceCachesNum int
			err := psql.Select("COUNT(id)").From("resource_caches").Where("version LIKE ?", fmt.Sprint("%", "time", "%")).RunWith(dbConn).QueryRow().Scan(&resourceCachesNum)
			Expect(err).ToNot(HaveOccurred())
			Expect(resourceCachesNum).To(Equal(1))

			By("getting the resource caches uses")
			var resourceCacheUsesNum int
			err = psql.Select("COUNT(*)").From("resource_cache_uses").RunWith(dbConn).QueryRow().Scan(&resourceCacheUsesNum)
			Expect(err).ToNot(HaveOccurred())
			// there is going to be one for image resource
			Expect(resourceCacheUsesNum).To(Equal(2))

			By("updating pipeline and removing resource")
			fly("set-pipeline", "-n", "-c", "pipelines/task-waiting.yml", "-p", "volume-gc-test")

			By("eventually expiring the resource cache volumes")
			Eventually(func() int {
				volumes := flyTable("volumes")
				resourceVolumeHandles := []string{}
				for _, volume := range volumes {
					// there is going to be one for image resource
					if volume["type"] == "resource" && strings.HasPrefix(volume["identifier"], "time:") {
						resourceVolumeHandles = append(resourceVolumeHandles, volume["handle"])
					}
				}
				return len(resourceVolumeHandles)
			}, 5*time.Minute, 10*time.Second).Should(BeZero())

			By("expiring the resource caches")
			err = psql.Select("COUNT(id)").From("resource_caches").Where("version LIKE ?", fmt.Sprint("%", "time", "%")).RunWith(dbConn).QueryRow().Scan(&resourceCachesNum)
			Expect(err).ToNot(HaveOccurred())
			Expect(resourceCachesNum).To(BeZero())

			By("expiring the resource caches uses")
			err = psql.Select("COUNT(*)").From("resource_cache_uses").RunWith(dbConn).QueryRow().Scan(&resourceCacheUsesNum)
			Expect(err).ToNot(HaveOccurred())
			// there is going to be one for image resource
			Expect(resourceCacheUsesNum).To(Equal(1))
		})
	})
})
