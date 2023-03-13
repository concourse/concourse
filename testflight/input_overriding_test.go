package testflight_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("A job with multiple inputs", func() {
	var (
		firstVersionA string
		firstVersionB string

		taskConfig       string
		localGitRepoBDir string
	)

	BeforeEach(func() {
		hash, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		hash2, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		setAndUnpausePipeline(
			"fixtures/many-inputs.yml",
			"-v", "hash-1="+hash.String(),
			"-v", "hash-2="+hash2.String(),
		)

		firstVersionA = newMockVersion("some-resource-a", "first-a")
		firstVersionB = newMockVersion("some-resource-b", "first-b")

		err = ioutil.WriteFile(
			filepath.Join(tmp, "task.yml"),
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

		taskConfig = filepath.Join(tmp, "task.yml")
		localGitRepoBDir = filepath.Join(tmp, "some-resource-b")

		err = os.Mkdir(localGitRepoBDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(
			filepath.Join(localGitRepoBDir, "version"),
			[]byte("some-overridden-version"),
			0644,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	It("can have its inputs used as the basis for a one-off build", func() {
		By("waiting for an initial build so the job has inputs")
		watch := fly("trigger-job", "-j", inPipeline("some-job"), "-w")
		Expect(watch).To(gbytes.Say("initializing"))
		Expect(watch).To(gbytes.Say("a has " + firstVersionA))
		Expect(watch).To(gbytes.Say("b has " + firstVersionB))
		Expect(watch).To(gbytes.Say("succeeded"))

		By("running a one-off with the same inputs and no local inputs")
		execute := flyIn(localGitRepoBDir, "execute", "-c", taskConfig,
			"--inputs-from", inPipeline("some-job"),
		)
		Expect(execute).To(gbytes.Say("initializing"))
		Expect(execute).To(gbytes.Say("a has " + firstVersionA))
		Expect(execute).To(gbytes.Say("b has " + firstVersionB))
		Expect(execute).To(gbytes.Say("succeeded"))

		By("running a one-off with one of the inputs overridden")
		execute = flyIn(localGitRepoBDir, "execute", "-c", taskConfig,
			"--inputs-from", inPipeline("some-job"),
			"--input", "some-resource-b="+localGitRepoBDir,
		)
		Expect(execute).To(gbytes.Say("initializing"))
		Expect(execute).To(gbytes.Say("a has " + firstVersionA))
		Expect(execute).To(gbytes.Say("b has some-overridden-version"))
		Expect(execute).To(gbytes.Say("succeeded"))
	})
})
