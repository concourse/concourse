package integration_test

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/testhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/vito/go-sse/sse"
)

var _ = Describe("Fly CLI", func() {
	var tmpdir string
	var buildDir string
	var taskConfigPath string

	var streaming chan struct{}
	var events chan atc.Event
	var uploadedBits chan struct{}

	var expectedPlan atc.Plan
	var taskPlan atc.Plan
	var workerArtifact = atc.WorkerArtifact{
		ID:   125,
		Name: "some-dir",
	}
	var planFactory atc.PlanFactory

	BeforeEach(func() {
		var err error
		tmpdir, err = ioutil.TempDir("", "fly-build-dir")
		Expect(err).NotTo(HaveOccurred())

		buildDir = filepath.Join(tmpdir, "fixture")

		err = os.Mkdir(buildDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		taskConfigPath = filepath.Join(buildDir, "task.yml")

		err = ioutil.WriteFile(
			taskConfigPath,
			[]byte(`---
platform: some-platform

image_resource:
  type: registry-image
  source:
    repository: ubuntu

inputs:
- name: fixture

params:
  FOO: bar
  BAZ: buzz
  X: 1
  EMPTY:

run:
  path: find
  args: [.]
`),
			0644,
		)
		Expect(err).NotTo(HaveOccurred())

		streaming = make(chan struct{})
		events = make(chan atc.Event)

		planFactory = atc.NewPlanFactory(0)

		taskPlan = planFactory.NewPlan(atc.TaskPlan{
			Name: "one-off",
			Config: &atc.TaskConfig{
				Platform: "some-platform",
				ImageResource: &atc.ImageResource{
					Type: "registry-image",
					Source: atc.Source{
						"repository": "ubuntu",
					},
				},
				Inputs: []atc.TaskInputConfig{
					{Name: "fixture"},
				},
				Params: map[string]string{
					"FOO":   "bar",
					"BAZ":   "buzz",
					"X":     "1",
					"EMPTY": "",
				},
				Run: atc.TaskRunConfig{
					Path: "find",
					Args: []string{"."},
				},
			},
		})

		expectedPlan = planFactory.NewPlan(atc.DoPlan{
			planFactory.NewPlan(atc.InParallelPlan{
				Steps: []atc.Plan{
					planFactory.NewPlan(atc.ArtifactInputPlan{
						ArtifactID: 125,
						Name:       filepath.Base(buildDir),
					}),
				},
			}),
			taskPlan,
		})
	})

	AfterEach(func() {
		os.RemoveAll(tmpdir)
		close(uploadedBits)
	})

	JustBeforeEach(func() {
		uploadedBits = make(chan struct{}, 5) // at most there should only be 2 uploads
		atcServer.RouteToHandler("POST", "/api/v1/teams/main/artifacts",
			ghttp.CombineHandlers(
				func(w http.ResponseWriter, req *http.Request) {
					gr, err := gzip.NewReader(req.Body)
					Expect(err).NotTo(HaveOccurred())

					tr := tar.NewReader(gr)

					hdr, err := tr.Next()
					Expect(err).NotTo(HaveOccurred())

					Expect(hdr.Name).To(Equal("./"))

					hdr, err = tr.Next()
					Expect(err).NotTo(HaveOccurred())

					Expect(hdr.Name).To(MatchRegexp("(./)?task.yml$"))

					uploadedBits <- struct{}{}
				},
				ghttp.RespondWith(201, `{"id":125}`),
			),
		)
		atcServer.RouteToHandler("POST", "/api/v1/teams/main/builds",
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/api/v1/teams/main/builds"),
				VerifyPlan(expectedPlan),
				func(w http.ResponseWriter, r *http.Request) {
					http.SetCookie(w, &http.Cookie{
						Name:    "Some-Cookie",
						Value:   "some-cookie-data",
						Path:    "/",
						Expires: time.Now().Add(1 * time.Minute),
					})
				},
				ghttp.RespondWith(201, `{"id":128}`),
			),
		)
		atcServer.RouteToHandler("GET", "/api/v1/builds/128/events",
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/api/v1/builds/128/events"),
				func(w http.ResponseWriter, r *http.Request) {
					flusher := w.(http.Flusher)

					w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
					w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
					w.Header().Add("Connection", "keep-alive")

					w.WriteHeader(http.StatusOK)

					flusher.Flush()

					close(streaming)

					id := 0

					for e := range events {
						payload, err := json.Marshal(event.Message{Event: e})
						Expect(err).NotTo(HaveOccurred())

						event := sse.Event{
							ID:   fmt.Sprintf("%d", id),
							Name: "event",
							Data: payload,
						}

						err = event.Write(w)
						Expect(err).NotTo(HaveOccurred())

						flusher.Flush()

						id++
					}

					err := sse.Event{
						Name: "end",
					}.Write(w)
					Expect(err).NotTo(HaveOccurred())
				},
			),
		)
		atcServer.RouteToHandler("GET", "/api/v1/builds/128/artifacts",
			ghttp.RespondWithJSONEncoded(200, []atc.WorkerArtifact{workerArtifact}),
		)

	})

	It("creates a build, streams output, uploads the bits, and polls until completion", func() {
		flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath)
		flyCmd.Dir = buildDir

		sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(streaming).Should(BeClosed())

		buildURL, _ := url.Parse(atcServer.URL())
		buildURL.Path = path.Join(buildURL.Path, "builds/128")
		Eventually(sess.Out).Should(gbytes.Say("executing build 128 at %s", buildURL.String()))

		events <- event.Log{Payload: "sup"}

		Eventually(sess.Out).Should(gbytes.Say("sup"))

		close(events)

		<-sess.Exited
		Expect(sess.ExitCode()).To(Equal(0))

		Expect(uploadedBits).To(HaveLen(1))
	})

	Context("when there is a pipeline job with the same input", func() {
		BeforeEach(func() {
			taskPlan.Task.ResourceTypes = atc.ResourceTypes{
				{
					Name:   "resource-type",
					Type:   "s3",
					Source: atc.Source{},
				},
			}

			planFactory := atc.NewPlanFactory(0)

			expectedCheckPlan := planFactory.NewPlan(atc.CheckPlan{
				Name:         "resource-type",
				ResourceType: "resource-type",
				Type:         "s3",
				Source:       atc.Source{},
				TypeImage:    atc.TypeImage{BaseType: "s3"},
				Interval: atc.CheckEvery{
					Interval: 0,
				},
			})
			expectedGetPlan := planFactory.NewPlan(atc.GetPlan{
				Name:        "resource-type",
				Type:        "s3",
				Source:      atc.Source{},
				TypeImage:   atc.TypeImage{BaseType: "s3"},
				VersionFrom: &expectedCheckPlan.ID,
			})
			expectedPlan = planFactory.NewPlan(atc.DoPlan{
				planFactory.NewPlan(atc.InParallelPlan{
					Steps: []atc.Plan{
						planFactory.NewPlan(atc.GetPlan{
							Name: "fixture",
							Type: "resource-type",
							TypeImage: atc.TypeImage{
								BaseType:  "s3",
								CheckPlan: &expectedCheckPlan,
								GetPlan:   &expectedGetPlan,
							},
						}),
					},
				}),
				taskPlan,
			})

			expectedQueryParams := "vars.branch=%22master%22"
			atcServer.RouteToHandler("POST", "/api/v1/teams/main/pipelines/some-pipeline/builds",
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/api/v1/teams/main/pipelines/some-pipeline/builds", expectedQueryParams),
					testhelpers.VerifyPlan(expectedPlan),
					func(w http.ResponseWriter, r *http.Request) {
						http.SetCookie(w, &http.Cookie{
							Name:    "Some-Cookie",
							Value:   "some-cookie-data",
							Path:    "/",
							Expires: time.Now().Add(1 * time.Minute),
						})
					},
					ghttp.RespondWith(201, `{"id":128}`),
				),
			)
			atcServer.RouteToHandler("GET", "/api/v1/teams/main/pipelines/some-pipeline/jobs/some-job/inputs",
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/some-pipeline/jobs/some-job/inputs", expectedQueryParams),
					ghttp.RespondWithJSONEncoded(200, []atc.BuildInput{{Name: "fixture", Type: "resource-type"}}),
				),
			)
			atcServer.RouteToHandler("GET", "/api/v1/teams/main/pipelines/some-pipeline/resource-types",
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/some-pipeline/resource-types", expectedQueryParams),
					ghttp.RespondWithJSONEncoded(200, atc.ResourceTypes{
						{
							Name:   "resource-type",
							Type:   "s3",
							Source: atc.Source{},
						},
					}),
				),
			)
		})

		It("creates a build, streams output, and polls until completion", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath, "-j", "some-pipeline/branch:master/some-job")
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(streaming).Should(BeClosed())

			buildURL, _ := url.Parse(atcServer.URL())
			buildURL.Path = path.Join(buildURL.Path, "builds/128")
			Eventually(sess.Out).Should(gbytes.Say("executing build 128 at %s", buildURL.String()))

			events <- event.Log{Payload: "sup"}

			Eventually(sess.Out).Should(gbytes.Say("sup"))

			close(events)

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))
		})

	})

	Context("when the build config is invalid", func() {
		BeforeEach(func() {
			// missing platform and run path
			err := ioutil.WriteFile(
				filepath.Join(buildDir, "task.yml"),
				[]byte(`---
run: {}
`),
				0644,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("prints the failure and exits 1", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath)
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess.Err).Should(gbytes.Say("missing"))

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(1))
		})
	})

	Context("when the build config is valid", func() {
		JustBeforeEach(func() {
			atcServer.RouteToHandler("POST", "/api/v1/teams/main/artifacts",
				ghttp.CombineHandlers(
					func(w http.ResponseWriter, req *http.Request) {
						uploadedBits <- struct{}{}
					},
					ghttp.RespondWith(201, `{"id":125}`),
				),
			)
		})

		Context("when task defines one input but it was not passed in as a flag", func() {
			It("uploads the current directory", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath)
				flyCmd.Dir = buildDir

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				buildURL, _ := url.Parse(atcServer.URL())
				buildURL.Path = path.Join(buildURL.Path, "builds/128")
				Eventually(sess.Out).Should(gbytes.Say("executing build 128 at %s", buildURL.String()))

				close(events)

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))

				Expect(uploadedBits).To(HaveLen(1))
			})
		})

		Context("when task defines 2 inputs but only 1 was passed as a flag", func() {
			var bardir string

			BeforeEach(func() {
				err := ioutil.WriteFile(
					filepath.Join(buildDir, "task.yml"),
					[]byte(`---
platform: some-platform

image_resource:
  type: registry-image
  source:
    repository: ubuntu

inputs:
- name: fixture
- name: bar

run:
  path: find
  args: [.]
`),
					0644,
				)
				bardir = filepath.Join(tmpdir, "bar")
				err = os.Mkdir(bardir, 0755)
				Expect(err).ToNot(HaveOccurred())

				taskPlan.Task.Config.Inputs = []atc.TaskInputConfig{
					{Name: "fixture"},
					{Name: "bar"},
				}
				taskPlan.Task.Config.Params = nil

				Expect(err).NotTo(HaveOccurred())
				expectedPlan = planFactory.NewPlan(atc.DoPlan{
					planFactory.NewPlan(atc.InParallelPlan{
						Steps: []atc.Plan{
							planFactory.NewPlan(atc.ArtifactInputPlan{
								ArtifactID: 125,
								Name:       filepath.Base(buildDir),
							}),
							planFactory.NewPlan(atc.ArtifactInputPlan{
								ArtifactID: 125,
								Name:       filepath.Base(bardir),
							}),
						},
					}),
					taskPlan,
				})

			})

			AfterEach(func() {
				os.RemoveAll(bardir)
			})

			Context("when the current directory name is the same as the missing input", func() {
				It("uploads the current directory", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "e",
						"-c", taskConfigPath,
						"-i", fmt.Sprintf("bar=%s", bardir),
					)
					flyCmd.Dir = buildDir

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					buildURL, _ := url.Parse(atcServer.URL())
					buildURL.Path = path.Join(buildURL.Path, "builds/128")
					Eventually(sess.Out).Should(gbytes.Say("executing build 128 at %s", buildURL.String()))

					close(events)

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))

					Expect(uploadedBits).To(HaveLen(2))
				})
			})

			Context("when the current directory name is not the same as the missing input", func() {
				BeforeEach(func() {
					err := ioutil.WriteFile(
						filepath.Join(buildDir, "task.yml"),
						[]byte(`---
platform: some-platform

image_resource:
  type: registry-image
  source:
    repository: ubuntu

inputs:
- name: foo
- name: bar

params:
  FOO: bar
  BAZ: buzz
  X: 1
  EMPTY:

run:
  path: find
  args: [.]
`),
						0644,
					)
					Expect(err).NotTo(HaveOccurred())
					(*expectedPlan.Do)[1].Task.Config.Inputs = []atc.TaskInputConfig{
						{Name: "foo"},
						{Name: "bar"},
					}
				})

				It("errors with the missing input", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "e",
						"-c", taskConfigPath,
						"-i", fmt.Sprintf("bar=%s", bardir),
					)
					flyCmd.Dir = buildDir

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Err).Should(gbytes.Say("error: missing required input `foo`"))

					close(events)

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))

					Expect(uploadedBits).To(HaveLen(1))
				})
			})
		})
	})

	Context("when arguments include input that is not a git repo", func() {

		Context("when arguments not include --include-ignored", func() {
			It("uploading with everything", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath, "-i", "fixture="+buildDir)

				flyCmd.Dir = buildDir

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				// sync with after create
				Eventually(streaming).Should(BeClosed())

				close(events)

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))

				Expect(uploadedBits).To(HaveLen(1))
			})
		})
	})

	Context("when arguments include input that is a git repo", func() {

		BeforeEach(func() {
			gitIgnorePath := filepath.Join(buildDir, ".gitignore")

			err := ioutil.WriteFile(gitIgnorePath, []byte(`*.test`), 0644)
			Expect(err).NotTo(HaveOccurred())

			fileToBeIgnoredPath := filepath.Join(buildDir, "dev.test")
			err = ioutil.WriteFile(fileToBeIgnoredPath, []byte(`test file content`), 0644)
			Expect(err).NotTo(HaveOccurred())

			err = os.Mkdir(filepath.Join(buildDir, ".git"), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = os.Mkdir(filepath.Join(buildDir, ".git/refs"), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = os.Mkdir(filepath.Join(buildDir, ".git/objects"), 0755)
			Expect(err).NotTo(HaveOccurred())

			gitHEADPath := filepath.Join(buildDir, ".git/HEAD")
			err = ioutil.WriteFile(gitHEADPath, []byte(`ref: refs/heads/master`), 0644)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when arguments not include --include-ignored", func() {
			It("by default apply .gitignore", func() {
				atcServer.RouteToHandler("POST", "/api/v1/teams/main/artifacts",
					ghttp.CombineHandlers(
						func(w http.ResponseWriter, req *http.Request) {
							gr, err := gzip.NewReader(req.Body)
							Expect(err).NotTo(HaveOccurred())

							tr := tar.NewReader(gr)

							var matchFound = false
							for {
								hdr, err := tr.Next()
								if err != nil {
									break
								}
								if strings.Contains(hdr.Name, "dev.test") {
									matchFound = true
									break
								}
							}

							Expect(matchFound).To(Equal(false))

							uploadedBits <- struct{}{}
						},
						ghttp.RespondWith(201, `{"id":125}`),
					),
				)
				flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath)
				flyCmd.Dir = buildDir

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				// sync with after create
				Eventually(streaming).Should(BeClosed())

				close(events)

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))

				Expect(uploadedBits).To(HaveLen(1))
			})
		})

		Context("when arguments include --include-ignored", func() {
			It("uploading with everything", func() {
				atcServer.RouteToHandler("POST", "/api/v1/teams/main/artifacts",
					ghttp.CombineHandlers(
						func(w http.ResponseWriter, req *http.Request) {
							Expect(req.FormValue("platform")).To(Equal("some-platform"))

							gr, err := gzip.NewReader(req.Body)
							Expect(err).NotTo(HaveOccurred())

							tr := tar.NewReader(gr)

							var matchFound = false
							for {
								hdr, err := tr.Next()
								if err != nil {
									break
								}
								if strings.Contains(hdr.Name, "dev.test") {
									matchFound = true
									break
								}
							}

							Expect(matchFound).To(Equal(true))
							uploadedBits <- struct{}{}
						},
						ghttp.RespondWith(201, `{"id":125}`),
					),
				)

				flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath, "--include-ignored")
				flyCmd.Dir = buildDir

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				// sync with after create
				Eventually(streaming).Should(BeClosed())

				close(events)

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))

				Expect(uploadedBits).To(HaveLen(1))
			})
		})
	})

	Context("when arguments are passed through", func() {
		BeforeEach(func() {
			(*expectedPlan.Do)[1].Task.Config.Run.Args = []string{".", "-name", `foo "bar" baz`}
		})

		It("inserts them into the config template", func() {
			atcServer.AllowUnhandledRequests = true

			flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath, "--", "-name", "foo \"bar\" baz")
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			// sync with after create
			Eventually(streaming).Should(BeClosed())

			close(events)

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))

			Expect(uploadedBits).To(HaveLen(1))
		})
	})

	Context("when tags are specified", func() {
		BeforeEach(func() {
			(*expectedPlan.Do)[1].Task.Tags = []string{"tag-1", "tag-2"}
		})

		It("sprinkles them on the task", func() {
			atcServer.AllowUnhandledRequests = true

			flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath, "--tag", "tag-1", "--tag", "tag-2")
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			// sync with after create
			Eventually(streaming).Should(BeClosed())

			close(events)

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))

			Expect(uploadedBits).To(HaveLen(1))
		})
	})

	Context("when invalid inputs are passed", func() {
		It("prints an error", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath, "-i", "fixture=.", "-i", "evan=.")
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess.Err).Should(gbytes.Say("unknown input `evan`"))

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(1))
		})

		Context("when input is not a folder", func() {
			It("prints an error", func() {
				testFile := filepath.Join(buildDir, "test-file.txt")
				err := ioutil.WriteFile(
					testFile,
					[]byte(`test file content`),
					0644,
				)
				Expect(err).NotTo(HaveOccurred())

				flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath, "-i", "fixture=./test-file.txt")
				flyCmd.Dir = buildDir

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err).Should(gbytes.Say("./test-file.txt not a folder"))

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
			})
		})

		Context("when invalid inputs are passed and the single valid input is correctly omitted", func() {
			It("prints an error about invalid inputs instead of missing inputs", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath, "-i", "evan=.")
				flyCmd.Dir = buildDir

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err).Should(gbytes.Say("unknown input `evan`"))

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
			})
		})
	})

	Context("when the task specifies no input", func() {
		BeforeEach(func() {
			err := ioutil.WriteFile(
				filepath.Join(buildDir, "task.yml"),
				[]byte(`---
platform: some-platform

image_resource:
  type: registry-image
  source:
    repository: ubuntu

inputs:

params:
  FOO: bar
  BAZ: buzz
  X: 1
  EMPTY:


run:
  path: find
  args: [.]
`),
				0644,
			)
			Expect(err).NotTo(HaveOccurred())
			(*expectedPlan.Do)[1].Task.Config.Inputs = nil
			(*expectedPlan.Do)[0].InParallel = &atc.InParallelPlan{}
		})

		It("shouldn't upload the current directory", func() {
			atcServer.RouteToHandler("POST", "/api/v1/teams/main/artifacts",
				ghttp.CombineHandlers(
					func(w http.ResponseWriter, req *http.Request) {
						uploadedBits <- struct{}{}
					},
					ghttp.RespondWith(201, `{"id":125}`),
				),
			)

			flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath)
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			close(events)

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))
			Expect(uploadedBits).To(HaveLen(0))
		})
	})

	Context("when the task specifies an optional input", func() {
		BeforeEach(func() {
			err := ioutil.WriteFile(
				filepath.Join(buildDir, "task.yml"),
				[]byte(`---
platform: some-platform

image_resource:
  type: registry-image
  source:
    repository: ubuntu

inputs:
- name: fixture
- name: some-optional-input
  optional: true

params:
  FOO: bar
  BAZ: buzz
  X: 1
  EMPTY:

run:
  path: find
  args: [.]
`),
				0644,
			)
			Expect(err).NotTo(HaveOccurred())
			(*expectedPlan.Do)[1].Task.Config.Inputs = []atc.TaskInputConfig{
				{Name: "fixture"},
				{Name: "some-optional-input", Optional: true},
			}
		})

		Context("when the required input is specified but the optional input is omitted", func() {
			It("runs successfully", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath, "-i", "fixture=.")
				flyCmd.Dir = buildDir

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(streaming).Should(BeClosed())

				buildURL, _ := url.Parse(atcServer.URL())
				buildURL.Path = path.Join(buildURL.Path, "builds/128")
				Eventually(sess.Out).Should(gbytes.Say("executing build 128 at %s", buildURL.String()))

				events <- event.Log{Payload: "sup"}

				Eventually(sess.Out).Should(gbytes.Say("sup"))

				close(events)

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))

				Expect(uploadedBits).To(HaveLen(1))
			})
		})

		Context("when the required input is not specified on the command line", func() {
			It("runs infers the required input successfully", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath)
				flyCmd.Dir = buildDir

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(streaming).Should(BeClosed())

				buildURL, _ := url.Parse(atcServer.URL())
				buildURL.Path = path.Join(buildURL.Path, "builds/128")
				Eventually(sess.Out).Should(gbytes.Say("executing build 128 at %s", buildURL.String()))

				events <- event.Log{Payload: "sup"}

				Eventually(sess.Out).Should(gbytes.Say("sup"))

				close(events)

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))

				Expect(uploadedBits).To(HaveLen(1))
			})
		})
	})

	Context("when the task specifies more than one required input", func() {
		BeforeEach(func() {
			err := ioutil.WriteFile(
				filepath.Join(buildDir, "task.yml"),
				[]byte(`---
platform: some-platform

image_resource:
  type: registry-image
  source:
    repository: ubuntu

inputs:
- name: fixture
- name: something

params:
  FOO: bar
  BAZ: buzz
  X: 1
  EMPTY:

run:
  path: find
  args: [.]
`),
				0644,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("When some required inputs are not passed", func() {
			It("Prints an error", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath, "-i", "fixture=.")
				flyCmd.Dir = buildDir

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err).Should(gbytes.Say("missing required input `something`"))

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
			})

		})

		Context("When no inputs are passed", func() {
			It("Prints an error", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath)
				flyCmd.Dir = buildDir

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err).Should(gbytes.Say("missing required input"))

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
			})
		})
	})

	Context("when running with --privileged", func() {
		BeforeEach(func() {
			(*expectedPlan.Do)[1].Task.Privileged = true
		})

		It("inserts them into the config template", func() {
			atcServer.AllowUnhandledRequests = true

			flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath, "--privileged")
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			// sync with after create
			Eventually(streaming).Should(BeClosed())

			close(events)

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))

			Expect(uploadedBits).To(HaveLen(1))
		})
	})

	Context("when running with bogus flags", func() {
		It("exits 1", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath, "--bogus-flag")
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess.Err).Should(gbytes.Say("unknown flag `bogus-flag'"))

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(1))
		})
	})

	Context("when running with invalid -j flag", func() {
		It("exits 1", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath, "-j", "some-pipeline/invalid/some-job")
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess.Err).Should(gbytes.Say("instance vars should be formatted as <key1:value1>\\(,<key2:value2>\\)"))

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(1))
		})
	})

	Context("when parameters are specified in the environment", func() {
		BeforeEach(func() {
			(*expectedPlan.Do)[1].Task.Config.Params = map[string]string{
				"FOO":   "newbar",
				"BAZ":   "buzz",
				"X":     "",
				"EMPTY": "",
			}
		})

		It("overrides the builds parameter values", func() {
			atcServer.AllowUnhandledRequests = true

			flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath)
			flyCmd.Dir = buildDir
			flyCmd.Env = append(os.Environ(), "FOO=newbar", "X=")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			// sync with after create
			Eventually(streaming).Should(BeClosed())

			close(events)

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))

			Expect(uploadedBits).To(HaveLen(1))
		})
	})

	Context("when the build is interrupted", func() {
		var aborted chan struct{}

		JustBeforeEach(func() {
			aborted = make(chan struct{})

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", "/api/v1/builds/128/abort"),
					func(w http.ResponseWriter, r *http.Request) {
						close(aborted)
					},
				),
			)
		})

		if runtime.GOOS != "windows" {
			Describe("with SIGINT", func() {
				It("aborts the build and exits nonzero", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath)
					flyCmd.Dir = buildDir

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).ToNot(HaveOccurred())

					Eventually(streaming).Should(BeClosed())

					Expect(uploadedBits).To(HaveLen(1))

					sess.Signal(os.Interrupt)

					Eventually(aborted).Should(BeClosed())

					events <- event.Status{Status: atc.StatusErrored}
					close(events)

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(2))
				})
			})

			Describe("with SIGTERM", func() {
				It("aborts the build and exits nonzero", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath)
					flyCmd.Dir = buildDir

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).ToNot(HaveOccurred())

					Eventually(streaming).Should(BeClosed())

					Expect(uploadedBits).To(HaveLen(1))

					sess.Signal(syscall.SIGTERM)

					Eventually(aborted).Should(BeClosed())

					events <- event.Status{Status: atc.StatusErrored}
					close(events)

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(2))
				})
			})
		}
	})

	Context("when the build succeeds", func() {
		It("exits 0", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath)
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(streaming).Should(BeClosed())

			events <- event.Status{Status: atc.StatusSucceeded}
			close(events)

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))

			Expect(uploadedBits).To(HaveLen(1))
		})
	})

	Context("when the build fails", func() {
		It("exits 1", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath)
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(streaming).Should(BeClosed())

			events <- event.Status{Status: atc.StatusFailed}
			close(events)

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(1))

			Expect(uploadedBits).To(HaveLen(1))
		})
	})

	Context("when the build errors", func() {
		It("exits 2", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath)
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(streaming).Should(BeClosed())

			events <- event.Status{Status: atc.StatusErrored}
			close(events)

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(2))

			Expect(uploadedBits).To(HaveLen(1))
		})
	})
})
