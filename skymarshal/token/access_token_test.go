package token_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/skymarshal/token"
	"github.com/concourse/concourse/skymarshal/token/tokenfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/square/go-jose.v2/jwt"
)

var _ = Describe("Access Tokens", func() {

	Describe("StoreAccessToken", func() {
		var (
			generator          *tokenfakes.FakeGenerator
			claimsParser       *tokenfakes.FakeClaimsParser
			accessTokenFactory *dbfakes.FakeAccessTokenFactory
			userFactory        *dbfakes.FakeUserFactory

			dummyLogger *lagertest.TestLogger
		)

		BeforeEach(func() {
			generator = new(tokenfakes.FakeGenerator)
			claimsParser = new(tokenfakes.FakeClaimsParser)
			accessTokenFactory = new(dbfakes.FakeAccessTokenFactory)
			userFactory = new(dbfakes.FakeUserFactory)

			dummyLogger = lagertest.NewTestLogger("whatever")
		})

		type testCase struct {
			it string

			path       string
			statusCode int
			body       string

			parseClaimsErrors   bool
			generateTokenErrors bool
			storeTokenErrors    bool
			storeUserErrors     bool

			expectStatusCode int
			expectBody       string
		}

		for _, t := range []testCase{
			{
				it: "forwards non-token requests",

				path:       "/sky/issuer/callback",
				statusCode: 200,
				body:       "some payload",

				expectStatusCode: 200,
				expectBody:       "some payload",
			},
			{
				it: "modifies the access token",

				path:       "/sky/issuer/token",
				statusCode: 200,
				body:       `{"access_token":"123","token_type":"bearer","expires_in":1234,"id_token":"a.b.c"}`,

				expectStatusCode: 200,
				expectBody:       `{"access_token":"123abc","token_type":"bearer","expires_in":1234,"id_token":"a.b.c"}`,
			},
			{
				it: "forwards failure response",

				path:       "/sky/issuer/token",
				statusCode: 418,
				body:       "i've made a huge mistake",

				expectStatusCode: 418,
				expectBody:       "i've made a huge mistake",
			},
			{
				it: "errors if parsing claims fails",

				path:       "/sky/issuer/token",
				statusCode: 200,
				body:       `{"access_token":"123","token_type":"bearer","expires_in":1234,"id_token":"invalid"}`,

				parseClaimsErrors: true,

				expectStatusCode: 500,
			},
			{
				it: "errors if generating token fails",

				path:       "/sky/issuer/token",
				statusCode: 200,
				body:       `{"access_token":"123","token_type":"bearer","expires_in":1234,"id_token":"a.b.c"}`,

				generateTokenErrors: true,

				expectStatusCode: 500,
			},
			{
				it: "errors if storing token fails",

				path:       "/sky/issuer/token",
				statusCode: 200,
				body:       `{"access_token":"123","token_type":"bearer","expires_in":1234,"id_token":"a.b.c"}`,

				storeTokenErrors: true,

				expectStatusCode: 500,
			},
			{
				it: "errors if storing user fails",

				path:       "/sky/issuer/token",
				statusCode: 200,
				body:       `{"access_token":"123","token_type":"bearer","expires_in":1234,"id_token":"a.b.c"}`,

				storeUserErrors: true,

				expectStatusCode: 500,
			},
		} {
			t := t

			It(t.it, func() {
				baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(t.statusCode)
					w.Write([]byte(t.body))
				})
				handler := token.StoreAccessToken(dummyLogger, baseHandler, generator, claimsParser, accessTokenFactory, userFactory)
				r, _ := http.NewRequest("GET", t.path, nil)
				rec := httptest.NewRecorder()

				if t.parseClaimsErrors {
					claimsParser.ParseClaimsReturns(db.Claims{}, errors.New("claims parse error"))
				}

				if t.generateTokenErrors {
					generator.GenerateAccessTokenReturns("", errors.New("generate error"))
				} else {
					generator.GenerateAccessTokenReturns("123abc", nil)
				}

				if t.storeTokenErrors {
					accessTokenFactory.CreateAccessTokenReturns(errors.New("store token error"))
				}

				if t.storeUserErrors {
					userFactory.CreateOrUpdateUserReturns(errors.New("upsert user error"))
				}

				handler.ServeHTTP(rec, r)

				result := rec.Result()
				Expect(result.StatusCode).To(Equal(t.expectStatusCode))
				Expect(rec.Body.String()).To(Equal(t.expectBody))
			})
		}
	})

	Describe("Token Generation", func() {
		It("generates a token with the unix timestamp", func() {
			factory := token.Factory{}
			expectExpiry := jwt.NewNumericDate(time.Now())
			rawToken, err := factory.GenerateAccessToken(db.Claims{
				Claims: jwt.Claims{Expiry: expectExpiry},
			})
			Expect(err).ToNot(HaveOccurred())
			expiry, err := factory.ParseExpiry(rawToken)
			Expect(err).ToNot(HaveOccurred())

			Expect(expiry).To(Equal(expectExpiry.Time()))
		})
	})
})
