package auditor

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Auditor
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
	switch action {
	case atc.GetBuild,
		atc.GetBuildPlan,
		atc.CreateBuild,
		atc.RerunJobBuild,
		atc.SetBuildComment,
		atc.ListBuilds,
		atc.BuildEvents,
		atc.BuildResources,
		atc.AbortBuild,
		atc.GetBuildPreparation,
		atc.ListBuildsWithVersionAsInput,
		atc.ListBuildsWithVersionAsOutput,
		atc.CreateArtifact,
		atc.GetArtifact,
		atc.ListBuildArtifacts:
		return a.EnableBuildAuditLog
	case atc.ListContainers,
		atc.GetContainer,
		atc.HijackContainer,
		atc.ListDestroyingContainers,
		atc.ReportWorkerContainers:
		return a.EnableContainerAuditLog
	case atc.GetJob,
		atc.CreateJobBuild,
		atc.ListAllJobs,
		atc.ListJobs,
		atc.ListJobBuilds,
		atc.ListJobInputs,
		atc.GetJobBuild,
		atc.PauseJob,
		atc.UnpauseJob,
		atc.ScheduleJob,
		atc.JobBadge,
		atc.MainJobBadge:
		return a.EnableJobAuditLog
	case atc.ListAllPipelines,
		atc.ListPipelines,
		atc.GetPipeline,
		atc.DeletePipeline,
		atc.OrderPipelines,
		atc.OrderPipelinesWithinGroup,
		atc.PausePipeline,
		atc.ArchivePipeline,
		atc.UnpausePipeline,
		atc.ExposePipeline,
		atc.HidePipeline,
		atc.RenamePipeline,
		atc.ListPipelineBuilds,
		atc.CreatePipelineBuild,
		atc.PipelineBadge:
		return a.EnablePipelineAuditLog
	case atc.ListAllResources,
		atc.ListResources,
		atc.ListResourceTypes,
		atc.GetResource,
		atc.UnpinResource,
		atc.SetPinCommentOnResource,
		atc.CheckResource,
		atc.CheckResourceWebHook,
		atc.CheckResourceType,
		atc.CheckPrototype,
		atc.ListResourceVersions,
		atc.GetResourceVersion,
		atc.EnableResourceVersion,
		atc.DisableResourceVersion,
		atc.PinResourceVersion,
		atc.ClearResourceCache,
		atc.GetDownstreamResourceCausality,
		atc.GetUpstreamResourceCausality,
		atc.ListSharedForResource,
		atc.ListSharedForResourceType,
		atc.ClearResourceVersions,
		atc.ClearResourceTypeVersions:
		return a.EnableResourceAuditLog
	case
		atc.SaveConfig,
		atc.GetConfig,
		atc.GetCC,
		atc.GetVersionsDB,
		atc.ClearTaskCache,
		atc.SetLogLevel,
		atc.GetLogLevel,
		atc.DownloadCLI,
		atc.GetInfo,
		atc.GetInfoCreds,
		atc.ListActiveUsersSince,
		atc.GetUser,
		atc.GetWall,
		atc.SetWall,
		atc.ClearWall:
		return a.EnableSystemAuditLog
	case atc.ListTeams,
		atc.SetTeam,
		atc.RenameTeam,
		atc.DestroyTeam,
		atc.ListTeamBuilds,
		atc.GetTeam:
		return a.EnableTeamAuditLog
	case atc.RegisterWorker,
		atc.LandWorker,
		atc.RetireWorker,
		atc.PruneWorker,
		atc.HeartbeatWorker,
		atc.ListWorkers,
		atc.DeleteWorker:
		return a.EnableWorkerAuditLog
	case atc.ListVolumes,
		atc.ListDestroyingVolumes,
		atc.ReportWorkerVolumes:
		return a.EnableVolumeAuditLog
	default:
		panic(fmt.Sprintf("unhandled action: %s", action))
	}
}

func (a *auditor) Audit(action string, userName string, r *http.Request) {
	err := r.ParseForm()
	if err == nil && a.ValidateAction(action) {
		a.logger.Info("audit", lager.Data{"action": action, "user": userName, "ip": getRemoteAddrIP(r.RemoteAddr), "parameters": r.Form})
	}
}

func getRemoteAddrIP(remoteAddr string) string {
	remoteIP := remoteAddr
	if strings.Contains(remoteAddr, ":") {
		remoteIP, _, _ = net.SplitHostPort(remoteAddr)
	}
	return remoteIP
}
