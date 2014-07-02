package limitedstream_test

import (
	"errors"
	"io"

	. "github.com/concourse/atc/limitedstream"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Limitedstream", func() {
	var fakeWriter *FakeWriteCloser
	var writer io.WriteCloser

	BeforeEach(func() {
		fakeWriter = new(FakeWriteCloser)

		writer = Writer{
			Limit:       16,
			WriteCloser: fakeWriter,
		}
	})

	Describe("Write", func() {
		It("splits payloads that exceed the limit", func() {
			fakeWriter.WriteStub = func(p []byte) (int, error) {
				return len(p), nil
			}

			n, err := writer.Write([]byte("1234567890123456789012345678901234567890"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(n).Should(Equal(40))

			Ω(fakeWriter.WriteCallCount()).Should(Equal(3))

			Ω(fakeWriter.WriteArgsForCall(0)).Should(Equal([]byte("1234567890123456")))
			Ω(fakeWriter.WriteArgsForCall(1)).Should(Equal([]byte("7890123456789012")))
			Ω(fakeWriter.WriteArgsForCall(2)).Should(Equal([]byte("34567890")))
		})

		Context("when writing through fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeWriter.WriteReturns(0, disaster)
			})

			It("returns the error", func() {
				_, err := writer.Write([]byte("sup"))
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("Close", func() {
		It("calls through to close", func() {
			err := writer.Close()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeWriter.CloseCallCount()).Should(Equal(1))
		})

		Context("when closing through fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeWriter.CloseReturns(disaster)
			})

			It("returns the error", func() {
				err := writer.Close()
				Ω(err).Should(Equal(disaster))
			})
		})
	})
})
