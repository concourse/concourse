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
				Type: OriginTypeGet,
				Name: "some-input",
			},
			Payload: "some log",
		},
		Status{
			Status: atc.StatusSucceeded,
		},
		StartTask{
			Time: time.Now().Unix(),
			Origin: Origin{
				Type:     OriginTypeTask,
				Name:     "build",
				Location: OriginLocation{1, 2},
			},
		},
		FinishTask{
			Time:       time.Now().Unix(),
			ExitStatus: 123,
			Origin: Origin{
				Type:     OriginTypeTask,
				Name:     "build",
				Location: OriginLocation{1, 2},
			},
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

	Describe("InitializeV10", func() {
		It("censors the task's params", func() {
			Ω(InitializeV10{
				TaskConfig: atc.TaskConfig{
					Image:  "some-image",
					Params: map[string]string{"super": "secret"},
					Run: atc.BuildRunConfig{
						Path: "ls",
					},
				},
			}.Censored()).Should(Equal(InitializeV10{
				TaskConfig: atc.TaskConfig{
					Image:  "some-image",
					Params: nil,
					Run: atc.BuildRunConfig{
						Path: "ls",
					},
				},
			}))
		})
	})

	Describe("InitializeTask", func() {
		It("censors the task's params", func() {
			Ω(InitializeTask{
				TaskConfig: atc.TaskConfig{
					Image:  "some-image",
					Params: map[string]string{"super": "secret"},
					Run: atc.BuildRunConfig{
						Path: "ls",
					},
				},
			}.Censored()).Should(Equal(InitializeTask{
				TaskConfig: atc.TaskConfig{
					Image:  "some-image",
					Params: nil,
					Run: atc.BuildRunConfig{
						Path: "ls",
					},
				},
			}))
		})
	})

	Describe("InputV20", func() {
		It("censors source and params", func() {
			Ω(InputV20{
				Plan: atc.InputPlan{
					Name:     "some-name",
					Resource: "some-resource",
					Type:     "git",
					Source:   atc.Source{"some": "secret"},
					Params:   atc.Params{"another": "secret"},
				},
				FetchedVersion:  atc.Version{"ref": "foo"},
				FetchedMetadata: []atc.MetadataField{{"public", "data"}},
			}.Censored()).Should(Equal(InputV20{
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

	Describe("OutputV20", func() {
		It("censors source and params", func() {
			Ω(OutputV20{
				Plan: atc.OutputPlan{
					Name:   "some-name",
					Type:   "git",
					On:     []atc.Condition{atc.ConditionSuccess},
					Source: atc.Source{"some": "secret"},
					Params: atc.Params{"another": "secret"},
				},
				CreatedVersion:  atc.Version{"ref": "foo"},
				CreatedMetadata: []atc.MetadataField{{"public", "data"}},
			}.Censored()).Should(Equal(OutputV20{
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

	Describe("FinishGet", func() {
		It("censors source and params", func() {
			Ω(FinishGet{
				Plan: GetPlan{
					Name:     "some-name",
					Resource: "some-resource",
					Type:     "git",
					Source:   atc.Source{"some": "secret"},
					Params:   atc.Params{"another": "secret"},
				},
				FetchedVersion:  atc.Version{"ref": "foo"},
				FetchedMetadata: []atc.MetadataField{{"public", "data"}},
			}.Censored()).Should(Equal(FinishGet{
				Plan: GetPlan{
					Name:     "some-name",
					Resource: "some-resource",
					Type:     "git",
				},
				FetchedVersion:  atc.Version{"ref": "foo"},
				FetchedMetadata: []atc.MetadataField{{"public", "data"}},
			}))
		})
	})

	Describe("FinishPut", func() {
		It("censors source and params", func() {
			Ω(FinishPut{
				Plan: PutPlan{
					Name:   "some-name",
					Type:   "git",
					Source: atc.Source{"some": "secret"},
					Params: atc.Params{"another": "secret"},
				},
				CreatedVersion:  atc.Version{"ref": "foo"},
				CreatedMetadata: []atc.MetadataField{{"public", "data"}},
			}.Censored()).Should(Equal(FinishPut{
				Plan: PutPlan{
					Name: "some-name",
					Type: "git",
				},
				CreatedVersion:  atc.Version{"ref": "foo"},
				CreatedMetadata: []atc.MetadataField{{"public", "data"}},
			}))
		})
	})
})
