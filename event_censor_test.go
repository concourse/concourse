package auth_test

import (
	"encoding/json"
	"fmt"
	"time"

	. "github.com/concourse/atc/auth"
	"github.com/concourse/turbine/api/builds"
	"github.com/concourse/turbine/event"
	"github.com/vito/go-sse/sse"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EventCensor", func() {
	var censor *EventCensor

	BeforeEach(func() {
		censor = &EventCensor{}
	})

	Describe("Censor", func() {
		Context("with the v0.0 protocol", func() {
			BeforeEach(func() {
				_, err := censor.Censor(sse.Event{
					Name: "version",
					Data: []byte("0.0"),
				})
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("passes log events through verbatim", func() {
				event := sse.Event{
					Name: "log",
					Data: []byte("hello world"),
				}

				Ω(censor.Censor(event)).Should(Equal(event))
			})
		})

		Context("with the v1.0 protocol", func() {
			BeforeEach(func() {
				_, err := censor.Censor(sse.Event{
					Name: "version",
					Data: []byte("1.0"),
				})
				Ω(err).ShouldNot(HaveOccurred())
			})

			for _, e := range []event.Event{
				event.Log{
					Origin: event.Origin{
						Type: event.OriginTypeInput,
						Name: "some-input",
					},
					Payload: "some log",
				},
				event.Status{
					Status: builds.StatusSucceeded,
				},
				event.Start{
					Time: time.Now().Unix(),
				},
				event.Finish{
					Time:       time.Now().Unix(),
					ExitStatus: 123,
				},
				event.Error{
					Message: "some error",
				},
			} {
				event := e

				Describe(fmt.Sprintf("writing a %T event", event), func() {
					It("passes it through verbatim", func() {
						payload, err := json.Marshal(event)
						Ω(err).ShouldNot(HaveOccurred())

						sseEvent := sse.Event{
							Name: string(event.EventType()),
							Data: payload,
						}

						Ω(censor.Censor(sseEvent)).Should(Equal(sseEvent))
					})
				})
			}

			Describe("censoring an Initialize event", func() {
				It("censors build parameters", func() {
					censored, err := censor.Censor(sse.Event{
						Name: "initialize",
						Data: []byte(`{
              "config": {
                "image": "some-image",
                "params": {"super":"secret"},
                "run": {"path": "ls"}
              }
            }`),
					})
					Ω(err).ShouldNot(HaveOccurred())

					Ω(censored.Name).Should(Equal("initialize"))
					Ω(censored.Data).Should(MatchJSON(`{
            "config": {
              "image": "some-image",
              "run": {"path": "ls"}
            }
          }`))
				})
			})

			Describe("writing an Input event", func() {
				It("censors source and params", func() {
					censored, err := censor.Censor(sse.Event{
						Name: "input",
						Data: []byte(`{
              "input": {
                "name": "some-name",
                "type": "git",
                "version": {"ref": "foo"},
                "source": {"some": "secret"},
                "params": {"another": "secret"},
                "metadata": [{"name": "public", "value": "data"}],
                "config_path": "config/path.yml"
              }
            }`),
					})
					Ω(err).ShouldNot(HaveOccurred())

					Ω(censored.Name).Should(Equal("input"))
					Ω(censored.Data).Should(MatchJSON(`{
            "input": {
              "name": "some-name",
              "type": "git",
              "version": {"ref": "foo"},
              "metadata": [{"name": "public", "value": "data"}],
              "config_path": "config/path.yml"
            }
          }`))
				})
			})

			Describe("writing an Output event", func() {
				It("censors source and params", func() {
					censored, err := censor.Censor(sse.Event{
						Name: "output",
						Data: []byte(`{
              "output": {
                "name": "some-name",
                "type": "git",
                "on": ["success"],
                "version": {"ref": "foo"},
                "source": {"some": "secret"},
                "params": {"another": "secret"},
                "metadata": [{"name": "public", "value": "data"}]
              }
            }`),
					})
					Ω(err).ShouldNot(HaveOccurred())

					Ω(censored.Name).Should(Equal("output"))
					Ω(censored.Data).Should(MatchJSON(`{
            "output": {
              "name": "some-name",
              "type": "git",
              "on": ["success"],
              "version": {"ref": "foo"},
              "metadata": [{"name": "public", "value": "data"}]
            }
          }`))
				})
			})
		})
	})
})
