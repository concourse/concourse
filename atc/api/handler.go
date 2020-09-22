package api

import (
	"net/http"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/artifactserver"
	"github.com/concourse/concourse/atc/api/buildserver"
	"github.com/concourse/concourse/atc/api/ccserver"
	"github.com/concourse/concourse/atc/api/checkserver"
	"github.com/concourse/concourse/atc/api/cliserver"
	"github.com/concourse/concourse/atc/api/configserver"
	"github.com/concourse/concourse/atc/api/containerserver"
	"github.com/concourse/concourse/atc/api/infoserver"
	"github.com/concourse/concourse/atc/api/jobserver"
	"github.com/concourse/concourse/atc/api/loglevelserver"
	"github.com/concourse/concourse/atc/api/pipelineserver"
	"github.com/concourse/concourse/atc/api/resourceserver"
	"github.com/concourse/concourse/atc/api/resourceserver/versionserver"
	"github.com/concourse/concourse/atc/api/teamserver"
	"github.com/concourse/concourse/atc/api/usersserver"
	"github.com/concourse/concourse/atc/api/volumeserver"
	"github.com/concourse/concourse/atc/api/wallserver"
	"github.com/concourse/concourse/atc/api/workerserver"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/gc"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/wrappa"
	"github.com/tedsuo/rata"
)

func NewHandler(
	logger lager.Logger,

	externalURL string,
	clusterName string,

	wrapper wrappa.Wrappa,

	dbTeamFactory db.TeamFactory,
	dbPipelineFactory db.PipelineFactory,
	dbJobFactory db.JobFactory,
	dbResourceFactory db.ResourceFactory,
	dbWorkerFactory db.WorkerFactory,
	volumeRepository db.VolumeRepository,
	containerRepository db.ContainerRepository,
	destroyer gc.Destroyer,
	dbBuildFactory db.BuildFactory,
	dbCheckFactory db.CheckFactory,
	dbResourceConfigFactory db.ResourceConfigFactory,
	dbUserFactory db.UserFactory,

	eventHandlerFactory buildserver.EventHandlerFactory,

	workerClient worker.Client,

	sink *lager.ReconfigurableSink,

	isTLSEnabled bool,

	cliDownloadsDir string,
	version string,
	workerVersion string,
	secretManager creds.Secrets,
	varSourcePool creds.VarSourcePool,
	credsManagers creds.Managers,
	interceptTimeoutFactory containerserver.InterceptTimeoutFactory,
	interceptUpdateInterval time.Duration,
	dbWall db.Wall,
	clock clock.Clock,
) (http.Handler, error) {

	absCLIDownloadsDir, err := filepath.Abs(cliDownloadsDir)
	if err != nil {
		return nil, err
	}

	pipelineHandlerFactory := pipelineserver.ScopedHandlerFactory{}
	buildHandlerFactory := buildserver.NewScopedHandlerFactory(logger)
	teamHandlerFactory := NewTeamScopedHandlerFactory(logger, dbTeamFactory)

	buildServer := buildserver.NewServer(logger, externalURL, dbTeamFactory, dbBuildFactory, eventHandlerFactory)
	checkServer := checkserver.NewServer(logger, dbCheckFactory)
	jobServer := jobserver.NewServer(logger, externalURL, secretManager, dbJobFactory, dbCheckFactory)
	resourceServer := resourceserver.NewServer(logger, secretManager, varSourcePool, dbCheckFactory, dbResourceFactory, dbResourceConfigFactory)

	versionServer := versionserver.NewServer(logger, externalURL)
	pipelineServer := pipelineserver.NewServer(logger, dbTeamFactory, dbPipelineFactory, externalURL)
	configServer := configserver.NewServer(logger, dbTeamFactory, secretManager)
	ccServer := ccserver.NewServer(logger, dbTeamFactory, externalURL)
	workerServer := workerserver.NewServer(logger, dbTeamFactory, dbWorkerFactory)
	logLevelServer := loglevelserver.NewServer(logger, sink)
	cliServer := cliserver.NewServer(logger, absCLIDownloadsDir)
	containerServer := containerserver.NewServer(logger, workerClient, secretManager, varSourcePool, interceptTimeoutFactory, interceptUpdateInterval, containerRepository, destroyer, clock)
	volumesServer := volumeserver.NewServer(logger, volumeRepository, destroyer)
	teamServer := teamserver.NewServer(logger, dbTeamFactory, externalURL)
	infoServer := infoserver.NewServer(logger, version, workerVersion, externalURL, clusterName, credsManagers)
	artifactServer := artifactserver.NewServer(logger, workerClient)
	usersServer := usersserver.NewServer(logger, dbUserFactory)
	wallServer := wallserver.NewServer(dbWall, logger)

	handlers := map[string]http.Handler{
		atc.GetConfig:  pipelineHandlerFactory.HandlerFor(configServer.GetConfig),
		atc.SaveConfig: http.HandlerFunc(configServer.SaveConfig),

		atc.GetCC: http.HandlerFunc(ccServer.GetCC),

		atc.ListBuilds:          http.HandlerFunc(buildServer.ListBuilds),
		atc.CreateBuild:         teamHandlerFactory.HandlerFor(buildServer.CreateBuild),
		atc.GetBuild:            buildHandlerFactory.HandlerFor(buildServer.GetBuild),
		atc.BuildResources:      buildHandlerFactory.HandlerFor(buildServer.BuildResources),
		atc.AbortBuild:          buildHandlerFactory.HandlerFor(buildServer.AbortBuild),
		atc.GetBuildPlan:        buildHandlerFactory.HandlerFor(buildServer.GetBuildPlan),
		atc.GetBuildPreparation: buildHandlerFactory.HandlerFor(buildServer.GetBuildPreparation),
		atc.BuildEvents:         buildHandlerFactory.HandlerFor(buildServer.BuildEvents),
		atc.ListBuildArtifacts:  buildHandlerFactory.HandlerFor(buildServer.GetBuildArtifacts),

		atc.GetCheck: http.HandlerFunc(checkServer.GetCheck),

		atc.ListAllJobs:    http.HandlerFunc(jobServer.ListAllJobs),
		atc.ListJobs:       pipelineHandlerFactory.HandlerFor(jobServer.ListJobs),
		atc.GetJob:         pipelineHandlerFactory.HandlerFor(jobServer.GetJob),
		atc.ListJobBuilds:  pipelineHandlerFactory.HandlerFor(jobServer.ListJobBuilds),
		atc.ListJobInputs:  pipelineHandlerFactory.HandlerFor(jobServer.ListJobInputs),
		atc.GetJobBuild:    pipelineHandlerFactory.HandlerFor(jobServer.GetJobBuild),
		atc.CreateJobBuild: pipelineHandlerFactory.HandlerFor(jobServer.CreateJobBuild),
		atc.RerunJobBuild:  pipelineHandlerFactory.HandlerFor(jobServer.RerunJobBuild),
		atc.PauseJob:       pipelineHandlerFactory.HandlerFor(jobServer.PauseJob),
		atc.UnpauseJob:     pipelineHandlerFactory.HandlerFor(jobServer.UnpauseJob),
		atc.ScheduleJob:    pipelineHandlerFactory.HandlerFor(jobServer.ScheduleJob),
		atc.JobBadge:       pipelineHandlerFactory.HandlerFor(jobServer.JobBadge),

		atc.ClearTaskCache: pipelineHandlerFactory.HandlerFor(jobServer.ClearTaskCache),

		atc.ListAllPipelines:    http.HandlerFunc(pipelineServer.ListAllPipelines),
		atc.ListPipelines:       http.HandlerFunc(pipelineServer.ListPipelines),
		atc.GetPipeline:         pipelineHandlerFactory.HandlerFor(pipelineServer.GetPipeline),
		atc.DeletePipeline:      pipelineHandlerFactory.HandlerFor(pipelineServer.DeletePipeline),
		atc.OrderPipelines:      http.HandlerFunc(pipelineServer.OrderPipelines),
		atc.PausePipeline:       pipelineHandlerFactory.HandlerFor(pipelineServer.PausePipeline),
		atc.ArchivePipeline:     pipelineHandlerFactory.HandlerFor(pipelineServer.ArchivePipeline),
		atc.UnpausePipeline:     pipelineHandlerFactory.HandlerFor(pipelineServer.UnpausePipeline),
		atc.ExposePipeline:      pipelineHandlerFactory.HandlerFor(pipelineServer.ExposePipeline),
		atc.HidePipeline:        pipelineHandlerFactory.HandlerFor(pipelineServer.HidePipeline),
		atc.GetVersionsDB:       pipelineHandlerFactory.HandlerFor(pipelineServer.GetVersionsDB),
		atc.RenamePipeline:      pipelineHandlerFactory.HandlerFor(pipelineServer.RenamePipeline),
		atc.ListPipelineBuilds:  pipelineHandlerFactory.HandlerFor(pipelineServer.ListPipelineBuilds),
		atc.CreatePipelineBuild: pipelineHandlerFactory.HandlerFor(pipelineServer.CreateBuild),
		atc.PipelineBadge:       pipelineHandlerFactory.HandlerFor(pipelineServer.PipelineBadge),

		atc.ListAllResources:        http.HandlerFunc(resourceServer.ListAllResources),
		atc.ListResources:           pipelineHandlerFactory.HandlerFor(resourceServer.ListResources),
		atc.ListResourceTypes:       pipelineHandlerFactory.HandlerFor(resourceServer.ListVersionedResourceTypes),
		atc.GetResource:             pipelineHandlerFactory.HandlerFor(resourceServer.GetResource),
		atc.UnpinResource:           pipelineHandlerFactory.HandlerFor(resourceServer.UnpinResource),
		atc.SetPinCommentOnResource: pipelineHandlerFactory.HandlerFor(resourceServer.SetPinCommentOnResource),
		atc.CheckResource:           pipelineHandlerFactory.HandlerFor(resourceServer.CheckResource),
		atc.CheckResourceWebHook:    pipelineHandlerFactory.HandlerFor(resourceServer.CheckResourceWebHook),
		atc.CheckResourceType:       pipelineHandlerFactory.HandlerFor(resourceServer.CheckResourceType),

		atc.ListResourceVersions:          pipelineHandlerFactory.HandlerFor(versionServer.ListResourceVersions),
		atc.GetResourceVersion:            pipelineHandlerFactory.HandlerFor(versionServer.GetResourceVersion),
		atc.EnableResourceVersion:         pipelineHandlerFactory.HandlerFor(versionServer.EnableResourceVersion),
		atc.DisableResourceVersion:        pipelineHandlerFactory.HandlerFor(versionServer.DisableResourceVersion),
		atc.PinResourceVersion:            pipelineHandlerFactory.HandlerFor(versionServer.PinResourceVersion),
		atc.ListBuildsWithVersionAsInput:  pipelineHandlerFactory.HandlerFor(versionServer.ListBuildsWithVersionAsInput),
		atc.ListBuildsWithVersionAsOutput: pipelineHandlerFactory.HandlerFor(versionServer.ListBuildsWithVersionAsOutput),
		atc.GetResourceCausality:          pipelineHandlerFactory.HandlerFor(versionServer.GetCausality),

		atc.ListWorkers:     http.HandlerFunc(workerServer.ListWorkers),
		atc.RegisterWorker:  http.HandlerFunc(workerServer.RegisterWorker),
		atc.LandWorker:      http.HandlerFunc(workerServer.LandWorker),
		atc.RetireWorker:    http.HandlerFunc(workerServer.RetireWorker),
		atc.PruneWorker:     http.HandlerFunc(workerServer.PruneWorker),
		atc.HeartbeatWorker: http.HandlerFunc(workerServer.HeartbeatWorker),
		atc.DeleteWorker:    http.HandlerFunc(workerServer.DeleteWorker),

		atc.SetLogLevel: http.HandlerFunc(logLevelServer.SetMinLevel),
		atc.GetLogLevel: http.HandlerFunc(logLevelServer.GetMinLevel),

		atc.DownloadCLI:  http.HandlerFunc(cliServer.Download),
		atc.GetInfo:      http.HandlerFunc(infoServer.Info),
		atc.GetInfoCreds: http.HandlerFunc(infoServer.Creds),

		atc.GetUser:              http.HandlerFunc(usersServer.GetUser),
		atc.ListActiveUsersSince: http.HandlerFunc(usersServer.GetUsersSince),

		atc.ListContainers:           teamHandlerFactory.HandlerFor(containerServer.ListContainers),
		atc.GetContainer:             teamHandlerFactory.HandlerFor(containerServer.GetContainer),
		atc.HijackContainer:          teamHandlerFactory.HandlerFor(containerServer.HijackContainer),
		atc.ListDestroyingContainers: http.HandlerFunc(containerServer.ListDestroyingContainers),
		atc.ReportWorkerContainers:   http.HandlerFunc(containerServer.ReportWorkerContainers),

		atc.ListVolumes:           teamHandlerFactory.HandlerFor(volumesServer.ListVolumes),
		atc.ListDestroyingVolumes: http.HandlerFunc(volumesServer.ListDestroyingVolumes),
		atc.ReportWorkerVolumes:   http.HandlerFunc(volumesServer.ReportWorkerVolumes),

		atc.ListTeams:      http.HandlerFunc(teamServer.ListTeams),
		atc.GetTeam:        http.HandlerFunc(teamServer.GetTeam),
		atc.SetTeam:        http.HandlerFunc(teamServer.SetTeam),
		atc.RenameTeam:     http.HandlerFunc(teamServer.RenameTeam),
		atc.DestroyTeam:    http.HandlerFunc(teamServer.DestroyTeam),
		atc.ListTeamBuilds: http.HandlerFunc(teamServer.ListTeamBuilds),

		atc.CreateArtifact: teamHandlerFactory.HandlerFor(artifactServer.CreateArtifact),
		atc.GetArtifact:    teamHandlerFactory.HandlerFor(artifactServer.GetArtifact),

		atc.GetWall:   http.HandlerFunc(wallServer.GetWall),
		atc.SetWall:   http.HandlerFunc(wallServer.SetWall),
		atc.ClearWall: http.HandlerFunc(wallServer.ClearWall),
	}

	return rata.NewRouter(atc.Routes, wrapper.Wrap(handlers))
}
