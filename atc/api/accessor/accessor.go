package accessor

import (
	"strings"

	"github.com/concourse/concourse/atc"
	jwt "github.com/dgrijalva/jwt-go"
)

//go:generate counterfeiter . Access

type Access interface {
	IsAuthenticated() bool
	IsAuthorized(string) bool
	IsAdmin() bool
	IsSystem() bool
	TeamNames() []string
	CSRFToken() string
}

type access struct {
	*jwt.Token
	action string
}

func (a *access) IsAuthenticated() bool {
	return a.Token.Valid
}

func (a *access) IsAuthorized(team string) bool {
	for _, teamRole := range a.TeamRoles() {

		var teamName, roleName string

		teamParts := strings.Split(teamRole, ":")

		if len(teamParts) == 1 {
			teamName = teamParts[0]
			roleName = "admin"

		} else if len(teamParts) > 1 {
			teamName = teamParts[0]
			roleName = teamParts[1]
		}

		if teamName == team {
			return a.HasPermission(roleName)
		}
	}
	return false
}

func (a *access) HasPermission(role string) bool {
	switch requiredRoles[a.action] {
	case "admin":
		return role == "admin"
	case "member":
		return role == "admin" || role == "member"
	case "viewer":
		return role == "admin" || role == "member" || role == "viewer"
	default:
		return false
	}
}

func (a *access) IsAdmin() bool {
	if claims, ok := a.Token.Claims.(jwt.MapClaims); ok {
		if isAdminClaim, ok := claims["is_admin"]; ok {
			isAdmin, ok := isAdminClaim.(bool)
			return ok && isAdmin
		}
	}
	return false
}

func (a *access) IsSystem() bool {
	if claims, ok := a.Token.Claims.(jwt.MapClaims); ok {
		if isSystemClaim, ok := claims["system"]; ok {
			isSystem, ok := isSystemClaim.(bool)
			return ok && isSystem
		}
	}
	return false
}

func (a *access) TeamRoles() []string {
	teamRoles := []string{}

	if claims, ok := a.Token.Claims.(jwt.MapClaims); ok {
		if teamsClaim, ok := claims["teams"]; ok {
			if teamsArr, ok := teamsClaim.([]interface{}); ok {
				for _, teamObj := range teamsArr {
					if teamRole, ok := teamObj.(string); ok {
						teamRoles = append(teamRoles, teamRole)
					}
				}
			}
		}
	}

	return teamRoles
}

func (a *access) TeamNames() []string {
	set := map[string]bool{}

	for _, teamRole := range a.TeamRoles() {
		if teamParts := strings.Split(teamRole, ":"); len(teamParts) > 0 {
			set[teamParts[0]] = true
		}
	}

	teams := []string{}
	for team, _ := range set {
		teams = append(teams, team)
	}

	return teams
}

func (a *access) CSRFToken() string {
	if claims, ok := a.Token.Claims.(jwt.MapClaims); ok {
		if csrfTokenClaim, ok := claims["csrf"]; ok {
			if csrfToken, ok := csrfTokenClaim.(string); ok {
				return csrfToken
			}
		}
	}
	return ""
}

var requiredRoles = map[string]string{
	atc.SaveConfig:                    "member",
	atc.GetConfig:                     "viewer",
	atc.GetBuild:                      "viewer",
	atc.GetBuildPlan:                  "viewer",
	atc.CreateBuild:                   "member",
	atc.ListBuilds:                    "viewer",
	atc.BuildEvents:                   "viewer",
	atc.BuildResources:                "viewer",
	atc.AbortBuild:                    "member",
	atc.GetBuildPreparation:           "viewer",
	atc.GetJob:                        "viewer",
	atc.CreateJobBuild:                "member",
	atc.ListAllJobs:                   "viewer",
	atc.ListJobs:                      "viewer",
	atc.ListJobBuilds:                 "viewer",
	atc.ListJobInputs:                 "viewer",
	atc.GetJobBuild:                   "viewer",
	atc.PauseJob:                      "member",
	atc.UnpauseJob:                    "member",
	atc.GetVersionsDB:                 "viewer",
	atc.JobBadge:                      "viewer",
	atc.MainJobBadge:                  "viewer",
	atc.ClearTaskCache:                "member",
	atc.ListAllResources:              "viewer",
	atc.ListResources:                 "viewer",
	atc.ListResourceTypes:             "viewer",
	atc.GetResource:                   "viewer",
	atc.PauseResource:                 "member",
	atc.UnpauseResource:               "member",
	atc.CheckResource:                 "member",
	atc.CheckResourceWebHook:          "member",
	atc.CheckResourceType:             "member",
	atc.ListResourceVersions:          "viewer",
	atc.GetResourceVersion:            "viewer",
	atc.EnableResourceVersion:         "member",
	atc.DisableResourceVersion:        "member",
	atc.ListBuildsWithVersionAsInput:  "viewer",
	atc.ListBuildsWithVersionAsOutput: "viewer",
	atc.GetResourceCausality:          "viewer",
	atc.ListAllPipelines:              "viewer",
	atc.ListPipelines:                 "viewer",
	atc.GetPipeline:                   "viewer",
	atc.DeletePipeline:                "member",
	atc.OrderPipelines:                "member",
	atc.PausePipeline:                 "member",
	atc.UnpausePipeline:               "member",
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
	atc.SetTeam:                       "admin",
	atc.RenameTeam:                    "admin",
	atc.DestroyTeam:                   "admin",
	atc.ListTeamBuilds:                "viewer",
	atc.SendInputToBuildPlan:          "member",
	atc.ReadOutputFromBuildPlan:       "member",
}
