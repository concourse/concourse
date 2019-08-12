package connection_test

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/url"

	"code.cloudfoundry.org/garden/routes"
	"github.com/concourse/concourse/atc/worker/gclient/connection"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/rata"
)

var _ = Describe("ConnectionHijacker", func() {
	Describe("constructing hijacker with a dialer", func() {
		var hijackStreamer connection.HijackStreamer

		BeforeEach(func() {
			dialer := func(string, string) (net.Conn, error) {
				return nil, errors.New("oh no I am hijacked")
			}
			hijackStreamer = connection.NewHijackStreamerWithDialer(dialer)
		})

		Context("when Hijack is called", func() {
			It("should use the dialer", func() {
				_, _, err := hijackStreamer.Hijack(
					context.TODO(),
					routes.Run,
					new(bytes.Buffer),
					rata.Params{
						"handle": "some-test-handle",
					},
					nil,
					"application/json",
				)
				Expect(err).To(MatchError("oh no I am hijacked"))
			})
		})

		Context("when Stream is called", func() {
			It("should use the dialer", func() {
				_, err := hijackStreamer.Stream(
					routes.Run,
					new(bytes.Buffer),
					rata.Params{
						"handle": "some-test-handle",
					},
					nil,
					"application/json",
				)

				pathError, ok := err.(*url.Error)
				Expect(ok).To(BeTrue())
				Expect(pathError.Err).To(MatchError("oh no I am hijacked"))
			})
		})
	})

})
