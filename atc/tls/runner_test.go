package tls_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"sync/atomic"
	"syscall"
	"time"

	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/concourse/concourse/atc"
	. "github.com/concourse/concourse/atc/tls"
	"github.com/madflojo/testcerts"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Runner", func() {
	It("Can reload", func() {
		reloaded := new(atomic.Bool)

		tmpDir, err := os.MkdirTemp("", "")
		Expect(err).ShouldNot(HaveOccurred())
		defer func() {
			os.RemoveAll(tmpDir)
		}()

		certFile, keyFile, err := testcerts.GenerateCertsToTempFile(tmpDir)
		Expect(err).ShouldNot(HaveOccurred())

		initialConfig := atc.DefaultTLSConfig()

		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		Expect(err).ShouldNot(HaveOccurred())
		initialConfig.Certificates = []tls.Certificate{cert}

		runner := NewReloadableListener("127.0.0.1:58008", testHandler(), initialConfig, reloader(certFile, keyFile, reloaded), lagertest.NewTestLogger("tls-reload"))
		defer func() { runner.Stop() }()

		process := ifrit.Invoke(runner)
		<-process.Ready()

		runner.Interrupter <- syscall.SIGHUP

		// to let the reload actually happen
		time.Sleep(time.Second)
		Expect(reloaded.Load()).Should(BeTrue())
	})
})

func reloader(oldCert, oldKey string, reloaded *atomic.Bool) ConfigReloader {
	return func() (*tls.Config, error) {
		GinkgoHelper()

		reloaded.Store(true)

		newCert, newKey, err := testcerts.GenerateCertsToTempFile("")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(newCert).ShouldNot(BeEquivalentTo(oldCert))
		Expect(newKey).ShouldNot(BeEquivalentTo(oldKey))

		newConfig := atc.DefaultTLSConfig()

		cert, err := tls.LoadX509KeyPair(newCert, newKey)
		Expect(err).ShouldNot(HaveOccurred())
		newConfig.Certificates = []tls.Certificate{cert}

		return newConfig, nil
	}
}

func testHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello")
	})
}
