package api_test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/worker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	workerfakes "github.com/concourse/atc/worker/fakes"
)

const (
	pipelineName1 = "pipeline-1"
	type1         = worker.ContainerTypeCheck
	name1         = "name-1"
	buildID1      = 1234
	containerID1  = "dh93mvi"
)

var _ = Describe("Pipelines API", func() {
	var (
		req *http.Request

		fakeContainer1              *workerfakes.FakeContainer
		expectedPresentedContainer1 present.PresentedContainer
	)

	BeforeEach(func() {
		fakeContainer1 = &workerfakes.FakeContainer{}
		fakeContainer1.IdentifierFromPropertiesReturns(
			worker.Identifier{
				PipelineName: pipelineName1,
				Type:         type1,
				Name:         name1,
				BuildID:      buildID1,
			})

		fakeContainer1.HandleReturns(containerID1)

		expectedPresentedContainer1 = present.PresentedContainer{
			ID:           containerID1,
			PipelineName: pipelineName1,
			Type:         type1,
			Name:         name1,
			BuildID:      buildID1,
		}
	})

	Describe("GET /api/v1/containers", func() {
		var (
			fakeContainer2 *workerfakes.FakeContainer
			fakeContainers []worker.Container

			expectedPresentedContainer2 present.PresentedContainer
			expectedPresentedContainers []present.PresentedContainer
		)

		BeforeEach(func() {
			fakeContainer1 = &workerfakes.FakeContainer{}
			fakeContainer1.IdentifierFromPropertiesReturns(
				worker.Identifier{
					PipelineName: pipelineName1,
					Type:         type1,
					Name:         name1,
					BuildID:      buildID1,
				})
			fakeContainer1.HandleReturns(containerID1)

			fakeContainer2 = &workerfakes.FakeContainer{}
			fakeContainer2.IdentifierFromPropertiesReturns(
				worker.Identifier{
					PipelineName: "pipeline-2",
					Type:         worker.ContainerTypePut,
					Name:         "name-2",
					BuildID:      4321,
				})
			fakeContainer2.HandleReturns("cfvwser")

			fakeContainers = []worker.Container{
				fakeContainer1,
				fakeContainer2,
			}

			expectedPresentedContainer2 = present.PresentedContainer{
				ID:           "cfvwser",
				PipelineName: "pipeline-2",
				Type:         worker.ContainerTypePut,
				Name:         "name-2",
				BuildID:      4321,
			}

			expectedPresentedContainers = []present.PresentedContainer{
				expectedPresentedContainer1,
				expectedPresentedContainer2,
			}

			fakeWorkerClient.FindContainersForIdentifierReturns(fakeContainers, nil)

			var err error
			req, err = http.NewRequest("GET", server.URL+"/api/v1/containers", nil)
			Ω(err).ShouldNot(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")
		})

		Context("with no params", func() {
			It("returns 200", func() {
				response, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(response.StatusCode).Should(Equal(http.StatusOK))
			})

			It("returns Content-Type application/json", func() {
				response, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(response.Header.Get("Content-Type")).Should(Equal("application/json"))
			})

			It("returns all containers", func() {
				response, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				b, err := ioutil.ReadAll(response.Body)
				Ω(err).ShouldNot(HaveOccurred())

				var actualPresentedContainers []present.PresentedContainer
				err = json.Unmarshal(b, &actualPresentedContainers)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(len(actualPresentedContainers)).To(Equal(len(expectedPresentedContainers)))
				for i, _ := range actualPresentedContainers {
					expected := expectedPresentedContainers[i]
					actual := actualPresentedContainers[i]

					Ω(actual.PipelineName).To(Equal(expected.PipelineName))
					Ω(actual.Type).To(Equal(expected.Type))
					Ω(actual.Name).To(Equal(expected.Name))
					Ω(actual.BuildID).To(Equal(expected.BuildID))
					Ω(actual.ID).To(Equal(expected.ID))
				}
			})

			It("releases all containers", func() {
				_, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				for _, c := range fakeContainers {
					fc := c.(*workerfakes.FakeContainer)
					Ω(fc.ReleaseCallCount()).Should(Equal(1))
				}
			})
		})

		Describe("querying with pipeline name", func() {
			BeforeEach(func() {
				req.URL.RawQuery = url.Values{
					"pipeline": []string{pipelineName1},
				}.Encode()
			})

			It("calls FindContainersForIdentifier with the queried pipeline name", func() {
				_, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				expectedArgs := worker.Identifier{
					PipelineName: pipelineName1,
				}
				Ω(fakeWorkerClient.FindContainersForIdentifierCallCount()).Should(Equal(1))
				Ω(fakeWorkerClient.FindContainersForIdentifierArgsForCall(0)).Should(Equal(expectedArgs))
			})
		})

		Describe("querying with type", func() {
			BeforeEach(func() {
				req.URL.RawQuery = url.Values{
					"type": []string{string(type1)},
				}.Encode()
			})

			It("calls FindContainersForIdentifier with the queried type", func() {
				_, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				expectedArgs := worker.Identifier{
					Type: type1,
				}
				Ω(fakeWorkerClient.FindContainersForIdentifierCallCount()).Should(Equal(1))
				Ω(fakeWorkerClient.FindContainersForIdentifierArgsForCall(0)).Should(Equal(expectedArgs))
			})
		})

		Describe("querying with name", func() {
			BeforeEach(func() {
				req.URL.RawQuery = url.Values{
					"name": []string{string(name1)},
				}.Encode()
			})

			It("calls FindContainersForIdentifier with the queried name", func() {
				_, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				expectedArgs := worker.Identifier{
					Name: name1,
				}
				Ω(fakeWorkerClient.FindContainersForIdentifierCallCount()).Should(Equal(1))
				Ω(fakeWorkerClient.FindContainersForIdentifierArgsForCall(0)).Should(Equal(expectedArgs))
			})
		})

		Describe("querying with build-id", func() {
			Context("when the buildID can be parsed as an int", func() {
				BeforeEach(func() {
					buildID1String := strconv.Itoa(buildID1)

					req.URL.RawQuery = url.Values{
						"build-id": []string{buildID1String},
					}.Encode()
				})

				It("calls FindContainersForIdentifier with the queried build id", func() {
					_, err := client.Do(req)
					Ω(err).ShouldNot(HaveOccurred())

					expectedArgs := worker.Identifier{
						BuildID: buildID1,
					}
					Ω(fakeWorkerClient.FindContainersForIdentifierCallCount()).Should(Equal(1))
					Ω(fakeWorkerClient.FindContainersForIdentifierArgsForCall(0)).Should(Equal(expectedArgs))
				})
			})

			Context("when the buildID fails to be parsed as an int", func() {
				BeforeEach(func() {
					req.URL.RawQuery = url.Values{
						"build-id": []string{"not-an-int"},
					}.Encode()
				})

				It("returns 400 Bad Request", func() {
					response, _ := client.Do(req)
					Ω(response.StatusCode).Should(Equal(http.StatusBadRequest))
				})

				It("does not lookup containers", func() {
					client.Do(req)

					Ω(fakeWorkerClient.FindContainersForIdentifierCallCount()).Should(Equal(0))
				})
			})
		})
	})

	Describe("GET /api/v1/containers/:id", func() {
		const (
			containerID = "23sxrfu"
		)

		BeforeEach(func() {
			fakeWorkerClient.LookupContainerReturns(fakeContainer1, nil)

			var err error
			req, err = http.NewRequest("GET", server.URL+"/api/v1/containers/"+containerID, nil)
			Ω(err).ShouldNot(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")
		})

		Context("when the container is not found", func() {
			BeforeEach(func() {
				fakeWorkerClient.LookupContainerReturns(fakeContainer1, worker.ErrContainerNotFound)
			})

			It("returns 404 Not Found", func() {
				response, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
			})
		})

		Context("when the container is found", func() {
			BeforeEach(func() {
				fakeWorkerClient.LookupContainerReturns(fakeContainer1, nil)
			})

			It("returns 200 OK", func() {
				response, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(response.StatusCode).Should(Equal(http.StatusOK))
			})

			It("returns Content-Type application/json", func() {
				response, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(response.Header.Get("Content-Type")).Should(Equal("application/json"))
			})

			It("performs lookup by id", func() {
				_, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeWorkerClient.LookupContainerCallCount()).Should(Equal(1))
				Ω(fakeWorkerClient.LookupContainerArgsForCall(0)).Should(Equal(containerID))
			})

			It("returns the container", func() {
				response, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				b, err := ioutil.ReadAll(response.Body)
				Ω(err).ShouldNot(HaveOccurred())

				var actual present.PresentedContainer
				err = json.Unmarshal(b, &actual)
				Ω(err).ShouldNot(HaveOccurred())

				expected := expectedPresentedContainer1

				Ω(actual.PipelineName).To(Equal(expected.PipelineName))
				Ω(actual.Type).To(Equal(expected.Type))
				Ω(actual.Name).To(Equal(expected.Name))
				Ω(actual.BuildID).To(Equal(expected.BuildID))
				Ω(actual.ID).To(Equal(expected.ID))
			})

			It("releases the container", func() {
				_, err := client.Do(req)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeContainer1.ReleaseCallCount()).Should(Equal(1))
			})
		})
	})
})
