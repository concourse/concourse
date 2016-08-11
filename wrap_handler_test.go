package auth_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/authfakes"
)

var _ = Describe("WrapHandler", func() {
	var (
		fakeValidator         *authfakes.FakeValidator
		fakeUserContextReader *authfakes.FakeUserContextReader

		server *httptest.Server
		client *http.Client

		authenticated   <-chan bool
		teamNameChan    <-chan string
		teamIDChan      <-chan int
		isAdminChan     <-chan bool
		isSystemChan    <-chan bool
		foundChan       <-chan bool
		systemFoundChan <-chan bool
	)

	BeforeEach(func() {
		fakeValidator = new(authfakes.FakeValidator)
		fakeUserContextReader = new(authfakes.FakeUserContextReader)

		a := make(chan bool, 1)
		tn := make(chan string, 1)
		ti := make(chan int, 1)
		ia := make(chan bool, 1)
		is := make(chan bool, 1)
		f := make(chan bool, 1)
		sf := make(chan bool, 1)

		authenticated = a
		teamNameChan = tn
		teamIDChan = ti
		isAdminChan = ia
		isSystemChan = is
		foundChan = f
		systemFoundChan = sf
		simpleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			a <- auth.IsAuthenticated(r)
			teamName, teamID, isAdmin, found := auth.GetTeam(r)
			isSystem, systemFound := r.Context().Value("system").(bool)

			f <- found
			sf <- systemFound
			tn <- teamName
			ti <- teamID
			ia <- isAdmin
			if systemFound {
				is <- isSystem
			}
		})

		server = httptest.NewServer(auth.WrapHandler(
			simpleHandler,
			fakeValidator,
			fakeUserContextReader,
		))

		client = &http.Client{
			Transport: &http.Transport{},
		}
	})

	Context("when a request is made", func() {
		var request *http.Request
		var response *http.Response

		BeforeEach(func() {
			var err error

			request, err = http.NewRequest("GET", server.URL, bytes.NewBufferString("hello"))
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the validator returns true", func() {
			BeforeEach(func() {
				fakeValidator.IsAuthenticatedReturns(true)
			})

			It("handles the request with the request authenticated", func() {
				Expect(<-authenticated).To(BeTrue())
			})
		})

		Context("when the validator returns false", func() {
			BeforeEach(func() {
				fakeValidator.IsAuthenticatedReturns(false)
			})

			It("handles the request with the request authenticated", func() {
				Expect(<-authenticated).To(BeFalse())
			})
		})

		Context("when the userContextReader finds team information", func() {
			BeforeEach(func() {
				fakeUserContextReader.GetTeamReturns("some-team", 9, true, true)
			})

			It("passes the team information along in the request object", func() {
				Expect(<-foundChan).To(BeTrue())
				Expect(<-teamNameChan).To(Equal("some-team"))
				Expect(<-teamIDChan).To(Equal(9))
				Expect(<-isAdminChan).To(BeTrue())
			})
		})

		Context("when the userContextReader does not find team information", func() {
			BeforeEach(func() {
				fakeUserContextReader.GetTeamReturns("", 0, false, false)
			})

			It("does not pass team information along in the request object", func() {
				Expect(<-foundChan).To(BeFalse())
			})
		})

		Context("when the userContextReader finds system information", func() {
			BeforeEach(func() {
				fakeUserContextReader.GetSystemReturns(true, true)
			})

			It("passes the system information along in the request object", func() {
				Expect(<-systemFoundChan).To(BeTrue())
				Expect(<-isSystemChan).To(BeTrue())
			})
		})

		Context("when the userContextReader does not find system information", func() {
			BeforeEach(func() {
				fakeUserContextReader.GetSystemReturns(false, false)
			})

			It("does not pass the system information along in the request object", func() {
				Expect(<-systemFoundChan).To(BeFalse())
			})
		})
	})
})
