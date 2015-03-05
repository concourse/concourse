package event_test

import (
	"fmt"
	"time"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/event"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Censorship", func() {
	for _, e := range []atc.Event{
		Log{
			Origin: Origin{
				Type: OriginTypeInput,
				Name: "some-input",
			},
			Payload: "some log",
		},
		Status{
			Status: atc.StatusSucceeded,
		},
		Start{
			Time: time.Now().Unix(),
		},
		Finish{
			Time:       time.Now().Unix(),
			ExitStatus: 123,
		},
		Error{
			Message: "some error",
		},
	} {
		event := e

		Describe(fmt.Sprintf("%T", event), func() {
			It("does nothing", func() {
				Ω(event.Censored()).Should(Equal(event))
			})
		})
	}

	Describe("Initialize", func() {
		It("censors the build params", func() {
			Ω(Initialize{
				BuildConfig: atc.BuildConfig{
					Image:  "some-image",
					Params: map[string]string{"super": "secret"},
					Run: atc.BuildRunConfig{
						Path: "ls",
					},
				},
			}.Censored()).Should(Equal(Initialize{
				BuildConfig: atc.BuildConfig{
					Image:  "some-image",
					Params: nil,
					Run: atc.BuildRunConfig{
						Path: "ls",
					},
				},
			}))
		})
	})

	Describe("Input", func() {
		It("censors source and params", func() {
			Ω(Input{
				Plan: atc.InputPlan{
					Name:     "some-name",
					Resource: "some-resource",
					Type:     "git",
					Source:   atc.Source{"some": "secret"},
					Params:   atc.Params{"another": "secret"},
				},
				FetchedVersion:  atc.Version{"ref": "foo"},
				FetchedMetadata: []atc.MetadataField{{"public", "data"}},
			}.Censored()).Should(Equal(Input{
				Plan: atc.InputPlan{
					Name:     "some-name",
					Resource: "some-resource",
					Type:     "git",
				},
				FetchedVersion:  atc.Version{"ref": "foo"},
				FetchedMetadata: []atc.MetadataField{{"public", "data"}},
			}))
		})
	})

	Describe("Output", func() {
		It("censors source and params", func() {
			Ω(Output{
				Plan: atc.OutputPlan{
					Name:   "some-name",
					Type:   "git",
					On:     []atc.Condition{atc.ConditionSuccess},
					Source: atc.Source{"some": "secret"},
					Params: atc.Params{"another": "secret"},
				},
				CreatedVersion:  atc.Version{"ref": "foo"},
				CreatedMetadata: []atc.MetadataField{{"public", "data"}},
			}.Censored()).Should(Equal(Output{
				Plan: atc.OutputPlan{
					Name: "some-name",
					Type: "git",
					On:   []atc.Condition{atc.ConditionSuccess},
				},
				CreatedVersion:  atc.Version{"ref": "foo"},
				CreatedMetadata: []atc.MetadataField{{"public", "data"}},
			}))
		})
	})
})
