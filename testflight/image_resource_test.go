package testflight_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Flying with an image_resource", func() {
	var (
		fixture string
	)

	BeforeEach(func() {
		fixture = filepath.Join(tmp, "fixture")

		err := os.MkdirAll(fixture, 0755)
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

		exec := spawnFlyIn(fixture, "execute", "-c", "task.yml")
		wait(exec, false)
		Expect(exec).To(gbytes.Say("/opt/resource/check"))
		Expect(exec).To(gbytes.Say("VERSION=hello-version"))
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

		exec := spawnFlyIn(fixture, "execute", "-c", "task.yml")
		wait(exec, false)
		Expect(exec).To(gbytes.Say("VERSION=hi-im-a-version"))
	})
})
