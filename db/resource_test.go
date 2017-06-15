package db_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds/credsfakes"
	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource", func() {
	var pipeline db.Pipeline

	BeforeEach(func() {
		var (
			created bool
			err     error
		)

		pipeline, created, err = defaultTeam.SavePipeline(
			"pipeline-with-resources",
			atc.Config{
				Resources: atc.ResourceConfigs{
					{
						Name:   "some-resource",
						Type:   "docker-image",
						Source: atc.Source{"some": "repository"},
					},
					{
						Name:   "some-other-resource",
						Type:   "git",
						Source: atc.Source{"some": "other-repository"},
					},
					{
						Name:   "some-secret-resource",
						Type:   "git",
						Source: atc.Source{"some": "((secret-repository))"},
					},
				},
			},
			0,
			db.PipelineUnpaused,
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(created).To(BeTrue())
	})

	Describe("(Pipeline).Resources", func() {
		var resources []db.Resource

		JustBeforeEach(func() {
			var err error
			resources, err = pipeline.Resources()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns the resources", func() {
			Expect(resources).To(HaveLen(3))

			ids := map[int]struct{}{}

			for _, r := range resources {
				ids[r.ID()] = struct{}{}

				switch r.Name() {
				case "some-resource":
					Expect(r.Type()).To(Equal("docker-image"))
					Expect(r.RawSource()).To(Equal(atc.Source{"some": "repository"}))
				case "some-other-resource":
					Expect(r.Type()).To(Equal("git"))
					Expect(r.RawSource()).To(Equal(atc.Source{"some": "other-repository"}))
				case "some-secret-resource":
					Expect(r.Type()).To(Equal("git"))
					Expect(r.RawSource()).To(Equal(atc.Source{"some": "((secret-repository))"}))
				}
			}
		})
	})

	Describe("(Pipeline).Resource", func() {
		var (
			err      error
			found    bool
			resource db.Resource
		)

		Context("when the resource exists", func() {
			JustBeforeEach(func() {
				resource, found, err = pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the resource", func() {
				Expect(found).To(BeTrue())
				Expect(resource.Name()).To(Equal("some-resource"))
				Expect(resource.Type()).To(Equal("docker-image"))
				Expect(resource.RawSource()).To(Equal(atc.Source{"some": "repository"}))
			})
		})

		Context("when the resource does not exist", func() {
			JustBeforeEach(func() {
				resource, found, err = pipeline.Resource("bonkers")
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns nil", func() {
				Expect(found).To(BeFalse())
				Expect(resource).To(BeNil())
			})
		})
	})

	Describe("Pause", func() {
		var (
			resource db.Resource
			err      error
			found    bool
		)

		JustBeforeEach(func() {
			resource, found, err = pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resource.Paused()).To(BeFalse())
		})

		It("pauses the resource", func() {
			err = resource.Pause()
			Expect(err).ToNot(HaveOccurred())

			found, err = resource.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resource.Paused()).To(BeTrue())
		})
	})

	Describe("Unpause", func() {
		var (
			resource db.Resource
			err      error
			found    bool
		)

		JustBeforeEach(func() {
			resource, found, err = pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = resource.Pause()
			Expect(err).ToNot(HaveOccurred())

			found, err = resource.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resource.Paused()).To(BeTrue())
		})

		It("pauses the resource", func() {
			err = resource.Unpause()
			Expect(err).ToNot(HaveOccurred())

			found, err = resource.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resource.Paused()).To(BeFalse())
		})
	})

	Describe("EvaluatedSource", func() {
		var (
			fakeVariables *credsfakes.FakeVariables
			resource      db.Resource
			err           error
			source        atc.Source
			found         bool
		)

		BeforeEach(func() {
			fakeVariables = new(credsfakes.FakeVariables)
		})

		JustBeforeEach(func() {
			source, err = resource.EvaluatedSource(fakeVariables)
		})

		Context("when the source needs to be told secrets", func() {
			BeforeEach(func() {
				resource, found, err = pipeline.Resource("some-secret-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			Context("and the credential manager doesn't have that secret", func() {
				BeforeEach(func() {
					fakeVariables.GetReturns(nil, false, nil)
				})

				It("errors", func() {
					Expect(err).To(HaveOccurred())
				})
			})

			Context("and the credential manager has the secret", func() {
				BeforeEach(func() {
					fakeVariables.GetReturns("i-see-dead-people", true, nil)
				})

				It("asks vault what its secret is", func() {
					Expect(fakeVariables.GetCallCount()).To(Equal(1))
					varDef := fakeVariables.GetArgsForCall(0)
					Expect(varDef.Name).To(Equal("secret-repository"))
				})

				It("puts the secrets in the config", func() {
					Expect(err).ToNot(HaveOccurred())
					Expect(source["some"]).To(Equal("i-see-dead-people"))
				})
			})
		})

		Context("when the source doesn't have any secrets", func() {
			BeforeEach(func() {
				resource, found, err = pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("does not ask credential manager for secrets", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeVariables.GetCallCount()).To(Equal(0))
				Expect(source["some"]).To(Equal("repository"))
			})
		})
	})
})
