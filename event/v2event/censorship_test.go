package v2event_test

import (
	"github.com/concourse/atc"
	. "github.com/concourse/atc/event/v2event"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Censorship", func() {
	Describe("censoring an Input event", func() {
		It("censors source and params", func() {
			Ω(Input{
				Plan: atc.InputPlan{
					Name:       "some-name",
					Resource:   "some-resource",
					Type:       "git",
					Source:     atc.Source{"some": "secret"},
					Params:     atc.Params{"another": "secret"},
					ConfigPath: "config/path.yml",
				},
				FetchedVersion:  atc.Version{"ref": "foo"},
				FetchedMetadata: []atc.MetadataField{{"public", "data"}},
			}.Censored()).Should(Equal(Input{
				Plan: atc.InputPlan{
					Name:       "some-name",
					Resource:   "some-resource",
					Type:       "git",
					ConfigPath: "config/path.yml",
				},
				FetchedVersion:  atc.Version{"ref": "foo"},
				FetchedMetadata: []atc.MetadataField{{"public", "data"}},
			}))
		})
	})

	Describe("censoring an Output event", func() {
		It("censors source and params", func() {
			Ω(Output{
				Plan: atc.OutputPlan{
					Name:   "some-name",
					Type:   "git",
					On:     []atc.OutputCondition{atc.OutputConditionSuccess},
					Source: atc.Source{"some": "secret"},
					Params: atc.Params{"another": "secret"},
				},
				CreatedVersion:  atc.Version{"ref": "foo"},
				CreatedMetadata: []atc.MetadataField{{"public", "data"}},
			}.Censored()).Should(Equal(Output{
				Plan: atc.OutputPlan{
					Name: "some-name",
					Type: "git",
					On:   []atc.OutputCondition{atc.OutputConditionSuccess},
				},
				CreatedVersion:  atc.Version{"ref": "foo"},
				CreatedMetadata: []atc.MetadataField{{"public", "data"}},
			}))
		})
	})
})
