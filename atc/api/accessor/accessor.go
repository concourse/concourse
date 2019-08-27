package accessor

import (
	"github.com/concourse/concourse/atc"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/mitchellh/mapstructure"
)

//go:generate counterfeiter . Access

type Access interface {
	HasToken() bool
	IsAuthenticated() bool
	IsAuthorized(string) bool
	IsAdmin() bool
	IsSystem() bool
	TeamNames() []string
	CSRFToken() string
	UserName() string
}

type access struct {
	*jwt.Token
	action string
}

func (a *access) HasToken() bool {
	return a.Token != nil
}

func (a *access) IsAuthenticated() bool {
	return a.HasToken() && a.Token.Valid
}

func (a *access) Claims() jwt.MapClaims {
	if a.IsAuthenticated() {
		if claims, ok := a.Token.Claims.(jwt.MapClaims); ok {
			return claims
		}
	}
	return jwt.MapClaims{}
}

func (a *access) IsAuthorized(team string) bool {
	if a.IsAdmin() {
		return true
	}
	for teamName, teamRoles := range a.TeamRoles() {
		if teamName != team {
			continue
		}
		for _, teamRole := range teamRoles {
			if a.hasPermission(teamRole) {
				return true
			}
		}
	}
	return false
}

func (a *access) hasPermission(role string) bool {
	switch requiredRoles[a.action] {
	case "owner":
		return role == "owner"
	case "member":
		return role == "owner" || role == "member"
	case "pipeline-operator":
		return role == "owner" || role == "member" || role == "pipeline-operator"
	case "viewer":
		return role == "owner" || role == "member" || role == "pipeline-operator" || role == "viewer"
	default:
		return false
	}
}

func (a *access) IsAdmin() bool {
	if isAdminClaim, ok := a.Claims()["is_admin"]; ok {
		isAdmin, ok := isAdminClaim.(bool)
		return ok && isAdmin
	}
	return false
}

func (a *access) IsSystem() bool {
	if isSystemClaim, ok := a.Claims()["system"]; ok {
		isSystem, ok := isSystemClaim.(bool)
		return ok && isSystem
	}
	return false
}

func (a *access) TeamNames() []string {

	teams := []string{}
	for teamName := range a.TeamRoles() {
		teams = append(teams, teamName)
	}

	return teams
}

func (a *access) TeamRoles() map[string][]string {
	teamRoles := map[string][]string{}

	if teamsClaim, ok := a.Claims()["teams"]; ok {

		// support legacy token format with team names array
		if teamsArr, ok := teamsClaim.([]interface{}); ok {
			for _, teamObj := range teamsArr {
				if teamName, ok := teamObj.(string); ok {
					teamRoles[teamName] = []string{"owner"}
				}
			}
		} else {
			_ = mapstructure.Decode(teamsClaim, &teamRoles)
		}
	}

	return teamRoles
}

func (a *access) CSRFToken() string {
	if csrfTokenClaim, ok := a.Claims()["csrf"]; ok {
		if csrfToken, ok := csrfTokenClaim.(string); ok {
			return csrfToken
		}
	}
	return ""
}

func (a *access) UserName() string {
	if userName, ok := a.Claims()["user_name"]; ok {
		if userName, ok := userName.(string); ok {
			return userName
		}
	} else if systemName, ok := a.Claims()["system"]; ok {
		if systemName == true {
			return "system"
		}
	}
	return ""
}

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
	atc.ListAllContainers:             "viewer",
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
