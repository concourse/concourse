package transport_test

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"code.cloudfoundry.org/garden"
	gconn "code.cloudfoundry.org/garden/client/connection"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/worker/transport"
	"github.com/concourse/atc/worker/transport/transportfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/db"
	"github.com/concourse/retryhttp/retryhttpfakes"
)

var _ = Describe("hijackStreamer", func() {
	var (
		savedWorker          *dbfakes.FakeWorker
		savedWorkerAddress   string
		fakeDB               *transportfakes.FakeTransportDB
		fakeRoundTripper     *transportfakes.FakeRoundTripper
		fakeHijackableClient *retryhttpfakes.FakeHijackableClient
		hijackStreamer       gconn.HijackStreamer
		fakeRequestGenerator *transportfakes.FakeRequestGenerator
		handler              string
		params               rata.Params
		query                url.Values
		contentType          string
	)
	BeforeEach(func() {
		savedWorkerAddress = "some-garden-addr"

		savedWorker = new(dbfakes.FakeWorker)

		savedWorker.GardenAddrReturns(&savedWorkerAddress)
		savedWorker.ExpiresAtReturns(time.Now().Add(123 * time.Minute))
		savedWorker.StateReturns(db.WorkerStateRunning)

		fakeDB = new(transportfakes.FakeTransportDB)
		fakeDB.GetWorkerReturns(savedWorker, true, nil)

		fakeRequestGenerator = new(transportfakes.FakeRequestGenerator)

		fakeRoundTripper = new(transportfakes.FakeRoundTripper)
		fakeHijackableClient = new(retryhttpfakes.FakeHijackableClient)

		hijackStreamer = &transport.WorkerHijackStreamer{
			HttpClient:       &http.Client{Transport: fakeRoundTripper},
			HijackableClient: fakeHijackableClient,
			Req:              fakeRequestGenerator,
		}

		handler = "Ping"
		params = map[string]string{"param1": "value1"}
		contentType = "application/json"
		query = map[string][]string{"key": []string{"some", "values"}}

		request, err := http.NewRequest("POST", "http://example.url", strings.NewReader("some-request-body"))
		Expect(err).NotTo(HaveOccurred())
		fakeRequestGenerator.CreateRequestReturns(request, nil)
	})

	Describe("hijackStreamer #Stream", func() {
		var (
			body             io.Reader
			actualReadCloser io.ReadCloser
			streamErr        error
			httpResp         http.Response
			expectedString   string
		)

		BeforeEach(func() {
			expectedString = "some-example-string"
			body = strings.NewReader(expectedString)

			fakeRoundTripper.RoundTripReturns(&httpResp, nil)
		})

		JustBeforeEach(func() {
			actualReadCloser, streamErr = hijackStreamer.Stream(handler, body, params, query, contentType)
		})

		Context("when httpResponse is success", func() {
			BeforeEach(func() {
				httpResp = http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(body)}
			})

			It("returns response body", func() {
				actualBodyBytes, err := ioutil.ReadAll(actualReadCloser)
				Expect(err).ToNot(HaveOccurred())
				Expect(expectedString).To(Equal(string(actualBodyBytes)))
			})
		})

		Context("when httpResponse is not success", func() {
			var fakeBody *transportfakes.FakeReadCloser

			BeforeEach(func() {
				fakeBody = new(transportfakes.FakeReadCloser)
				httpResp = http.Response{StatusCode: http.StatusTeapot, Body: fakeBody}

				bodyBuf, _ := json.Marshal(garden.Error{Err: errors.New("some-error")})
				realBody := strings.NewReader(string(bodyBuf))
				fakeBody.ReadStub = func(buf []byte) (int, error) {
					Expect(fakeBody.CloseCallCount()).To(BeZero())
					return realBody.Read(buf)
				}
			})

			It("closes httpResp.Body and returns error", func() {
				Expect(actualReadCloser).To(BeNil())
				Expect(streamErr).To(HaveOccurred())
				Expect(fakeBody.CloseCallCount()).To(Equal(1))
				Expect(streamErr).To(MatchError("some-error"))
			})
		})

		Context("when httpResponse is not success with bad response", func() {
			var fakeBody *transportfakes.FakeReadCloser

			BeforeEach(func() {
				fakeBody = new(transportfakes.FakeReadCloser)
				httpResp = http.Response{StatusCode: http.StatusTeapot, Body: fakeBody}

				realBody := strings.NewReader("some-error")
				fakeBody.ReadStub = func(buf []byte) (int, error) {
					Expect(fakeBody.CloseCallCount()).To(BeZero())
					return realBody.Read(buf)
				}
			})

			It("closes httpResp.Body and returns bad response", func() {
				Expect(actualReadCloser).To(BeNil())
				Expect(streamErr).To(HaveOccurred())
				Expect(fakeBody.CloseCallCount()).To(Equal(1))
				Expect(streamErr).To(MatchError(fmt.Errorf("bad response: %s", errors.New("invalid character 's' looking for beginning of value"))))
			})
		})

		Context("when httpResponse fails", func() {
			BeforeEach(func() {
				httpResp = http.Response{StatusCode: http.StatusTeapot, Body: ioutil.NopCloser(body)}
			})

			It("returns error", func() {
				Expect(actualReadCloser).To(BeNil())
				Expect(streamErr).To(HaveOccurred())
			})

			It("creates request with the right arguments", func() {
				Expect(fakeRequestGenerator.CreateRequestCallCount()).To(Equal(1))
				actualHandler, actualParams, actualBody := fakeRequestGenerator.CreateRequestArgsForCall(0)
				Expect(actualHandler).To(Equal(handler))
				Expect(actualParams).To(Equal(params))
				Expect(actualBody).To(Equal(body))
			})

			It("httpClient makes the right request", func() {
				expectedRequest, err := http.NewRequest("POST", "http://example.url", strings.NewReader("some-request-body"))
				expectedRequest.Header.Add("Content-Type", "application/json")
				expectedRequest.URL.RawQuery = "key=some&key=values"
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeRoundTripper.RoundTripCallCount()).To(Equal(1))
				actualRequest := fakeRoundTripper.RoundTripArgsForCall(0)
				Expect(actualRequest.Method).To(Equal(expectedRequest.Method))
				Expect(actualRequest.URL).To(Equal(expectedRequest.URL))
				Expect(actualRequest.Header).To(Equal(expectedRequest.Header))

				s, err := ioutil.ReadAll(actualRequest.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(s)).To(Equal("some-request-body"))
			})

		})
	})

	Describe("hijackStreamer #Hijack", func() {
		var (
			body                   io.Reader
			hijackError            error
			httpResp               http.Response
			expectedString         string
			actualHijackedConn     net.Conn
			actualResponseReader   *bufio.Reader
			fakeHijackCloser       *retryhttpfakes.FakeHijackCloser
			expectedResponseReader *bufio.Reader
			fakeHijackedConn       *retryhttpfakes.FakeConn
		)

		BeforeEach(func() {
			expectedResponseReader = new(bufio.Reader)
			fakeHijackedConn = new(retryhttpfakes.FakeConn)

			expectedString = "some-example-string"
			body = strings.NewReader(expectedString)
			fakeHijackCloser = new(retryhttpfakes.FakeHijackCloser)
		})

		JustBeforeEach(func() {
			actualHijackedConn, actualResponseReader, hijackError = hijackStreamer.Hijack(handler, body, params, query, contentType)
		})

		Context("when request is successful", func() {
			BeforeEach(func() {
				fakeHijackableClient.DoReturns(&httpResp, fakeHijackCloser, nil)
				httpResp = http.Response{StatusCode: http.StatusOK}
				fakeHijackCloser.HijackReturns(fakeHijackedConn, expectedResponseReader)
			})

			It("returns success response and hijackCloser", func() {
				Expect(hijackError).ToNot(HaveOccurred())
				Expect(fakeHijackCloser.HijackCallCount()).To(Equal(1))
				Expect(actualHijackedConn).To(Equal(fakeHijackedConn))
				Expect(actualResponseReader).To(Equal(expectedResponseReader))
			})
		})

		Context("when httpResponse is not success", func() {
			var fakeBody *transportfakes.FakeReadCloser

			BeforeEach(func() {
				fakeHijackableClient.DoReturns(&httpResp, fakeHijackCloser, nil)
				fakeBody = new(transportfakes.FakeReadCloser)
				httpResp = http.Response{StatusCode: http.StatusTeapot, Body: fakeBody}

				realBody := strings.NewReader("some-error")
				fakeBody.ReadStub = func(buf []byte) (int, error) {
					Expect(fakeBody.CloseCallCount()).To(BeZero())
					return realBody.Read(buf)
				}
			})

			It("closes httpResp.Body, hijackCloser and returns error", func() {
				Expect(fakeHijackCloser).NotTo(BeNil())
				Expect(hijackError).To(HaveOccurred())
				Expect(fakeBody.CloseCallCount()).To(Equal(1))
				Expect(hijackError).To(MatchError(fmt.Errorf("Backend error: Exit status: %d, message: %s", httpResp.StatusCode, "some-error")))
				Expect(fakeHijackCloser.CloseCallCount()).To(Equal(1))
			})
		})

		Context("when httpResponse is not success with bad response", func() {
			var fakeBody *transportfakes.FakeReadCloser

			BeforeEach(func() {
				fakeHijackableClient.DoReturns(&httpResp, fakeHijackCloser, nil)
				fakeBody = new(transportfakes.FakeReadCloser)
				httpResp = http.Response{StatusCode: http.StatusTeapot, Body: fakeBody}

				fakeBody.ReadStub = func(buf []byte) (int, error) {
					Expect(fakeBody.CloseCallCount()).To(BeZero())
					return 0, errors.New("error reading")
				}
			})

			It("closes httpResp.Body and returns bad response", func() {
				Expect(fakeHijackCloser).NotTo(BeNil())
				Expect(hijackError).To(HaveOccurred())
				Expect(fakeBody.CloseCallCount()).To(Equal(1))
				Expect(hijackError).To(MatchError(fmt.Errorf("Backend error: Exit status: %d, error reading response body: %s", httpResp.StatusCode, "error reading")))
				Expect(fakeHijackCloser.CloseCallCount()).To(Equal(1))
			})
		})

		Context("when httpResponse fails", func() {
			BeforeEach(func() {
				fakeHijackableClient.DoReturns(nil, nil, errors.New("Request failed"))
				httpResp = http.Response{StatusCode: http.StatusTeapot, Body: ioutil.NopCloser(body)}
			})

			It("returns error", func() {
				Expect(hijackError).To(HaveOccurred())
				Expect(actualHijackedConn).To(BeNil())
				Expect(actualResponseReader).To(BeNil())
			})

			It("makes the right request", func() {
				expectedRequest, err := http.NewRequest("POST", "http://example.url", strings.NewReader("some-request-body"))
				expectedRequest.Header.Add("Content-Type", "application/json")
				expectedRequest.URL.RawQuery = "key=some&key=values"
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeHijackableClient.DoCallCount()).To(Equal(1))
				actualRequest := fakeHijackableClient.DoArgsForCall(0)

				Expect(actualRequest.Method).To(Equal(expectedRequest.Method))
				Expect(actualRequest.URL).To(Equal(expectedRequest.URL))
				Expect(actualRequest.Header).To(Equal(expectedRequest.Header))

				s, err := ioutil.ReadAll(actualRequest.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(s)).To(Equal("some-request-body"))
			})
		})
	})
})
