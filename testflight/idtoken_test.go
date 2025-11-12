package testflight_test

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/concourse/concourse/atc/creds/idtoken"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A pipeline containing idtoken var sources", Ordered, func() {

	var testPipelineName string
	var jwks *jose.JSONWebKeySet
	var outputText []byte

	type claimStruct struct {
		jwt.Claims
		Team     string `json:"team"`
		Pipeline string `json:"pipeline"`
	}

	BeforeAll(func() {
		testPipelineName = pipelineName
		var err error
		jwks, err = getJWKS(config.ATCURL)
		Expect(err).ToNot(HaveOccurred())

		setAndUnpausePipeline("fixtures/idtoken.yml")
		watch := fly("trigger-job", "-j", inPipeline("print-creds"), "-w")
		Expect(watch).To(gexec.Exit(0))
		outputText = watch.Buffer().Contents()
	})

	It("creates valid default idtoken", func() {
		token := extractIDtokenFromBuffer(outputText, "default-token")
		Expect(token).ToNot(BeEmpty())

		parsed, err := jwt.ParseSigned(token, []jose.SignatureAlgorithm{idtoken.DefaultAlgorithm})
		Expect(err).ToNot(HaveOccurred())
		var claims claimStruct
		err = parsed.Claims(jwks, &claims)
		Expect(err).To(Succeed())

		Expect(claims.Audience).To(ContainElement("sts.amazonaws.com"))
		Expect(claims.Team).To(Equal(teamName))
		Expect(claims.Pipeline).To(Equal(testPipelineName))
		Expect(claims.Subject).To(Equal(teamName + "/" + testPipelineName))
	})

	It("creates valid custom idtoken", func() {
		token := extractIDtokenFromBuffer(outputText, "custom-token")
		Expect(token).ToNot(BeEmpty())

		parsed, err := jwt.ParseSigned(token, []jose.SignatureAlgorithm{jose.ES256})
		Expect(err).ToNot(HaveOccurred())
		var claims claimStruct
		err = parsed.Claims(jwks, &claims)
		Expect(err).To(Succeed())

		Expect(claims.Audience).To(ContainElement("sts.amazonaws.com"))
		Expect(claims.Team).To(Equal(teamName))
		Expect(claims.Pipeline).To(Equal(testPipelineName))

		Expect(parsed.Headers[0].Algorithm).To(Equal("ES256"))
		Expect(claims.Subject).To(Equal(teamName))
	})

	It("publishes correct issuer in OpenID configuration", func() {
		config, err := getOpenIDConfiguration(config.ATCURL)
		Expect(err).ToNot(HaveOccurred())

		issuer, ok := config["issuer"].(string)
		Expect(ok).To(BeTrue())
		Expect(issuer).ToNot(BeEmpty())

		jwksURI, ok := config["jwks_uri"].(string)
		Expect(ok).To(BeTrue())
		Expect(jwksURI).To(Equal(issuer + "/.well-known/jwks.json"))
	})
})

func extractIDtokenFromBuffer(buffer []byte, whichToken string) string {
	tokenMatcher := regexp.MustCompile("(?m)" + whichToken + ": (.*)$")
	tokenMatches := tokenMatcher.FindSubmatch(buffer)
	if len(tokenMatches) != 2 {
		return ""
	}
	return string(tokenMatches[1])
}

func getOpenIDConfiguration(atcURL string) (map[string]interface{}, error) {
	resp, err := http.Get(atcURL + "/.well-known/openid-configuration")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var config map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&config)
	return config, err
}

func getJWKS(atcurl string) (*jose.JSONWebKeySet, error) {
	url, err := getJWKSURL(atcurl)
	if err != nil {
		return nil, err
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	var jwks jose.JSONWebKeySet

	err = json.NewDecoder(resp.Body).Decode(&jwks)
	if err != nil {
		return nil, err
	}

	return &jwks, nil
}

func getJWKSURL(atcurl string) (string, error) {
	url := atcurl + "/.well-known/openid-configuration"
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	type oidcConfig struct {
		JWKSURI string `json:"jwks_uri"`
	}

	var conf oidcConfig

	err = json.NewDecoder(resp.Body).Decode(&conf)
	if err != nil {
		return "", err
	}

	return conf.JWKSURI, nil
}
