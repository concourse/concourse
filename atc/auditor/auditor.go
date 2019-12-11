package auditor

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
)

//go:generate counterfeiter . Auditor

func NewAuditor(
	EnableBuildAuditLog bool,
	EnableContainerAuditLog bool,
	EnableJobAuditLog bool,
	EnablePipelineAuditLog bool,
	EnableResourceAuditLog bool,
	EnableSystemAuditLog bool,
	EnableTeamAuditLog bool,
	EnableWorkerAuditLog bool,
	EnableVolumeAuditLog bool,
	logger lager.Logger,
) *auditor {
	return &auditor{
		EnableBuildAuditLog:     EnableBuildAuditLog,
		EnableContainerAuditLog: EnableContainerAuditLog,
		EnableJobAuditLog:       EnableJobAuditLog,
		EnablePipelineAuditLog:  EnablePipelineAuditLog,
		EnableResourceAuditLog:  EnableResourceAuditLog,
		EnableSystemAuditLog:    EnableSystemAuditLog,
		EnableTeamAuditLog:      EnableTeamAuditLog,
		EnableWorkerAuditLog:    EnableWorkerAuditLog,
		EnableVolumeAuditLog:    EnableVolumeAuditLog,
		logger:                  logger,
	}
}

type Auditor interface {
	Audit(action string, userName string, r *http.Request)
}

type auditor struct {
	EnableBuildAuditLog     bool
	EnableContainerAuditLog bool
	EnableJobAuditLog       bool
	EnablePipelineAuditLog  bool
	EnableResourceAuditLog  bool
	EnableSystemAuditLog    bool
	EnableTeamAuditLog      bool
	EnableWorkerAuditLog    bool
	EnableVolumeAuditLog    bool
	logger                  lager.Logger
}

func (a *auditor) ValidateAction(action string) bool {
	switch loggingLevels[action] {
	case "EnableBuildAuditLog":
		return a.EnableBuildAuditLog
	case "EnableContainerAuditLog":
		return a.EnableContainerAuditLog
	case "EnableJobAuditLog":
		return a.EnableJobAuditLog
	case "EnablePipelineAuditLog":
		return a.EnablePipelineAuditLog
	case "EnableResourceAuditLog":
		return a.EnableResourceAuditLog
	case "EnableSystemAuditLog":
		return a.EnableSystemAuditLog
	case "EnableTeamAuditLog":
		return a.EnableTeamAuditLog
	case "EnableWorkerAuditLog":
		return a.EnableWorkerAuditLog
	case "EnableVolumeAuditLog":
		return a.EnableVolumeAuditLog
	default:
		return false
	}
}

func (a *auditor) Audit(action string, userName string, r *http.Request) {
	err := r.ParseForm()
	if err == nil && a.ValidateAction(action) {
		a.logger.Info("audit", lager.Data{"action": action, "user": userName, "parameters": r.Form})
	}
}

var loggingLevels = map[string]string{
	atc.SaveConfig:                    "EnableSystemAuditLog",
	atc.GetConfig:                     "EnableSystemAuditLog",
	atc.GetCC:                         "EnableSystemAuditLog",
	atc.GetBuild:                      "EnableBuildAuditLog",
	atc.GetBuildPlan:                  "EnableBuildAuditLog",
	atc.CreateBuild:                   "EnableBuildAuditLog",
	atc.ListBuilds:                    "EnableBuildAuditLog",
	atc.BuildEvents:                   "EnableBuildAuditLog",
	atc.BuildResources:                "EnableBuildAuditLog",
	atc.AbortBuild:                    "EnableBuildAuditLog",
	atc.GetBuildPreparation:           "EnableBuildAuditLog",
	atc.GetJob:                        "EnableJobAuditLog",
	atc.CreateJobBuild:                "EnableJobAuditLog",
	atc.ListAllJobs:                   "EnableJobAuditLog",
	atc.ListJobs:                      "EnableJobAuditLog",
	atc.ListJobBuilds:                 "EnableJobAuditLog",
	atc.ListJobInputs:                 "EnableJobAuditLog",
	atc.GetJobBuild:                   "EnableJobAuditLog",
	atc.PauseJob:                      "EnableJobAuditLog",
	atc.UnpauseJob:                    "EnableJobAuditLog",
	atc.ScheduleJob:                   "EnableJobAuditLog",
	atc.GetVersionsDB:                 "EnableSystemAuditLog",
	atc.JobBadge:                      "EnableJobAuditLog",
	atc.MainJobBadge:                  "EnableJobAuditLog",
	atc.ClearTaskCache:                "EnableSystemAuditLog",
	atc.ListAllResources:              "EnableResourceAuditLog",
	atc.ListResources:                 "EnableResourceAuditLog",
	atc.ListResourceTypes:             "EnableResourceAuditLog",
	atc.GetResource:                   "EnableResourceAuditLog",
	atc.UnpinResource:                 "EnableResourceAuditLog",
	atc.SetPinCommentOnResource:       "EnableResourceAuditLog",
	atc.CheckResource:                 "EnableResourceAuditLog",
	atc.CheckResourceWebHook:          "EnableResourceAuditLog",
	atc.CheckResourceType:             "EnableResourceAuditLog",
	atc.ListResourceVersions:          "EnableResourceAuditLog",
	atc.GetResourceVersion:            "EnableResourceAuditLog",
	atc.EnableResourceVersion:         "EnableResourceAuditLog",
	atc.DisableResourceVersion:        "EnableResourceAuditLog",
	atc.PinResourceVersion:            "EnableResourceAuditLog",
	atc.ListBuildsWithVersionAsInput:  "EnableBuildAuditLog",
	atc.ListBuildsWithVersionAsOutput: "EnableBuildAuditLog",
	atc.GetResourceCausality:          "EnableResourceAuditLog",
	atc.ListAllPipelines:              "EnablePipelineAuditLog",
	atc.ListPipelines:                 "EnablePipelineAuditLog",
	atc.GetPipeline:                   "EnablePipelineAuditLog",
	atc.DeletePipeline:                "EnablePipelineAuditLog",
	atc.OrderPipelines:                "EnablePipelineAuditLog",
	atc.PausePipeline:                 "EnablePipelineAuditLog",
	atc.UnpausePipeline:               "EnablePipelineAuditLog",
	atc.ExposePipeline:                "EnablePipelineAuditLog",
	atc.HidePipeline:                  "EnablePipelineAuditLog",
	atc.RenamePipeline:                "EnablePipelineAuditLog",
	atc.ListPipelineBuilds:            "EnablePipelineAuditLog",
	atc.CreatePipelineBuild:           "EnablePipelineAuditLog",
	atc.PipelineBadge:                 "EnablePipelineAuditLog",
	atc.RegisterWorker:                "EnableWorkerAuditLog",
	atc.LandWorker:                    "EnableWorkerAuditLog",
	atc.RetireWorker:                  "EnableWorkerAuditLog",
	atc.PruneWorker:                   "EnableWorkerAuditLog",
	atc.HeartbeatWorker:               "EnableWorkerAuditLog",
	atc.ListWorkers:                   "EnableWorkerAuditLog",
	atc.DeleteWorker:                  "EnableWorkerAuditLog",
	atc.SetLogLevel:                   "EnableSystemAuditLog",
	atc.GetLogLevel:                   "EnableSystemAuditLog",
	atc.DownloadCLI:                   "EnableSystemAuditLog",
	atc.GetInfo:                       "EnableSystemAuditLog",
	atc.GetInfoCreds:                  "EnableSystemAuditLog",
	atc.ListContainers:                "EnableContainerAuditLog",
	atc.GetContainer:                  "EnableContainerAuditLog",
	atc.HijackContainer:               "EnableContainerAuditLog",
	atc.ListDestroyingContainers:      "EnableContainerAuditLog",
	atc.ReportWorkerContainers:        "EnableContainerAuditLog",
	atc.ListVolumes:                   "EnableVolumeAuditLog",
	atc.ListDestroyingVolumes:         "EnableVolumeAuditLog",
	atc.ReportWorkerVolumes:           "EnableVolumeAuditLog",
	atc.ListTeams:                     "EnableTeamAuditLog",
	atc.SetTeam:                       "EnableTeamAuditLog",
	atc.RenameTeam:                    "EnableTeamAuditLog",
	atc.DestroyTeam:                   "EnableTeamAuditLog",
	atc.ListTeamBuilds:                "EnableTeamAuditLog",
	atc.CreateArtifact:                "EnableBuildAuditLog",
	atc.GetArtifact:                   "EnableBuildAuditLog",
	atc.ListBuildArtifacts:            "EnableBuildAuditLog",
}
