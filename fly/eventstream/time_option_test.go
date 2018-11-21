package eventstream_test

import (
	"time"

	"github.com/concourse/concourse/fly/eventstream"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Time Option", func() {

	Describe("FormatTimeAsString", func() {
		Context("When timestamp is non-zero", func() {
			eventLog := eventstream.EventLog{Timestamp: 0}

			It("should return empty spaces", func() {
				actualTime := eventLog.FormatTimeAsString()
				Expect(actualTime).Should(Equal("          "))
				Expect(len(actualTime)).Should(Equal(10))
			})
		})

		Context("When timestamp is unix time", func() {
			eventLog := eventstream.EventLog{Timestamp: time.Now().Unix()}

			It("should return time as a string", func() {
				actualTime := eventLog.FormatTimeAsString()
				Expect(actualTime).Should(MatchRegexp(`\d{2}:\d{2}:\d{2}\s{2}`))
				Expect(len(actualTime)).Should(Equal(10))
			})
		})

	})
})
