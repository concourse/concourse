package pipelines_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with multiple inputs", func() {
	var (
		firstVersionA string
		firstVersionB string

		tmpdir           string
		taskConfig       string
		localGitRepoBDir string
	)

	BeforeEach(func() {
		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/many-inputs.yml",
		)

		firstVersionA = newMockVersion("some-resource-a", "first-a")
		firstVersionB = newMockVersion("some-resource-b", "first-b")

		var err error
		tmpdir, err = ioutil.TempDir("", "fly-test")
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(
			filepath.Join(tmpdir, "task.yml"),
			[]byte(`---
platform: linux

image_resource:
  type: mock
  source: {mirror_self: true}

inputs:
- name: some-resource-a
- name: some-resource-b

run:
  path: sh
  args:
    - -c
    - |
      echo a has $(cat some-resource-a/version)
      echo b has $(cat some-resource-b/version)
`),
			0644,
		)
		Expect(err).NotTo(HaveOccurred())

		taskConfig = filepath.Join(tmpdir, "task.yml")
		localGitRepoBDir = filepath.Join(tmpdir, "some-resource-b")

		err = os.Mkdir(localGitRepoBDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(
			filepath.Join(localGitRepoBDir, "version"),
			[]byte("some-overridden-version"),
			0644,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpdir)
	})

	It("can have its inputs used as the basis for a one-off build", func() {
		By("waiting for an initial build so the job has inputs")
		watch := flyHelper.TriggerJob(pipelineName, "some-job")
		<-watch.Exited
		Expect(watch.ExitCode()).To(Equal(0))
		Expect(watch).To(gbytes.Say("initializing"))
		Expect(watch).To(gbytes.Say("a has " + firstVersionA))
		Expect(watch).To(gbytes.Say("b has " + firstVersionB))
		Expect(watch).To(gbytes.Say("succeeded"))

		By("running a one-off with the same inputs and no local inputs")
		execute := flyHelper.Execute(
			localGitRepoBDir,
			"-c", taskConfig,
			"--inputs-from", pipelineName+"/some-job",
		)
		<-execute.Exited
		Expect(execute).To(gbytes.Say("initializing"))
		Expect(execute).To(gbytes.Say("a has " + firstVersionA))
		Expect(execute).To(gbytes.Say("b has " + firstVersionB))
		Expect(execute).To(gbytes.Say("succeeded"))
		Expect(execute).To(gexec.Exit(0))

		By("running a one-off with one of the inputs overridden")
		execute = flyHelper.Execute(localGitRepoBDir,
			"-c", taskConfig,
			"--inputs-from", pipelineName+"/some-job",
			"--input", "some-resource-b="+localGitRepoBDir,
		)
		<-execute.Exited
		Expect(execute).To(gbytes.Say("initializing"))
		Expect(execute).To(gbytes.Say("a has " + firstVersionA))
		Expect(execute).To(gbytes.Say("b has some-overridden-version"))
		Expect(execute).To(gbytes.Say("succeeded"))
		Expect(execute).To(gexec.Exit(0))
	})
})
