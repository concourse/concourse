package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/concourse/worker/reaper/api"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerServer", func() {
	var (
		handler      http.Handler
		logger       *lagertest.TestLogger
		gardenClient *gardenfakes.FakeClient
		err          error
		recorder     *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		gardenClient = new(gardenfakes.FakeClient)
		logger = lagertest.NewTestLogger("container-server")
		handler, err = NewHandler(logger, gardenClient)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Server is running", func() {
		Describe("Ping the ContainerServer", func() {
			Context("Garden server is available", func() {
				JustBeforeEach(func() {
					gardenClient.PingReturns(nil)
					recorder = httptest.NewRecorder()
					request, _ := http.NewRequest("GET", "/ping", nil)
					handler.ServeHTTP(recorder, request)
				})

				It("Responds with 200 OK", func() {
					Expect(recorder.Result().StatusCode).To(Equal(http.StatusOK))
				})
			})

			Context("Garden server is unavailable", func() {
				JustBeforeEach(func() {
					gardenClient.PingReturns(errors.New("some-error"))
					recorder = httptest.NewRecorder()
					request, _ := http.NewRequest("GET", "/ping", nil)
					handler.ServeHTTP(recorder, request)
				})

				It("Responds with 500 Internal Server Error", func() {
					Expect(recorder.Result().StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

		})

		Describe("Request destruction of containers", func() {
			Context("All containers are found and destroyed", func() {
				JustBeforeEach(func() {
					containerHandles := []string{"container-one", "container-two"}
					bcList, _ := json.Marshal(containerHandles)
					gardenClient.DestroyReturns(nil)
					recorder = httptest.NewRecorder()
					request, _ := http.NewRequest("DELETE", "/containers/destroy", bytes.NewReader(bcList))
					handler.ServeHTTP(recorder, request)
				})

				It("Responds with 204 No Content", func() {
					Expect(recorder.Result().StatusCode).To(Equal(http.StatusNoContent))
				})

				It("Calls garden client.Destroy for each container handle passed in", func() {
					Expect(gardenClient.DestroyCallCount()).To(Equal(2))
				})
			})

			Context("Containers are not found and destroyed", func() {
				JustBeforeEach(func() {
					containerHandles := []string{"container-one", "container-two", "container-three"}
					bcList, _ := json.Marshal(containerHandles)
					gardenClient.DestroyReturns(garden.ContainerNotFoundError{Handle: "container-one"})
					recorder = httptest.NewRecorder()
					request, _ := http.NewRequest("DELETE", "/containers/destroy", bytes.NewReader(bcList))
					handler.ServeHTTP(recorder, request)
				})

				It("Responds with 204 No Content", func() {
					Expect(recorder.Result().StatusCode).To(Equal(http.StatusNoContent))
				})

				It("Calls garden client.Destroy for each container handle passed in", func() {
					Expect(gardenClient.DestroyCallCount()).To(Equal(3))
				})
			})

			Context("Garden container lookups cause an error", func() {
				JustBeforeEach(func() {
					containerHandles := []string{"container-one", "container-two", "container-three"}
					bcList, _ := json.Marshal(containerHandles)
					gardenClient.DestroyReturns(errors.New("some-error"))
					recorder = httptest.NewRecorder()
					request, _ := http.NewRequest("DELETE", "/containers/destroy", bytes.NewReader(bcList))
					handler.ServeHTTP(recorder, request)
				})

				It("Responds with 500 Internal Server Error", func() {
					Expect(recorder.Result().StatusCode).To(Equal(http.StatusInternalServerError))
				})

				It("Calls garden client.Destroy for each container handle passed in", func() {
					Expect(gardenClient.DestroyCallCount()).To(Equal(3))
				})
			})

			Context("Request body is not formed properly", func() {
				JustBeforeEach(func() {
					containerHandles := map[string]string{"container1": "handle1", "container2": "handle2"}
					bcList, _ := json.Marshal(containerHandles)
					gardenClient.DestroyReturns(errors.New("some-error"))
					recorder = httptest.NewRecorder()
					request, _ := http.NewRequest("DELETE", "/containers/destroy", bytes.NewReader(bcList))
					handler.ServeHTTP(recorder, request)
				})

				It("Responds with 400 Bad Request Error", func() {
					Expect(recorder.Result().StatusCode).To(Equal(http.StatusBadRequest))
				})

				It("Calls garden client.Destroy for each container handle passed in", func() {
					Expect(gardenClient.DestroyCallCount()).To(Equal(0))
				})
			})
		})

		Describe("Request list of containers", func() {
			Context("returns list of containers", func() {
				JustBeforeEach(func() {
					container1 := &gardenfakes.FakeContainer{}
					container1.HandleReturns("handle1")
					container2 := &gardenfakes.FakeContainer{}
					container2.HandleReturns("handle2")

					containerList := []garden.Container{container1, container2}
					gardenClient.ContainersReturns(containerList, nil)
					recorder = httptest.NewRecorder()
					request, _ := http.NewRequest("GET", "/containers/list", nil)
					handler.ServeHTTP(recorder, request)
				})

				It("Responds with 200", func() {
					Expect(recorder.Result().StatusCode).To(Equal(http.StatusOK))
					respBody, err := ioutil.ReadAll(recorder.Result().Body)
					Expect(err).NotTo(HaveOccurred())

					var handles []string
					err = json.Unmarshal(respBody, &handles)
					Expect(err).NotTo(HaveOccurred())
					Expect(handles).To(ContainElement("handle2"))
					Expect(handles).To(ContainElement("handle1"))
				})

				It("Calls garden client.Destroy for each container handle passed in", func() {
					Expect(gardenClient.ContainersCallCount()).To(Equal(1))
				})
			})

			Context("Garden client has an error", func() {
				JustBeforeEach(func() {
					containerList := []garden.Container{}
					gardenClient.ContainersReturns(containerList, errors.New("bad-happ"))
					recorder = httptest.NewRecorder()
					request, _ := http.NewRequest("GET", "/containers/list", nil)
					handler.ServeHTTP(recorder, request)
				})

				It("Responds with 500 Internal Server Error", func() {
					Expect(recorder.Result().StatusCode).To(Equal(http.StatusInternalServerError))
				})

				It("Calls garden client.Destroy for each container handle passed in", func() {
					Expect(gardenClient.ContainersCallCount()).To(Equal(1))
				})
			})
		})
	})
})
