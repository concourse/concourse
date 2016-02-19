package rc_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

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
		Expect(err).ToNot(HaveOccurred())

		if runtime.GOOS == "windows" {
			os.Setenv("USERPROFILE", tmpDir)
		} else {
			os.Setenv("HOME", tmpDir)
		}

		flyrc = filepath.Join(userHomeDir(), ".flyrc")
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("Insecure Flag", func() {
		Describe("when 'insecure' is set to false in the flyrc", func() {
			var targetName rc.TargetName

			BeforeEach(func() {
				targetName = "foo"
				err := rc.SaveTarget(
					targetName,
					"some api url",
					false,
					nil,
				)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the rc insecure flag as false", func() {
				returnedTarget, err := rc.SelectTarget(targetName)
				Expect(err).ToNot(HaveOccurred())
				Expect(returnedTarget.Insecure).To(BeFalse())
			})
		})

		Describe("when 'insecure' is set to true in the flyrc", func() {
			var targetName rc.TargetName

			BeforeEach(func() {
				targetName = "foo"
				err := rc.SaveTarget(
					targetName,
					"some api url",
					true,
					nil,
				)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the rc insecure flag as true", func() {
				returnedTarget, err := rc.SelectTarget(targetName)
				Expect(err).NotTo(HaveOccurred())
				Expect(returnedTarget.Insecure).To(BeTrue())
			})
		})
	})

	Context("when selecting a target that does not exist", func() {
		It("returns UnknownTargetError", func() {
			_, err := rc.SelectTarget("bogus")
			Expect(err).To(Equal(rc.UnknownTargetError{"bogus"}))
		})
	})

	Context("when a target is not specified", func() {
		It("returns ErrNoTargetSpecified", func() {
			_, err := rc.SelectTarget("")
			Expect(err).To(Equal(rc.ErrNoTargetSpecified))
		})
	})
})
