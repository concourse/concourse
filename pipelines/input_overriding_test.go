package pipelines_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with multiple inputs", func() {
	var (
		gitServerA *gitserver.Server
		gitServerB *gitserver.Server

		firstGuidA string
		firstGuidB string

		tmpdir           string
		taskConfig       string
		localGitRepoBDir string
	)

	BeforeEach(func() {
		gitServerA = gitserver.Start(client)
		gitServerB = gitserver.Start(client)

		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/many-inputs.yml",
			"-v", "git-server-a="+gitServerA.URI(),
			"-v", "git-server-b="+gitServerB.URI(),
		)

		firstGuidA = gitServerA.Commit()
		firstGuidB = gitServerB.Commit()

		var err error
		tmpdir, err = ioutil.TempDir("", "fly-test")
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(
			filepath.Join(tmpdir, "task.yml"),
			[]byte(`---
platform: linux
image_resource:
  type: docker-image
  source: {repository: busybox}
inputs:
- name: git-repo-a
- name: git-repo-b
run:
  path: sh
  args:
    - -c
    - |
      echo a has $(cat git-repo-a/guids)
      echo b has $(cat git-repo-b/guids)
`),
			0644,
		)
		Expect(err).NotTo(HaveOccurred())

		taskConfig = filepath.Join(tmpdir, "task.yml")
		localGitRepoBDir = filepath.Join(tmpdir, "git-repo-b")

		err = os.Mkdir(localGitRepoBDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(
			filepath.Join(localGitRepoBDir, "guids"),
			[]byte("some-overridden-guid"),
			0644,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpdir)

		gitServerA.Stop()
		gitServerB.Stop()
	})

	It("can have its inputs used as the basis for a one-off build", func() {
		By("waiting for an initial build so the job has inputs")
		watch := flyHelper.Watch(pipelineName, "some-job")
		Expect(watch).To(gbytes.Say("initializing"))
		Expect(watch).To(gbytes.Say("a has " + firstGuidA))
		Expect(watch).To(gbytes.Say("b has " + firstGuidB))
		Expect(watch).To(gbytes.Say("succeeded"))
		Expect(watch).To(gexec.Exit(0))

		By("running a one-off with the same inputs and no local inputs")
		execute := flyHelper.Execute(
			localGitRepoBDir,
			"-c", taskConfig,
			"--inputs-from", pipelineName+"/some-job",
		)
		<-execute.Exited
		Expect(execute).To(gbytes.Say("initializing"))
		Expect(execute).To(gbytes.Say("a has " + firstGuidA))
		Expect(execute).To(gbytes.Say("b has " + firstGuidB))
		Expect(execute).To(gbytes.Say("succeeded"))
		Expect(execute).To(gexec.Exit(0))

		By("running a one-off with one of the inputs overridden")
		execute = flyHelper.Execute(localGitRepoBDir,
			"-c", taskConfig,
			"--inputs-from", pipelineName+"/some-job",
			"--input", "git-repo-b="+localGitRepoBDir,
		)
		<-execute.Exited
		Expect(execute).To(gbytes.Say("initializing"))
		Expect(execute).To(gbytes.Say("a has " + firstGuidA))
		Expect(execute).To(gbytes.Say("b has some-overridden-guid"))
		Expect(execute).To(gbytes.Say("succeeded"))
		Expect(execute).To(gexec.Exit(0))
	})
})
