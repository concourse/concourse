package rc_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/concourse/fly/rc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Targets", func() {
	var tmpDir string
	var flyrc string

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "fly-test")
		Expect(err).NotTo(HaveOccurred())

		os.Setenv("HOME", tmpDir)
		os.Setenv("HOMEPATH", tmpDir)
		os.Unsetenv("HOMEDRIVE")

		flyrc = filepath.Join(userHomeDir(), ".flyrc")
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)

		os.Unsetenv("HOME")
		os.Unsetenv("HOMEPATH")
	})

	Describe("Insecure Flag", func() {
		Describe("When 'insecure' is set to false in the flyrc", func() {
			var targetName string
			BeforeEach(func() {
				targetName = "foo"
				rc.CreateOrUpdateTargets(
					targetName,
					rc.NewTarget("Don't matter", "Don't matter", "Don't matter", "Don't matter", false),
				)
			})

			It("the global insecure flag overrides the return value", func() {
				returnedTarget, err := rc.SelectTarget(targetName, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(returnedTarget.Insecure).To(BeTrue())
			})
		})

		Describe("When 'insecure' is set to true in the flyrc", func() {
			var targetName string
			BeforeEach(func() {
				targetName = "foo"
				rc.CreateOrUpdateTargets(
					targetName,
					rc.NewTarget("Don't matter", "Don't matter", "Don't matter", "Don't matter", true),
				)
			})

			It("the rc insecure flag value is returned", func() {
				returnedTarget, err := rc.SelectTarget(targetName, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(returnedTarget.Insecure).To(BeTrue())
			})
		})

		Describe("When the target does not exist in the flyrc", func() {
			var targetName string
			BeforeEach(func() {
				targetName = "https://foo.com"
			})

			It("and the insecure flag is not passed, insecure is set to false", func() {
				returnedTarget, err := rc.SelectTarget(targetName, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(returnedTarget.Insecure).To(BeFalse())
			})

			It("and the insecure flag is passed, insecure is set to true", func() {
				returnedTarget, err := rc.SelectTarget(targetName, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(returnedTarget.Insecure).To(BeTrue())
			})
		})
	})
})
