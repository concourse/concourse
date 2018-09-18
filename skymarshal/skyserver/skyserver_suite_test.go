package skyserver_test

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/skymarshal/skyserver"
	"github.com/concourse/skymarshal/token/tokenfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var (
	fakeTeamFactory   *dbfakes.FakeTeamFactory
	fakeTokenVerifier *tokenfakes.FakeVerifier
	fakeTokenIssuer   *tokenfakes.FakeIssuer
	skyServer         *httptest.Server
	dexServer         *ghttp.Server
	client            *http.Client
	cookieJar         *cookiejar.Jar
	signingKey        *rsa.PrivateKey
	config            *skyserver.SkyConfig
)

func TestSkyServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sky Server Suite")
}

var _ = BeforeEach(func() {
	var err error

	fakeTeam := new(dbfakes.FakeTeam)
	fakeTeam.IDReturns(734)

	fakeTeamFactory = new(dbfakes.FakeTeamFactory)
	fakeTeamFactory.FindTeamReturns(fakeTeam, true, nil)
	fakeTeamFactory.GetByIDReturns(fakeTeam)

	fakeTokenVerifier = new(tokenfakes.FakeVerifier)
	fakeTokenIssuer = new(tokenfakes.FakeIssuer)

	dexServer = ghttp.NewTLSServer()
	dexIssuerUrl := dexServer.URL() + "/sky/issuer"

	signingKey, err = rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())

	cookieJar, err = cookiejar.New(nil)
	Expect(err).ToNot(HaveOccurred())

	config = &skyserver.SkyConfig{
		Logger:          lagertest.NewTestLogger("sky"),
		TokenVerifier:   fakeTokenVerifier,
		TokenIssuer:     fakeTokenIssuer,
		DexClientID:     "dex-client-id",
		DexClientSecret: "dex-client-secret",
		DexIssuerURL:    dexIssuerUrl,
		DexHttpClient:   dexServer.HTTPTestServer.Client(),
		SigningKey:      signingKey,
	}

	server, err := skyserver.NewSkyServer(config)
	Expect(err).NotTo(HaveOccurred())

	skyServer = httptest.NewUnstartedServer(skyserver.NewSkyHandler(server))
})

var _ = JustBeforeEach(func() {
	client = skyServer.Client()
	client.Jar = cookieJar
})

var _ = AfterEach(func() {
	skyServer.Close()
	dexServer.Close()
})
