package atcclient_test

import (
	"io"

	"github.com/concourse/atc"
	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/atcclient/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ATC Handler CLI", func() {
	Describe("GetCLIReader", func() {
		var (
			fakeConnection   *fakes.FakeConnection
			expectedArch     string
			expectedPlatform string
			expectedReturn   io.ReadCloser
		)

		BeforeEach(func() {
			fakeConnection = new(fakes.FakeConnection)
			client = atcclient.NewClient(fakeConnection)

			expectedArch = "fake_arch"
			expectedPlatform = "fake_platform"
			expectedReturn = &io.PipeReader{}

			expectedRequest := atcclient.Request{
				RequestName: atc.DownloadCLI,
				Queries: map[string]string{
					"arch":     expectedArch,
					"platform": expectedPlatform,
				},
				ReturnResponseBody: true,
			}

			fakeConnection.SendStub = func(request atcclient.Request, response *atcclient.Response) error {
				Expect(request).To(Equal(expectedRequest))
				response.Result = expectedReturn
				return nil
			}
		})

		It("returns an unclosed io.ReaderCloser", func() {
			readerCloser, err := client.GetCLIReader(expectedArch, expectedPlatform)
			Expect(err).NotTo(HaveOccurred())
			Expect(readerCloser).To(Equal(expectedReturn))
		})
	})
})
