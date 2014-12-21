package v1event_test

import (
	"fmt"
	"time"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/event/v1event"
	"github.com/concourse/turbine"

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
				Input: turbine.Input{
					Name:       "some-name",
					Resource:   "some-resource",
					Type:       "git",
					Source:     turbine.Source{"some": "secret"},
					Params:     turbine.Params{"another": "secret"},
					ConfigPath: "config/path.yml",
					Version:    turbine.Version{"ref": "foo"},
					Metadata:   []turbine.MetadataField{{"public", "data"}},
				},
			}.Censored()).Should(Equal(Input{
				Input: turbine.Input{
					Name:       "some-name",
					Resource:   "some-resource",
					Type:       "git",
					ConfigPath: "config/path.yml",
					Version:    turbine.Version{"ref": "foo"},
					Metadata:   []turbine.MetadataField{{"public", "data"}},
				},
			}))
		})
	})

	Describe("Output", func() {
		It("censors source and params", func() {
			Ω(Output{
				Output: turbine.Output{
					Name:     "some-name",
					Type:     "git",
					On:       []turbine.OutputCondition{turbine.OutputConditionSuccess},
					Source:   turbine.Source{"some": "secret"},
					Params:   turbine.Params{"another": "secret"},
					Version:  turbine.Version{"ref": "foo"},
					Metadata: []turbine.MetadataField{{"public", "data"}},
				},
			}.Censored()).Should(Equal(Output{
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
