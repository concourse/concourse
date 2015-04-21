package main_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	gclient "github.com/cloudfoundry-incubator/garden/client"
	gconn "github.com/cloudfoundry-incubator/garden/client/connection"
	gfakes "github.com/cloudfoundry-incubator/garden/fakes"
	gserver "github.com/cloudfoundry-incubator/garden/server"
	"github.com/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/pivotal-golang/localip"
	"golang.org/x/crypto/ssh"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var tsaPath string

var _ = BeforeSuite(func() {
	var err error
	tsaPath, err = gexec.Build("github.com/concourse/tsa/cmd/tsa")

	Ω(err).ShouldNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

func generateSSHKeypair() (string, string) {
	path, err := ioutil.TempDir("", "tsa-key")
	Ω(err).ShouldNot(HaveOccurred())

	privateKey := filepath.Join(path, "id_rsa")

	keygen, err := gexec.Start(exec.Command("ssh-keygen", "-t", "rsa", "-N", "", "-f", privateKey), GinkgoWriter, GinkgoWriter)
	Ω(err).ShouldNot(HaveOccurred())

	keygen.Wait(5 * time.Second)

	return privateKey, privateKey + ".pub"
}

var _ = Describe("TSA SSH Registrar", func() {
	Describe("registering workers with ATC", func() {
		var (
			forwardHost string

			tsaPort           int
			heartbeatInterval = 1 * time.Second
			tsaProcess        ifrit.Process

			gardenAddr  string
			fakeBackend *gfakes.FakeBackend

			gardenServer *gserver.GardenServer
			atcServer    *ghttp.Server

			hostKey    string
			hostKeyPub string

			authorizedKeysFile string

			userKnownHostsFile string

			userKey    string
			userKeyPub string
		)

		BeforeEach(func() {
			tsaPort = 9800 + GinkgoParallelNode()

			gardenPort := 9001 + GinkgoParallelNode()
			gardenAddr = fmt.Sprintf("127.0.0.1:%d", gardenPort)

			fakeBackend = new(gfakes.FakeBackend)

			gardenServer = gserver.New("tcp", gardenAddr, 0, fakeBackend, lagertest.NewTestLogger("garden"))
			err := gardenServer.Start()
			Ω(err).ShouldNot(HaveOccurred())

			atcServer = ghttp.NewServer()

			hostKey, hostKeyPub = generateSSHKeypair()

			userKnownHosts, err := ioutil.TempFile("", "known-hosts")
			Ω(err).ShouldNot(HaveOccurred())

			defer userKnownHosts.Close()

			userKnownHostsFile = userKnownHosts.Name()

			_, err = fmt.Fprintf(userKnownHosts, "[127.0.0.1]:%d ", tsaPort)
			Ω(err).ShouldNot(HaveOccurred())

			pub, err := os.Open(hostKeyPub)
			Ω(err).ShouldNot(HaveOccurred())

			defer pub.Close()

			_, err = io.Copy(userKnownHosts, pub)
			Ω(err).ShouldNot(HaveOccurred())

			userKey, userKeyPub = generateSSHKeypair()

			authorizedKeys, err := ioutil.TempFile("", "authorized-keys")
			Ω(err).ShouldNot(HaveOccurred())

			defer authorizedKeys.Close()

			authorizedKeysFile = authorizedKeys.Name()

			userPrivateKeyBytes, err := ioutil.ReadFile(userKey)
			Ω(err).ShouldNot(HaveOccurred())

			userSigner, err := ssh.ParsePrivateKey(userPrivateKeyBytes)
			Ω(err).ShouldNot(HaveOccurred())

			_, err = authorizedKeys.Write(ssh.MarshalAuthorizedKey(userSigner.PublicKey()))
			Ω(err).ShouldNot(HaveOccurred())

			forwardHost, err = localip.LocalIP()
			Ω(err).ShouldNot(HaveOccurred())

			tsaCommand := exec.Command(
				tsaPath,
				"-listenPort", strconv.Itoa(tsaPort),
				"-hostKey", hostKey,
				"-authorizedKeys", authorizedKeysFile,
				"-atcAPIURL", atcServer.URL(),
				"-heartbeatInterval", heartbeatInterval.String(),
				"-forwardHost", forwardHost,
			)

			tsaRunner := ginkgomon.New(ginkgomon.Config{
				Command:       tsaCommand,
				Name:          "tsa",
				StartCheck:    "tsa.listening",
				AnsiColorCode: "32m",
			})

			tsaProcess = ginkgomon.Invoke(tsaRunner)
		})

		AfterEach(func() {
			atcServer.Close()
			gardenServer.Stop()
			ginkgomon.Interrupt(tsaProcess)
		})

		Describe("SSHing", func() {
			var sshSess *gexec.Session
			var sshStdin io.Writer
			var sshArgv []string

			BeforeEach(func() {
				sshArgv = []string{
					"127.0.0.1",
					"-p", strconv.Itoa(tsaPort),
					"-o", "UserKnownHostsFile=" + userKnownHostsFile,
				}
			})

			JustBeforeEach(func() {
				ssh := exec.Command("ssh", sshArgv...)

				var err error
				sshStdin, err = ssh.StdinPipe()
				Ω(err).ShouldNot(HaveOccurred())

				sshSess, err = gexec.Start(
					ssh,
					gexec.NewPrefixedWriter("\x1b[32m[o]\x1b[0m\x1b[33m[ssh]\x1b[0m ", GinkgoWriter),
					gexec.NewPrefixedWriter("\x1b[91m[e]\x1b[0m\x1b[33m[ssh]\x1b[0m ", GinkgoWriter),
				)
				Ω(err).ShouldNot(HaveOccurred())
			})

			AfterEach(func() {
				sshSess.Interrupt().Wait(10 * time.Second)
			})

			Context("with a valid key", func() {
				BeforeEach(func() {
					sshArgv = append(sshArgv, "-i", userKey)
				})

				Context("when running register-worker", func() {
					BeforeEach(func() {
						sshArgv = append(sshArgv, "register-worker")
					})

					It("does not exit", func() {
						Consistently(sshSess, 1).ShouldNot(gexec.Exit())
					})

					Describe("sending a worker payload on stdin", func() {
						type registration struct {
							worker atc.Worker
							ttl    time.Duration
						}

						var workerPayload atc.Worker
						var registered chan registration

						BeforeEach(func() {
							workerPayload = atc.Worker{
								Addr: gardenAddr,

								Platform: "linux",
								Tags:     []string{"some", "tags"},

								ResourceTypes: []atc.WorkerResourceType{
									{Type: "resource-type-a", Image: "resource-image-a"},
									{Type: "resource-type-b", Image: "resource-image-b"},
								},
							}

							registered = make(chan registration)

							atcServer.RouteToHandler("POST", "/api/v1/workers", func(w http.ResponseWriter, r *http.Request) {
								var worker atc.Worker
								err := json.NewDecoder(r.Body).Decode(&worker)
								Ω(err).ShouldNot(HaveOccurred())

								ttl, err := time.ParseDuration(r.URL.Query().Get("ttl"))
								Ω(err).ShouldNot(HaveOccurred())

								registered <- registration{worker, ttl}
							})

							stubs := make(chan func() ([]garden.Container, error), 4)

							stubs <- func() ([]garden.Container, error) {
								return []garden.Container{
									new(gfakes.FakeContainer),
									new(gfakes.FakeContainer),
									new(gfakes.FakeContainer),
								}, nil
							}

							stubs <- func() ([]garden.Container, error) {
								return []garden.Container{
									new(gfakes.FakeContainer),
									new(gfakes.FakeContainer),
								}, nil
							}

							stubs <- func() ([]garden.Container, error) {
								return nil, errors.New("garden was weeded")
							}

							stubs <- func() ([]garden.Container, error) {
								return []garden.Container{
									new(gfakes.FakeContainer),
								}, nil
							}

							fakeBackend.ContainersStub = func(garden.Properties) ([]garden.Container, error) {
								return (<-stubs)()
							}
						})

						JustBeforeEach(func() {
							err := json.NewEncoder(sshStdin).Encode(workerPayload)
							Ω(err).ShouldNot(HaveOccurred())
						})

						It("continuously registers it with the ATC as long as it works", func() {
							expectedWorkerPayload := workerPayload

							expectedWorkerPayload.ActiveContainers = 3

							a := time.Now()
							Ω(<-registered).Should(Equal(registration{
								worker: expectedWorkerPayload,
								ttl:    2 * heartbeatInterval,
							}))

							expectedWorkerPayload.ActiveContainers = 2

							b := time.Now()
							Ω(<-registered).Should(Equal(registration{
								worker: expectedWorkerPayload,
								ttl:    2 * heartbeatInterval,
							}))

							Ω(b.Sub(a)).Should(BeNumerically("~", heartbeatInterval, 1*time.Second))

							Consistently(registered, 2*heartbeatInterval).ShouldNot(Receive())

							expectedWorkerPayload.ActiveContainers = 1

							c := time.Now()
							Ω(<-registered).Should(Equal(registration{
								worker: expectedWorkerPayload,
								ttl:    2 * heartbeatInterval,
							}))

							Ω(c.Sub(b)).Should(BeNumerically("~", 3*heartbeatInterval, 1*time.Second))

							Eventually(sshSess.Out).Should(gbytes.Say("heartbeat"))
						})

						Context("when the client goes away", func() {
							It("stops forwarding", func() {
								time.Sleep(heartbeatInterval)

								sshSess.Interrupt().Wait(10 * time.Second)

								time.Sleep(heartbeatInterval)

								// siphon off any existing registrations
							dance:
								for {
									select {
									case <-registered:
									default:
										break dance
									}
								}

								Consistently(registered, 2*heartbeatInterval).ShouldNot(Receive())
							})
						})
					})
				})

				Context("when running forward-worker", func() {
					BeforeEach(func() {
						sshArgv = append(sshArgv, "-R", fmt.Sprintf("0.0.0.0:0:%s", gardenAddr), "forward-worker")
					})

					It("does not exit", func() {
						Consistently(sshSess, 1).ShouldNot(gexec.Exit())
					})

					Describe("sending a worker payload on stdin", func() {
						type registration struct {
							worker atc.Worker
							ttl    time.Duration
						}

						var workerPayload atc.Worker
						var registered chan registration

						BeforeEach(func() {
							workerPayload = atc.Worker{
								Platform: "linux",
								Tags:     []string{"some", "tags"},

								ResourceTypes: []atc.WorkerResourceType{
									{Type: "resource-type-a", Image: "resource-image-a"},
									{Type: "resource-type-b", Image: "resource-image-b"},
								},
							}

							registered = make(chan registration)

							atcServer.RouteToHandler("POST", "/api/v1/workers", func(w http.ResponseWriter, r *http.Request) {
								var worker atc.Worker
								err := json.NewDecoder(r.Body).Decode(&worker)
								Ω(err).ShouldNot(HaveOccurred())

								ttl, err := time.ParseDuration(r.URL.Query().Get("ttl"))
								Ω(err).ShouldNot(HaveOccurred())

								registered <- registration{worker, ttl}
							})

							stubs := make(chan func() ([]garden.Container, error), 4)

							stubs <- func() ([]garden.Container, error) {
								return []garden.Container{
									new(gfakes.FakeContainer),
									new(gfakes.FakeContainer),
									new(gfakes.FakeContainer),
								}, nil
							}

							stubs <- func() ([]garden.Container, error) {
								return []garden.Container{
									new(gfakes.FakeContainer),
									new(gfakes.FakeContainer),
								}, nil
							}

							stubs <- func() ([]garden.Container, error) {
								return nil, errors.New("garden was weeded")
							}

							stubs <- func() ([]garden.Container, error) {
								return []garden.Container{
									new(gfakes.FakeContainer),
								}, nil
							}

							fakeBackend.ContainersStub = func(garden.Properties) ([]garden.Container, error) {
								return (<-stubs)()
							}
						})

						JustBeforeEach(func() {
							err := json.NewEncoder(sshStdin).Encode(workerPayload)
							Ω(err).ShouldNot(HaveOccurred())
						})

						It("forwards garden API calls through the tunnel", func() {
							registration := <-registered
							addr := registration.worker.Addr

							client := gclient.New(gconn.New("tcp", addr))

							fakeBackend.CreateReturns(new(gfakes.FakeContainer), nil)

							_, err := client.Create(garden.ContainerSpec{})
							Ω(err).ShouldNot(HaveOccurred())

							Ω(fakeBackend.CreateCallCount()).Should(Equal(1))
						})

						It("continuously registers it with the ATC as long as it works", func() {
							a := time.Now()
							registration := <-registered
							Ω(registration.ttl).Should(Equal(2 * heartbeatInterval))

							// shortcut for equality w/out checking addr
							expectedWorkerPayload := workerPayload
							expectedWorkerPayload.Addr = registration.worker.Addr
							expectedWorkerPayload.ActiveContainers = 3
							Ω(registration.worker).Should(Equal(expectedWorkerPayload))

							host, _, err := net.SplitHostPort(registration.worker.Addr)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(host).Should(Equal(forwardHost))

							b := time.Now()
							registration = <-registered
							Ω(registration.ttl).Should(Equal(2 * heartbeatInterval))

							// shortcut for equality w/out checking addr
							expectedWorkerPayload = workerPayload
							expectedWorkerPayload.Addr = registration.worker.Addr
							expectedWorkerPayload.ActiveContainers = 2
							Ω(registration.worker).Should(Equal(expectedWorkerPayload))

							host, _, err = net.SplitHostPort(registration.worker.Addr)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(host).Should(Equal(forwardHost))

							Ω(b.Sub(a)).Should(BeNumerically("~", heartbeatInterval, 1*time.Second))

							Consistently(registered, 2*heartbeatInterval).ShouldNot(Receive())

							c := time.Now()
							registration = <-registered
							Ω(registration.ttl).Should(Equal(2 * heartbeatInterval))

							// shortcut for equality w/out checking addr
							expectedWorkerPayload = workerPayload
							expectedWorkerPayload.Addr = registration.worker.Addr
							expectedWorkerPayload.ActiveContainers = 1
							Ω(registration.worker).Should(Equal(expectedWorkerPayload))

							host, _, err = net.SplitHostPort(registration.worker.Addr)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(host).Should(Equal(forwardHost))

							Ω(c.Sub(b)).Should(BeNumerically("~", 3*heartbeatInterval, 1*time.Second))
						})

						Context("when the client goes away", func() {
							It("stops registering", func() {
								time.Sleep(heartbeatInterval)

								sshSess.Interrupt().Wait(10 * time.Second)

								time.Sleep(heartbeatInterval)

								// siphon off any existing registrations
							dance:
								for {
									select {
									case <-registered:
									default:
										break dance
									}
								}

								Consistently(registered, 2*heartbeatInterval).ShouldNot(Receive())
							})
						})
					})
				})

				Context("when running forward-worker with multiple forwarded addresses", func() {
					BeforeEach(func() {
						sshArgv = append(sshArgv, "-R", "0.0.0.0:0:8.6.7.5:7777", "-R", "0.0.0.0:0:3.0.9.9:7777", "forward-worker")
					})

					It("rejects the request", func() {
						Eventually(sshSess.Err, 10).Should(gbytes.Say("Allocated port"))
						Eventually(sshSess.Err, 10).Should(gbytes.Say("remote port forwarding failed"))
					})
				})

				Context("when running a bogus command", func() {
					BeforeEach(func() {
						sshArgv = append(sshArgv, "bogus-command")
					})

					It("exits with failure", func() {
						Eventually(sshSess, 10).Should(gexec.Exit(255))
					})
				})
			})

			Context("with an invalid key", func() {
				BeforeEach(func() {
					badPrivKey, _ := generateSSHKeypair()
					sshArgv = append(sshArgv, "-i", badPrivKey)
				})

				It("exits with failure", func() {
					Eventually(sshSess, 10).Should(gexec.Exit(255))
				})
			})
		})
	})
})
