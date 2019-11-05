package accessor

import (
	"code.cloudfoundry.org/lager"
	"gopkg.in/yaml.v2"
	"io/ioutil"

	"github.com/concourse/concourse/atc"
)

// requiredRoles should be a const, never be updated.
var requiredRoles = map[string]string{
	atc.SaveConfig:                    "member",
	atc.GetConfig:                     "viewer",
	atc.GetCC:                         "viewer",
	atc.GetBuild:                      "viewer",
	atc.GetCheck:                      "viewer",
	atc.GetBuildPlan:                  "viewer",
	atc.CreateBuild:                   "member",
	atc.ListBuilds:                    "viewer",
	atc.BuildEvents:                   "viewer",
	atc.BuildResources:                "viewer",
	atc.AbortBuild:                    "pipeline-operator",
	atc.GetBuildPreparation:           "viewer",
	atc.GetJob:                        "viewer",
	atc.CreateJobBuild:                "pipeline-operator",
	atc.ListAllJobs:                   "viewer",
	atc.ListJobs:                      "viewer",
	atc.ListJobBuilds:                 "viewer",
	atc.ListJobInputs:                 "viewer",
	atc.GetJobBuild:                   "viewer",
	atc.PauseJob:                      "pipeline-operator",
	atc.UnpauseJob:                    "pipeline-operator",
	atc.GetVersionsDB:                 "viewer",
	atc.JobBadge:                      "viewer",
	atc.MainJobBadge:                  "viewer",
	atc.ClearTaskCache:                "pipeline-operator",
	atc.ListAllResources:              "viewer",
	atc.ListResources:                 "viewer",
	atc.ListResourceTypes:             "viewer",
	atc.GetResource:                   "viewer",
	atc.UnpinResource:                 "pipeline-operator",
	atc.SetPinCommentOnResource:       "pipeline-operator",
	atc.CheckResource:                 "pipeline-operator",
	atc.CheckResourceWebHook:          "pipeline-operator",
	atc.CheckResourceType:             "pipeline-operator",
	atc.ListResourceVersions:          "viewer",
	atc.GetResourceVersion:            "viewer",
	atc.EnableResourceVersion:         "pipeline-operator",
	atc.DisableResourceVersion:        "pipeline-operator",
	atc.PinResourceVersion:            "pipeline-operator",
	atc.ListBuildsWithVersionAsInput:  "viewer",
	atc.ListBuildsWithVersionAsOutput: "viewer",
	atc.GetResourceCausality:          "viewer",
	atc.ListAllPipelines:              "viewer",
	atc.ListPipelines:                 "viewer",
	atc.GetPipeline:                   "viewer",
	atc.DeletePipeline:                "member",
	atc.OrderPipelines:                "member",
	atc.PausePipeline:                 "pipeline-operator",
	atc.UnpausePipeline:               "pipeline-operator",
	atc.ExposePipeline:                "member",
	atc.HidePipeline:                  "member",
	atc.RenamePipeline:                "member",
	atc.ListPipelineBuilds:            "viewer",
	atc.CreatePipelineBuild:           "member",
	atc.PipelineBadge:                 "viewer",
	atc.RegisterWorker:                "member",
	atc.LandWorker:                    "member",
	atc.RetireWorker:                  "member",
	atc.PruneWorker:                   "member",
	atc.HeartbeatWorker:               "member",
	atc.ListWorkers:                   "viewer",
	atc.DeleteWorker:                  "member",
	atc.SetLogLevel:                   "member",
	atc.GetLogLevel:                   "viewer",
	atc.DownloadCLI:                   "viewer",
	atc.GetInfo:                       "viewer",
	atc.GetInfoCreds:                  "viewer",
	atc.ListContainers:                "viewer",
	atc.GetContainer:                  "viewer",
	atc.HijackContainer:               "member",
	atc.ListDestroyingContainers:      "viewer",
	atc.ReportWorkerContainers:        "member",
	atc.ListVolumes:                   "viewer",
	atc.ListDestroyingVolumes:         "viewer",
	atc.ReportWorkerVolumes:           "member",
	atc.ListTeams:                     "viewer",
	atc.GetTeam:                       "viewer",
	atc.SetTeam:                       "owner",
	atc.RenameTeam:                    "owner",
	atc.DestroyTeam:                   "owner",
	atc.ListTeamBuilds:                "viewer",
	atc.CreateArtifact:                "member",
	atc.GetArtifact:                   "member",
	atc.ListBuildArtifacts:            "viewer",
}

type CustomActionRoleMap map[string][]string

//go:generate counterfeiter . ActionRoleMap

type ActionRoleMap interface {
	RoleOfAction(string) string
}

//go:generate counterfeiter . ActionRoleMapModifier

type ActionRoleMapModifier interface {
	CustomizeActionRoleMap(lager.Logger, CustomActionRoleMap) error
}

func ParseCustomActionRoleMap(filename string, mapping *CustomActionRoleMap) error {
	if filename == "" {
		return nil
	}

	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(content, mapping)
	if err != nil {
		return err
	}

	return nil
}
