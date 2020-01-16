package db_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
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
		It("with no expiration", func() {
			err := dbWall.SetWall(msgOnly)
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeClock.NowCallCount()).To(Equal(0))

			wall, err := dbWall.GetWall()
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeClock.NowCallCount()).To(Equal(1))
			Expect(wall.Message).To(Equal(msgOnly.Message))
			Expect(wall.TTL).To(Equal(time.Duration(0)))
		})
		Context("with an expiration", func() {
			It("the message has not expired", func() {
				err := dbWall.SetWall(msgWithTTL)
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeClock.NowCallCount()).To(Equal(1))

				fakeClock.NowReturns(startTime.Add(time.Second))
				fakeClock.UntilReturns(30 * time.Second)
				wall, err := dbWall.GetWall()
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeClock.NowCallCount()).To(Equal(2))
				Expect(fakeClock.UntilCallCount()).To(Equal(1))
				Expect(wall.Message).To(Equal(msgWithTTL.Message))
				Expect(wall.TTL > 0).To(Equal(true), "TTL is set")
			})

			It("the message has expired", func() {
				err := dbWall.SetWall(msgWithTTL)
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeClock.NowCallCount()).To(Equal(1))

				fakeClock.NowReturns(startTime.Add(time.Hour))
				wall, err := dbWall.GetWall()
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeClock.NowCallCount()).To(Equal(2))
				Expect(wall.Message).To(Equal(""))
				Expect(wall.TTL).To(Equal(time.Duration(0)))
			})
		})
	})

	Context("multiple messages are set", func() {
		It("returns the last message that was set", func() {
			dbWall.SetWall(atc.Wall{Message: "test 1"})
			dbWall.SetWall(atc.Wall{Message: "test 2"})
			dbWall.SetWall(atc.Wall{Message: "test 3"})

			wall, err := dbWall.GetWall()
			Expect(err).ToNot(HaveOccurred())
			Expect(wall.Message).To(Equal("test 3"))
			Expect(wall.TTL).To(Equal(time.Duration(0)))
		})
	})

	Context("clear the message", func() {
		It("returns no message", func() {
			dbWall.SetWall(msgOnly)
			wall, err := dbWall.GetWall()
			Expect(err).ToNot(HaveOccurred())
			Expect(wall).To(Equal(msgOnly), "check: ensure the message has been set before proceeding")

			err = dbWall.Clear()
			Expect(err).ToNot(HaveOccurred())
			wall, err = dbWall.GetWall()
			Expect(err).ToNot(HaveOccurred())
			Expect(wall.Message).To(Equal(""))
			Expect(wall.TTL).To(Equal(time.Duration(0)))
		})
	})
})
