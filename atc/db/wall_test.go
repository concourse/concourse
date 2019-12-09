package db_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("Wall", func() {
	var (
		testMessage string
	)

	BeforeEach(func() {
		testMessage = "this is the test message!?"
	})

	Context(" a message is set", func() {
		BeforeEach(func() {
			err := wall.SetMessage(testMessage)
			Expect(err).ToNot(HaveOccurred())
		})

		It("with no expiration", func() {
			message, err := wall.GetMessage()
			Expect(err).ToNot(HaveOccurred())
			Expect(message).To(Equal(testMessage))
		})
		Context("with an expiration", func() {
			It("the message has not expired", func() {
				err := wall.SetExpiration(time.Minute)
				Expect(err).ToNot(HaveOccurred())

				message, err := wall.GetMessage()
				Expect(err).ToNot(HaveOccurred())
				Expect(message).To(Equal(testMessage))
			})

			It("the message has expired", func() {
				err := wall.SetExpiration(time.Millisecond)
				Expect(err).ToNot(HaveOccurred())

				time.Sleep(time.Millisecond)

				message, err := wall.GetMessage()
				Expect(err).ToNot(HaveOccurred())
				Expect(message).To(Equal(""))
			})
		})
	})

	Context("multiple messages are set", func() {
		It("returns the last message that was set", func() {
			wall.SetMessage("test 1")
			wall.SetMessage("test 2")
			wall.SetMessage("test 3")

			msg, err := wall.GetMessage()
			Expect(err).ToNot(HaveOccurred())
			Expect(msg).To(Equal("test 3"))
		})
	})

	Context("clear the message", func() {
		It("returns no message", func() {
			wall.SetMessage(testMessage)
			msg, err := wall.GetMessage()
			Expect(err).ToNot(HaveOccurred())
			Expect(msg).To(Equal(testMessage), "ensure the message has been set")

			wall.Clear()
			msg, err = wall.GetMessage()
			Expect(err).ToNot(HaveOccurred())
			Expect(msg).To(Equal(""))
		})
	})

	Context("expiration", func() {
		Context("when a message is set", func() {
			Context("the expiration is set", func() {
				It("returns the expiration", func() {
					wall.SetMessage(testMessage)
					wall.SetExpiration(time.Minute)
					expectedExpiration := time.Now().Add(time.Minute)

					expiration, err := wall.GetExpiration()
					Expect(err).ToNot(HaveOccurred())
					// Compare time as strings because otherwise they're always off by a few nanoseconds
					Expect(expiration.Format(time.ANSIC)).To(Equal(expectedExpiration.Format(time.ANSIC)))
				})
			})

			Context("the expiration is not set", func() {
				It("returns the zero value of time", func() {
					expiration, err := wall.GetExpiration()
					Expect(err).ToNot(HaveOccurred())
					Expect(expiration.IsZero()).To(Equal(true))
				})
			})
		})

		Context("when no message is set", func() {
			It("no error is returned", func() {
				err := wall.SetExpiration(time.Minute)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
