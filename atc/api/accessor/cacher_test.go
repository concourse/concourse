package accessor_test

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cacher", func() {
	var (
		fakeNotifications *accessorfakes.FakeNotifications
		fakeTeamFactory   *dbfakes.FakeTeamFactory
		fetchedTeams      []db.Team
		fakeTeam          *dbfakes.FakeTeam

		teamFetcher accessor.TeamFetcher
		notifier    chan bool
		teams       []db.Team
		err         error
	)

	BeforeEach(func() {
		fakeTeam = new(dbfakes.FakeTeam)
		teams = []db.Team{fakeTeam}

		notifier = make(chan bool, 1)
		fakeNotifications = new(accessorfakes.FakeNotifications)
		fakeNotifications.ListenReturns(notifier, nil)
		fakeTeamFactory = new(dbfakes.FakeTeamFactory)
		fakeTeamFactory.GetTeamsReturns(teams, nil)
	})

	JustBeforeEach(func() {
		teamFetcher = accessor.NewCacher(lager.NewLogger("test"), fakeNotifications, fakeTeamFactory)
		fetchedTeams, err = teamFetcher.GetTeams()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when there is no cache found", func() {
		It("fetch teams from DB once", func() {
			Expect(fakeTeamFactory.GetTeamsCallCount()).To(Equal(1))
			Expect(fetchedTeams).To(Equal(teams))
		})
	})

	Context("when there is cache found", func() {
		JustBeforeEach(func() {
			fetchedTeams, err = teamFetcher.GetTeams()
			Expect(err).NotTo(HaveOccurred())
		})

		It("does not fetch teams from DB again but read it from cache", func() {
			Expect(fakeTeamFactory.GetTeamsCallCount()).To(Equal(1))
			Expect(fetchedTeams).To(Equal(teams))
		})
	})

	Context("when it receives a notification", func() {
		JustBeforeEach(func() {
			notifier <- true
			time.Sleep(time.Second)
		})

		It("fetch teams again from DB since cache is deleted", func() {
			fetchedTeams, err = teamFetcher.GetTeams()
			Eventually(fakeTeamFactory.GetTeamsCallCount()).Should(Equal(2))
		})
	})
})
