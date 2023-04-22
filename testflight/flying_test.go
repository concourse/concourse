package testflight_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Flying", func() {
	var fixture, input1, input2 string

	BeforeEach(func() {
		var err error

		fixture = filepath.Join(tmp, "fixture")
		input1 = filepath.Join(tmp, "input-1")
		input2 = filepath.Join(tmp, "input-2")

		err = os.MkdirAll(fixture, 0755)
		Expect(err).NotTo(HaveOccurred())

		err = os.MkdirAll(input1, 0755)
		Expect(err).NotTo(HaveOccurred())

		err = os.MkdirAll(input2, 0755)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(
			filepath.Join(fixture, "run"),
			[]byte(`#!/bin/sh
echo some output
echo FOO is $FOO
echo ARGS are "$@"
exit 0
`),
			0755,
		)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(
			filepath.Join(tmp, "task.yml"),
			[]byte(`---
platform: linux

image_resource:
  type: mock
  source: {mirror_self: true}

inputs:
- name: fixture
- name: input-1
- name: input-2
  optional: true

outputs:
- name: output-1
- name: output-2

params:
  FOO: 1

run:
  path: fixture/run
`),
			0644,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	It("works", func() {
		execS := flyIn(tmp, "execute", "-c", "task.yml", "-i", "fixture=./fixture", "-i", "input-1=./input-1", "-i", "input-2=./input-2", "--", "SOME", "ARGS")
		Expect(execS).To(gbytes.Say("some output"))
		Expect(execS).To(gbytes.Say("FOO is 1"))
		Expect(execS).To(gbytes.Say("ARGS are SOME ARGS"))
	})

	Describe("hijacking", func() {
		It("executes an interactive command in a running task's container", func() {
			err := ioutil.WriteFile(
				filepath.Join(fixture, "run"),
				[]byte(`#!/bin/sh
mkfifo /tmp/fifo
echo waiting-for-hijack
cat < /tmp/fifo
`),
				0755,
			)
			Expect(err).NotTo(HaveOccurred())

			execS := spawnFlyIn(tmp, "execute", "-c", "task.yml", "-i", "fixture=./fixture", "-i", "input-1=./input-1", "-i", "input-2=./input-2")

			Eventually(execS).Should(gbytes.Say("executing build"))
			buildRegex := regexp.MustCompile(`executing build (\d+)`)
			matches := buildRegex.FindSubmatch(execS.Out.Contents())
			buildID := string(matches[1])

			Eventually(execS).Should(gbytes.Say("waiting-for-hijack"))

			envS := fly("intercept", "-b", buildID, "-s", "one-off", "--", "env")
			Expect(envS.Out).To(gbytes.Say("FOO=1"))

			fly("intercept", "-b", buildID, "-s", "one-off", "--", "sh", "-c", "echo marco > /tmp/fifo")

			Eventually(execS).Should(gbytes.Say("marco"))
			Eventually(execS).Should(gexec.Exit(0))
		})
	})

	Describe("uploading inputs with and without --include-ignored", func() {
		BeforeEach(func() {
			gitIgnorePath := filepath.Join(input1, ".gitignore")

			err := ioutil.WriteFile(gitIgnorePath, []byte(`*.exist`), 0644)
			Expect(err).NotTo(HaveOccurred())

			fileToBeIgnoredPath := filepath.Join(input1, "expect-not-to.exist")
			err = ioutil.WriteFile(fileToBeIgnoredPath, []byte(`ignored file content`), 0644)
			Expect(err).NotTo(HaveOccurred())

			fileToBeIncludedPath := filepath.Join(input2, "expect-to.exist")
			err = ioutil.WriteFile(fileToBeIncludedPath, []byte(`included file content`), 0644)
			Expect(err).NotTo(HaveOccurred())

			file1 := filepath.Join(input1, "file-1")
			err = ioutil.WriteFile(file1, []byte(`file-1 contents`), 0644)
			Expect(err).NotTo(HaveOccurred())

			file2 := filepath.Join(input2, "file-2")
			err = ioutil.WriteFile(file2, []byte(`file-2 contents`), 0644)
			Expect(err).NotTo(HaveOccurred())

			err = os.Mkdir(filepath.Join(input1, ".git"), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = os.Mkdir(filepath.Join(input1, ".git/refs"), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = os.Mkdir(filepath.Join(input1, ".git/objects"), 0755)
			Expect(err).NotTo(HaveOccurred())

			gitHEADPath := filepath.Join(input1, ".git/HEAD")
			err = ioutil.WriteFile(gitHEADPath, []byte(`ref: refs/heads/master`), 0644)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(
				filepath.Join(fixture, "run"),
				[]byte(`#!/bin/sh
cp -a input-1/. output-1/
cp -a input-2/. output-2/
`),
				0755,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("uploads git repo input and non git repo input, IGNORING things in the .gitignore for git repo inputs", func() {
			flyIn(tmp, "execute", "-c", "task.yml", "-i", "fixture=./fixture", "-i", "input-1=./input-1", "-i", "input-2=./input-2", "-o", "output-1=./output-1", "-o", "output-2=./output-2")

			fileToBeIgnoredPath := filepath.Join(tmp, "output-1", "expect-not-to.exist")
			fileToBeIncludedPath := filepath.Join(tmp, "output-2", "expect-to.exist")
			file1 := filepath.Join(tmp, "output-1", "file-1")
			file2 := filepath.Join(tmp, "output-2", "file-2")

			_, err := ioutil.ReadFile(fileToBeIgnoredPath)
			Expect(err).To(HaveOccurred())

			Expect(ioutil.ReadFile(fileToBeIncludedPath)).To(Equal([]byte("included file content")))
			Expect(ioutil.ReadFile(file1)).To(Equal([]byte("file-1 contents")))
			Expect(ioutil.ReadFile(file2)).To(Equal([]byte("file-2 contents")))
		})

		It("uploads git repo input and non git repo input, INCLUDING things in the .gitignore for git repo inputs", func() {
			flyIn(tmp, "execute", "--include-ignored", "-c", "task.yml", "-i", "fixture=./fixture", "-i", "input-1=./input-1", "-i", "input-2=./input-2", "-o", "output-1=./output-1", "-o", "output-2=./output-2")

			fileToBeIgnoredPath := filepath.Join(tmp, "output-1", "expect-not-to.exist")
			fileToBeIncludedPath := filepath.Join(tmp, "output-2", "expect-to.exist")
			file1 := filepath.Join(tmp, "output-1", "file-1")
			file2 := filepath.Join(tmp, "output-2", "file-2")

			Expect(ioutil.ReadFile(fileToBeIgnoredPath)).To(Equal([]byte("ignored file content")))
			Expect(ioutil.ReadFile(fileToBeIncludedPath)).To(Equal([]byte("included file content")))
			Expect(ioutil.ReadFile(file1)).To(Equal([]byte("file-1 contents")))
			Expect(ioutil.ReadFile(file2)).To(Equal([]byte("file-2 contents")))
		})
	})

	Describe("pulling down outputs", func() {
		It("works", func() {
			err := ioutil.WriteFile(
				filepath.Join(fixture, "run"),
				[]byte(`#!/bin/sh
echo hello > output-1/file-1
echo world > output-2/file-2
`),
				0755,
			)
			Expect(err).NotTo(HaveOccurred())

			flyIn(tmp, "execute", "-c", "task.yml", "-i", "fixture=./fixture", "-i", "input-1=./input-1", "-i", "input-2=./input-2", "-o", "output-1=./output-1", "-o", "output-2=./output-2")

			file1 := filepath.Join(tmp, "output-1", "file-1")
			file2 := filepath.Join(tmp, "output-2", "file-2")

			Expect(ioutil.ReadFile(file1)).To(Equal([]byte("hello\n")))
			Expect(ioutil.ReadFile(file2)).To(Equal([]byte("world\n")))
		})
	})

	Describe("aborting", func() {
		It("terminates the running task", func() {
			err := ioutil.WriteFile(
				filepath.Join(fixture, "run"),
				[]byte(`#!/bin/sh
trap "echo task got sigterm; exit 1" SIGTERM
sleep 1000 &
echo waiting-for-abort
wait
`),
				0755,
			)
			Expect(err).NotTo(HaveOccurred())

			execS := spawnFlyIn(tmp, "execute", "-c", "task.yml", "-i", "fixture=./fixture", "-i", "input-1=./input-1", "-i", "input-2=./input-2")

			Eventually(execS).Should(gbytes.Say("waiting-for-abort"))

			execS.Signal(syscall.SIGTERM)

			Eventually(execS).Should(gbytes.Say("task got sigterm"))
			Eventually(execS).Should(gbytes.Say("interrupted"))

			// build should have been aborted
			Eventually(execS).Should(gexec.Exit(3))
		})
	})

	Context("when an optional input is not provided", func() {
		It("runs the task without error", func() {
			err := ioutil.WriteFile(
				filepath.Join(fixture, "run"),
				[]byte(`#!/bin/sh
ls`),
				0755,
			)
			Expect(err).NotTo(HaveOccurred())

			execS := flyIn(tmp, "execute", "-c", "task.yml", "-i", "fixture=./fixture", "-i", "input-1=./input-1")

			fileList := string(execS.Out.Contents())
			Expect(fileList).To(ContainSubstring("fixture"))
			Expect(fileList).To(ContainSubstring("input-1"))
			Expect(fileList).NotTo(ContainSubstring("input-2"))
		})
	})

	Context("when execute with -j inputs-from", func() {
		BeforeEach(func() {
			setAndUnpausePipeline("fixtures/config-test.yml")

			taskFileContents := `---
platform: linux

image_resource:
  type: mock
  source: {mirror_self: true}

inputs:
- name: some-resource

run:
  path: cat
  args: [some-resource/version]
`

			err := ioutil.WriteFile(
				filepath.Join(tmp, "task.yml"),
				[]byte(taskFileContents),
				0644,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("runs the task without error", func() {
			By("having an initial version")
			fly("check-resource", "-r", pipelineName+"/some-resource", "-f", "version:first-version")

			By("satisfying the job's passed constraint for the first version")
			fly("trigger-job", "-w", "-j", pipelineName+"/upstream-job")

			By("making sure the second job runs before we use its inputs")
			fly("trigger-job", "-w", "-j", pipelineName+"/downstream-job")

			By("executing using the first version via -j")
			execS := flyIn(tmp, "execute", "-c", "task.yml", "-j", pipelineName+"/downstream-job")
			Expect(execS).To(gbytes.Say("first-version"))

			By("finding another version that doesn't yet satisfy the passed constraint")
			fly("check-resource", "-r", pipelineName+"/some-resource", "-f", "version:second-version")

			By("still executing using the first version via -j")
			execS = flyIn(tmp, "execute", "-c", "task.yml", "-j", pipelineName+"/downstream-job")
			Expect(execS).To(gbytes.Say("first-version"))

			By("satisfying the job's passed constraint for the second version")
			fly("trigger-job", "-w", "-j", pipelineName+"/upstream-job")

			By("making sure the second job runs before we use its inputs")
			fly("trigger-job", "-w", "-j", pipelineName+"/downstream-job")

			By("now executing using the second version via -j")
			execS = flyIn(tmp, "execute", "-c", "task.yml", "-j", pipelineName+"/downstream-job")
			Expect(execS).To(gbytes.Say("second-version"))
		})
	})

	Context("when execute with -j inputs-from and task has input mapping", func() {
		BeforeEach(func() {

			taskFileContents := `---
platform: linux

image_resource:
  type: mock
  source: {mirror_self: true}

inputs:
- name: mapped-resource

run:
  path: cat
  args: [mapped-resource/version]
`

			err := ioutil.WriteFile(
				filepath.Join(tmp, "task.yml"),
				[]byte(taskFileContents),
				0644,
			)
			Expect(err).NotTo(HaveOccurred())

			fly("set-pipeline", "-n", "-p", pipelineName, "-c", "fixtures/input-mapping-test.yml", "-v", "task_content="+taskFileContents+"")

			fly("unpause-pipeline", "-p", pipelineName)
		})

		It("runs the task without error", func() {
			By("having an initial version")
			fly("check-resource", "-r", pipelineName+"/some-resource", "-f", "version:first-version")

			By("satisfying the job's passed constraint for the first version")
			fly("trigger-job", "-w", "-j", pipelineName+"/upstream-job")

			By("making sure the second job runs before we use its inputs")
			fly("trigger-job", "-w", "-j", pipelineName+"/downstream-job")

			By("executing using the first version via -j")
			execS := flyIn(tmp, "execute", "-c", "task.yml", "-j", pipelineName+"/downstream-job", "-m", "mapped-resource=some-resource")
			Expect(execS).To(gbytes.Say("first-version"))

			By("finding another version that doesn't yet satisfy the passed constraint")
			fly("check-resource", "-r", pipelineName+"/some-resource", "-f", "version:second-version")

			By("still executing using the first version via -j")
			execS = flyIn(tmp, "execute", "-c", "task.yml", "-j", pipelineName+"/downstream-job", "-m", "mapped-resource=some-resource")
			Expect(execS).To(gbytes.Say("first-version"))

			By("satisfying the job's passed constraint for the second version")
			fly("trigger-job", "-w", "-j", pipelineName+"/upstream-job")

			By("making sure the second job runs before we use its inputs")
			fly("trigger-job", "-w", "-j", pipelineName+"/downstream-job")

			By("now executing using the second version via -j")
			execS = flyIn(tmp, "execute", "-c", "task.yml", "-j", pipelineName+"/downstream-job", "-m", "mapped-resource=some-resource")
			Expect(execS).To(gbytes.Say("second-version"))
		})
	})

	Context("when execute with -j inputs-from and task has no image_resource specified", func() {
		BeforeEach(func() {

			taskFileContents := `---
platform: linux

inputs:
- name: some-resource

run:
  path: cat
  args: [some-resource/version]
`

			err := ioutil.WriteFile(
				filepath.Join(tmp, "task.yml"),
				[]byte(taskFileContents),
				0644,
			)
			Expect(err).NotTo(HaveOccurred())

			fly("set-pipeline", "-n", "-p", pipelineName, "-c", "fixtures/image-resource-test.yml", "-v", "task_content="+taskFileContents+"")

			fly("unpause-pipeline", "-p", pipelineName)
		})

		It("runs the task without error", func() {
			By("having an initial version")
			fly("check-resource", "-r", pipelineName+"/some-resource", "-f", "version:first-version")

			By("satisfying the job's passed constraint for the first version")
			fly("trigger-job", "-w", "-j", pipelineName+"/upstream-job")

			By("making sure the second job runs before we use its inputs")
			fly("trigger-job", "-w", "-j", pipelineName+"/downstream-job")

			By("executing using the first version via -j")
			execS := flyIn(tmp, "execute", "-c", "task.yml", "-j", pipelineName+"/downstream-job", "--image", "some-image")
			Expect(execS).To(gbytes.Say("first-version"))

			By("finding another version that doesn't yet satisfy the passed constraint")
			fly("check-resource", "-r", pipelineName+"/some-resource", "-f", "version:second-version")

			By("still executing using the first version via -j")
			execS = flyIn(tmp, "execute", "-c", "task.yml", "-j", pipelineName+"/downstream-job", "--image", "some-image")
			Expect(execS).To(gbytes.Say("first-version"))

			By("satisfying the job's passed constraint for the second version")
			fly("trigger-job", "-w", "-j", pipelineName+"/upstream-job")

			By("making sure the second job runs before we use its inputs")
			fly("trigger-job", "-w", "-j", pipelineName+"/downstream-job")

			By("now executing using the second version via -j")
			execS = flyIn(tmp, "execute", "-c", "task.yml", "-j", pipelineName+"/downstream-job", "--image", "some-image")
			Expect(execS).To(gbytes.Say("second-version"))
		})
	})

	Context("when the input is custom resource", func() {
		BeforeEach(func() {
			taskFileContents := `---
platform: linux

image_resource:
  type: mock
  source: {mirror_self: true}

inputs:
- name: some-resource

run:
  path: cat
  args: [some-resource/version]
`
			err := ioutil.WriteFile(
				filepath.Join(tmp, "task.yml"),
				[]byte(taskFileContents),
				0644,
			)
			Expect(err).NotTo(HaveOccurred())

			setAndUnpausePipeline("fixtures/custom-resource-type.yml")
		})

		Context("when -j is specified", func() {
			BeforeEach(func() {
				By("making sure the job runs before we use its inputs")
				fly("trigger-job", "-w", "-j", pipelineName+"/input-test")
			})

			It("runs the task without error by infer the pipeline resource types from -j", func() {
				execS := flyIn(tmp, "execute", "-c", "task.yml", "-j", pipelineName+"/input-test")
				Expect(execS).To(gbytes.Say("custom-type-version"))
			})
		})

		Context("when -j is not specified and local input in custom resource type is provided", func() {
			BeforeEach(func() {
				err := ioutil.WriteFile(
					filepath.Join(fixture, "version"),
					[]byte(`#!/bin/sh
echo hello from fixture
`),
					0755,
				)
				Expect(err).NotTo(HaveOccurred())
			})

			It("runs the task without error", func() {
				execS := flyIn(tmp, "execute", "-c", "task.yml", "-i", "some-resource=./fixture")
				Expect(execS).To(gbytes.Say("hello from fixture"))
			})
		})
	})
})
