package rc_test

import (
	"bytes"
	"net"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/square/certstrap/pkix"

	"testing"
)

func TestRc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RC Suite")
}

func userHomeDir() string {
	return os.Getenv("HOME")
}

var rootCA, clientCert, clientKey []byte

const bytesSeparator = "BYTES_SEPARATOR"

var _ = SynchronizedBeforeSuite(func() []byte {
	key, err := pkix.CreateRSAKey(1024)
	Expect(err).ToNot(HaveOccurred())

	ca, err := pkix.CreateCertificateAuthority(key, "", time.Now().Add(time.Hour), "", "", "", "", "server-ca", nil)
	Expect(err).ToNot(HaveOccurred())

	serverKey, err := pkix.CreateRSAKey(1024)
	Expect(err).ToNot(HaveOccurred())

	serverName := "server"

	serverCSR, err := pkix.CreateCertificateSigningRequest(serverKey, "", []net.IP{net.ParseIP("127.0.0.1")}, []string{serverName}, nil, "", "", "", "", "")
	Expect(err).ToNot(HaveOccurred())

	serverCert, err := pkix.CreateCertificateHost(ca, key, serverCSR, time.Now().Add(time.Hour))
	Expect(err).ToNot(HaveOccurred())

	clientKey, err := pkix.CreateRSAKey(1024)
	Expect(err).ToNot(HaveOccurred())

	clientKeyBytes, err := clientKey.ExportPrivate()
	Expect(err).ToNot(HaveOccurred())

	clientCSR, err := pkix.CreateCertificateSigningRequest(clientKey, "", nil, nil, nil, "", "", "", "", "concourse")
	Expect(err).ToNot(HaveOccurred())

	clientCert, err := pkix.CreateCertificateHost(ca, key, clientCSR, time.Now().Add(time.Hour))
	Expect(err).ToNot(HaveOccurred())

	serverCertBytes, err := serverCert.Export()
	Expect(err).ToNot(HaveOccurred())

	clientCertBytes, err := clientCert.Export()
	Expect(err).ToNot(HaveOccurred())

	return bytes.Join([][]byte{
		serverCertBytes,
		clientCertBytes,
		clientKeyBytes,
	}, []byte(bytesSeparator))
},
	func(data []byte) {
		splitData := bytes.Split(data, []byte(bytesSeparator))
		Expect(splitData).To(HaveLen(3))

		rootCA = splitData[0]
		clientCert = splitData[1]
		clientKey = splitData[2]
	})
