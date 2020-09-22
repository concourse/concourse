package wrappa

import (
	"context"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/db"
	"github.com/tedsuo/rata"
)

type FetchPipelineWrappa struct {
	TeamFactory db.TeamFactory
}

func (wrappa FetchPipelineWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	for name, handler := range handlers {
		newHandler := handler

		switch atc.RouteAction(name) {
		case atc.GetConfig,
			atc.ListJobs,
			atc.GetJob,
			atc.ListJobBuilds,
			atc.CreateJobBuild,
			atc.RerunJobBuild,
			atc.ListJobInputs,
			atc.GetJobBuild,
			atc.PauseJob,
			atc.UnpauseJob,
			atc.ScheduleJob,
			atc.JobBadge,
			atc.ClearTaskCache,
			atc.GetPipeline,
			atc.DeletePipeline,
			atc.PausePipeline,
			atc.ArchivePipeline,
			atc.UnpausePipeline,
			atc.ExposePipeline,
			atc.HidePipeline,
			atc.GetVersionsDB,
			atc.RenamePipeline,
			atc.ListPipelineBuilds,
			atc.CreatePipelineBuild,
			atc.PipelineBadge,
			atc.ListResources,
			atc.ListResourceTypes,
			atc.GetResource,
			atc.CheckResource,
			atc.CheckResourceWebHook,
			atc.CheckResourceType,
			atc.ListResourceVersions,
			atc.GetResourceVersion,
			atc.EnableResourceVersion,
			atc.DisableResourceVersion,
			atc.PinResourceVersion,
			atc.UnpinResource,
			atc.SetPinCommentOnResource,
			atc.ListBuildsWithVersionAsInput,
			atc.ListBuildsWithVersionAsOutput,
			atc.GetResourceCausality:
			newHandler = fetchPipelineByNameHandler{handler: handler, teamFactory: wrappa.TeamFactory}

		case atc.SaveConfig,
			atc.GetBuild,
			atc.BuildResources,
			atc.GetBuildPreparation,
			atc.BuildEvents,
			atc.GetBuildPlan,
			atc.ListBuildArtifacts,
			atc.AbortBuild,
			atc.PruneWorker,
			atc.LandWorker,
			atc.RetireWorker,
			atc.ListDestroyingVolumes,
			atc.ListDestroyingContainers,
			atc.ReportWorkerContainers,
			atc.ReportWorkerVolumes,
			atc.CreateBuild,
			atc.GetContainer,
			atc.HijackContainer,
			atc.ListContainers,
			atc.ListWorkers,
			atc.RegisterWorker,
			atc.HeartbeatWorker,
			atc.DeleteWorker,
			atc.GetTeam,
			atc.SetTeam,
			atc.ListTeamBuilds,
			atc.RenameTeam,
			atc.DestroyTeam,
			atc.ListVolumes,
			atc.GetUser,
			atc.DownloadCLI,
			atc.GetInfo,
			atc.GetCheck,
			atc.ListTeams,
			atc.ListAllPipelines,
			atc.ListPipelines,
			atc.ListAllJobs,
			atc.ListAllResources,
			atc.ListBuilds,
			atc.GetWall,
			atc.GetLogLevel,
			atc.ListActiveUsersSince,
			atc.SetLogLevel,
			atc.GetInfoCreds,
			atc.SetWall,
			atc.ClearWall,
			atc.GetCC,
			atc.OrderPipelines,
			atc.CreateArtifact,
			atc.GetArtifact:
			newHandler = handler

		// think about it!
		default:
			panic("you missed a spot (" + name + ")")
		}

		wrapped[name] = newHandler
	}

	return wrapped
}

type fetchPipelineByNameHandler struct {
	teamFactory db.TeamFactory
	handler     http.Handler
}

func (f fetchPipelineByNameHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	teamName := r.FormValue(":team_name")
	pipelineName := r.FormValue(":pipeline_name")

	_, ok := r.Context().Value(auth.PipelineContextKey).(db.Pipeline)
	if ok {
		f.handler.ServeHTTP(w, r)
		return
	}
	dbTeam, found, err := f.teamFactory.FindTeam(teamName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	pipeline, found, err := dbTeam.Pipeline(pipelineName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	newCtx := context.WithValue(r.Context(), auth.PipelineContextKey, pipeline)
	f.handler.ServeHTTP(w, r.WithContext(newCtx))
}
