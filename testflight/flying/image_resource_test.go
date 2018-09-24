package flying_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/concourse/concourse/testflight/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Flying with an image_resource", func() {
	var (
		tmpdir  string
		fixture string
	)

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
ls /bin
`),
			0755,
		)
		Expect(err).NotTo(HaveOccurred())

	})

	AfterEach(func() {
		os.RemoveAll(tmpdir)
	})

	It("propagates the rootfs and metadata to the task", func() {
		err := ioutil.WriteFile(
			filepath.Join(fixture, "task.yml"),
			[]byte(`---
platform: linux

image_resource:
  type: mock
  source:
    mirror_self: true
    initial_version: hello-version

inputs:
- name: fixture

run:
  path: sh
  args:
  - -c
  - |
    ls /opt/resource/check
    env
`),
			0644,
		)
		Expect(err).NotTo(HaveOccurred())
		fly := exec.Command(flyBin, "-t", targetedConcourse, "execute", "-c", "task.yml")
		fly.Dir = fixture

		session := helpers.StartFly(fly)
		<-session.Exited
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session).To(gbytes.Say("/opt/resource/check"))
		Expect(session).To(gbytes.Say("VERSION=hello-version"))
	})

	It("allows a version to be specified", func() {
		err := ioutil.WriteFile(
			filepath.Join(fixture, "task.yml"),
			[]byte(`---
platform: linux

image_resource:
  type: mock
  source: {mirror_self: true}
  version: {version: hi-im-a-version}

inputs:
- name: fixture

run:
  path: env
`),
			0644,
		)
		Expect(err).NotTo(HaveOccurred())

		fly := exec.Command(flyBin, "-t", targetedConcourse, "execute", "-c", "task.yml")
		fly.Dir = fixture
		session := helpers.StartFly(fly)
		<-session.Exited
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session).To(gbytes.Say("VERSION=hi-im-a-version"))
	})
})
