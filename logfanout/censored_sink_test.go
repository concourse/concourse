package logfanout_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	. "github.com/concourse/atc/logfanout"
	"github.com/concourse/atc/logfanout/fakes"
	"github.com/concourse/turbine/api/builds"
	"github.com/concourse/turbine/event"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CensoredSink", func() {
	var (
		conn *fakes.FakeJSONWriteCloser

		sink Sink
	)

	BeforeEach(func() {
		conn = new(fakes.FakeJSONWriteCloser)

		sink = NewCensoredSink(conn)
	})

	Describe("WriteMessage", func() {
		Context("with the v0.0 protocol", func() {
			BeforeEach(func() {
				err := sink.WriteMessage(rawMSG(`{"version": "0.0"}`))
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("forwards log events", func() {
				err := sink.WriteMessage(rawMSG(`{"log":"hello world"}`))
				Ω(err).ShouldNot(HaveOccurred())

				Ω(conn.WriteJSONCallCount()).Should(Equal(2))
				Ω(conn.WriteJSONArgsForCall(0)).Should(Equal(rawMSG(`{"version": "0.0"}`)))
				Ω(conn.WriteJSONArgsForCall(1)).Should(Equal(rawMSG(`{"log":"hello world"}`)))
			})
		})

		Context("with the v1.0 protocol", func() {
			BeforeEach(func() {
				err := sink.WriteMessage(rawMSG(`{"version": "1.0"}`))
				Ω(err).ShouldNot(HaveOccurred())
			})

			for _, msg := range []event.Message{
				{
					event.Log{
						Origin: event.Origin{
							Type: event.OriginTypeInput,
							Name: "some-input",
						},
						Payload: "some log",
					},
				},
				{
					event.Status{
						Status: builds.StatusSucceeded,
					},
				},
				{
					event.Start{
						Time: time.Now().Unix(),
					},
				},
				{
					event.Finish{
						Time:       time.Now().Unix(),
						ExitStatus: 123,
					},
				},
				{
					event.Error{
						Message: "some error",
					},
				},
			} {
				message := msg

				Describe(fmt.Sprintf("writing a %T event", msg.Event), func() {
					BeforeEach(func() {
						payload, err := json.Marshal(message)
						Ω(err).ShouldNot(HaveOccurred())

						err = sink.WriteMessage(rawMSG(string(payload)))
						Ω(err).ShouldNot(HaveOccurred())
					})

					It("passes it through verbatim", func() {
						Ω(conn.WriteJSONCallCount()).Should(Equal(2))
						Ω(conn.WriteJSONArgsForCall(0)).Should(Equal(rawMSG(`{"version": "1.0"}`)))
						Ω(conn.WriteJSONArgsForCall(1)).Should(Equal(message))
					})
				})
			}

			Describe("writing an Initialize event", func() {
				BeforeEach(func() {
					err := sink.WriteMessage(rawMSG(`{
						"type": "initialize",
						"event": {
							"config": {
								"image": "some-image",
								"params": {"super":"secret"}
							}
						}
					}`))
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("censors build parameters", func() {
					Ω(conn.WriteJSONCallCount()).Should(Equal(2))
					Ω(conn.WriteJSONArgsForCall(0)).Should(Equal(rawMSG(`{"version": "1.0"}`)))
					Ω(conn.WriteJSONArgsForCall(1)).Should(Equal(event.Message{
						Event: event.Initialize{
							BuildConfig: builds.Config{
								Image: "some-image",
							},
						},
					}))
				})
			})

			Describe("writing an Input event", func() {
				BeforeEach(func() {
					err := sink.WriteMessage(rawMSG(`{
						"type": "input",
						"event": {
							"input": {
								"name": "some-name",
								"type": "git",
								"version": {"ref": "foo"},
								"source": {"some": "secret"},
								"params": {"another": "secret"},
								"metadata": [{"name": "public", "value": "data"}],
								"config_path": "config/path.yml"
							}
						}
					}`))
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("censors source and params", func() {
					Ω(conn.WriteJSONCallCount()).Should(Equal(2))
					Ω(conn.WriteJSONArgsForCall(0)).Should(Equal(rawMSG(`{"version": "1.0"}`)))
					Ω(conn.WriteJSONArgsForCall(1)).Should(Equal(event.Message{
						Event: event.Input{
							Input: builds.Input{
								Name:    "some-name",
								Type:    "git",
								Version: builds.Version{"ref": "foo"},
								Source:  nil,
								Params:  nil,
								Metadata: []builds.MetadataField{
									{"public", "data"},
								},
								ConfigPath: "config/path.yml",
							},
						},
					}))
				})
			})

			Describe("writing an Output event", func() {
				BeforeEach(func() {
					err := sink.WriteMessage(rawMSG(`{
						"type": "output",
						"event": {
							"output": {
								"name": "some-name",
								"type": "git",
								"on": ["success"],
								"version": {"ref": "foo"},
								"source": {"some": "secret"},
								"params": {"another": "secret"},
								"metadata": [{"name": "public", "value": "data"}]
							}
						}
					}`))
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("censors source and params", func() {
					Ω(conn.WriteJSONCallCount()).Should(Equal(2))
					Ω(conn.WriteJSONArgsForCall(0)).Should(Equal(rawMSG(`{"version": "1.0"}`)))
					Ω(conn.WriteJSONArgsForCall(1)).Should(Equal(event.Message{
						Event: event.Output{
							Output: builds.Output{
								Name:    "some-name",
								Type:    "git",
								On:      []builds.OutputCondition{builds.OutputConditionSuccess},
								Version: builds.Version{"ref": "foo"},
								Source:  nil,
								Params:  nil,
								Metadata: []builds.MetadataField{
									{"public", "data"},
								},
							},
						},
					}))
				})
			})
		})
	})

	Describe("Close", func() {
		It("closes the backing connection", func() {
			Ω(conn.CloseCallCount()).Should(Equal(0))

			err := sink.Close()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(conn.CloseCallCount()).Should(Equal(1))
		})

		Context("when the backing connection errors", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				conn.CloseReturns(disaster)
			})

			It("returns the error", func() {
				Ω(sink.Close()).Should(Equal(disaster))
			})
		})
	})
})
