package topgun_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	. "github.com/concourse/concourse/topgun"
	. "github.com/concourse/concourse/topgun/common"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = XDescribe("Vault", func() {
	var tokenDuration = 30 * time.Second

	BeforeEach(func() {
		if !strings.Contains(string(Bosh("releases").Out.Contents()), "vault") {
			Skip("vault release not uploaded")
		}
	})

	Describe("A deployment with vault", func() {
		var (
			v             *vault
			varsStore     *os.File
			vaultInstance *BoshInstance
		)

		BeforeEach(func() {
			var err error

			varsStore, err = ioutil.TempFile("", "vars-store.yml")
			Expect(err).ToNot(HaveOccurred())
			Expect(varsStore.Close()).To(Succeed())

			Deploy(
				"deployments/concourse.yml",
				"-o", "operations/add-vault.yml",
				"-v", "web_instances=0",
				"-v", "vault_url=dontcare",
				"-v", "vault_client_token=dontcare",
				"-v", "vault_auth_backend=dontcare",
				"-v", "vault_auth_params=dontcare",
			)

			vaultInstance = JobInstance("vault")
			v = newVault(vaultInstance.IP)

			v.Run("policy-write", "concourse", "vault/concourse-policy.hcl")
		})

		AfterEach(func() {
			Expect(os.Remove(varsStore.Name())).To(Succeed())
		})

		testTokenRenewal := func() {
			Context("when enough time has passed such that token would have expired", func() {
				BeforeEach(func() {
					v.Run("write", "concourse/main/team_secret", "value=some_team_secret")
					v.Run("write", "concourse/main/resource_version", "value=some_exposed_version_secret")

					By("waiting for long enough that the initial token would have expired")
					time.Sleep(tokenDuration)
				})

				It("renews the token", func() {
					watch := Fly.StartWithEnv(
						[]string{
							"EXPECTED_TEAM_SECRET=some_team_secret",
							"EXPECTED_RESOURCE_VERSION_SECRET=some_exposed_version_secret",
						},
						"execute", "-c", "tasks/credential-management.yml",
					)
					Wait(watch)
					Expect(watch).To(gbytes.Say("all credentials matched expected values"))
				})
			})
		}

		Context("with token auth", func() {
			BeforeEach(func() {
				By("creating a periodic token")
				create := v.Run("token-create", "-period", tokenDuration.String(), "-policy", "concourse")
				content := string(create.Out.Contents())
				token := regexp.MustCompile(`token\s+(.*)`).FindStringSubmatch(content)[1]

				By("renewing the token throughout the deploy")
				renewing := new(sync.WaitGroup)
				stopRenewing := make(chan struct{})

				defer func() {
					By("not renewing the token anymore, leaving it to Concourse")
					close(stopRenewing)
					renewing.Wait()
				}()

				renewTicker := time.NewTicker(5 * time.Second)
				renewing.Add(1)
				go func() {
					defer renewing.Done()
					defer GinkgoRecover()

					for {
						select {
						case <-renewTicker.C:
							v.Run("token-renew", token)
						case <-stopRenewing:
							return
						}
					}
				}()

				By("deploying concourse with the token")
				Deploy(
					"deployments/concourse.yml",
					"-o", "operations/add-vault.yml",
					"--vars-store", varsStore.Name(),
					"-v", "vault_url="+v.URI(),
					"-v", "vault_ip="+v.IP(),
					"-v", "web_instances=1",
					"-v", "vault_client_token="+token,
					"-v", `vault_auth_backend=""`,
					"-v", "vault_auth_params={}",
				)
			})

			testCredentialManagement(func() {
				v.Run("write", "concourse/main/team_secret", "value=some_team_secret")
				v.Run("write", "concourse/main/pipeline-creds-test/assertion_script", "value="+assertionScript)
				v.Run("write", "concourse/main/pipeline-creds-test/canary", "value=some_canary")
				v.Run("write", "concourse/main/pipeline-creds-test/resource_type_secret", "value=some_resource_type_secret")
				v.Run("write", "concourse/main/pipeline-creds-test/resource_secret", "value=some_resource_secret")
				v.Run("write", "concourse/main/pipeline-creds-test/job_secret", "username=some_username", "password=some_password")
				v.Run("write", "concourse/main/pipeline-creds-test/resource_version", "value=some_exposed_version_secret")
			}, func() {
				v.Run("write", "concourse/main/team_secret", "value=some_team_secret")
				v.Run("write", "concourse/main/resource_version", "value=some_exposed_version_secret")
			})

			testTokenRenewal()
		})

		Context("with TLS auth", func() {
			BeforeEach(func() {
				Deploy(
					"deployments/concourse.yml",
					"-o", "operations/add-vault.yml",
					"--vars-store", varsStore.Name(),
					"-o", "operations/enable-vault-tls.yml",
					"-v", "vault_url="+v.URI(),
					"-v", "vault_ip="+v.IP(),
					"-v", "vault_client_token=dontcare",
					"-v", `vault_auth_backend=""`,
					"-v", "vault_auth_params={}",
					"-v", "web_instances=0",
				)

				vaultCACertFile, err := ioutil.TempFile("", "vault-ca.cert")
				Expect(err).ToNot(HaveOccurred())

				vaultCACert := vaultCACertFile.Name()

				session := Bosh("interpolate", "--path", "/vault_ca/certificate", varsStore.Name())
				_, err = fmt.Fprintf(vaultCACertFile, "%s", session.Out.Contents())
				Expect(err).ToNot(HaveOccurred())
				Expect(vaultCACertFile.Close()).To(Succeed())

				v.SetCA(vaultCACert)
				v.Unseal()

				v.Run("auth-enable", "cert")
				v.Run(
					"write",
					"auth/cert/certs/concourse",
					"display_name=concourse",
					"certificate=@"+vaultCACert,
					"policies=concourse",
					fmt.Sprintf("ttl=%d", tokenDuration/time.Second),
				)

				Deploy(
					"deployments/concourse.yml",
					"-o", "operations/add-vault.yml",
					"--vars-store", varsStore.Name(),
					"-o", "operations/enable-vault-tls.yml",
					"-v", "vault_url="+v.URI(),
					"-v", "vault_ip="+v.IP(),
					"-v", "web_instances=1",
					"-v", `vault_client_token=""`,
					"-v", "vault_auth_backend=cert",
					"-v", "vault_auth_params={}",
				)
			})

			testTokenRenewal()
		})

		Context("with approle auth", func() {
			BeforeEach(func() {
				v.Run("auth-enable", "approle")

				v.Run(
					"write",
					"auth/approle/role/concourse",
					"policies=concourse",
					fmt.Sprintf("period=%d", tokenDuration/time.Second),
				)

				getRole := v.Run("read", "auth/approle/role/concourse/role-id")
				content := string(getRole.Out.Contents())
				roleID := regexp.MustCompile(`role_id\s+(.*)`).FindStringSubmatch(content)[1]

				createSecret := v.Run("write", "-f", "auth/approle/role/concourse/secret-id")
				content = string(createSecret.Out.Contents())
				secretID := regexp.MustCompile(`secret_id\s+(.*)`).FindStringSubmatch(content)[1]

				Deploy(
					"deployments/concourse.yml",
					"-o", "operations/add-vault.yml",
					"--vars-store", varsStore.Name(),
					"-v", "vault_url="+v.URI(),
					"-v", "vault_ip="+v.IP(),
					"-v", "web_instances=1",
					"-v", `vault_client_token=""`,
					"-v", "vault_auth_backend=approle",
					"-v", `vault_auth_params={"role_id":"`+roleID+`","secret_id":"`+secretID+`"}`,
				)
			})

			testTokenRenewal()
		})
	})
})

type vault struct {
	ip               string
	key1, key2, key3 string
	token            string
	caCert           string
}

func newVault(ip string) *vault {
	v := &vault{
		ip: ip,
	}
	v.init()
	return v
}

func (v *vault) SetCA(filename string) { v.caCert = filename }
func (v *vault) IP() string            { return v.ip }
func (v *vault) ClientToken() string   { return v.token }
func (v *vault) URI() string {
	if v.caCert == "" {
		return "http://" + v.ip + ":8200"
	}

	return "https://" + v.ip + ":8200"
}

func (v *vault) Run(command string, args ...string) *gexec.Session {
	env := append(
		os.Environ(),
		"VAULT_ADDR="+v.URI(),
		"VAULT_TOKEN="+v.token,
	)
	if v.caCert != "" {
		env = append(
			env,
			"VAULT_CACERT="+v.caCert,
			"VAULT_SKIP_VERIFY=true",
		)
	}
	session := Start(env, "vault", append([]string{command}, args...)...)
	Wait(session)
	return session
}

func (v *vault) init() {
	init := v.Run("init")
	content := string(init.Out.Contents())
	v.key1 = regexp.MustCompile(`Unseal Key 1: (.*)`).FindStringSubmatch(content)[1]
	v.key2 = regexp.MustCompile(`Unseal Key 2: (.*)`).FindStringSubmatch(content)[1]
	v.key3 = regexp.MustCompile(`Unseal Key 3: (.*)`).FindStringSubmatch(content)[1]
	v.token = regexp.MustCompile(`Initial Root Token: (.*)`).FindStringSubmatch(content)[1]
	v.Unseal()
	v.Run("mount", "-path", "concourse/main", "generic")
}

func (v *vault) Unseal() {
	v.Run("unseal", v.key1)
	v.Run("unseal", v.key2)
	v.Run("unseal", v.key3)
}
