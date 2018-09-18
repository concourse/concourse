package wrappa

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/api/auth"
	"github.com/tedsuo/rata"
)

type APIAuthWrappa struct {
	checkPipelineAccessHandlerFactory   auth.CheckPipelineAccessHandlerFactory
	checkBuildReadAccessHandlerFactory  auth.CheckBuildReadAccessHandlerFactory
	checkBuildWriteAccessHandlerFactory auth.CheckBuildWriteAccessHandlerFactory
	checkWorkerTeamAccessHandlerFactory auth.CheckWorkerTeamAccessHandlerFactory
}

func NewAPIAuthWrappa(
	checkPipelineAccessHandlerFactory auth.CheckPipelineAccessHandlerFactory,
	checkBuildReadAccessHandlerFactory auth.CheckBuildReadAccessHandlerFactory,
	checkBuildWriteAccessHandlerFactory auth.CheckBuildWriteAccessHandlerFactory,
	checkWorkerTeamAccessHandlerFactory auth.CheckWorkerTeamAccessHandlerFactory,
) *APIAuthWrappa {
	return &APIAuthWrappa{
		checkPipelineAccessHandlerFactory:   checkPipelineAccessHandlerFactory,
		checkBuildReadAccessHandlerFactory:  checkBuildReadAccessHandlerFactory,
		checkBuildWriteAccessHandlerFactory: checkBuildWriteAccessHandlerFactory,
		checkWorkerTeamAccessHandlerFactory: checkWorkerTeamAccessHandlerFactory,
	}
}

func (wrappa *APIAuthWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	rejector := auth.UnauthorizedRejector{}

	for name, handler := range handlers {
		newHandler := handler

		switch name {
		// unauthenticated / delegating to handler
		case atc.DownloadCLI,
			atc.CheckResourceWebHook,
			atc.GetInfo,
			atc.ListTeams,
			atc.ListAllPipelines,
			atc.ListPipelines,
			atc.ListAllJobs,
			atc.ListAllResources,
			atc.ListBuilds,
			atc.MainJobBadge:

		// pipeline is public or authorized
		case atc.GetBuild,
			atc.BuildResources,
			atc.GetBuildPlan:
			newHandler = wrappa.checkBuildReadAccessHandlerFactory.AnyJobHandler(handler, rejector)

		// pipeline and job are public or authorized
		case atc.GetBuildPreparation,
			atc.BuildEvents:
			newHandler = wrappa.checkBuildReadAccessHandlerFactory.CheckIfPrivateJobHandler(handler, rejector)

		// resource belongs to authorized team
		case atc.AbortBuild,
			atc.SendInputToBuildPlan,
			atc.ReadOutputFromBuildPlan:
			newHandler = wrappa.checkBuildWriteAccessHandlerFactory.HandlerFor(handler, rejector)

		// requester is system, admin team, or worker owning team
		case atc.PruneWorker,
			atc.LandWorker,
			atc.RetireWorker,
			atc.ListDestroyingVolumes,
			atc.ListDestroyingContainers,
			atc.ReportWorkerContainers,
			atc.ReportWorkerVolumes:
			newHandler = wrappa.checkWorkerTeamAccessHandlerFactory.HandlerFor(handler, rejector)

		// pipeline is public or authorized
		case atc.GetPipeline,
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
			atc.GetResourceCausality,
			atc.GetResourceVersion,
			atc.ListResources,
			atc.ListResourceTypes,
			atc.ListResourceVersions:
			newHandler = wrappa.checkPipelineAccessHandlerFactory.HandlerFor(handler, rejector)

		// authenticated
		case atc.CreateBuild,
			atc.GetContainer,
			atc.HijackContainer,
			atc.ListContainers,
			atc.ListWorkers,
			atc.RegisterWorker,
			atc.HeartbeatWorker,
			atc.DeleteWorker,
			atc.SetTeam,
			atc.ListTeamBuilds,
			atc.RenameTeam,
			atc.DestroyTeam,
			atc.ListVolumes:
			newHandler = auth.CheckAuthenticationHandler(handler, rejector)

		case atc.GetLogLevel,
			atc.SetLogLevel,
			atc.GetInfoCreds:
			newHandler = auth.CheckAdminHandler(handler, rejector)

		// authorized (requested team matches resource team)
		case atc.CheckResource,
			atc.CheckResourceType,
			atc.CreateJobBuild,
			atc.CreatePipelineBuild,
			atc.DeletePipeline,
			atc.DisableResourceVersion,
			atc.EnableResourceVersion,
			atc.GetConfig,
			atc.GetVersionsDB,
			atc.ListJobInputs,
			atc.OrderPipelines,
			atc.PauseJob,
			atc.PausePipeline,
			atc.PauseResource,
			atc.RenamePipeline,
			atc.UnpauseJob,
			atc.UnpausePipeline,
			atc.UnpauseResource,
			atc.ExposePipeline,
			atc.HidePipeline,
			atc.SaveConfig,
			atc.ClearTaskCache:
			newHandler = auth.CheckAuthorizationHandler(handler, rejector)

		// think about it!
		default:
			panic("you missed a spot")
		}

		wrapped[name] = auth.CSRFValidationHandler(newHandler, rejector)
	}

	return wrapped
}
