package db_test

import (
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Wall", func() {
	var (
		msgOnly    = atc.Wall{Message: "this is a test message!"}
		msgWithTTL = atc.Wall{Message: "this is a test message!", TTL: time.Minute}
		startTime  = time.Now()
	)

	Context(" a message is set", func() {
		BeforeEach(func() {
			fakeClock = dbfakes.FakeClock{}
			fakeClock.NowReturns(startTime)
		})
		Context("with no expiration", func() {
			It("successfully sets the wall", func() {
				err := dbWall.SetWall(msgOnly)
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeClock.NowCallCount()).To(Equal(0))
			})

			It("successfully gets the wall", func() {
				_ = dbWall.SetWall(msgOnly)
				actualWall, err := dbWall.GetWall()
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeClock.NowCallCount()).To(Equal(1))
				Expect(actualWall).To(Equal(msgOnly))
			})

		})
		Context("with an expiration", func() {
			It("successfully sets the wall", func() {
				err := dbWall.SetWall(msgWithTTL)
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeClock.NowCallCount()).To(Equal(1))
			})

			Context("the message has not expired", func() {
				Context("and gets a wall", func() {
					BeforeEach(func() {
						fakeClock.NowReturns(startTime.Add(time.Second))
						fakeClock.UntilReturns(30 * time.Second)
					})

					Specify("successfully", func() {
						_ = dbWall.SetWall(msgWithTTL)
						_, err := dbWall.GetWall()
						Expect(err).ToNot(HaveOccurred())
						Expect(fakeClock.NowCallCount()).To(Equal(2))
						Expect(fakeClock.UntilCallCount()).To(Equal(1))
					})

					Specify("with the TTL field set", func() {
						_ = dbWall.SetWall(msgWithTTL)

						actualWall, _ := dbWall.GetWall()
						msgWithTTL.TTL = 30 * time.Second
						Expect(actualWall).To(Equal(msgWithTTL))
					})
				})
			})

			Context("the message has expired", func() {
				It("returns no message", func() {
					_ = dbWall.SetWall(msgWithTTL)
					fakeClock.NowReturns(startTime.Add(time.Hour))

					actualWall, err := dbWall.GetWall()
					Expect(err).ToNot(HaveOccurred())
					Expect(fakeClock.NowCallCount()).To(Equal(2))
					Expect(actualWall).To(Equal(atc.Wall{}))
				})
			})
		})
	})

	Context("multiple messages are set", func() {
		It("returns the last message that was set", func() {
			expectedWall := atc.Wall{Message: "test 3"}
			dbWall.SetWall(atc.Wall{Message: "test 1"})
			dbWall.SetWall(atc.Wall{Message: "test 2"})
			dbWall.SetWall(expectedWall)

			actualWall, err := dbWall.GetWall()
			Expect(err).ToNot(HaveOccurred())
			Expect(actualWall).To(Equal(expectedWall))
		})
	})

	Context("clearing the wall", func() {
		BeforeEach(func() {
			dbWall.SetWall(msgOnly)
			actualWall, err := dbWall.GetWall()
			Expect(err).ToNot(HaveOccurred())
			Expect(actualWall).To(Equal(msgOnly), "ensure the message has been set before proceeding")
		})
		It("returns no error", func() {
			err := dbWall.Clear()
			Expect(err).ToNot(HaveOccurred())
		})
		It("GetWall returns no message after clearing the wall", func() {
			_ = dbWall.Clear()

			actualWall, err := dbWall.GetWall()
			Expect(err).ToNot(HaveOccurred())
			Expect(actualWall).To(Equal(atc.Wall{}))
		})
	})
})
