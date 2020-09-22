package auditor_test

import (
	"net/http"

	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/auditor"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Audit", func() {
	var (
		aud                     auditor.Auditor
		dummyAction             string
		userName                string
		logger                  *lagertest.TestLogger
		req                     *http.Request
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

	BeforeEach(func() {
		userName = "test"

		var err error
		req, err = http.NewRequest("GET", "localhost:8080", nil)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		logger = lagertest.NewTestLogger("access_handler")

		aud = auditor.NewAuditor(
			EnableBuildAuditLog,
			EnableContainerAuditLog,
			EnableJobAuditLog,
			EnablePipelineAuditLog,
			EnableResourceAuditLog,
			EnableSystemAuditLog,
			EnableTeamAuditLog,
			EnableWorkerAuditLog,
			EnableVolumeAuditLog,
			logger,
		)
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
	Context("when audit is called", func() {
		BeforeEach(func() {
			EnableBuildAuditLog = true
			EnableContainerAuditLog = true
			EnableJobAuditLog = true
			EnablePipelineAuditLog = true
			EnableResourceAuditLog = true
			EnableSystemAuditLog = true
			EnableTeamAuditLog = true
			EnableWorkerAuditLog = true
			EnableVolumeAuditLog = true
		})
		It("all routes are handled and does not panic", func() {
			for _, route := range atc.Routes {
				aud.Audit(atc.RouteAction(route.Name), userName, req)
			}
			logs := logger.Logs()
			Expect(len(logs)).ToNot(Equal(0))
		})
	})

	Describe("EnableBuildAuditLog", func() {

		Context("When EnableBuildAudit is false with a Build action", func() {
			BeforeEach(func() {
				EnableBuildAuditLog = false
				dummyAction = "GetBuildPlan"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})

		})

		Context("When EnableBuildAudit is true with Build action", func() {
			BeforeEach(func() {
				EnableBuildAuditLog = true
				dummyAction = "GetBuildPlan"
			})

			It("Create a log including the action", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(logs[0].Data["action"]).To(Equal(dummyAction))
			})
		})

		Context("When EnableBuildAudit is true with Non Build action", func() {
			BeforeEach(func() {
				EnableBuildAuditLog = true
				dummyAction = "SaveConfig"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})
		})

		Context("When EnableBuildAudit is false with Non Build action", func() {
			BeforeEach(func() {
				EnableBuildAuditLog = false
				dummyAction = "SaveConfig"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})
		})
	})

	Describe("EnableContainerAuditLog", func() {

		Context("When EnableContainerAuditLog is false with a Container action", func() {
			BeforeEach(func() {
				EnableContainerAuditLog = false
				dummyAction = "GetContainer"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})

		})

		Context("When EnableContainerAuditLog is true with a Container action", func() {
			BeforeEach(func() {
				EnableContainerAuditLog = true
				dummyAction = "GetContainer"
			})

			It("Create a log including the action", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(logs[0].Data["action"]).To(Equal(dummyAction))
			})
		})

		Context("When EnableContainerAuditLog is true with Non Container action", func() {
			BeforeEach(func() {
				EnableContainerAuditLog = true
				dummyAction = "SaveConfig"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})
		})

		Context("When EnableContainerAuditLog is false with Non Container action", func() {
			BeforeEach(func() {
				EnableContainerAuditLog = false
				dummyAction = "SaveConfig"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})
		})
	})

	Describe("EnableJobAuditLog", func() {

		Context("When EnableJobAuditLog is false with a Job action", func() {
			BeforeEach(func() {
				EnableJobAuditLog = false
				dummyAction = "GetJob"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})

		})

		Context("When EnableJobAuditLog is true with a Job action", func() {
			BeforeEach(func() {
				EnableJobAuditLog = true
				dummyAction = "GetJob"
			})

			It("Create a log including the action", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(logs[0].Data["action"]).To(Equal(dummyAction))
			})
		})

		Context("When EnableJobAuditLog is true with Non Job action", func() {
			BeforeEach(func() {
				EnableJobAuditLog = true
				dummyAction = "SaveConfig"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})
		})

		Context("When EnableJobAuditLog is false with Non Job action", func() {
			BeforeEach(func() {
				EnableJobAuditLog = false
				dummyAction = "SaveConfig"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})
		})
	})

	Describe("EnablePipelineAuditLog", func() {

		Context("When EnablePipelineAuditLog is false with a Pipeline action", func() {
			BeforeEach(func() {
				EnablePipelineAuditLog = false
				dummyAction = "GetPipeline"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})

		})

		Context("When EnablePipelineAuditLog is true with a Pipeline action", func() {
			BeforeEach(func() {
				EnablePipelineAuditLog = true
				dummyAction = "GetPipeline"
			})

			It("Create a log including the action", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(logs[0].Data["action"]).To(Equal(dummyAction))
			})
		})

		Context("When EnablePipelineAuditLog is true with Non Pipeline action", func() {
			BeforeEach(func() {
				EnablePipelineAuditLog = true
				dummyAction = "SaveConfig"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})
		})

		Context("When EnablePipelineAuditLog is false with Non Pipeline action", func() {
			BeforeEach(func() {
				EnablePipelineAuditLog = false
				dummyAction = "SaveConfig"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})
		})
	})

	Describe("EnableResourceAuditLog", func() {

		Context("When EnableResourceAuditLog is false with a Resource action", func() {
			BeforeEach(func() {
				EnableResourceAuditLog = false
				dummyAction = "GetResource"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})

		})

		Context("When EnableResourceAuditLog is true with a Resource action", func() {
			BeforeEach(func() {
				EnableResourceAuditLog = true
				dummyAction = "GetResource"
			})

			It("Create a log including the action", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(logs[0].Data["action"]).To(Equal(dummyAction))
			})
		})

		Context("When EnableResourceAuditLog is true with Non Resource action", func() {
			BeforeEach(func() {
				EnableResourceAuditLog = true
				dummyAction = "SaveConfig"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})
		})

		Context("When EnableResourceAuditLog is false with Non Resource action", func() {
			BeforeEach(func() {
				EnableResourceAuditLog = false
				dummyAction = "SaveConfig"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})
		})
	})

	Describe("EnableSystemAuditLog", func() {

		Context("When EnableSystemAuditLog is false with a System action", func() {
			BeforeEach(func() {
				EnableSystemAuditLog = false
				dummyAction = "SaveConfig"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})

		})

		Context("When EnableSystemAuditLog is true with a System action", func() {
			BeforeEach(func() {
				EnableSystemAuditLog = true
				dummyAction = "SaveConfig"
			})

			It("Create a log including the action", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(logs[0].Data["action"]).To(Equal(dummyAction))
			})
		})

		Context("When EnableSystemAuditLog is true with Non System action", func() {
			BeforeEach(func() {
				EnableSystemAuditLog = true
				dummyAction = "GetBuild"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})
		})

		Context("When EnableSystemAuditLog is false with Non System action", func() {
			BeforeEach(func() {
				EnableSystemAuditLog = false
				dummyAction = "GetBuild"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})
		})
	})

	Describe("EnableTeamAuditLog", func() {

		Context("When EnableTeamAuditLog is false with a Resource action", func() {
			BeforeEach(func() {
				EnableTeamAuditLog = false
				dummyAction = "ListTeams"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})

		})

		Context("When EnableTeamAuditLog is true with a Resource action", func() {
			BeforeEach(func() {
				EnableTeamAuditLog = true
				dummyAction = "ListTeams"
			})

			It("Create a log including the action", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(logs[0].Data["action"]).To(Equal(dummyAction))
			})
		})

		Context("When EnableTeamAuditLog is true with Non Resource action", func() {
			BeforeEach(func() {
				EnableTeamAuditLog = true
				dummyAction = "SaveConfig"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})
		})

		Context("When EnableTeamAuditLog is false with Non Resource action", func() {
			BeforeEach(func() {
				EnableTeamAuditLog = false
				dummyAction = "SaveConfig"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})
		})
	})

	Describe("EnableWorkerAuditLog", func() {

		Context("When EnableWorkerAuditLog is false with a Resource action", func() {
			BeforeEach(func() {
				EnableWorkerAuditLog = false
				dummyAction = "ListWorkers"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})

		})

		Context("When EnableWorkerAuditLog is true with a Resource action", func() {
			BeforeEach(func() {
				EnableWorkerAuditLog = true
				dummyAction = "ListWorkers"
			})

			It("Create a log including the action", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(logs[0].Data["action"]).To(Equal(dummyAction))
			})
		})

		Context("When EnableWorkerAuditLog is true with Non Resource action", func() {
			BeforeEach(func() {
				EnableWorkerAuditLog = true
				dummyAction = "SaveConfig"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})
		})

		Context("When EnableWorkerAuditLog is false with Non Resource action", func() {
			BeforeEach(func() {
				EnableWorkerAuditLog = false
				dummyAction = "SaveConfig"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})
		})
	})

	Describe("EnableVolumeAuditLog", func() {

		Context("When EnableVolumeAuditLog is false with a Resource action", func() {
			BeforeEach(func() {
				EnableVolumeAuditLog = false
				dummyAction = "ListVolumes"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})

		})

		Context("When EnableVolumeAuditLog is true with a Resource action", func() {
			BeforeEach(func() {
				EnableVolumeAuditLog = true
				dummyAction = "ListVolumes"
			})

			It("Create a log including the action", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(logs[0].Data["action"]).To(Equal(dummyAction))
			})
		})

		Context("When EnableVolumeAuditLog is true with Non Resource action", func() {
			BeforeEach(func() {
				EnableVolumeAuditLog = true
				dummyAction = "SaveConfig"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})
		})

		Context("When EnableVolumeAuditLog is false with Non Resource action", func() {
			BeforeEach(func() {
				EnableVolumeAuditLog = false
				dummyAction = "SaveConfig"
			})

			It("Doesn't create a log", func() {
				aud.Audit(dummyAction, userName, req)
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(0))
			})
		})
	})
})
