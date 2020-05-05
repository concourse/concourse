package wrappa_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/wrappa"
	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RejectArchivedWrappa", func() {
	var (
		raWrappa         *wrappa.RejectArchivedWrappa
		raHandlerFactory wrappa.RejectArchivedHandlerFactory
	)

	BeforeEach(func() {
		fakeTeamFactory := new(dbfakes.FakeTeamFactory)
		raHandlerFactory = wrappa.NewRejectArchivedHandlerFactory(fakeTeamFactory)
		raWrappa = wrappa.NewRejectArchivedWrappa(raHandlerFactory)
	})

	It("wraps endpoints", func() {
		inputHandlers := rata.Handlers{}

		for _, route := range atc.Routes {
			inputHandlers[route.Name] = &stupidHandler{}
		}

		expectedHandlers := rata.Handlers{

			atc.PausePipeline:           raHandlerFactory.RejectArchived(inputHandlers[atc.PausePipeline]),
			atc.CreateJobBuild:          raHandlerFactory.RejectArchived(inputHandlers[atc.CreateJobBuild]),
			atc.ScheduleJob:             raHandlerFactory.RejectArchived(inputHandlers[atc.ScheduleJob]),
			atc.CheckResource:           raHandlerFactory.RejectArchived(inputHandlers[atc.CheckResource]),
			atc.CheckResourceType:       raHandlerFactory.RejectArchived(inputHandlers[atc.CheckResourceType]),
			atc.DisableResourceVersion:  raHandlerFactory.RejectArchived(inputHandlers[atc.DisableResourceVersion]),
			atc.EnableResourceVersion:   raHandlerFactory.RejectArchived(inputHandlers[atc.EnableResourceVersion]),
			atc.PinResourceVersion:      raHandlerFactory.RejectArchived(inputHandlers[atc.PinResourceVersion]),
			atc.UnpinResource:           raHandlerFactory.RejectArchived(inputHandlers[atc.UnpinResource]),
			atc.SetPinCommentOnResource: raHandlerFactory.RejectArchived(inputHandlers[atc.SetPinCommentOnResource]),
			atc.RerunJobBuild:           raHandlerFactory.RejectArchived(inputHandlers[atc.RerunJobBuild]),

			atc.GetConfig:                     inputHandlers[atc.GetConfig],
			atc.GetBuild:                      inputHandlers[atc.GetBuild],
			atc.BuildResources:                inputHandlers[atc.BuildResources],
			atc.BuildEvents:                   inputHandlers[atc.BuildEvents],
			atc.ListBuildArtifacts:            inputHandlers[atc.ListBuildArtifacts],
			atc.GetBuildPreparation:           inputHandlers[atc.GetBuildPreparation],
			atc.GetBuildPlan:                  inputHandlers[atc.GetBuildPlan],
			atc.AbortBuild:                    inputHandlers[atc.AbortBuild],
			atc.PruneWorker:                   inputHandlers[atc.PruneWorker],
			atc.LandWorker:                    inputHandlers[atc.LandWorker],
			atc.ReportWorkerContainers:        inputHandlers[atc.ReportWorkerContainers],
			atc.ReportWorkerVolumes:           inputHandlers[atc.ReportWorkerVolumes],
			atc.RetireWorker:                  inputHandlers[atc.RetireWorker],
			atc.ListDestroyingContainers:      inputHandlers[atc.ListDestroyingContainers],
			atc.ListDestroyingVolumes:         inputHandlers[atc.ListDestroyingVolumes],
			atc.GetPipeline:                   inputHandlers[atc.GetPipeline],
			atc.GetJobBuild:                   inputHandlers[atc.GetJobBuild],
			atc.PipelineBadge:                 inputHandlers[atc.PipelineBadge],
			atc.JobBadge:                      inputHandlers[atc.JobBadge],
			atc.ListJobs:                      inputHandlers[atc.ListJobs],
			atc.GetJob:                        inputHandlers[atc.GetJob],
			atc.ListJobBuilds:                 inputHandlers[atc.ListJobBuilds],
			atc.ListPipelineBuilds:            inputHandlers[atc.ListPipelineBuilds],
			atc.GetResource:                   inputHandlers[atc.GetResource],
			atc.ListBuildsWithVersionAsInput:  inputHandlers[atc.ListBuildsWithVersionAsInput],
			atc.ListBuildsWithVersionAsOutput: inputHandlers[atc.ListBuildsWithVersionAsOutput],
			atc.ListResources:                 inputHandlers[atc.ListResources],
			atc.ListResourceTypes:             inputHandlers[atc.ListResourceTypes],
			atc.ListResourceVersions:          inputHandlers[atc.ListResourceVersions],
			atc.GetResourceCausality:          inputHandlers[atc.GetResourceCausality],
			atc.GetResourceVersion:            inputHandlers[atc.GetResourceVersion],
			atc.CreateBuild:                   inputHandlers[atc.CreateBuild],
			atc.GetContainer:                  inputHandlers[atc.GetContainer],
			atc.HijackContainer:               inputHandlers[atc.HijackContainer],
			atc.ListContainers:                inputHandlers[atc.ListContainers],
			atc.ListVolumes:                   inputHandlers[atc.ListVolumes],
			atc.ListTeamBuilds:                inputHandlers[atc.ListTeamBuilds],
			atc.ListWorkers:                   inputHandlers[atc.ListWorkers],
			atc.RegisterWorker:                inputHandlers[atc.RegisterWorker],
			atc.HeartbeatWorker:               inputHandlers[atc.HeartbeatWorker],
			atc.DeleteWorker:                  inputHandlers[atc.DeleteWorker],
			atc.GetTeam:                       inputHandlers[atc.GetTeam],
			atc.SetTeam:                       inputHandlers[atc.SetTeam],
			atc.RenameTeam:                    inputHandlers[atc.RenameTeam],
			atc.DestroyTeam:                   inputHandlers[atc.DestroyTeam],
			atc.GetUser:                       inputHandlers[atc.GetUser],
			atc.GetInfo:                       inputHandlers[atc.GetInfo],
			atc.GetCheck:                      inputHandlers[atc.GetCheck],
			atc.DownloadCLI:                   inputHandlers[atc.DownloadCLI],
			atc.CheckResourceWebHook:          inputHandlers[atc.CheckResourceWebHook],
			atc.ListAllPipelines:              inputHandlers[atc.ListAllPipelines],
			atc.ListBuilds:                    inputHandlers[atc.ListBuilds],
			atc.ListPipelines:                 inputHandlers[atc.ListPipelines],
			atc.ListAllJobs:                   inputHandlers[atc.ListAllJobs],
			atc.ListAllResources:              inputHandlers[atc.ListAllResources],
			atc.ListTeams:                     inputHandlers[atc.ListTeams],
			atc.MainJobBadge:                  inputHandlers[atc.MainJobBadge],
			atc.GetWall:                       inputHandlers[atc.GetWall],
			atc.GetLogLevel:                   inputHandlers[atc.GetLogLevel],
			atc.SetLogLevel:                   inputHandlers[atc.SetLogLevel],
			atc.GetInfoCreds:                  inputHandlers[atc.GetInfoCreds],
			atc.ListActiveUsersSince:          inputHandlers[atc.ListActiveUsersSince],
			atc.SetWall:                       inputHandlers[atc.SetWall],
			atc.ClearWall:                     inputHandlers[atc.ClearWall],
			atc.DeletePipeline:                inputHandlers[atc.DeletePipeline],
			atc.GetCC:                         inputHandlers[atc.GetCC],
			atc.GetVersionsDB:                 inputHandlers[atc.GetVersionsDB],
			atc.ListJobInputs:                 inputHandlers[atc.ListJobInputs],
			atc.OrderPipelines:                inputHandlers[atc.OrderPipelines],
			atc.PauseJob:                      inputHandlers[atc.PauseJob],
			atc.ArchivePipeline:               inputHandlers[atc.ArchivePipeline],
			atc.RenamePipeline:                inputHandlers[atc.RenamePipeline],
			atc.SaveConfig:                    inputHandlers[atc.SaveConfig],
			atc.UnpauseJob:                    inputHandlers[atc.UnpauseJob],
			atc.UnpausePipeline:               inputHandlers[atc.UnpausePipeline],
			atc.ExposePipeline:                inputHandlers[atc.ExposePipeline],
			atc.HidePipeline:                  inputHandlers[atc.HidePipeline],
			atc.CreatePipelineBuild:           inputHandlers[atc.CreatePipelineBuild],
			atc.ClearTaskCache:                inputHandlers[atc.ClearTaskCache],
			atc.CreateArtifact:                inputHandlers[atc.CreateArtifact],
			atc.GetArtifact:                   inputHandlers[atc.GetArtifact],
		}

		wrappedHandlers := raWrappa.Wrap(inputHandlers)

		for name, _ := range inputHandlers {
			Expect(wrappedHandlers[name]).To(BeIdenticalTo(expectedHandlers[name]), "handler is "+name)
		}
	})
})
