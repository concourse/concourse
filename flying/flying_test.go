package flying_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mgutz/ansi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Flying", func() {
	var tmpdir string
	var fixture string
	var atcURL string

	BeforeEach(func() {
		var err error

		atcURL = "http://10.244.15.2:8080"

		tmpdir, err = ioutil.TempDir("", "fly-test")
		Ω(err).ShouldNot(HaveOccurred())

		fixture = filepath.Join(tmpdir, "fixture")

		err = os.MkdirAll(fixture, 0755)
		Ω(err).ShouldNot(HaveOccurred())

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
		Ω(err).ShouldNot(HaveOccurred())

		err = ioutil.WriteFile(
			filepath.Join(fixture, "build.yml"),
			[]byte(`---
platform: linux

image: /var/vcap/packages/busybox

inputs:
  - name: fixture

params:
  FOO: 1

run:
  path: fixture/run
`),
			0644,
		)
		Ω(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpdir)
	})

	start := func(cmd *exec.Cmd) *gexec.Session {
		session, err := gexec.Start(
			cmd,
			gexec.NewPrefixedWriter(
				fmt.Sprintf("%s%s ", ansi.Color("[o]", "green"), ansi.Color("[fly]", "blue")),
				GinkgoWriter,
			),
			gexec.NewPrefixedWriter(
				fmt.Sprintf("%s%s ", ansi.Color("[e]", "red+bright"), ansi.Color("[fly]", "blue")),
				GinkgoWriter,
			),
		)
		Ω(err).ShouldNot(HaveOccurred())

		return session
	}

	It("works", func() {
		fly := exec.Command(flyBin, "-t", atcURL, "execute", "-c", "build.yml", "--", "SOME", "ARGS")
		fly.Dir = fixture

		session := start(fly)

		Eventually(session, 30*time.Second).Should(gexec.Exit(0))

		Ω(session).Should(gbytes.Say("some output"))
		Ω(session).Should(gbytes.Say("FOO is 1"))
		Ω(session).Should(gbytes.Say("ARGS are SOME ARGS"))
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
			Ω(err).ShouldNot(HaveOccurred())

			fly := exec.Command(flyBin, "-t", atcURL, "execute")
			fly.Dir = fixture

			flyS := start(fly)

			Eventually(flyS, 30*time.Second).Should(gbytes.Say("waiting"))

			hijack := exec.Command(flyBin, "-t", atcURL, "hijack", "--", "sh", "-c", "echo marco > /tmp/fifo")

			hijackS := start(hijack)

			Eventually(flyS, 15*time.Second).Should(gbytes.Say("marco"))

			Eventually(hijackS, 5*time.Second).Should(gexec.Exit())

			Eventually(flyS, 10*time.Second).Should(gexec.Exit(0))
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
			Ω(err).ShouldNot(HaveOccurred())

			fly := exec.Command(flyBin, "-t", atcURL, "execute")
			fly.Dir = fixture

			flyS := start(fly)

			Eventually(flyS, 30*time.Second).Should(gbytes.Say("waiting"))

			flyS.Signal(syscall.SIGTERM)

			Eventually(flyS, 10*time.Second).Should(gbytes.Say("task got sigterm"))

			// build should have been aborted
			Eventually(flyS, 10*time.Second).Should(gexec.Exit(3))
		})
	})
})
