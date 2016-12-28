package topgun_test

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"time"

	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("[#129726011] Worker landing", func() {
	var dbConn *sql.DB

	BeforeEach(func() {
		var err error
		dbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://atc:dummy-password@%s:5432/atc?sslmode=disable", atcIP))
		Expect(err).ToNot(HaveOccurred())
	})

	waitForLandingWorker := func() string {
		var landingWorkerName string
		Eventually(func() string {
			rows, err := psql.Select("name, state").From("workers").RunWith(dbConn).Query()
			Expect(err).ToNot(HaveOccurred())

			for rows.Next() {
				var name string
				var state string

				err := rows.Scan(&name, &state)
				Expect(err).ToNot(HaveOccurred())

				if state != "landing" {
					continue
				}

				if landingWorkerName != "" {
					Fail("multiple workers landing")
				}

				landingWorkerName = name
			}

			return landingWorkerName
		}).ShouldNot(BeEmpty())

		return landingWorkerName
	}

	Context("with two workers available", func() {
		BeforeEach(func() {
			Deploy("deployments/two-forwarded-workers.yml")
		})

		Describe("restarting the worker", func() {
			var landingWorkerName string
			var restartSession *gexec.Session

			JustBeforeEach(func() {
				restartSession = spawnBosh("restart", "worker/0")
				landingWorkerName = waitForLandingWorker()
			})

			AfterEach(func() {
				<-restartSession.Exited
			})

			It("is not used for new workloads", func() {
				for i := 0; i < 10; i++ {
					fly("execute", "-c", "tasks/tiny.yml")
					rows, err := psql.Select("id, worker_name").From("containers").RunWith(dbConn).Query()
					Expect(err).ToNot(HaveOccurred())

					usedWorkers := map[string]struct{}{}
					for rows.Next() {
						var id int
						var workerName string
						err := rows.Scan(&id, &workerName)
						Expect(err).ToNot(HaveOccurred())
						usedWorkers[workerName] = struct{}{}
					}

					Expect(usedWorkers).To(HaveLen(1))
					Expect(usedWorkers).ToNot(ContainElement(landingWorkerName))
				}
			})

			Context("with a build in-flight", func() {
				var buildSession *gexec.Session
				var buildID string

				BeforeEach(func() {
					buildSession = spawnFly("execute", "-c", "tasks/wait.yml")
					Eventually(buildSession).Should(gbytes.Say("executing build"))

					buildRegex := regexp.MustCompile(`executing build (\d+)`)
					matches := buildRegex.FindSubmatch(buildSession.Out.Contents())
					buildID = string(matches[1])

					Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))
				})

				AfterEach(func() {
					buildSession.Signal(os.Interrupt)
					<-buildSession.Exited
				})

				It("waits for the build", func() {
					Eventually(restartSession).Should(gbytes.Say(`Updating (instance|job)`))
					Consistently(restartSession, 5*time.Minute).ShouldNot(gexec.Exit())
				})

				It("finishes restarting once the build is done", func() {
					By("hijacking the build to tell it to finish")
					Eventually(func() int {
						session := spawnFlyInteractive(
							bytes.NewBufferString("3\n"),
							"hijack",
							"-b", buildID,
							"-s", "one-off",
							"touch", "/tmp/stop-waiting",
						)

						<-session.Exited
						return session.ExitCode()
					}).Should(Equal(0))

					By("waiting for the build to exit")
					Eventually(buildSession).Should(gbytes.Say("done"))
					<-buildSession.Exited
					Expect(buildSession.ExitCode()).To(Equal(0))

					By("successfully restarting")
					<-restartSession.Exited
					Expect(restartSession.ExitCode()).To(Equal(0))
				})
			})
		})

		// Describe("recreating the worker", func() {
		// 	var landingWorkerName string
		// 	var recreateSession *gexec.Session

		// 	JustBeforeEach(func() {
		// 		recreateSession = spawnBosh("recreate", "worker/0")
		// 		landingWorkerName = waitForLandingWorker()
		// 	})

		// 	Describe("after the recreate is complete", func() {
		// 		XIt("no longer has the volumes", func() {
		// 		})
		// 	})
		// })
	})

	Context("with one worker", func() {
		BeforeEach(func() {
			Deploy("deployments/one-forwarded-worker.yml")
		})

		Describe("restarting the worker", func() {
			var restartSession *gexec.Session

			JustBeforeEach(func() {
				restartSession = spawnBosh("restart", "worker/0")
			})

			Context("with volumes and containers present", func() {
				var preservedContainerID string

				BeforeEach(func() {
					By("setting pipeline that creates volumes for image")
					fly("set-pipeline", "-n", "-c", "pipelines/get-task.yml", "-p", "topgun")

					By("unpausing the pipeline")
					fly("unpause-pipeline", "-p", "topgun")

					By("triggering a job")
					buildSession := spawnFly("trigger-job", "-w", "-j", "topgun/simple-job")
					Eventually(buildSession).Should(gbytes.Say("Pulling .*busybox.*"))
					<-buildSession.Exited
					Expect(buildSession.ExitCode()).To(Equal(0))

					By("getting identifier for check container")
					hijackSession := spawnFly("hijack", "-c", "topgun/tick-tock", "--", "hostname")
					<-hijackSession.Exited
					Expect(buildSession.ExitCode()).To(Equal(0))

					preservedContainerID = string(hijackSession.Out.Contents())
				})

				It("keeps volumes and containers after restart", func() {
					By("completing the restart")
					<-restartSession.Exited
					Expect(restartSession.ExitCode()).To(Equal(0))

					By("retaining cached image resource in second job build")
					buildSession := spawnFly("trigger-job", "-w", "-j", "topgun/simple-job")
					Eventually(buildSession).Should(gbytes.Say("using version of resource found in cache"))
					<-buildSession.Exited
					Expect(buildSession.ExitCode()).To(Equal(0))

					By("retaining check containers")
					hijackSession := spawnFly("hijack", "-c", "topgun/tick-tock", "--", "hostname")
					<-hijackSession.Exited
					Expect(buildSession.ExitCode()).To(Equal(0))

					currentContainerID := string(hijackSession.Out.Contents())
					Expect(currentContainerID).To(Equal(preservedContainerID))
				})
			})

			Context("with an interruptible build in-flight", func() {
				var buildSession *gexec.Session

				BeforeEach(func() {
					By("setting pipeline that has an infinite but interruptible job")
					fly("set-pipeline", "-n", "-c", "pipelines/interruptible.yml", "-p", "topgun")

					By("unpausing the pipeline")
					fly("unpause-pipeline", "-p", "topgun")

					By("triggering a job")
					buildSession = spawnFly("trigger-job", "-w", "-j", "topgun/interruptible-job")
					Eventually(buildSession).Should(gbytes.Say("waiting forever"))
				})

				It("does not wait for the build", func() {
					By("completing the restart without the drain timeout kicking in")
					Eventually(restartSession, 5*time.Minute).Should(gexec.Exit(0))
				})
			})
		})
	})
})
