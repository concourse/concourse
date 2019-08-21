package testflight_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Admin priviliges", func() {
	var tmpDir, ogHome, pipelineName, pipelineConfig string
	priviligedAdminTarget := testflightFlyTarget + "-padmin"
	newTeamName := "priviliged-admin-test-team"

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "fly-test")
		Expect(err).ToNot(HaveOccurred())

		ogHome = os.Getenv("HOME")
		os.Setenv("HOME", tmpDir)

		Eventually(func() *gexec.Session {
			login := spawnFlyLogin(adminFlyTarget)
			<-login.Exited
			return login
		}, 2*time.Minute, time.Second).Should(gexec.Exit(0))

		pipelineName = randomPipelineName()
		pipelineConfig = filepath.Join(tmpDir, "pipeline.yml")

		err = ioutil.WriteFile(pipelineConfig,
			[]byte(`---
resources:
- name: time-test
  type: time
  source: {interval: 1s}

jobs:
  - name: admin-sample-job
    public: true
    plan:
      - get: time-test
      - task: simple-task
        config:
          platform: linux
          image_resource:
            type: registry-image
            source: { repository: busybox }
          run:
            path: /bin/sh
            args:
            - -c
            - |
              echo Hello, world
              sleep 5`), 0644)
		Expect(err).NotTo(HaveOccurred())

		fly("-t", adminFlyTarget, "set-team", "--non-interactive", "-n", newTeamName, "--local-user", "guest")
		wait(spawnFlyLogin(priviligedAdminTarget, "-n", newTeamName))

	})

	AfterEach(func() {
		fly("-t", priviligedAdminTarget, "destroy-team", "--non-interactive", "-n", newTeamName+"-new")
		os.RemoveAll(tmpDir)
		os.Setenv("HOME", ogHome)
	})

	FContext("Team-scoped commands", func() {

		It("Admin user is able to perform all team-scoped commands", func() {
			fly("-t", priviligedAdminTarget, "set-pipeline", "--non-interactive", "-p", pipelineName, "-c", pipelineConfig)
			fly("-t", priviligedAdminTarget, "unpause-pipeline", "-p", pipelineName)
			fly("-t", priviligedAdminTarget, "pause-job", "-j", pipelineName+"/admin-sample-job")
			fly("-t", priviligedAdminTarget, "unpause-job", "-j", pipelineName+"/admin-sample-job")

			fly("-t", priviligedAdminTarget, "expose-pipeline", "-p", pipelineName)
			fly("-t", priviligedAdminTarget, "hide-pipeline", "-p", pipelineName)

			sess := fly("-t", priviligedAdminTarget, "get-pipeline", "-p", pipelineName)
			Expect(sess.Out.Contents()).To(ContainSubstring("echo"))

			sess = fly("-t", priviligedAdminTarget, "jobs", "-p", pipelineName)
			Expect(sess.Out.Contents()).To(ContainSubstring("admin-sample-job"))

			sess = fly("-t", priviligedAdminTarget, "resources", "-p", pipelineName)
			Expect(sess.Out.Contents()).To(ContainSubstring("time-test"))

			fly("-t", priviligedAdminTarget, "trigger-job", "-j", pipelineName+"/admin-sample-job")

			fly("-t", priviligedAdminTarget, "watch", "-j", pipelineName+"/admin-sample-job")

			fly("-t", priviligedAdminTarget, "rename-pipeline", "-o", pipelineName, "-n", pipelineName+"-new")

			fly("-t", priviligedAdminTarget, "pause-pipeline", "-p", pipelineName+"-new")
			fly("-t", priviligedAdminTarget, "destroy-pipeline", "--non-interactive", "-p", pipelineName+"-new")

			fly("-t", priviligedAdminTarget, "rename-team", "-o", newTeamName, "-n", newTeamName+"-new")
		})
	})
})
