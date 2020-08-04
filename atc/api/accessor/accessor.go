package accessor

import (
	"fmt"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter . Access

type Access interface {
	HasToken() bool
	IsAuthenticated() bool
	IsAuthorized(string) bool
	IsAdmin() bool
	IsSystem() bool
	TeamNames() []string
	TeamRoles() map[string][]string
	Claims() Claims
}

type Claims struct {
	Sub       string
	Name      string
	UserID    string
	UserName  string
	Email     string
	Connector string
}

type Verification struct {
	HasToken     bool
	IsTokenValid bool
	RawClaims    map[string]interface{}
}

type access struct {
	verification      Verification
	requiredRole      string
	systemClaimKey    string
	systemClaimValues []string
	teams             []db.Team
	teamRoles         map[string][]string
	isAdmin           bool
}

func NewAccessor(
	verification Verification,
	requiredRole string,
	systemClaimKey string,
	systemClaimValues []string,
	teams []db.Team,
) *access {
	a := &access{
		verification:      verification,
		requiredRole:      requiredRole,
		systemClaimKey:    systemClaimKey,
		systemClaimValues: systemClaimValues,
		teams:             teams,
	}
	a.computeTeamRoles()
	return a
}


func (a *access) computeTeamRoles() {
	a.teamRoles = map[string][]string{}

	for _, team := range a.teams {
		roles := a.rolesForTeam(team.Auth())
		if len(roles) > 0 {
			a.teamRoles[team.Name()] = roles
		}
		if team.Admin() && contains(roles, "owner") {
			a.isAdmin = true
		}
	}
}

func contains(arr []string, val string) bool {
	for _, v := range arr {
		if v == val {
			return true
		}
	}
	return false
}

func (a *access) rolesForTeam(auth atc.TeamAuth) []string {
	roleSet := map[string]bool{}

	groups := a.groups()
	connectorID := a.connectorID()
	userID := a.userID()
	userName := a.UserName()

	for role, auth := range auth {
		userAuth := auth["users"]
		groupAuth := auth["groups"]

		// backwards compatibility for allow-all-users
		if len(userAuth) == 0 && len(groupAuth) == 0 {
			roleSet[role] = true
		}

		for _, user := range userAuth {
			if userID != "" {
				if strings.EqualFold(user, fmt.Sprintf("%v:%v", connectorID, userID)) {
					roleSet[role] = true
				}
			}
			if userName != "" {
				if strings.EqualFold(user, fmt.Sprintf("%v:%v", connectorID, userName)) {
					roleSet[role] = true
				}
			}
		}

		for _, group := range groupAuth {
			for _, claimGroup := range groups {
				if claimGroup != "" {
					if strings.EqualFold(group, fmt.Sprintf("%v:%v", connectorID, claimGroup)) {
						roleSet[role] = true
					}
				}
			}
		}
	}

	var roles []string
	for role := range roleSet {
		roles = append(roles, role)
	}
	return roles
}

func (a *access) HasToken() bool {
	return a.verification.HasToken
}

func (a *access) IsAuthenticated() bool {
	return a.verification.IsTokenValid
}

func (a *access) IsAuthorized(teamName string) bool {
	return a.isAdmin || a.hasPermission(a.teamRoles[teamName])
}

func (a *access) TeamNames() []string {
	teamNames := []string{}
	for _, team := range a.teams {
		if a.isAdmin || a.hasPermission(a.teamRoles[team.Name()]) {
			teamNames = append(teamNames, team.Name())
		}
	}

	return teamNames
}

func (a *access) hasPermission(roles []string) bool {
	for _, role := range roles {
		switch a.requiredRole {
		case OwnerRole:
			return role == OwnerRole
		case MemberRole:
			return role == OwnerRole || role == MemberRole
		case OperatorRole:
			return role == OwnerRole || role == MemberRole || role == OperatorRole
		case ViewerRole:
			return role == OwnerRole || role == MemberRole || role == OperatorRole || role == ViewerRole
		default:
			return false
		}
	}
	return false
}

func (a *access) claims() map[string]interface{} {
	if a.IsAuthenticated() {
		return a.verification.RawClaims
	}
	return map[string]interface{}{}
}

func (a *access) federatedClaims() map[string]interface{} {
	if raw, ok := a.claims()["federated_claims"]; ok {
		if claim, ok := raw.(map[string]interface{}); ok {
			return claim
		}
	}
	return map[string]interface{}{}
}

func (a *access) federatedClaim(name string) string {
	if raw, ok := a.federatedClaims()[name]; ok {
		if claim, ok := raw.(string); ok {
			return claim
		}
	}
	return ""
}

func (a *access) claim(name string) string {
	if raw, ok := a.claims()[name]; ok {
		if claim, ok := raw.(string); ok {
			return claim
		}
	}
	return ""
}

func (a *access) UserName() string {
	return a.federatedClaim("user_name")
}

func (a *access) userID() string {
	return a.federatedClaim("user_id")
}

func (a *access) connectorID() string {
	return a.federatedClaim("connector_id")
}

func (a *access) groups() []string {
	groups := []string{}
	if raw, ok := a.claims()["groups"]; ok {
		if rawGroups, ok := raw.([]interface{}); ok {
			for _, rawGroup := range rawGroups {
				if group, ok := rawGroup.(string); ok {
					groups = append(groups, group)
				}
			}
		}
	}
	return groups
}

func (a *access) IsAdmin() bool {
	return a.isAdmin
}

func (a *access) IsSystem() bool {
	if claim := a.claim(a.systemClaimKey); claim != "" {
		for _, value := range a.systemClaimValues {
			if value == claim {
				return true
			}
		}
	}
	return false
}

func (a *access) TeamRoles() map[string][]string {
	return a.teamRoles
}

func (a *access) Claims() Claims {
	return Claims{
		Sub:       a.claim("sub"),
		Name:      a.claim("name"),
		Email:     a.claim("email"),
		UserID:    a.userID(),
		UserName:  a.UserName(),
		Connector: a.connectorID(),
	}
}
