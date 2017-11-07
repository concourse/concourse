package acceptance_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gexec"
)

const BASIC_AUTH = "basic"
const BASIC_AUTH_NO_PASSWORD = "basic-no-password"
const BASIC_AUTH_NO_USERNAME = "basic-no-username"
const GITHUB_AUTH = "github"
const GITHUB_AUTH_NO_CLIENT_SECRET = "github-no-secret"
const GITHUB_AUTH_NO_TEAM = "github-no-team"
const GITHUB_ENTERPRISE_AUTH = "github-enterprise"
const UAA_AUTH = "cf"
const UAA_AUTH_NO_CLIENT_SECRET = "cf-no-secret"
const UAA_AUTH_NO_TOKEN_URL = "cf-no-token-url"
const UAA_AUTH_NO_SPACE = "cf-no-space"
const GENERIC_OAUTH_AUTH = "generic-oauth"
const GENERIC_OAUTH_AUTH_PARAMS = "generic-oauth-params"
const GENERIC_OAUTH_AUTH_NO_CLIENT_SECRET = "generic-oauth-no-secret"
const GENERIC_OAUTH_AUTH_NO_TOKEN_URL = "generic-oauth-no-token-url"
const GENERIC_OAUTH_AUTH_NO_DISPLAY_NAME = "generic-oauth-no-display-name"
const NOT_CONFIGURED_AUTH = "not-configured"
const LOG_LEVEL = "log-level"
const NO_AUTH = "no-really-i-dont-want-any-auth"

type ATCCommand struct {
	atcBin                 string
	atcServerNumber        uint16
	tlsFlags               []string
	telemetryOptIn         bool
	authTypes              []string
	postgresDataSourceName string
	pemPrivateKeyFile      string
	cfCACertFile           string

	process                    ifrit.Process
	port                       uint16
	tlsPort                    uint16
	tlsCertificateOrganization string
	tmpDir                     string
}

const pemPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIBOgIBAAJBALKZD0nEffqM1ACuak0bijtqE2QrI/KLADv7l3kK3ppMyCuLKoF0
fd7Ai2KW5ToIwzFofvJcS/STa6HA5gQenRUCAwEAAQJBAIq9amn00aS0h/CrjXqu
/ThglAXJmZhOMPVn4eiu7/ROixi9sex436MaVeMqSNf7Ex9a8fRNfWss7Sqd9eWu
RTUCIQDasvGASLqmjeffBNLTXV2A5g4t+kLVCpsEIZAycV5GswIhANEPLmax0ME/
EO+ZJ79TJKN5yiGBRsv5yvx5UiHxajEXAiAhAol5N4EUyq6I9w1rYdhPMGpLfk7A
IU2snfRJ6Nq2CQIgFrPsWRCkV+gOYcajD17rEqmuLrdIRexpg8N1DOSXoJ8CIGlS
tAboUGBxTDq3ZroNism3DaMIbKPyYrAqhKov1h5V
-----END RSA PRIVATE KEY-----`

const cfCACert = `-----BEGIN CERTIFICATE-----
MIICsjCCAhugAwIBAgIJAJgyGeIL1aiPMA0GCSqGSIb3DQEBBQUAMEUxCzAJBgNV
BAYTAkFVMRMwEQYDVQQIEwpTb21lLVN0YXRlMSEwHwYDVQQKExhJbnRlcm5ldCBX
aWRnaXRzIFB0eSBMdGQwIBcNMTUwMzE5MjE1NzAxWhgPMjI4ODEyMzEyMTU3MDFa
MEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIEwpTb21lLVN0YXRlMSEwHwYDVQQKExhJ
bnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJ
AoGBAOTD37e9wnQz5fHVPdQdU8rjokOVuFj0wBtQLNO7B2iN+URFaP2wi0KOU0ye
njISc5M/mpua7Op72/cZ3+bq8u5lnQ8VcjewD1+f3LCq+Os7iE85A/mbEyT1Mazo
GGo9L/gfz5kNq78L9cQp5lrD04wF0C05QtL8LVI5N9SqT7mlAgMBAAGjgacwgaQw
HQYDVR0OBBYEFNtN+q97oIhvyUEC+/Sc4q0ASv4zMHUGA1UdIwRuMGyAFNtN+q97
oIhvyUEC+/Sc4q0ASv4zoUmkRzBFMQswCQYDVQQGEwJBVTETMBEGA1UECBMKU29t
ZS1TdGF0ZTEhMB8GA1UEChMYSW50ZXJuZXQgV2lkZ2l0cyBQdHkgTHRkggkAmDIZ
4gvVqI8wDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQUFAAOBgQCZKuxfGc/RrMlz
aai4+5s0GnhSuq0CdfnpwZR+dXsjMO6dlrD1NgQoQVhYO7UbzktwU1Hz9Mc3XE7t
HCu8gfq+3WRUgddCQnYJUXtig2yAqmHf/WGR9yYYnfMUDKa85i0inolq1EnLvgVV
K4iijxtW0XYe5R1Od6lWOEKZ6un9Ag==
-----END CERTIFICATE-----
`

func NewATCCommand(
	atcBin string,
	atcServerNumber uint16,
	postgresDataSourceName string,
	tlsFlags []string,
	telemetryOptIn bool,
	authTypes ...string,
) *ATCCommand {
	return &ATCCommand{
		atcBin:                     atcBin,
		atcServerNumber:            atcServerNumber,
		postgresDataSourceName:     postgresDataSourceName,
		tlsFlags:                   tlsFlags,
		authTypes:                  authTypes,
		telemetryOptIn:             telemetryOptIn,
		tlsCertificateOrganization: "Acme Co",
	}
}

func (a *ATCCommand) URL(path string) string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", a.port, path)
}

func (a *ATCCommand) TLSURL(path string) string {
	return fmt.Sprintf("https://127.0.0.1:%d%s", a.tlsPort, path)
}

func (a *ATCCommand) Start() error {
	err := a.prepare()
	if err != nil {
		return err
	}

	atcCommand := a.getATCCommand()
	atcRunner := ginkgomon.New(ginkgomon.Config{
		Command:       atcCommand,
		Name:          "atc",
		StartCheck:    "atc.listening",
		AnsiColorCode: "32m",
	})

	a.process = ginkgomon.Invoke(atcRunner)

	return nil
}

func (a *ATCCommand) StartAndWait() (*gexec.Session, error) {
	err := a.prepare()
	if err != nil {
		return nil, err
	}

	return gexec.Start(a.getATCCommand(), GinkgoWriter, GinkgoWriter)
}

func (a *ATCCommand) Stop() {
	ginkgomon.Interrupt(a.process)
	os.RemoveAll(a.tmpDir)
}

func (a *ATCCommand) prepare() error {
	var err error
	a.tmpDir, err = ioutil.TempDir("", "")
	if err != nil {
		return err
	}

	cfCACertFile, err := ioutil.TempFile(a.tmpDir, "cf-ca-certificate")
	if err != nil {
		return err
	}
	a.cfCACertFile = cfCACertFile.Name()

	err = ioutil.WriteFile(a.cfCACertFile, []byte(cfCACert), 0644)
	if err != nil {
		return err
	}

	pemPrivateKeyFile, err := ioutil.TempFile(a.tmpDir, "accceptance-signing-key")
	if err != nil {
		return err
	}
	a.pemPrivateKeyFile = pemPrivateKeyFile.Name()

	err = ioutil.WriteFile(a.pemPrivateKeyFile, []byte(pemPrivateKey), 0644)
	if err != nil {
		return err
	}

	err = a.createCert()
	if err != nil {
		return err
	}

	return nil
}

func (a *ATCCommand) getATCCommand() *exec.Cmd {
	a.port = 5697 + uint16(GinkgoParallelNode()) + (a.atcServerNumber * 100)
	debugPort := 6697 + uint16(GinkgoParallelNode()) + (a.atcServerNumber * 100)

	params := []string{
		"--bind-port", fmt.Sprintf("%d", a.port),
		"--debug-bind-port", fmt.Sprintf("%d", debugPort),
		"--peer-url", fmt.Sprintf("http://127.0.0.1:%d", a.port),
		"--postgres-data-source", a.postgresDataSourceName,
		"--external-url", fmt.Sprintf("http://127.0.0.1:%d", a.port),
		"--session-signing-key", a.pemPrivateKeyFile,
	}

	for _, authType := range a.authTypes {
		switch authType {
		case BASIC_AUTH:
			params = append(params,
				"--basic-auth-username", "admin",
				"--basic-auth-password", "password",
			)
		case BASIC_AUTH_NO_PASSWORD:
			params = append(params,
				"--basic-auth-username", "admin",
			)
		case BASIC_AUTH_NO_USERNAME:
			params = append(params,
				"--basic-auth-password", "password",
			)
		case GITHUB_AUTH:
			params = append(params,
				"--github-auth-client-id", "admin",
				"--github-auth-client-secret", "password",
				"--github-auth-organization", "myorg",
				"--github-auth-team", "myorg/all",
				"--github-auth-user", "myuser",
			)
		case GITHUB_AUTH_NO_CLIENT_SECRET:
			params = append(params,
				"--github-auth-client-id", "admin",
				"--github-auth-organization", "myorg",
				"--github-auth-team", "myorg/all",
				"--github-auth-user", "myuser",
			)
		case GITHUB_AUTH_NO_TEAM:
			params = append(params,
				"--github-auth-client-id", "admin",
				"--github-auth-client-secret", "password",
			)
		case GITHUB_ENTERPRISE_AUTH:
			params = append(params,
				"--github-auth-client-id", "admin",
				"--github-auth-client-secret", "password",
				"--github-auth-organization", "myorg",
				"--github-auth-team", "myorg/all",
				"--github-auth-user", "myuser",
				"--github-auth-auth-url", "https://github.example.com/login/oauth/authorize",
				"--github-auth-token-url", "https://github.example.com/login/oauth/access_token",
				"--github-auth-api-url", "https://github.example.com/api/v3/",
			)
		case UAA_AUTH:
			params = append(params,
				"--uaa-auth-client-id", "admin",
				"--uaa-auth-client-secret", "password",
				"--uaa-auth-cf-space", "myspace",
				"--uaa-auth-auth-url", "https://uaa.example.com/oauth/authorize",
				"--uaa-auth-token-url", "https://uaa.example.com/oauth/token",
				"--uaa-auth-cf-url", "https://cf.example.com/api",
				"--uaa-auth-cf-ca-cert", a.cfCACertFile,
			)
		case UAA_AUTH_NO_CLIENT_SECRET:
			params = append(params,
				"--uaa-auth-client-id", "admin",
			)
		case UAA_AUTH_NO_SPACE:
			params = append(params,
				"--uaa-auth-client-id", "admin",
				"--uaa-auth-client-secret", "password",
			)
		case UAA_AUTH_NO_TOKEN_URL:
			params = append(params,
				"--uaa-auth-client-id", "admin",
				"--uaa-auth-client-secret", "password",
				"--uaa-auth-cf-space", "myspace",
				"--uaa-auth-auth-url", "https://uaa.example.com/oauth/authorize",
				"--uaa-auth-cf-url", "https://cf.example.com/api",
			)
		case GENERIC_OAUTH_AUTH:
			params = append(params,
				"--generic-oauth-display-name", "Example",
				"--generic-oauth-client-id", "admin",
				"--generic-oauth-client-secret", "password",
				"--generic-oauth-auth-url", "https://goa.example.com/oauth/authorize",
				"--generic-oauth-token-url", "https://goa.example.com/oauth/token",
			)
		case GENERIC_OAUTH_AUTH_PARAMS:
			params = append(params,
				"--generic-oauth-display-name", "Example",
				"--generic-oauth-client-id", "admin",
				"--generic-oauth-client-secret", "password",
				"--generic-oauth-auth-url", "https://goa.example.com/oauth/authorize",
				"--generic-oauth-auth-url-param", "param1:value1",
				"--generic-oauth-auth-url-param", "param2:value2",
				"--generic-oauth-token-url", "https://goa.example.com/oauth/token",
			)
		case GENERIC_OAUTH_AUTH_NO_CLIENT_SECRET:
			params = append(params,
				"--generic-oauth-display-name", "Example",
				"--generic-oauth-client-id", "admin",
			)
		case GENERIC_OAUTH_AUTH_NO_TOKEN_URL:
			params = append(params,
				"--generic-oauth-display-name", "Example",
				"--generic-oauth-client-id", "admin",
				"--generic-oauth-client-secret", "password",
				"--generic-oauth-auth-url", "https://goa.example.com/oauth/authorize",
			)
		case GENERIC_OAUTH_AUTH_NO_DISPLAY_NAME:
			params = append(params,
				"--generic-oauth-client-id", "admin",
				"--generic-oauth-client-secret", "password",
				"--generic-oauth-auth-url", "https://goa.example.com/oauth/authorize",
				"--generic-oauth-token-url", "https://goa.example.com/oauth/token",
			)
		case NO_AUTH:
			params = append(params, "--no-really-i-dont-want-any-auth")
		case NOT_CONFIGURED_AUTH:
		default:
			panic("unknown auth type")
		}
	}

	if len(a.tlsFlags) > 0 {
		a.tlsPort = 7697 + uint16(GinkgoParallelNode()) + (a.atcServerNumber * 100)
		params = append(params, "--external-url", fmt.Sprintf("https://127.0.0.1:%d/", a.tlsPort))

		for _, tlsFlag := range a.tlsFlags {
			switch tlsFlag {
			case "--tls-bind-port":
				params = append(params, "--tls-bind-port", fmt.Sprintf("%d", a.tlsPort))
			case "--tls-cert":
				params = append(params, "--tls-cert", filepath.Join(a.tmpDir, "server.pem"))
			case "--tls-key":
				params = append(params, "--tls-key", filepath.Join(a.tmpDir, "server.key"))
			}
		}
	}

	if a.telemetryOptIn {
		params = append(params, "--telemetry-opt-in")
	}

	return exec.Command(a.atcBin, params...)
}

func (a *ATCCommand) createCert() error {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("failed to generate serial number: %s", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{a.tlsCertificateOrganization},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(24 * time.Hour),

		IsCA:                  true,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses: []net.IP{
			net.IP{127, 0, 0, 1},
		},
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &(priv.PublicKey), priv)
	if err != nil {
		return err
	}

	certOut, err := os.Create(filepath.Join(a.tmpDir, "server.pem"))
	if err != nil {
		return err
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyOut, err := os.OpenFile(filepath.Join(a.tmpDir, "server.key"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	pemBlockForKey := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}
	pem.Encode(keyOut, pemBlockForKey)
	keyOut.Close()

	return nil
}
