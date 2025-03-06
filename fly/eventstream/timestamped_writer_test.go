package eventstream_test

import (
	"bytes"
	"time"

	"github.com/concourse/concourse/fly/eventstream"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Helper function to get the timestamp string in the current timezone
func timestampString(timestamp int64) string {
	return time.Unix(timestamp, 0).Format("15:04:05")
}

var _ = Describe("TimestampedWriter", func() {
	var (
		buffer        *bytes.Buffer
		showTimestamp bool
		writer        *eventstream.TimestampedWriter
	)

	BeforeEach(func() {
		buffer = new(bytes.Buffer)
	})

	JustBeforeEach(func() {
		writer = eventstream.NewTimestampedWriter(buffer, showTimestamp)
	})

	Context("when timestamps are enabled", func() {
		BeforeEach(func() {
			showTimestamp = true
		})

		It("prepends timestamp to the first line", func() {
			timestamp := int64(1614589123) // 2021-03-01 10:12:03 UTC
			writer.SetTimestamp(timestamp)

			_, err := writer.Write([]byte("first line\n"))
			Expect(err).NotTo(HaveOccurred())

			expectedTimestamp := timestampString(timestamp)
			Expect(buffer.String()).To(Equal(expectedTimestamp + "  first line\n"))
		})

		It("prepends timestamp to each new line", func() {
			timestamp := int64(1614589123) // 2021-03-01 10:12:03 UTC
			writer.SetTimestamp(timestamp)

			_, err := writer.Write([]byte("first line\nsecond line\n"))
			Expect(err).NotTo(HaveOccurred())

			expectedTimestamp := timestampString(timestamp)
			expected := expectedTimestamp + "  first line\n" + expectedTimestamp + "  second line\n"
			Expect(buffer.String()).To(Equal(expected))
		})

		It("handles multiple writes correctly", func() {
			timestamp := int64(1614589123) // 2021-03-01 10:12:03 UTC
			writer.SetTimestamp(timestamp)

			_, err := writer.Write([]byte("first "))
			Expect(err).NotTo(HaveOccurred())

			_, err = writer.Write([]byte("line\n"))
			Expect(err).NotTo(HaveOccurred())

			_, err = writer.Write([]byte("second line\n"))
			Expect(err).NotTo(HaveOccurred())

			expectedTimestamp := timestampString(timestamp)
			expected := expectedTimestamp + "  first line\n" + expectedTimestamp + "  second line\n"
			Expect(buffer.String()).To(Equal(expected))
		})

		It("does not add timestamp in the middle of a line", func() {
			timestamp := int64(1614589123) // 2021-03-01 10:12:03 UTC
			writer.SetTimestamp(timestamp)

			_, err := writer.Write([]byte("first part, "))
			Expect(err).NotTo(HaveOccurred())

			_, err = writer.Write([]byte("second part\n"))
			Expect(err).NotTo(HaveOccurred())

			expectedTimestamp := timestampString(timestamp)
			Expect(buffer.String()).To(Equal(expectedTimestamp + "  first part, second part\n"))
		})

		It("handles fragmented newlines correctly", func() {
			timestamp := int64(1614589123) // 2021-03-01 10:12:03 UTC
			writer.SetTimestamp(timestamp)

			_, err := writer.Write([]byte("first line"))
			Expect(err).NotTo(HaveOccurred())

			_, err = writer.Write([]byte("\n"))
			Expect(err).NotTo(HaveOccurred())

			_, err = writer.Write([]byte("second line\n"))
			Expect(err).NotTo(HaveOccurred())

			expectedTimestamp := timestampString(timestamp)
			expected := expectedTimestamp + "  first line\n" + expectedTimestamp + "  second line\n"
			Expect(buffer.String()).To(Equal(expected))
		})

		It("can change timestamp between writes", func() {
			timestamp1 := int64(1614589123) // 2021-03-01 10:12:03 UTC
			writer.SetTimestamp(timestamp1)

			_, err := writer.Write([]byte("first line\n"))
			Expect(err).NotTo(HaveOccurred())

			timestamp2 := int64(1614589183) // 2021-03-01 10:13:03 UTC
			writer.SetTimestamp(timestamp2)

			_, err = writer.Write([]byte("second line\n"))
			Expect(err).NotTo(HaveOccurred())

			expected := timestampString(timestamp1) + "  first line\n" +
				timestampString(timestamp2) + "  second line\n"
			Expect(buffer.String()).To(Equal(expected))
		})

		It("handles empty timestamp correctly", func() {
			writer.SetTimestamp(0) // Empty timestamp

			_, err := writer.Write([]byte("error line\n"))
			Expect(err).NotTo(HaveOccurred())

			Expect(buffer.String()).To(Equal("          error line\n"))
		})
	})

	Context("when timestamps are disabled", func() {
		BeforeEach(func() {
			showTimestamp = false
		})

		It("does not prepend timestamp", func() {
			writer.SetTimestamp(1614589123) // 2021-03-01 10:12:03 UTC

			_, err := writer.Write([]byte("first line\nsecond line\n"))
			Expect(err).NotTo(HaveOccurred())

			Expect(buffer.String()).To(Equal("first line\nsecond line\n"))
		})
	})
})
