package uaa

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/oauth2"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/urljoiner"
	"github.com/dgrijalva/jwt-go"
)

type SpaceVerifier struct {
	spaceGUIDs []string
	cfAPIURL   string
}

func NewSpaceVerifier(
	spaceGUIDs []string,
	cfAPIURL string,
) SpaceVerifier {
	return SpaceVerifier{
		spaceGUIDs: spaceGUIDs,
		cfAPIURL:   cfAPIURL,
	}
}

type UAAToken struct {
	UserID string `json:"user_id"`
}

type CFSpaceDevelopersResponse struct {
	NextUrl   string       `json:"next_url"`
	UserInfos []CFUserInfo `json:"resources"`
}

type CFUserInfo struct {
	Metadata struct {
		GUID string `json:"guid"`
	} `json:"metadata"`
}

func (verifier SpaceVerifier) Verify(logger lager.Logger, httpClient *http.Client) (bool, error) {
	oauth2Transport, ok := httpClient.Transport.(*oauth2.Transport)
	if !ok {
		return false, errors.New("httpClient transport must be of type oauth2.Transport")
	}

	token, err := oauth2Transport.Source.Token()
	if err != nil {
		return false, err
	}

	tokenParts := strings.Split(token.AccessToken, ".")
	if len(tokenParts) < 2 {
		return false, errors.New("access token contains an invalid number of segments")
	}

	decodedClaims, err := jwt.DecodeSegment(tokenParts[1])
	if err != nil {
		return false, err
	}

	var uaaToken UAAToken
	err = json.Unmarshal(decodedClaims, &uaaToken)
	if err != nil {
		return false, err
	}

	if uaaToken.UserID == "" {
		return false, fmt.Errorf("not able to retrieve 'user_id' property from UAA access token")
	}

	for _, verifierSpaceGUID := range verifier.spaceGUIDs {
		spaceURL := urljoiner.Join(verifier.cfAPIURL, "v2", "spaces", verifierSpaceGUID, "developers?results-per-page=100")

		hasAccess, nextUrl, err := verifier.isSpaceDeveloper(logger, httpClient, spaceURL, uaaToken.UserID)
		if err != nil {
			return false, err
		}

		if hasAccess {
			return true, nil
		}

		for nextUrl != "" {
			spaceURL = urljoiner.Join(verifier.cfAPIURL, nextUrl)
			hasAccess, nextUrl, err = verifier.isSpaceDeveloper(logger, httpClient, spaceURL, uaaToken.UserID)
			if err != nil {
				return false, err
			}

			if hasAccess {
				return true, nil
			}
		}
	}

	logger.Info("not-in-spaces", lager.Data{
		"want": verifier.spaceGUIDs,
	})

	return false, nil
}

func (verifier SpaceVerifier) isSpaceDeveloper(
	logger lager.Logger,
	httpClient *http.Client,
	cfApiURL string,
	userGUID string,
) (bool, string, error) {
	logger.Info("cf-request", lager.Data{"url": cfApiURL})
	response, err := httpClient.Get(cfApiURL)
	if err != nil {
		return false, "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		logger.Info("unexpected response from CF API URL", lager.Data{
			"statusCode": response.StatusCode,
			"status":     response.Status,
		})
		return false, "", nil
	}

	var cfSpaceDevelopersResponse CFSpaceDevelopersResponse
	err = json.NewDecoder(response.Body).Decode(&cfSpaceDevelopersResponse)

	for _, userInfo := range cfSpaceDevelopersResponse.UserInfos {
		if userInfo.Metadata.GUID == userGUID {
			return true, cfSpaceDevelopersResponse.NextUrl, nil
		}
	}

	return false, cfSpaceDevelopersResponse.NextUrl, nil
}
