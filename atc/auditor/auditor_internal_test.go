package auditor

import (
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Audit", func() {
	var (
		aud                     *auditor
		dummyAction             string
		EnableBuildAuditLog     bool
		EnableContainerAuditLog bool
		EnableJobAuditLog       bool
		EnablePipelineAuditLog  bool
		EnableResourceAuditLog  bool
		EnableSystemAuditLog    bool
		EnableTeamAuditLog      bool
		EnableWorkerAuditLog    bool
		EnableVolumeAuditLog    bool
	)

	JustBeforeEach(func() {
		aud = &auditor{
			EnableBuildAuditLog,
			EnableContainerAuditLog,
			EnableJobAuditLog,
			EnablePipelineAuditLog,
			EnableResourceAuditLog,
			EnableSystemAuditLog,
			EnableTeamAuditLog,
			EnableWorkerAuditLog,
			EnableVolumeAuditLog,
			lagertest.NewTestLogger("access_handler"),
		}

		aud.ValidateAction("GetBuildPlan")

	})

	AfterEach(func() {
		EnableBuildAuditLog = false
		EnableContainerAuditLog = false
		EnableJobAuditLog = false
		EnablePipelineAuditLog = false
		EnableResourceAuditLog = false
		EnableSystemAuditLog = false
		EnableTeamAuditLog = false
		EnableWorkerAuditLog = false
		EnableVolumeAuditLog = false
	})

	Describe("EnableBuildAuditLog", func() {

		Context("When EnableBuildAudit is false with a Build action", func() {
			BeforeEach(func() {
				EnableBuildAuditLog = false
				dummyAction = "GetBuildPlan"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})

		Context("When EnableBuildAudit is true with Build action", func() {
			BeforeEach(func() {
				EnableBuildAuditLog = true
				dummyAction = "GetBuildPlan"
			})

			It("returns true on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeTrue())
			})
		})

		Context("When EnableBuildAudit is true with Non Build action", func() {
			BeforeEach(func() {
				EnableBuildAuditLog = true
				dummyAction = "SaveConfig"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})

		Context("When EnableBuildAudit is false with Non Build action", func() {
			BeforeEach(func() {
				EnableBuildAuditLog = false
				dummyAction = "SaveConfig"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})
	})


	Describe("EnableContainerAuditLog", func() {

		Context("When EnableContainerAuditLog is false with a Container action", func() {
			BeforeEach(func() {
				EnableContainerAuditLog = false
				dummyAction = "GetContainer"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})

		Context("When EnableContainerAuditLog is true with a Container action", func() {
			BeforeEach(func() {
				EnableContainerAuditLog = true
				dummyAction = "GetContainer"
			})

			It("returns true on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeTrue())
			})
		})

		Context("When EnableContainerAuditLog is true with Non Container action", func() {
			BeforeEach(func() {
				EnableContainerAuditLog = true
				dummyAction = "SaveConfig"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})

		Context("When EnableContainerAuditLog is false with Non Container action", func() {
			BeforeEach(func() {
				EnableContainerAuditLog = false
				dummyAction = "SaveConfig"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})
	})

	Describe("EnableJobAuditLog", func() {

		Context("When EnableJobAuditLog is false with a Job action", func() {
			BeforeEach(func() {
				EnableJobAuditLog = false
				dummyAction = "GetJob"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})

		Context("When EnableJobAuditLog is true with a Job action", func() {
			BeforeEach(func() {
				EnableJobAuditLog = true
				dummyAction = "GetJob"
			})

			It("returns true on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeTrue())
			})
		})

		Context("When EnableJobAuditLog is true with Non Job action", func() {
			BeforeEach(func() {
				EnableJobAuditLog = true
				dummyAction = "SaveConfig"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})

		Context("When EnableJobAuditLog is false with Non Job action", func() {
			BeforeEach(func() {
				EnableJobAuditLog = false
				dummyAction = "SaveConfig"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})
	})


	Describe("EnablePipelineAuditLog", func() {

		Context("When EnablePipelineAuditLog is false with a Pipeline action", func() {
			BeforeEach(func() {
				EnablePipelineAuditLog = false
				dummyAction = "GetPipeline"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})

		Context("When EnablePipelineAuditLog is true with a Pipeline action", func() {
			BeforeEach(func() {
				EnablePipelineAuditLog = true
				dummyAction = "GetPipeline"
			})

			It("returns true on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeTrue())
			})
		})

		Context("When EnablePipelineAuditLog is true with Non Pipeline action", func() {
			BeforeEach(func() {
				EnablePipelineAuditLog = true
				dummyAction = "SaveConfig"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})

		Context("When EnablePipelineAuditLog is false with Non Pipeline action", func() {
			BeforeEach(func() {
				EnablePipelineAuditLog = false
				dummyAction = "SaveConfig"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})
	})



	Describe("EnableResourceAuditLog", func() {

		Context("When EnableResourceAuditLog is false with a Resource action", func() {
			BeforeEach(func() {
				EnableResourceAuditLog = false
				dummyAction = "GetResource"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})

		Context("When EnableResourceAuditLog is true with a Resource action", func() {
			BeforeEach(func() {
				EnableResourceAuditLog = true
				dummyAction = "GetResource"
			})

			It("returns true on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeTrue())
			})
		})

		Context("When EnableResourceAuditLog is true with Non Resource action", func() {
			BeforeEach(func() {
				EnableResourceAuditLog = true
				dummyAction = "SaveConfig"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})

		Context("When EnableResourceAuditLog is false with Non Resource action", func() {
			BeforeEach(func() {
				EnableResourceAuditLog = false
				dummyAction = "SaveConfig"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})
	})



	Describe("EnableSystemAuditLog", func() {

		Context("When EnableSystemAuditLog is false with a System action", func() {
			BeforeEach(func() {
				EnableSystemAuditLog = false
				dummyAction = "SaveConfig"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})

		Context("When EnableSystemAuditLog is true with a System action", func() {
			BeforeEach(func() {
				EnableSystemAuditLog = true
				dummyAction = "SaveConfig"
			})

			It("returns true on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeTrue())
			})
		})

		Context("When EnableSystemAuditLog is true with Non System action", func() {
			BeforeEach(func() {
				EnableSystemAuditLog = true
				dummyAction = "GetBuild"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})

		Context("When EnableSystemAuditLog is false with Non System action", func() {
			BeforeEach(func() {
				EnableSystemAuditLog = false
				dummyAction = "GetBuild"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})
	})


	Describe("EnableTeamAuditLog", func() {

		Context("When EnableTeamAuditLog is false with a Resource action", func() {
			BeforeEach(func() {
				EnableTeamAuditLog = false
				dummyAction = "ListTeams"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})

		Context("When EnableTeamAuditLog is true with a Resource action", func() {
			BeforeEach(func() {
				EnableTeamAuditLog = true
				dummyAction = "ListTeams"
			})

			It("returns true on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeTrue())
			})
		})

		Context("When EnableTeamAuditLog is true with Non Resource action", func() {
			BeforeEach(func() {
				EnableTeamAuditLog = true
				dummyAction = "SaveConfig"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})

		Context("When EnableTeamAuditLog is false with Non Resource action", func() {
			BeforeEach(func() {
				EnableTeamAuditLog = false
				dummyAction = "SaveConfig"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})
	})

	Describe("EnableWorkerAuditLog", func() {

		Context("When EnableWorkerAuditLog is false with a Resource action", func() {
			BeforeEach(func() {
				EnableWorkerAuditLog = false
				dummyAction = "ListWorkers"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})

		Context("When EnableWorkerAuditLog is true with a Resource action", func() {
			BeforeEach(func() {
				EnableWorkerAuditLog = true
				dummyAction = "ListWorkers"
			})

			It("returns true on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeTrue())
			})
		})

		Context("When EnableWorkerAuditLog is true with Non Resource action", func() {
			BeforeEach(func() {
				EnableWorkerAuditLog = true
				dummyAction = "SaveConfig"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})

		Context("When EnableWorkerAuditLog is false with Non Resource action", func() {
			BeforeEach(func() {
				EnableWorkerAuditLog = false
				dummyAction = "SaveConfig"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})
	})

	Describe("EnableVolumeAuditLog", func() {

		Context("When EnableVolumeAuditLog is false with a Resource action", func() {
			BeforeEach(func() {
				EnableVolumeAuditLog = false
				dummyAction = "ListVolumes"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})

		Context("When EnableVolumeAuditLog is true with a Resource action", func() {
			BeforeEach(func() {
				EnableVolumeAuditLog = true
				dummyAction = "ListVolumes"
			})

			It("returns true on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeTrue())
			})
		})

		Context("When EnableVolumeAuditLog is true with Non Resource action", func() {
			BeforeEach(func() {
				EnableVolumeAuditLog = true
				dummyAction = "SaveConfig"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})

		Context("When EnableVolumeAuditLog is false with Non Resource action", func() {
			BeforeEach(func() {
				EnableVolumeAuditLog = false
				dummyAction = "SaveConfig"
			})

			It("returns false on Validation", func() {
				Expect(aud.ValidateAction(dummyAction)).To(BeFalse())
			})
		})
	})
})
