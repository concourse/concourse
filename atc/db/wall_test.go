package db_test

import (
	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("Wall", func() {
	var (
		noExpiryMsg      = atc.Wall{Message: "this is a test message!"}
		testMsg          = noExpiryMsg
		msgExpiresIn1Min = atc.Wall{Message: "this is a test message!", TTL: time.Minute}
		msgExpiresIn1Ms  = atc.Wall{Message: "this is a test message!", TTL: time.Millisecond}
	)

	Context(" a message is set", func() {
		It("with no expiration", func() {
			err := dbWall.SetWall(noExpiryMsg)
			Expect(err).ToNot(HaveOccurred())

			wall, err := dbWall.GetWall()
			Expect(err).ToNot(HaveOccurred())
			Expect(wall.Message).To(Equal(noExpiryMsg.Message))
			Expect(wall.TTL).To(Equal(time.Duration(0)))
		})
		Context("with an expiration", func() {
			It("the message has not expired", func() {
				err := dbWall.SetWall(msgExpiresIn1Min)
				Expect(err).ToNot(HaveOccurred())

				wall, err := dbWall.GetWall()
				Expect(err).ToNot(HaveOccurred())
				Expect(wall.Message).To(Equal(msgExpiresIn1Min.Message))
				Expect(wall.TTL > 0).To(Equal(true), "TTL is set")
			})

			It("the message has expired", func() {
				err := dbWall.SetWall(msgExpiresIn1Ms)
				Expect(err).ToNot(HaveOccurred())

				time.Sleep(time.Millisecond)

				wall, err := dbWall.GetWall()
				Expect(err).ToNot(HaveOccurred())
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
			dbWall.SetWall(testMsg)
			wall, err := dbWall.GetWall()
			Expect(err).ToNot(HaveOccurred())
			Expect(wall).To(Equal(testMsg), "check: ensure the message has been set before proceeding")

			err = dbWall.Clear()
			Expect(err).ToNot(HaveOccurred())
			wall, err = dbWall.GetWall()
			Expect(err).ToNot(HaveOccurred())
			Expect(wall.Message).To(Equal(""))
			Expect(wall.TTL).To(Equal(time.Duration(0)))
		})
	})
})
