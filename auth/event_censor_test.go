package auth_test

import (
	"fmt"
	"time"

	. "github.com/concourse/atc/auth"
	"github.com/concourse/turbine"
	"github.com/concourse/turbine/event"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EventCensor", func() {
	Describe("Censor", func() {
		for _, e := range []event.Event{
			event.Log{
				Origin: event.Origin{
					Type: event.OriginTypeInput,
					Name: "some-input",
				},
				Payload: "some log",
			},
			event.Status{
				Status: turbine.StatusSucceeded,
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

			Describe(fmt.Sprintf("censoring a %T event", event), func() {
				It("passes it through verbatim", func() {
					Ω(Censor(event)).Should(Equal(event))
				})
			})
		}

		Describe("censoring an Initialize event", func() {
			It("censors build parameters", func() {
				Ω(Censor(event.Initialize{
					BuildConfig: turbine.Config{
						Image:  "some-image",
						Params: map[string]string{"super": "secret"},
						Run: turbine.RunConfig{
							Path: "ls",
						},
					},
				})).Should(Equal(event.Initialize{
					BuildConfig: turbine.Config{
						Image: "some-image",
						Run: turbine.RunConfig{
							Path: "ls",
						},
					},
				}))
			})
		})

		Describe("writing an Input event", func() {
			It("censors source and params", func() {
				Ω(Censor(event.Input{
					Input: turbine.Input{
						Name:       "some-name",
						Resource:   "some-resource",
						Type:       "git",
						Version:    turbine.Version{"ref": "foo"},
						Source:     turbine.Source{"some": "secret"},
						Params:     turbine.Params{"another": "secret"},
						Metadata:   []turbine.MetadataField{{"public", "data"}},
						ConfigPath: "config/path.yml",
					},
				})).Should(Equal(event.Input{
					Input: turbine.Input{
						Name:       "some-name",
						Resource:   "some-resource",
						Type:       "git",
						Version:    turbine.Version{"ref": "foo"},
						Metadata:   []turbine.MetadataField{{"public", "data"}},
						ConfigPath: "config/path.yml",
					},
				}))
			})
		})

		Describe("writing an Output event", func() {
			It("censors source and params", func() {
				Ω(Censor(event.Output{
					Output: turbine.Output{
						Name:     "some-name",
						Type:     "git",
						On:       []turbine.OutputCondition{turbine.OutputConditionSuccess},
						Version:  turbine.Version{"ref": "foo"},
						Source:   turbine.Source{"some": "secret"},
						Params:   turbine.Params{"another": "secret"},
						Metadata: []turbine.MetadataField{{"public", "data"}},
					},
				})).Should(Equal(event.Output{
					Output: turbine.Output{
						Name:     "some-name",
						Type:     "git",
						On:       []turbine.OutputCondition{turbine.OutputConditionSuccess},
						Version:  turbine.Version{"ref": "foo"},
						Metadata: []turbine.MetadataField{{"public", "data"}},
					},
				}))
			})
		})
	})
})
