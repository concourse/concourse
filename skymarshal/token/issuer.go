package token

import (
	"errors"
	"strings"
	"time"

	"github.com/concourse/concourse/atc/db"
	"golang.org/x/oauth2"
)

//go:generate counterfeiter . Issuer
type Issuer interface {
	Issue(*VerifiedClaims) (*oauth2.Token, error)
}

func NewIssuer(teamFactory db.TeamFactory, generator Generator, duration time.Duration) Issuer {
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

func (i *issuer) Issue(verifiedClaims *VerifiedClaims) (*oauth2.Token, error) {
	if verifiedClaims.UserID == "" {
		return nil, errors.New("Missing user id in verified claims")
	}

	if verifiedClaims.ConnectorID == "" {
		return nil, errors.New("Missing connector id in verified claims")
	}

	dbTeams, err := i.TeamFactory.GetTeams()
	if err != nil {
		return nil, err
	}

	sub := verifiedClaims.Sub
	email := verifiedClaims.Email
	name := verifiedClaims.Name
	userID := verifiedClaims.UserID
	userName := verifiedClaims.UserName
	connectorID := verifiedClaims.ConnectorID
	claimGroups := verifiedClaims.Groups

	isAdmin := false
	teamSet := map[string]map[string]bool{}

	for _, team := range dbTeams {
		teamSet[team.Name()] = map[string]bool{}

		for role, auth := range team.Auth() {
			userAuth := auth["users"]
			groupAuth := auth["groups"]

			// backwards compatibility for allow-all-users
			if len(userAuth) == 0 && len(groupAuth) == 0 {
				teamSet[team.Name()][role] = true
				isAdmin = isAdmin || (team.Admin() && role == "owner")
			}

			for _, user := range userAuth {
				if strings.EqualFold(user, connectorID+":"+userID) {
					teamSet[team.Name()][role] = true
					isAdmin = isAdmin || (team.Admin() && role == "owner")
				}
				if userName != "" {
					if strings.EqualFold(user, connectorID+":"+userName) {
						teamSet[team.Name()][role] = true
						isAdmin = isAdmin || (team.Admin() && role == "owner")
					}
				}
			}

			for _, group := range groupAuth {
				for _, claimGroup := range claimGroups {

					parts := strings.Split(claimGroup, ":")

					if len(parts) > 0 {
						// match the provider plus the org e.g. github:org-name
						if strings.EqualFold(group, connectorID+":"+parts[0]) {
							teamSet[team.Name()][role] = true
							isAdmin = isAdmin || (team.Admin() && role == "owner")
						}

						// match the provider plus the entire claim group e.g. github:org-name:team-name
						if strings.EqualFold(group, connectorID+":"+claimGroup) {
							teamSet[team.Name()][role] = true
							isAdmin = isAdmin || (team.Admin() && role == "owner")
						}
					}
				}
			}
		}
	}

	teams := map[string][]string{}
	for team, roles := range teamSet {
		for role, _ := range roles {
			teams[team] = append(teams[team], role)
		}
	}

	if len(teams) == 0 {
		return nil, errors.New("user doesn't belong to any team")
	}

	return i.Generator.Generate(map[string]interface{}{
		"sub":       sub,
		"email":     email,
		"name":      name,
		"user_id":   userID,
		"user_name": userName,
		"teams":     teams,
		"is_admin":  isAdmin,
		"exp":       time.Now().Add(i.Duration).Unix(),
		"csrf":      RandomString(),
	})
}
