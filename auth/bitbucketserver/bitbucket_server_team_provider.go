package bitbucketserver

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/auth/verifier"
	"github.com/dghubble/oauth1"
	"github.com/jessevdk/go-flags"
	"strings"
)

const ProviderName = "bitbucket-server"
const DisplayName = "Bitbucket Server"

var Scopes = []string{"team"}

func init() {
	provider.Register(ProviderName, BitbucketServerTeamProvider{})
}

type BitbucketServerTeamProvider struct {
}

func (BitbucketServerTeamProvider) AddAuthGroup(group *flags.Group) provider.AuthConfig {
	flags := &BitbucketServerAuthConfig{}

	bGroup, err := group.AddGroup("Bitbucket Server Authentication", "", flags)
	if err != nil {
		panic(err)
	}

	bGroup.Namespace = "bitbucket-server-auth"

	return flags
}

func (BitbucketServerTeamProvider) ProviderConstructor(config provider.AuthConfig, redirectURL string) (provider.Provider, bool) {
	bitbucketAuth := config.(*BitbucketServerAuthConfig)

	block, _ := pem.Decode([]byte(bitbucketAuth.PrivateKey))

	var rsa *rsa.PrivateKey
	var err error
	switch block.Type {
	case "RSA PRIVATE KEY":
		rsa, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, false
		}
	}

	endpoint := oauth1.Endpoint{
		RequestTokenURL: strings.TrimRight(bitbucketAuth.Endpoint, "/") + "/plugins/servlet/oauth/request-token",
		AuthorizeURL:    strings.TrimRight(bitbucketAuth.Endpoint, "/") + "/plugins/servlet/oauth/authorize",
		AccessTokenURL:  strings.TrimRight(bitbucketAuth.Endpoint, "/") + "/plugins/servlet/oauth/access-token",
	}

	return &BitbucketServerProvider{
		Verifier: verifier.NewVerifierBasket(
			NewUserVerifier(bitbucketAuth.Users),
		),
		Config: &oauth1.Config{
			ConsumerKey: bitbucketAuth.ConsumerKey,
			CallbackURL: redirectURL,
			Endpoint:    endpoint,
			Signer: &oauth1.RSASigner{
				PrivateKey: rsa,
			},
		},
		secrets: make(map[string]string),
	}, true
}

func (BitbucketServerTeamProvider) UnmarshalConfig(config *json.RawMessage) (provider.AuthConfig, error) {
	flags := &BitbucketServerAuthConfig{}
	if config != nil {
		err := json.Unmarshal(*config, &flags)
		if err != nil {
			return nil, err
		}
	}
	return flags, nil
}
