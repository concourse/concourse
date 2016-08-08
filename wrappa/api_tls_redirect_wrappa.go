package wrappa

import (
	"github.com/concourse/atc"
	"github.com/tedsuo/rata"
)

type APITLSRedirectWrappa struct {
	externalHost string
}

func NewAPITLSRedirectWrappa(
	host string,
) *APITLSRedirectWrappa {
	return &APITLSRedirectWrappa{
		externalHost: host,
	}
}

func (wrappa *APITLSRedirectWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	for name, handler := range handlers {
		newHandler := handler

		switch name {
		//wrap everything that is GET or HEAD
		case atc.GetConfig,
			atc.ListBuilds,
			atc.GetBuild,
			atc.GetBuildPlan,
			atc.BuildEvents,
			atc.BuildResources,
			atc.GetBuildPreparation,
			atc.ListJobs,
			atc.GetJob,
			atc.ListJobBuilds,
			atc.ListJobInputs,
			atc.GetJobBuild,
			atc.JobBadge,
			atc.ListPipelines,
			atc.GetPipeline,
			atc.GetVersionsDB,
			atc.ListResources,
			atc.GetResource,
			atc.ListResourceVersions,
			atc.ListBuildsWithVersionAsInput,
			atc.ListBuildsWithVersionAsOutput,
			atc.ListWorkers,
			atc.GetLogLevel,
			atc.DownloadCLI,
			atc.GetInfo,
			atc.ListContainers,
			atc.GetContainer,
			atc.HijackContainer,
			atc.ListVolumes,
			atc.ListAuthMethods,
			atc.GetAuthToken,
			atc.ListAllPipelines,
			atc.ListTeams:
			newHandler = RedirectingAPIHandler(wrappa.externalHost)

			//except ReadPipe
		case atc.ReadPipe,
			atc.CreateBuild,
			atc.AbortBuild,
			atc.CreateJobBuild,
			atc.CheckResource,
			atc.CreatePipe,
			atc.RegisterWorker,
			atc.DeletePipeline,
			atc.SaveConfig,
			atc.PauseJob,
			atc.UnpauseJob,
			atc.OrderPipelines,
			atc.PausePipeline,
			atc.UnpausePipeline,
			atc.RenamePipeline,
			atc.PauseResource,
			atc.UnpauseResource,
			atc.EnableResourceVersion,
			atc.DisableResourceVersion,
			atc.WritePipe,
			atc.SetLogLevel,
			atc.SetTeam,
			atc.ConcealPipeline,
			atc.RevealPipeline:

		default:
			panic("you missed a spot")
		}

		wrapped[name] = newHandler
	}

	return wrapped
}
