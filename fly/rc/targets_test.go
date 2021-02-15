package rc_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/rc"
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

		os.Setenv("HOME", tmpDir)

		flyrc = filepath.Join(userHomeDir(), ".flyrc")
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("LoadTargets", func() {
		Context("when config file does not have team", func() {
			BeforeEach(func() {
				flyrcContents := `targets:
  some-target:
    api: http://concourse.com
    token:
      type: Bearer
      value: some-token`
				ioutil.WriteFile(flyrc, []byte(flyrcContents), 0777)
			})

			It("loads target with default team", func() {
				targets, err := rc.LoadTargets()
				Expect(err).ToNot(HaveOccurred())
				Expect(targets).To(Equal(rc.Targets{
					"some-target": {
						API:      "http://concourse.com",
						TeamName: atc.DefaultTeamName,
						Token: &rc.TargetToken{
							Type:  "Bearer",
							Value: "some-token",
						},
					},
				}))
			})
		})
	})

	Describe("Deleting Target", func() {
		BeforeEach(func() {
			flyrcContents := `targets:
  target-name:
    api: http://concourse.com
    team: some-team
    token:
      type: Bearer
      value: some-token
  new-target:
    api: some-api
    team: another-team
    token:
      type: Bearer
      value: some-other-token`
			ioutil.WriteFile(flyrc, []byte(flyrcContents), 0777)
		})
		Describe("DeleteTarget", func() {
			Context("when provided with target name to delete", func() {
				BeforeEach(func() {
					err := rc.DeleteTarget("target-name")
					Expect(err).ToNot(HaveOccurred())
				})
				It("should delete target from flyrc", func() {
					returnedTargets, err := rc.LoadTargets()
					Expect(err).ToNot(HaveOccurred())
					Expect(returnedTargets).To(Equal(rc.Targets{
						"new-target": {
							API:      "some-api",
							TeamName: "another-team",
							Token: &rc.TargetToken{
								Type:  "Bearer",
								Value: "some-other-token",
							},
						}}))
				})
			})
		})
		Describe("DeleteAllTargets", func() {
			Context("when deleting all targets", func() {
				BeforeEach(func() {
					err := rc.DeleteAllTargets()
					Expect(err).ToNot(HaveOccurred())
				})
				It("should delete all targets from flyrc", func() {
					returnedTargets, err := rc.LoadTargets()
					Expect(err).ToNot(HaveOccurred())
					Expect(returnedTargets).To(Equal(rc.Targets{}))
				})
			})
		})
	})

	Describe("UpdateTarget", func() {
		BeforeEach(func() {
			flyrcContents := `targets:
  some-target:
    api: http://concourse.com
    team: main
    token:
      type: Bearer
      value: some-token`
			ioutil.WriteFile(flyrc, []byte(flyrcContents), 0777)
		})
		Context("when props are provided for update", func() {
			It("should update target to specified prop attributes", func() {
				targetProps := rc.TargetProps{
					API:      "new-api",
					TeamName: "other-team",
				}
				err := rc.UpdateTargetProps("some-target", targetProps)
				Expect(err).ToNot(HaveOccurred())

				targets, err := rc.LoadTargets()
				Expect(err).ToNot(HaveOccurred())
				Expect(targets).To(Equal(rc.Targets{
					"some-target": {
						API:      "new-api",
						TeamName: "other-team",
						Token: &rc.TargetToken{
							Type:  "Bearer",
							Value: "some-token",
						},
					},
				}))
			})
		})

		Context("when target name is provided for update", func() {
			It("should update target name and keep old prop attributes", func() {
				err := rc.UpdateTargetName("some-target", "some-other-target")
				Expect(err).ToNot(HaveOccurred())

				targets, err := rc.LoadTargets()
				Expect(err).ToNot(HaveOccurred())
				Expect(targets).To(Equal(rc.Targets{
					"some-other-target": {
						API:      "http://concourse.com",
						TeamName: "main",
						Token: &rc.TargetToken{
							Type:  "Bearer",
							Value: "some-token",
						},
					},
				}))
			})
		})
	})

	Describe("SaveTarget", func() {
		Context("when managing .flyrc", func() {
			BeforeEach(func() {
				if runtime.GOOS == "windows" {
					Skip("these tests are UNIX-specific")
				}
			})

			It("creates any new file with 0600 permissions", func() {
				err := rc.SaveTarget("foo", "url", false, "main", nil, "", "", "")
				Expect(err).ToNot(HaveOccurred())
				fi, statErr := os.Stat(flyrc)
				Expect(statErr).To(BeNil())
				Expect(fi.Mode().Perm()).To(Equal(os.FileMode(0600)))
			})

			Describe("when the file exists with 0755 permissions", func() {
				BeforeEach(func() {
					err := ioutil.WriteFile(flyrc, []byte{}, 0755)
					Expect(err).ToNot(HaveOccurred())
				})

				It("preserves those permissions", func() {
					err := rc.SaveTarget("foo", "url", false, "main", nil, "", "", "")
					Expect(err).ToNot(HaveOccurred())
					fi, statErr := os.Stat(flyrc)
					Expect(statErr).To(BeNil())
					Expect(fi.Mode().Perm()).To(Equal(os.FileMode(0755)))
				})
			})
		})

		Describe("CA Cert Flag", func() {
			Describe("when 'ca_cert' is not set in the flyrc", func() {
				var targetName rc.TargetName
				BeforeEach(func() {
					targetName = "foo"
					err := rc.SaveTarget(
						targetName,
						"some api url",
						false,
						"main",
						nil,
						"",
						"",
						"",
					)
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns the rc empty ca-cert", func() {
					returnedTarget, err := rc.LoadTarget(targetName, false)
					Expect(err).ToNot(HaveOccurred())
					Expect(returnedTarget.CACert()).To(BeEmpty())
				})
			})

			Describe("when 'ca_cert' is set in the flyrc", func() {
				var targetName rc.TargetName

				BeforeEach(func() {
					targetName = "foo"
					err := rc.SaveTarget(
						targetName,
						"some api url",
						true,
						"main",
						nil,
						rsaCertPEM,
						"",
						"",
					)
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns the rc insecure flag as true", func() {
					returnedTarget, err := rc.LoadTarget(targetName, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(returnedTarget.CACert()).To(Equal(rsaCertPEM))
				})
			})
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
						"main",
						nil,
						"",
						"",
						"",
					)
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns the rc insecure flag as false", func() {
					returnedTarget, err := rc.LoadTarget(targetName, false)
					Expect(err).ToNot(HaveOccurred())
					Expect(returnedTarget.TLSConfig().InsecureSkipVerify).To(BeFalse())
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
						"main",
						nil,
						"",
						"",
						"",
					)
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns the rc insecure flag as true", func() {
					returnedTarget, err := rc.LoadTarget(targetName, false)
					Expect(err).ToNot(HaveOccurred())
					Expect(returnedTarget.TLSConfig().InsecureSkipVerify).To(BeTrue())
				})
			})
		})

		Context("when selecting a target that does not exist", func() {
			It("returns UnknownTargetError", func() {
				_, err := rc.LoadTarget("bogus", false)
				Expect(err).To(Equal(rc.UnknownTargetError{"bogus"}))
			})
		})

		Context("when a target is not specified", func() {
			It("returns ErrNoTargetSpecified", func() {
				_, err := rc.LoadTarget("", false)
				Expect(err).To(Equal(rc.ErrNoTargetSpecified))
			})
		})
	})
})
