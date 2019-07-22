package resourceserver_test

import (
	"errors"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/resourceserver"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Checker", func() {
	var (
		err     error
		created bool

		fakeSecrets       *credsfakes.FakeSecrets
		fakeCheckFactory  *dbfakes.FakeCheckFactory
		fakeResource      *dbfakes.FakeResource
		fakeResourceType  *dbfakes.FakeResourceType
		fakeResourceTypes []db.ResourceType
		fromVersion       atc.Version

		checker resourceserver.Checker
	)

	BeforeEach(func() {
		fakeSecrets = new(credsfakes.FakeSecrets)
		fakeCheckFactory = new(dbfakes.FakeCheckFactory)
		fromVersion = atc.Version{"from": "version"}

		fakeResource = new(dbfakes.FakeResource)
		fakeResource.NameReturns("some-name")
		fakeResource.TagsReturns([]string{"tag-a", "tag-b"})
		fakeResource.SourceReturns(atc.Source{"some": "source"})

		fakeResourceType = new(dbfakes.FakeResourceType)
		fakeResourceType.NameReturns("some-type")
		fakeResourceType.TypeReturns("some-base-type")
		fakeResourceType.TagsReturns([]string{"some-tag"})
		fakeResourceType.SourceReturns(atc.Source{"some": "type-source"})

		fakeResourceTypes = []db.ResourceType{fakeResourceType}

		checker = resourceserver.NewChecker(
			fakeSecrets,
			fakeCheckFactory,
			time.Minute*1,
		)
	})

	JustBeforeEach(func() {
		_, created, err = checker.Check(fakeResource, fakeResourceTypes, fromVersion)
	})

	Describe("Check", func() {

		Context("when the resource parent type is not a custom type", func() {
			BeforeEach(func() {
				fakeResource.TypeReturns("base-type")
			})

			Context("when the configured timeout is not parseable", func() {
				BeforeEach(func() {
					fakeResource.CheckTimeoutReturns("not-a-duration")
				})

				It("errors", func() {
					Expect(err).To(HaveOccurred())
				})
			})

			Context("when the configured timeout is parseable", func() {
				BeforeEach(func() {
					fakeResource.CheckTimeoutReturns("10s")
				})

				Context("when evaluating the source fails", func() {
					BeforeEach(func() {
						fakeResource.SourceReturns(atc.Source{"some": "((secret))"})
						fakeSecrets.GetReturns("", nil, false, nil)
					})

					It("errors", func() {
						Expect(err).To(HaveOccurred())
					})
				})

				Context("when evaluating the source succeeds", func() {
					BeforeEach(func() {
						fakeResource.SourceReturns(atc.Source{"some": "((secret))"})
						fakeSecrets.GetReturns("source", nil, true, nil)
					})

					Context("when evaluating the resource types source fails", func() {
						BeforeEach(func() {
							fakeResourceType.SourceReturns(atc.Source{"some": "((other-secret))"})
							fakeSecrets.GetReturns("", nil, false, nil)
						})

						It("errors", func() {
							Expect(err).To(HaveOccurred())
						})
					})

					Context("when evaluating the resource types source succeeds", func() {
						BeforeEach(func() {
							fakeResourceType.SourceReturns(atc.Source{"some": "((other-secret))"})
							fakeSecrets.GetReturns("source", nil, true, nil)
						})

						Context("when updating the resource config scope fails", func() {
							BeforeEach(func() {
								fakeResource.SetResourceConfigReturns(nil, errors.New("nope"))
							})

							It("errors", func() {
								Expect(err).To(HaveOccurred())
							})
						})

						Context("when updating the resource config scope succeeds", func() {
							var fakeResourceConfig *dbfakes.FakeResourceConfig
							var fakeResourceConfigScope *dbfakes.FakeResourceConfigScope

							BeforeEach(func() {
								fakeResourceConfig = new(dbfakes.FakeResourceConfig)
								fakeResourceConfig.OriginBaseResourceTypeReturns(&db.UsedBaseResourceType{ID: 10})

								fakeResourceConfigScope = new(dbfakes.FakeResourceConfigScope)
								fakeResourceConfigScope.ResourceConfigReturns(fakeResourceConfig)

								fakeResource.SetResourceConfigReturns(fakeResourceConfigScope, nil)
							})

							Context("when fromVersion is not nil", func() {
								BeforeEach(func() {
									fromVersion = atc.Version{"version": "a"}
								})

								It("creates a check", func() {
									Expect(fakeCheckFactory.CreateCheckCallCount()).To(Equal(1))
								})

								It("creates a plan with a version", func() {
									_, _, _, _, manuallyTriggered, plan := fakeCheckFactory.CreateCheckArgsForCall(0)
									Expect(manuallyTriggered).To(BeTrue())
									Expect(plan.Check.FromVersion).To(Equal(atc.Version{"version": "a"}))
									Expect(plan.Check.Name).To(Equal("some-name"))
									Expect(plan.Check.Type).To(Equal("base-type"))
									Expect(plan.Check.Source).To(Equal(atc.Source{"some": "source"}))
									Expect(plan.Check.Tags).To(ConsistOf("tag-a", "tag-b"))
									Expect(plan.Check.Timeout).To(Equal("10s"))
								})
							})

							Context("when fromVersion is nil", func() {
								BeforeEach(func() {
									fromVersion = nil
								})

								Context("when fetching the latest version fails", func() {
									BeforeEach(func() {
										fakeResourceConfigScope.LatestVersionReturns(nil, false, errors.New("nope"))
									})

									It("errors", func() {
										Expect(err).To(HaveOccurred())
									})
								})

								Context("when fetching the latest version returns not found", func() {
									BeforeEach(func() {
										fakeResourceConfigScope.LatestVersionReturns(nil, false, nil)
									})

									It("creates a check", func() {
										Expect(fakeCheckFactory.CreateCheckCallCount()).To(Equal(1))
									})

									It("creates a plan with a nil version", func() {
										_, _, _, _, manuallyTriggered, plan := fakeCheckFactory.CreateCheckArgsForCall(0)
										Expect(manuallyTriggered).To(BeTrue())
										Expect(plan.Check.FromVersion).To(BeNil())
										Expect(plan.Check.Name).To(Equal("some-name"))
										Expect(plan.Check.Type).To(Equal("base-type"))
										Expect(plan.Check.Source).To(Equal(atc.Source{"some": "source"}))
										Expect(plan.Check.Tags).To(ConsistOf("tag-a", "tag-b"))
										Expect(plan.Check.Timeout).To(Equal("10s"))
									})
								})

								Context("when fetching the latest version returns a version", func() {
									var fakeResourceConfigVersion *dbfakes.FakeResourceConfigVersion

									BeforeEach(func() {
										fakeResourceConfigVersion = new(dbfakes.FakeResourceConfigVersion)
										fakeResourceConfigVersion.VersionReturns(db.Version{"some": "version"})

										fakeResourceConfigScope.LatestVersionReturns(fakeResourceConfigVersion, true, nil)
									})

									It("creates a check", func() {
										Expect(fakeCheckFactory.CreateCheckCallCount()).To(Equal(1))
									})

									It("creates a plan with a version", func() {
										_, _, _, _, manuallyTriggered, plan := fakeCheckFactory.CreateCheckArgsForCall(0)
										Expect(manuallyTriggered).To(BeTrue())
										Expect(plan.Check.FromVersion).To(Equal(atc.Version{"some": "version"}))
										Expect(plan.Check.Name).To(Equal("some-name"))
										Expect(plan.Check.Type).To(Equal("base-type"))
										Expect(plan.Check.Source).To(Equal(atc.Source{"some": "source"}))
										Expect(plan.Check.Tags).To(ConsistOf("tag-a", "tag-b"))
										Expect(plan.Check.Timeout).To(Equal("10s"))
									})

									Context("when the check factory create fails", func() {
										BeforeEach(func() {
											fakeCheckFactory.CreateCheckReturns(nil, false, errors.New("nope"))
										})

										It("errors", func() {
											Expect(err).To(HaveOccurred())
										})
									})

									Context("when the check factory create returns false", func() {
										BeforeEach(func() {
											fakeCheckFactory.CreateCheckReturns(nil, false, nil)
										})

										It("succeeds", func() {
											Expect(err).NotTo(HaveOccurred())
										})

										It("returns created", func() {
											Expect(created).To(BeFalse())
										})
									})

									Context("when the check factory create returns true", func() {
										BeforeEach(func() {
											fakeCheckFactory.CreateCheckReturns(nil, true, nil)
										})

										It("succeeds", func() {
											Expect(err).NotTo(HaveOccurred())
										})

										It("returns created", func() {
											Expect(created).To(BeTrue())
										})

										It("sends a notification for the checker to run", func() {
											Expect(fakeCheckFactory.NotifyCheckerCallCount()).To(Equal(1))
										})
									})
								})
							})
						})
					})
				})
			})
		})

		Context("when the resource has a parent type", func() {
			BeforeEach(func() {
				fakeResource.TypeReturns("custom-type")
				fakeResource.PipelineIDReturns(1)
				fakeResourceType.NameReturns("custom-type")
				fakeResourceType.PipelineIDReturns(1)
			})

			Context("when the resource and type are properly configured", func() {
				BeforeEach(func() {
					fakeResourceType.LastCheckEndTimeReturns(time.Now().Add(-time.Hour))
					fakeResource.LastCheckEndTimeReturns(time.Now().Add(-time.Hour))

					fakeResourceConfig := new(dbfakes.FakeResourceConfig)
					fakeResourceConfig.OriginBaseResourceTypeReturns(&db.UsedBaseResourceType{ID: 10})

					fakeResourceConfigVersion := new(dbfakes.FakeResourceConfigVersion)
					fakeResourceConfigVersion.VersionReturns(db.Version{"some": "version"})

					fakeResourceConfigScope := new(dbfakes.FakeResourceConfigScope)
					fakeResourceConfigScope.ResourceConfigReturns(fakeResourceConfig)
					fakeResourceConfigScope.LatestVersionReturns(fakeResourceConfigVersion, true, nil)

					fakeResourceType.SetResourceConfigReturns(fakeResourceConfigScope, nil)
					fakeResource.SetResourceConfigReturns(fakeResourceConfigScope, nil)
				})

				Context("when the parent type has no version", func() {
					BeforeEach(func() {
						fakeResourceType.VersionReturns(nil)
					})

					It("errors", func() {
						Expect(err).To(HaveOccurred())
					})
				})

				Context("when the parent type has a version", func() {
					BeforeEach(func() {
						fakeResourceType.VersionReturns(atc.Version{"some": "version"})
					})

					It("creates a check for the resource", func() {
						Expect(fakeCheckFactory.CreateCheckCallCount()).To(Equal(1))

						_, _, _, _, manuallyTriggered, plan := fakeCheckFactory.CreateCheckArgsForCall(0)
						Expect(manuallyTriggered).To(BeTrue())
						Expect(plan.Check.FromVersion).To(Equal(atc.Version{"from": "version"}))
						Expect(plan.Check.Name).To(Equal("some-name"))
						Expect(plan.Check.Type).To(Equal("custom-type"))
						Expect(plan.Check.Source).To(Equal(atc.Source{"some": "source"}))
						Expect(plan.Check.Tags).To(ConsistOf("tag-a", "tag-b"))
						Expect(plan.Check.Timeout).To(Equal("1m0s"))
					})

					It("sends a notification for the checker to run", func() {
						Expect(fakeCheckFactory.NotifyCheckerCallCount()).To(Equal(1))
					})
				})
			})
		})
	})
})
