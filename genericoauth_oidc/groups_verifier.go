package genericoauth_oidc

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"golang.org/x/oauth2"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/skymarshal/verifier"
	"github.com/dgrijalva/jwt-go"
)

type GroupsVerifier struct {
	UID              []string
	groups           []string
	customGroupsName string
}

func NewGroupsVerifier(
	UID              []string,
	groups           []string,
	customGroupsName string,
) verifier.Verifier {
	return GroupsVerifier{
		UID: UID,
		groups: groups,
		customGroupsName: customGroupsName,
	}
}

type GenericOAuthOIDCToken struct {
	UID    string   `json:"sub"`
	Groups []string `json:"-"`
}

func (verifier GroupsVerifier) Verify(logger lager.Logger, httpClient *http.Client) (bool, error) {
	oauth2Transport, ok := httpClient.Transport.(*oauth2.Transport)
	if !ok {
		return false, errors.New("httpClient transport must be of type oauth2.Transport")
	}

	token, err := oauth2Transport.Source.Token()
	if err != nil {
		return false, err
	}

	idToken := token.Extra("id_token")
	var tokenParts []string
	if idToken != nil {
		tokenParts = strings.Split(idToken.(string), ".")
		if len(tokenParts) < 3 {
			return false, errors.New("id token contains an invalid number of segments")
		}
	} else {
		return false, errors.New("could not find id_token in token response")
	}

	decodedClaims, err := jwt.DecodeSegment(tokenParts[1])
	if err != nil {
		return false, err
	}

	var oauthOIDCToken GenericOAuthOIDCToken
	err = json.Unmarshal(decodedClaims, &oauthOIDCToken)
	if err != nil {
		return false, err
	}
	
	groupName := "groups"
	if verifier.customGroupsName != "" {
		groupName = verifier.customGroupsName
	}

	var claimMap map[string]*json.RawMessage
	err = json.Unmarshal(decodedClaims, &claimMap)
	if err != nil {
		return false, err
	}

	if jsonGroups, ok := claimMap[groupName]; ok {
		var groupsObj interface{}
		var groups []string
		err = json.Unmarshal(*jsonGroups, &groupsObj)
		if err != nil {
			return false, err
		}
		if groupsArr, ok := groupsObj.([]interface{}); ok {
			for _, group := range groupsArr {
				groups = append(groups, group.(string))
			}
		} else if group, ok := groupsObj.(string); ok {
			groups = append(groups, group)
		} else {
			return false, errors.New("Could not parse groups from JWT response")
		}
		oauthOIDCToken.Groups = groups
	} else {
		oauthOIDCToken.Groups = []string{}
	}

	for _, uid := range verifier.UID {
		if oauthOIDCToken.UID == uid {
			logger.Info("oauth-successful-authentication-uid", lager.Data{
				"user": oauthOIDCToken.UID,
			})
			return true, nil
		}
	}

	for _, userGroup := range oauthOIDCToken.Groups {
		for _, verifierGroup := range verifier.groups {
			if userGroup == verifierGroup {
				logger.Info("oauth-successful-authentication-groups", lager.Data{
					"group": userGroup,
					"user": oauthOIDCToken.UID,
				})
				return true, nil
			}
		}
	}

	logger.Info("does-not-belong-to-group", lager.Data{
		"have": oauthOIDCToken.Groups,
		"want": verifier.groups,
		"user": oauthOIDCToken.UID,
	})

	return false, nil
}
