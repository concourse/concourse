package flying_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"

	"github.com/concourse/testflight/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Flying", func() {
	var tmpdir string
	var fixture string

	BeforeEach(func() {
		var err error

		tmpdir, err = ioutil.TempDir("", "fly-test")
		Expect(err).NotTo(HaveOccurred())

		fixture = filepath.Join(tmpdir, "fixture")

		err = os.MkdirAll(fixture, 0755)
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
			filepath.Join(fixture, "task.yml"),
			[]byte(`---
platform: linux

image_resource:
  type: docker-image
  source: {repository: busybox}

inputs:
- name: fixture

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

	AfterEach(func() {
		os.RemoveAll(tmpdir)
	})

	It("works", func() {
		fly := exec.Command(flyBin, "-t", targetedConcourse, "execute", "-c", "task.yml", "--", "SOME", "ARGS")
		fly.Dir = fixture

		session := helpers.StartFly(fly)

		Eventually(session).Should(gexec.Exit(0))

		Expect(session).To(gbytes.Say("some output"))
		Expect(session).To(gbytes.Say("FOO is 1"))
		Expect(session).To(gbytes.Say("ARGS are SOME ARGS"))
	})

	Describe("hijacking", func() {
		It("executes an interactive command in a running task's container", func() {
			err := ioutil.WriteFile(
				filepath.Join(fixture, "run"),
				[]byte(`#!/bin/sh
mkfifo /tmp/fifo
echo waiting
cat < /tmp/fifo
`),
				0755,
			)
			Expect(err).NotTo(HaveOccurred())

			fly := exec.Command(flyBin, "-t", targetedConcourse, "execute", "-c", "task.yml")
			fly.Dir = fixture

			flyS := helpers.StartFly(fly)

			Eventually(flyS).Should(gbytes.Say("executing build"))

			buildRegex := regexp.MustCompile(`executing build (\d+)`)
			matches := buildRegex.FindSubmatch(flyS.Out.Contents())
			buildID := string(matches[1])

			Eventually(flyS).Should(gbytes.Say("waiting"))

			hijack := exec.Command(flyBin, "-t", targetedConcourse, "hijack", "-b", buildID, "-s", "one-off", "--", "sh", "-c", "echo marco > /tmp/fifo")

			hijackIn, err := hijack.StdinPipe()
			Expect(err).NotTo(HaveOccurred())

			hijackS := helpers.StartFly(hijack)

			Eventually(hijackS).Should(gbytes.Say("type: task"))

			re, err := regexp.Compile("([0-9]): .+ type: task")
			Expect(err).NotTo(HaveOccurred())

			taskNumber := re.FindStringSubmatch(string(hijackS.Out.Contents()))[1]
			fmt.Fprintln(hijackIn, taskNumber)

			Eventually(flyS).Should(gbytes.Say("marco"))

			Eventually(hijackS).Should(gexec.Exit())

			Eventually(flyS).Should(gexec.Exit(0))
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

			fly := exec.Command(flyBin, "-t", targetedConcourse, "execute", "-c", "task.yml", "-o", "output-1=./output-1", "-o", "output-2=./output-2")
			fly.Dir = fixture

			session := helpers.StartFly(fly)
			<-session.Exited

			Expect(session.ExitCode()).To(Equal(0))

			file1 := filepath.Join(fixture, "output-1", "file-1")
			file2 := filepath.Join(fixture, "output-2", "file-2")

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
echo waiting
wait
`),
				0755,
			)
			Expect(err).NotTo(HaveOccurred())

			fly := exec.Command(flyBin, "-t", targetedConcourse, "execute", "-c", "task.yml")
			fly.Dir = fixture

			flyS := helpers.StartFly(fly)

			Eventually(flyS).Should(gbytes.Say("waiting"))

			flyS.Signal(syscall.SIGTERM)

			Eventually(flyS).Should(gbytes.Say("task got sigterm"))

			// build should have been aborted
			Eventually(flyS).Should(gexec.Exit(3))
		})
	})
})
