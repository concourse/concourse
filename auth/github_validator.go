package auth

import (
	"net/http"
	"strings"

	"github.com/octokit/go-octokit/octokit"
	"github.com/pivotal-golang/lager"
)

type GitHubOrganizationValidator struct {
	Organization string
	Client       GitHubClient
}

func (validator GitHubOrganizationValidator) IsAuthenticated(r *http.Request) bool {
	authorizationHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authorizationHeader, "Token ") {
		return false
	}
	accessToken := authorizationHeader[6:]

	orgs, err := validator.Client.GetOrganizations(accessToken)
	if err != nil {
		return false
	}

	for _, org := range orgs {
		if org == validator.Organization {
			return true
		}
	}

	return false
}

//go:generate counterfeiter . GitHubClient

type GitHubClient interface {
	GetOrganizations(accessToken string) ([]string, error)
}

type gitHubClient struct {
	logger lager.Logger
}

func NewGitHubClient(logger lager.Logger) GitHubClient {
	return &gitHubClient{
		logger: logger,
	}
}

func (ghc *gitHubClient) GetOrganizations(accessToken string) ([]string, error) {
	tokenAuth := octokit.TokenAuth{AccessToken: accessToken}
	client := octokit.NewClientWith("https://api.github.com/", "concourse-api", tokenAuth, &http.Client{
		Transport: &http.Transport{},
	})

	orgs, result := client.Organization().YourOrganizations(&octokit.YourOrganizationsURL, octokit.M{})
	if result.Err != nil {
		ghc.logger.Error("failed-to-get-github-organizations", result.Err)
		return nil, result.Err
	}

	organizations := []string{}
	for _, org := range orgs {
		organizations = append(organizations, org.Login)
	}
	return organizations, nil
}
