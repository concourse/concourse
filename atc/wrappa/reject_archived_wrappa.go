package wrappa

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/pipelineserver"
	"github.com/tedsuo/rata"
)

type RejectArchivedWrappa struct {
	handlerFactory pipelineserver.RejectArchivedHandlerFactory
}

func NewRejectArchivedWrappa(factory pipelineserver.RejectArchivedHandlerFactory) *RejectArchivedWrappa {
	return &RejectArchivedWrappa{
		handlerFactory: factory,
	}
}

func (rw *RejectArchivedWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	for name, handler := range handlers {
		newHandler := handler

		switch name {
		case
			atc.PausePipeline,
			atc.UnpausePipeline,
			atc.CreateJobBuild,
			atc.ScheduleJob,
			atc.CheckResource,
			atc.CheckResourceType,
			atc.DisableResourceVersion,
			atc.EnableResourceVersion,
			atc.PinResourceVersion,
			atc.UnpinResource,
			atc.SetPinCommentOnResource,
			atc.RerunJobBuild:

			newHandler = rw.handlerFactory.RejectArchived(handler)

			// leave the handler as-is
		case
			atc.GetConfig,
			atc.GetBuild,
			atc.BuildResources,
			atc.BuildEvents,
			atc.ListBuildArtifacts,
			atc.GetBuildPreparation,
			atc.GetBuildPlan,
			atc.AbortBuild,
			atc.PruneWorker,
			atc.LandWorker,
			atc.ReportWorkerContainers,
			atc.ReportWorkerVolumes,
			atc.RetireWorker,
			atc.ListDestroyingContainers,
			atc.ListDestroyingVolumes,
			atc.GetPipeline,
			atc.GetJobBuild,
			atc.PipelineBadge,
			atc.JobBadge,
			atc.ListJobs,
			atc.GetJob,
			atc.ListJobBuilds,
			atc.ListPipelineBuilds,
			atc.GetResource,
			atc.ListBuildsWithVersionAsInput,
			atc.ListBuildsWithVersionAsOutput,
			atc.ListResources,
			atc.ListResourceTypes,
			atc.ListResourceVersions,
			atc.GetResourceCausality,
			atc.GetResourceVersion,
			atc.CreateBuild,
			atc.GetContainer,
			atc.HijackContainer,
			atc.ListContainers,
			atc.ListVolumes,
			atc.ListTeamBuilds,
			atc.ListWorkers,
			atc.RegisterWorker,
			atc.HeartbeatWorker,
			atc.DeleteWorker,
			atc.GetTeam,
			atc.SetTeam,
			atc.RenameTeam,
			atc.DestroyTeam,
			atc.GetUser,
			atc.GetInfo,
			atc.GetCheck,
			atc.DownloadCLI,
			atc.CheckResourceWebHook,
			atc.ListAllPipelines,
			atc.ListBuilds,
			atc.ListPipelines,
			atc.ListAllJobs,
			atc.ListAllResources,
			atc.ListTeams,
			atc.GetWall,
			atc.GetLogLevel,
			atc.SetLogLevel,
			atc.GetInfoCreds,
			atc.ListActiveUsersSince,
			atc.SetWall,
			atc.ClearWall,
			atc.DeletePipeline,
			atc.GetCC,
			atc.GetVersionsDB,
			atc.ListJobInputs,
			atc.OrderPipelines,
			atc.PauseJob,
			atc.ArchivePipeline,
			atc.RenamePipeline,
			atc.SaveConfig,
			atc.UnpauseJob,
			atc.ExposePipeline,
			atc.HidePipeline,
			atc.CreatePipelineBuild,
			atc.ClearTaskCache,
			atc.CreateArtifact,
			atc.GetArtifact:

		default:
			panic("how do archived pipelines affect your endpoint?")
		}

		wrapped[name] = newHandler
	}

	return wrapped
}
