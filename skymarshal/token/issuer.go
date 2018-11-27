package token

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/db"
	"golang.org/x/oauth2"
)

//go:generate counterfeiter . Issuer
type Issuer interface {
	Issue(*VerifiedClaims) (*oauth2.Token, error)
}

func NewIssuer(teamFactory db.TeamFactory, generator Generator, duration time.Duration) *issuer {
	return &issuer{
		TeamFactory: teamFactory,
		Generator:   generator,
		Duration:    duration,
	}
}

type issuer struct {
	TeamFactory db.TeamFactory
	Generator   Generator
	Duration    time.Duration
}

func (self *issuer) Issue(verifiedClaims *VerifiedClaims) (*oauth2.Token, error) {

	// TODO are these two checks necessary?
	if self.TeamFactory == nil {
		return nil, errors.New("Missing team factory")
	}

	if self.Generator == nil {
		return nil, errors.New("Missing token generator")
	}

	if verifiedClaims.UserID == "" {
		return nil, errors.New("Missing user id in verified claims")
	}

	if verifiedClaims.ConnectorID == "" {
		return nil, errors.New("Missing connector id in verified claims")
	}

	dbTeams, err := self.TeamFactory.GetTeams()
	if err != nil {
		return nil, err
	}

	sub := verifiedClaims.Sub
	email := verifiedClaims.Email
	name := verifiedClaims.Name
	userId := verifiedClaims.UserID
	userName := verifiedClaims.UserName
	connectorId := verifiedClaims.ConnectorID
	claimGroups := verifiedClaims.Groups

	isAdmin := false
	teamSet := map[string]map[string]bool{}

	for _, team := range dbTeams {
		teamSet[team.Name()] = map[string]bool{}

		for role, auth := range team.Auth() {
			userAuth := auth["users"]
			groupAuth := auth["groups"]

			if len(userAuth) == 0 && len(groupAuth) == 0 {
				teamSet[team.Name()][role] = true
				isAdmin = isAdmin || team.Admin()
			}

			for _, user := range userAuth {
				if strings.EqualFold(user, connectorId+":"+userId) {
					teamSet[team.Name()][role] = true
					isAdmin = isAdmin || team.Admin()
				}
				if userName != "" {
					if strings.EqualFold(user, connectorId+":"+userName) {
						teamSet[team.Name()][role] = true
						isAdmin = isAdmin || team.Admin()
					}
				}
			}

			for _, group := range groupAuth {
				for _, claimGroup := range claimGroups {

					parts := strings.Split(claimGroup, ":")

					if len(parts) > 0 {
						// match the provider plus the org e.g. github:org-name
						if strings.EqualFold(group, connectorId+":"+parts[0]) {
							teamSet[team.Name()][role] = true
							isAdmin = isAdmin || team.Admin()
						}

						// match the provider plus the entire claim group e.g. github:org-name:team-name
						if strings.EqualFold(group, connectorId+":"+claimGroup) {
							teamSet[team.Name()][role] = true
							isAdmin = isAdmin || team.Admin()
						}
					}
				}
			}
		}
	}

	teams := map[string][]string{}
	for team, roles := range teamSet {
		rolesArray := []string{}
		for role := range roles {
			rolesArray = append(rolesArray, role)
		}
		roleComparator := func(i, j int) bool {
			return accessor.CompareRoles(rolesArray[i], rolesArray[j])
		}
		sort.Slice(rolesArray, roleComparator)
		teams[team] = rolesArray
	}

	if len(teams) == 0 {
		return nil, errors.New("user doesn't belong to any team")
	}

	return self.Generator.Generate(map[string]interface{}{
		"sub":       sub,
		"email":     email,
		"name":      name,
		"user_id":   userId,
		"user_name": userName,
		"teams":     teams,
		"is_admin":  isAdmin,
		"exp":       time.Now().Add(self.Duration).Unix(),
		"csrf":      RandomString(),
	})
}
