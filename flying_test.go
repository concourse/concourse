package testflight_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/kr/pty"
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
		Ω(err).ShouldNot(HaveOccurred())

		fixture = filepath.Join(tmpdir, "fixture")

		err = os.MkdirAll(fixture, 0755)
		Ω(err).ShouldNot(HaveOccurred())

		err = ioutil.WriteFile(
			filepath.Join(fixture, "run"),
			[]byte(`#!/bin/bash
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
			[]byte(fmt.Sprintf(`---
image: %s

params:
  FOO: 1

run:
  path: fixture/run
`, helperRootfs)),
			0644,
		)
		Ω(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpdir)
	})

	It("works", func() {
		fly := exec.Command(builtComponents["fly"], "--", "SOME", "ARGS")
		fly.Dir = fixture

		session, err := gexec.Start(fly, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(session, 1*time.Minute).Should(gexec.Exit(0))

		Ω(session).Should(gbytes.Say("some output"))
		Ω(session).Should(gbytes.Say("FOO is 1"))
		Ω(session).Should(gbytes.Say("ARGS are SOME ARGS"))
	})

	Describe("hijacking", func() {
		It("executes an interactive command in a running build's container", func() {
			err := ioutil.WriteFile(
				filepath.Join(fixture, "run"),
				[]byte(`#!/bin/bash
mkfifo /tmp/fifo
echo waiting
cat < /tmp/fifo
echo polo > /tmp/fifo
`),
				0755,
			)
			Ω(err).ShouldNot(HaveOccurred())

			fly := exec.Command(builtComponents["fly"], "--", "SOME", "ARGS")
			fly.Dir = fixture

			flyS, err := gexec.Start(fly, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(flyS, 10*time.Second).Should(gbytes.Say("waiting"))

			pty, tty, err := pty.Open()
			Ω(err).ShouldNot(HaveOccurred())

			hijack := exec.Command(builtComponents["fly"], "hijack")
			hijack.Stdin = tty

			hijackS, err := gexec.Start(hijack, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(hijackS, 10*time.Second).Should(gbytes.Say("# "))

			_, err = pty.WriteString("marco\n")
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(flyS).Should(gbytes.Say("marco"))

			Eventually(hijackS).Should(gbytes.Say("# "))

			_, err = pty.WriteString("cat < /tmp/fifo\n")
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(hijackS).Should(gbytes.Say("polo"))

			Eventually(flyS).Should(gexec.Exit(0))

			pty.Close()

			Eventually(hijackS).Should(gexec.Exit(0))
		})
	})
})
