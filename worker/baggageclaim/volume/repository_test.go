package volume_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/concourse/go-archive/tgzfs"

	"github.com/concourse/concourse/worker/baggageclaim/uidgid/uidgidfakes"
	"github.com/concourse/concourse/worker/baggageclaim/volume"
	"github.com/concourse/concourse/worker/baggageclaim/volume/volumefakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Repository", func() {
	var (
		fakeFilesystem             *volumefakes.FakeFilesystem
		fakeLocker                 *volumefakes.FakeLockManager
		fakePrivilegedNamespacer   *uidgidfakes.FakeNamespacer
		fakeUnprivilegedNamespacer *uidgidfakes.FakeNamespacer

		repository volume.Repository
	)

	BeforeEach(func() {
		fakeFilesystem = new(volumefakes.FakeFilesystem)
		fakeLocker = new(volumefakes.FakeLockManager)
		fakePrivilegedNamespacer = new(uidgidfakes.FakeNamespacer)
		fakeUnprivilegedNamespacer = new(uidgidfakes.FakeNamespacer)

		repository = volume.NewRepository(
			fakeFilesystem,
			fakeLocker,
			fakePrivilegedNamespacer,
			fakeUnprivilegedNamespacer,
		)
	})

	Describe("CreateVolume", func() {
		var (
			fakeStrategy *volumefakes.FakeStrategy
			properties   volume.Properties
			privileged   bool

			createdVolume volume.Volume
			createErr     error
		)

		BeforeEach(func() {
			fakeStrategy = new(volumefakes.FakeStrategy)
			properties = volume.Properties{"some": "properties"}
			privileged = false
		})

		JustBeforeEach(func() {
			createdVolume, createErr = repository.CreateVolume(
				context.Background(),
				"some-handle",
				fakeStrategy,
				properties,
				privileged,
			)
		})

		Context("when a new volume can be materialized with the strategy", func() {
			var fakeInitVolume *volumefakes.FakeFilesystemInitVolume

			BeforeEach(func() {
				fakeInitVolume = new(volumefakes.FakeFilesystemInitVolume)
				fakeStrategy.MaterializeReturns(fakeInitVolume, nil)
			})

			Context("when setting the properties and privileged succeeds", func() {

				BeforeEach(func() {
					fakeInitVolume.StorePropertiesReturns(nil)
					fakeInitVolume.DataPathReturns("init-data-path")
					fakeInitVolume.StorePrivilegedReturns(nil)
				})

				Context("when the volume can be initialized", func() {
					var fakeLiveVolume *volumefakes.FakeFilesystemLiveVolume

					BeforeEach(func() {
						fakeLiveVolume = new(volumefakes.FakeFilesystemLiveVolume)
						fakeLiveVolume.HandleReturns("live-handle")
						fakeLiveVolume.DataPathReturns("live-data-path")
						fakeInitVolume.InitializeReturns(fakeLiveVolume, nil)
					})

					It("succeeds", func() {
						Expect(createErr).To(BeNil())
					})

					It("returns the created volume", func() {
						Expect(createdVolume).To(Equal(volume.Volume{
							Handle:     "live-handle",
							Path:       "live-data-path",
							Properties: properties,
						}))
					})

					It("materialized with the correct volume, fs, and driver", func() {
						_, handle, fs, _ := fakeStrategy.MaterializeArgsForCall(0)
						Expect(handle).ToNot(BeEmpty())
						Expect(fs).To(Equal(fakeFilesystem))
					})

					It("does not destroy the volume (due to busted cleanup logic)", func() {
						Expect(fakeInitVolume.DestroyCallCount()).To(Equal(0))
					})

					Context("when the volume is privileged", func() {
						BeforeEach(func() {
							privileged = true
						})

						It("stores volume privileged with the right value", func() {
							Expect(fakeInitVolume.StorePrivilegedCallCount()).To(Equal(1))
							Expect(fakeInitVolume.StorePrivilegedArgsForCall(0)).To(Equal(true))
						})

						It("namespaces the data path as unprivileged before initialization", func() {
							Expect(fakeUnprivilegedNamespacer.NamespacePathCallCount()).To(Equal(0))
							Expect(fakePrivilegedNamespacer.NamespacePathCallCount()).To(Equal(1))
							_, path := fakePrivilegedNamespacer.NamespacePathArgsForCall(0)
							Expect(path).To(Equal("init-data-path"))
						})

						Context("when namespacing fails", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakePrivilegedNamespacer.NamespacePathReturns(disaster)
							})

							It("returns the error", func() {
								Expect(createErr).To(Equal(disaster))
							})

							It("destroys the initializing volume", func() {
								Expect(fakeInitVolume.DestroyCallCount()).To(Equal(1))
							})
						})
					})

					Context("when the volume is not privileged", func() {
						BeforeEach(func() {
							privileged = false
						})

						It("stores volume privileged with the right value", func() {
							Expect(fakeInitVolume.StorePrivilegedCallCount()).To(Equal(1))
							Expect(fakeInitVolume.StorePrivilegedArgsForCall(0)).To(Equal(false))
						})

						It("namespaces the data path as unprivileged before initialization", func() {
							Expect(fakePrivilegedNamespacer.NamespacePathCallCount()).To(Equal(0))
							Expect(fakeUnprivilegedNamespacer.NamespacePathCallCount()).To(Equal(1))
							_, path := fakeUnprivilegedNamespacer.NamespacePathArgsForCall(0)
							Expect(path).To(Equal("init-data-path"))
						})

						Context("when namespacing fails", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeUnprivilegedNamespacer.NamespacePathReturns(disaster)
							})

							It("returns the error", func() {
								Expect(createErr).To(Equal(disaster))
							})

							It("destroys the initializing volume", func() {
								Expect(fakeInitVolume.DestroyCallCount()).To(Equal(1))
							})
						})
					})
				})

				Context("when the volume cannot be initialized", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeInitVolume.InitializeReturns(nil, disaster)
					})

					It("cleans up the volume", func() {
						Expect(fakeInitVolume.DestroyCallCount()).To(Equal(1))
					})

					It("returns the error", func() {
						Expect(createErr).To(Equal(disaster))
					})
				})
			})

			Context("when storing the properties fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeInitVolume.StorePropertiesReturns(disaster)
				})

				It("cleans up the volume", func() {
					Expect(fakeInitVolume.DestroyCallCount()).To(Equal(1))
				})

				It("does not initialize the volume", func() {
					Expect(fakeInitVolume.InitializeCallCount()).To(Equal(0))
				})

				It("returns the error", func() {
					Expect(createErr).To(Equal(disaster))
				})
			})

			Context("when storing the privileged fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeInitVolume.StorePrivilegedReturns(disaster)
				})

				It("cleans up the volume", func() {
					Expect(fakeInitVolume.DestroyCallCount()).To(Equal(1))
				})

				It("does not initialize the volume", func() {
					Expect(fakeInitVolume.InitializeCallCount()).To(Equal(0))
				})

				It("returns the error", func() {
					Expect(createErr).To(Equal(disaster))
				})
			})
		})

		Context("when creating the volume fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeStrategy.MaterializeReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(createErr).To(Equal(disaster))
			})
		})
	})

	Describe("DestroyVolume", func() {
		var destroyErr error

		JustBeforeEach(func() {
			destroyErr = repository.DestroyVolume(context.Background(), "some-volume")
		})

		Context("when the volume can be found", func() {
			var fakeVolume *volumefakes.FakeFilesystemLiveVolume

			BeforeEach(func() {
				fakeVolume = new(volumefakes.FakeFilesystemLiveVolume)
				fakeFilesystem.LookupVolumeReturns(fakeVolume, true, nil)
			})

			Context("when destroying the volume succeeds", func() {
				BeforeEach(func() {
					fakeVolume.DestroyReturns(nil)
				})

				It("returns nil", func() {
					Expect(destroyErr).To(BeNil())
				})

				It("looked up using the correct handle", func() {
					handle := fakeFilesystem.LookupVolumeArgsForCall(0)
					Expect(handle).To(Equal("some-volume"))
				})
			})

			Context("when destroying the volume fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeVolume.DestroyReturns(disaster)
				})

				It("returns the error", func() {
					Expect(destroyErr).To(Equal(disaster))
				})
			})
		})

		Context("when looking up the volume fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeFilesystem.LookupVolumeReturns(nil, false, disaster)
			})

			It("returns the error", func() {
				Expect(destroyErr).To(Equal(disaster))
			})
		})

		Context("when the volume can not be found", func() {
			BeforeEach(func() {
				fakeFilesystem.LookupVolumeReturns(nil, false, nil)
			})

			It("returns ErrVolumeDoesNotExist", func() {
				Expect(destroyErr).To(Equal(volume.ErrVolumeDoesNotExist))
			})
		})
	})

	Describe("DestroyVolumeAndDescendants", func() {
		var destroyErr error

		JustBeforeEach(func() {
			destroyErr = repository.DestroyVolumeAndDescendants(context.Background(), "parent")
		})

		Context("when the volume and its children can be found", func() {
			var (
				fakeParent     *volumefakes.FakeFilesystemLiveVolume
				fakeChild      *volumefakes.FakeFilesystemLiveVolume
				fakeSibling    *volumefakes.FakeFilesystemLiveVolume
				fakeGrandchild *volumefakes.FakeFilesystemLiveVolume
				fakeRoommate   *volumefakes.FakeFilesystemLiveVolume
			)

			BeforeEach(func() {
				fakeParent = new(volumefakes.FakeFilesystemLiveVolume)
				fakeChild = new(volumefakes.FakeFilesystemLiveVolume)
				fakeSibling = new(volumefakes.FakeFilesystemLiveVolume)
				fakeGrandchild = new(volumefakes.FakeFilesystemLiveVolume)
				fakeRoommate = new(volumefakes.FakeFilesystemLiveVolume)

				fakeParent.HandleReturns("parent")
				fakeChild.HandleReturns("child")
				fakeSibling.HandleReturns("sibling")
				fakeGrandchild.HandleReturns("grandchild")
				fakeRoommate.HandleReturns("unrelated")

				fakeChild.ParentReturns(fakeParent, true, nil)
				fakeSibling.ParentReturns(fakeParent, true, nil)
				fakeGrandchild.ParentReturns(fakeChild, true, nil)

				fakeFilesystem.ListVolumesReturns([]volume.FilesystemLiveVolume{
					fakeParent,
					fakeChild,
					fakeSibling,
					fakeGrandchild,
					fakeRoommate,
				}, nil)
				fakeFilesystem.LookupVolumeStub = func(handle string) (volume.FilesystemLiveVolume, bool, error) {
					if handle == "child" {
						return fakeChild, true, nil
					}
					if handle == "parent" {
						return fakeParent, true, nil
					}
					if handle == "grandchild" {
						return fakeGrandchild, true, nil
					}
					if handle == "sibling" {
						return fakeSibling, true, nil
					}
					if handle == "unrelated" {
						return fakeRoommate, true, nil
					}
					return nil, false, nil
				}
			})

			It("wipes out the whole family", func() {
				Expect(fakeParent.DestroyCallCount()).To(Equal(1))
				Expect(fakeChild.DestroyCallCount()).To(Equal(1))
				Expect(fakeSibling.DestroyCallCount()).To(Equal(1))
				Expect(fakeGrandchild.DestroyCallCount()).To(Equal(1))
				Expect(fakeRoommate.DestroyCallCount()).To(Equal(0))
			})
		})

		Context("when looking up the volume fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeFilesystem.ListVolumesReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(destroyErr).To(Equal(disaster))
			})
		})

		Context("when the volume can not be found", func() {
			BeforeEach(func() {
				fakeFilesystem.ListVolumesReturns([]volume.FilesystemLiveVolume{}, nil)
			})

			It("returns ErrVolumeDoesNotExist and does not recurse", func() {
				Expect(destroyErr).To(Equal(volume.ErrVolumeDoesNotExist))
				Expect(fakeFilesystem.ListVolumesCallCount()).To(Equal(1))
			})
		})
	})

	Describe("ListVolumes", func() {
		var (
			queryProperties volume.Properties

			corruptedVolumes []string
			volumes          volume.Volumes
			listErr          error
		)

		BeforeEach(func() {
			queryProperties = volume.Properties{}
		})

		JustBeforeEach(func() {
			volumes, corruptedVolumes, listErr = repository.ListVolumes(context.Background(), queryProperties)
		})

		Context("when volumes are found in the filesystem", func() {
			var fakeVolume1 *volumefakes.FakeFilesystemLiveVolume
			var fakeVolume2 *volumefakes.FakeFilesystemLiveVolume
			var fakeVolume3 *volumefakes.FakeFilesystemLiveVolume
			var fakeVolume4 *volumefakes.FakeFilesystemLiveVolume

			BeforeEach(func() {
				fakeVolume1 = new(volumefakes.FakeFilesystemLiveVolume)
				fakeVolume1.HandleReturns("handle-1")
				fakeVolume1.DataPathReturns("handle-1-data-path")
				fakeVolume1.LoadPropertiesReturns(volume.Properties{"a": "a", "b": "b"}, nil)
				fakeVolume1.LoadPrivilegedReturns(true, nil)

				fakeVolume2 = new(volumefakes.FakeFilesystemLiveVolume)
				fakeVolume2.HandleReturns("handle-2")
				fakeVolume2.DataPathReturns("handle-2-data-path")
				fakeVolume2.LoadPropertiesReturns(volume.Properties{"a": "a"}, nil)
				fakeVolume2.LoadPrivilegedReturns(false, nil)

				fakeVolume3 = new(volumefakes.FakeFilesystemLiveVolume)
				fakeVolume3.HandleReturns("handle-3")
				fakeVolume3.DataPathReturns("handle-3-data-path")
				fakeVolume3.LoadPropertiesReturns(volume.Properties{"b": "b"}, nil)
				fakeVolume3.LoadPrivilegedReturns(true, nil)

				fakeVolume4 = new(volumefakes.FakeFilesystemLiveVolume)
				fakeVolume4.HandleReturns("handle-4")
				fakeVolume4.DataPathReturns("handle-4-data-path")
				fakeVolume4.LoadPropertiesReturns(volume.Properties{}, nil)
				fakeVolume4.LoadPrivilegedReturns(false, nil)

				fakeFilesystem.ListVolumesReturns([]volume.FilesystemLiveVolume{
					fakeVolume1,
					fakeVolume2,
					fakeVolume3,
					fakeVolume4,
				}, nil)
			})

			Context("when no properties are given", func() {
				BeforeEach(func() {
					queryProperties = volume.Properties{}
				})

				It("succeeds", func() {
					Expect(listErr).ToNot(HaveOccurred())
				})

				It("returns all volumes", func() {
					Expect(volumes).To(Equal(volume.Volumes{
						{
							Handle:     "handle-1",
							Path:       "handle-1-data-path",
							Properties: volume.Properties{"a": "a", "b": "b"},
							Privileged: true,
						},
						{
							Handle:     "handle-2",
							Path:       "handle-2-data-path",
							Properties: volume.Properties{"a": "a"},
							Privileged: false,
						},
						{
							Handle:     "handle-3",
							Path:       "handle-3-data-path",
							Properties: volume.Properties{"b": "b"},
							Privileged: true,
						},
						{
							Handle:     "handle-4",
							Path:       "handle-4-data-path",
							Properties: volume.Properties{},
							Privileged: false,
						},
					}))
				})

				Context("when hydrating one of the volumes fails", func() {
					Context("with ErrVolumeDoesNotExist", func() {
						BeforeEach(func() {
							fakeVolume2.LoadPropertiesReturns(nil, volume.ErrVolumeDoesNotExist)
						})

						It("is not included in the response", func() {
							Expect(volumes).To(Equal(volume.Volumes{
								{
									Handle:     "handle-1",
									Path:       "handle-1-data-path",
									Properties: volume.Properties{"a": "a", "b": "b"},
									Privileged: true,
								},
								{
									Handle:     "handle-3",
									Path:       "handle-3-data-path",
									Properties: volume.Properties{"b": "b"},
									Privileged: true,
								},
								{
									Handle:     "handle-4",
									Path:       "handle-4-data-path",
									Properties: volume.Properties{},
									Privileged: false,
								},
							}))
						})
					})

					Context("with any other error", func() {
						BeforeEach(func() {
							fakeVolume2.LoadPropertiesReturns(nil, errors.New("nope"))
						})

						It("returns corrupted and working volumes", func() {
							Expect(volumes).To(Equal(volume.Volumes{
								{
									Handle:     "handle-1",
									Path:       "handle-1-data-path",
									Properties: volume.Properties{"a": "a", "b": "b"},
									Privileged: true,
								},
								{
									Handle:     "handle-3",
									Path:       "handle-3-data-path",
									Properties: volume.Properties{"b": "b"},
									Privileged: true,
								},
								{
									Handle:     "handle-4",
									Path:       "handle-4-data-path",
									Properties: volume.Properties{},
									Privileged: false,
								},
							}))

							Expect(corruptedVolumes).To(ConsistOf(fakeVolume2.Handle()))
						})
					})
				})
			})

			Context("when properties are given", func() {
				BeforeEach(func() {
					queryProperties = volume.Properties{"a": "a"}
				})

				It("returns only volumes whose properties match", func() {
					Expect(volumes).To(Equal(volume.Volumes{
						{
							Handle:     "handle-1",
							Path:       "handle-1-data-path",
							Properties: volume.Properties{"a": "a", "b": "b"},
							Privileged: true,
						},
						{
							Handle:     "handle-2",
							Path:       "handle-2-data-path",
							Properties: volume.Properties{"a": "a"},
							Privileged: false,
						},
					}))
				})

				Context("when hydrating one of the volumes fails", func() {
					Context("with ErrVolumeDoesNotExist", func() {
						BeforeEach(func() {
							fakeVolume2.LoadPropertiesReturns(nil, volume.ErrVolumeDoesNotExist)
						})

						It("is not included in the response", func() {
							Expect(volumes).To(Equal(volume.Volumes{
								{
									Handle:     "handle-1",
									Path:       "handle-1-data-path",
									Properties: volume.Properties{"a": "a", "b": "b"},
									Privileged: true,
								},
							}))
						})
					})

					Context("with any other error", func() {
						BeforeEach(func() {
							fakeVolume2.LoadPropertiesReturns(nil, errors.New("nope"))
						})

						It("returns corrupted and working volumes", func() {
							Expect(volumes).To(Equal(volume.Volumes{
								{
									Handle:     "handle-1",
									Path:       "handle-1-data-path",
									Properties: volume.Properties{"a": "a", "b": "b"},
									Privileged: true,
								},
							}))

							Expect(corruptedVolumes).To(ConsistOf(fakeVolume2.Handle()))
						})
					})
				})
			})
		})

		Context("when listing the volumes on the filesystem fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeFilesystem.ListVolumesReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(listErr).To(Equal(disaster))
			})
		})
	})

	Describe("GetVolume", func() {
		var (
			foundVolume volume.Volume
			found       bool
			getErr      error
		)

		JustBeforeEach(func() {
			foundVolume, found, getErr = repository.GetVolume(context.Background(), "some-volume")
		})

		Context("when the volume is found in the filesystem", func() {
			var fakeVolume *volumefakes.FakeFilesystemLiveVolume

			BeforeEach(func() {
				fakeVolume = new(volumefakes.FakeFilesystemLiveVolume)
				fakeVolume.HandleReturns("some-volume")
				fakeVolume.DataPathReturns("some-data-path")
				fakeVolume.LoadPropertiesReturns(volume.Properties{"a": "a", "b": "b"}, nil)
				fakeVolume.LoadPrivilegedReturns(true, nil)

				fakeFilesystem.LookupVolumeReturns(fakeVolume, true, nil)
			})

			It("succeeds", func() {
				Expect(getErr).ToNot(HaveOccurred())
			})

			It("found it by looking for the right handle", func() {
				handle := fakeFilesystem.LookupVolumeArgsForCall(0)
				Expect(handle).To(Equal("some-volume"))
			})

			It("returns the volume and true", func() {
				Expect(found).To(BeTrue())
				Expect(foundVolume).To(Equal(volume.Volume{
					Handle:     "some-volume",
					Path:       "some-data-path",
					Properties: volume.Properties{"a": "a", "b": "b"},
					Privileged: true,
				}))
			})

			Context("when hydrating one the volume fails", func() {
				Context("with ErrVolumeDoesNotExist", func() {
					BeforeEach(func() {
						fakeVolume.LoadPropertiesReturns(nil, volume.ErrVolumeDoesNotExist)
					})

					It("succeeds", func() {
						Expect(getErr).ToNot(HaveOccurred())
					})

					It("returns no volume and false", func() {
						Expect(found).To(BeFalse())
						Expect(foundVolume).To(BeZero())
					})
				})

				Context("with any other error", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeVolume.LoadPropertiesReturns(nil, disaster)
					})

					It("returns the error", func() {
						Expect(getErr).To(Equal(disaster))
					})
				})
			})
		})

		Context("when the volume is not found on the filesystem", func() {
			BeforeEach(func() {
				fakeFilesystem.LookupVolumeReturns(nil, false, nil)
			})

			It("succeeds", func() {
				Expect(getErr).ToNot(HaveOccurred())
			})

			It("returns no volume and false", func() {
				Expect(found).To(BeFalse())
				Expect(foundVolume).To(BeZero())
			})
		})

		Context("when looking up the volume on the filesystem fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeFilesystem.LookupVolumeReturns(nil, false, disaster)
			})

			It("returns the error", func() {
				Expect(getErr).To(Equal(disaster))
			})
		})
	})

	Describe("SetProperty", func() {
		var (
			setErr error
		)

		JustBeforeEach(func() {
			setErr = repository.SetProperty(context.Background(), "some-volume", "some-property", "some-value")
		})

		Context("when the volume is found in the filesystem", func() {
			var fakeVolume *volumefakes.FakeFilesystemLiveVolume

			BeforeEach(func() {
				fakeVolume = new(volumefakes.FakeFilesystemLiveVolume)
				fakeVolume.HandleReturns("some-volume")
				fakeVolume.DataPathReturns("some-data-path")
				fakeVolume.LoadPropertiesReturns(volume.Properties{"a": "a", "b": "b"}, nil)

				fakeFilesystem.LookupVolumeReturns(fakeVolume, true, nil)
			})

			Context("when storing the new properties succeeds", func() {
				BeforeEach(func() {
					fakeVolume.StorePropertiesReturns(nil)
				})

				It("succeeds", func() {
					Expect(setErr).ToNot(HaveOccurred())
				})

				It("found it by looking for the right handle", func() {
					handle := fakeFilesystem.LookupVolumeArgsForCall(0)
					Expect(handle).To(Equal("some-volume"))
				})

				It("stored the right properties", func() {
					newProperties := fakeVolume.StorePropertiesArgsForCall(0)
					Expect(newProperties).To(Equal(volume.Properties{
						"a":             "a",
						"b":             "b",
						"some-property": "some-value",
					}))
				})
			})

			Context("when storing the new properties fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeVolume.StorePropertiesReturns(disaster)
				})

				It("returns the error", func() {
					Expect(setErr).To(Equal(disaster))
				})
			})

			Context("when hydrating the volume fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeVolume.LoadPropertiesReturns(nil, disaster)
				})

				It("returns the error", func() {
					Expect(setErr).To(Equal(disaster))
				})
			})
		})

		Context("when the volume is not found on the filesystem", func() {
			BeforeEach(func() {
				fakeFilesystem.LookupVolumeReturns(nil, false, nil)
			})

			It("returns ErrVolumeDoesNotExist", func() {
				Expect(setErr).To(Equal(volume.ErrVolumeDoesNotExist))
			})
		})

		Context("when looking up the volume on the filesystem fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeFilesystem.LookupVolumeReturns(nil, false, disaster)
			})

			It("returns the error", func() {
				Expect(setErr).To(Equal(disaster))
			})
		})
	})

	Describe("GetPrivileged", func() {
		var (
			getErr error
		)

		JustBeforeEach(func() {
			_, getErr = repository.GetPrivileged(context.Background(), "some-volume")
		})

		Context("when the volume is found in the filesystem", func() {
			var fakeVolume *volumefakes.FakeFilesystemLiveVolume

			BeforeEach(func() {
				fakeVolume = new(volumefakes.FakeFilesystemLiveVolume)
				fakeVolume.HandleReturns("some-volume")
				fakeVolume.DataPathReturns("some-data-path")

				fakeFilesystem.LookupVolumeReturns(fakeVolume, true, nil)
			})

			Context("when getting privileged succeeds", func() {
				BeforeEach(func() {
					fakeVolume.LoadPrivilegedReturns(true, nil)
				})

				It("succeeds", func() {
					Expect(getErr).ToNot(HaveOccurred())
				})
			})

			Context("when getting privileged fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeVolume.LoadPrivilegedReturns(false, disaster)
				})

				It("returns the error", func() {
					Expect(getErr).To(Equal(disaster))
				})
			})
		})

		Context("when the volume is not found on the filesystem", func() {
			BeforeEach(func() {
				fakeFilesystem.LookupVolumeReturns(nil, false, nil)
			})

			It("returns ErrVolumeDoesNotExist", func() {
				Expect(getErr).To(Equal(volume.ErrVolumeDoesNotExist))
			})
		})

		Context("when looking up the volume on the filesystem fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeFilesystem.LookupVolumeReturns(nil, false, disaster)
			})

			It("returns the error", func() {
				Expect(getErr).To(Equal(disaster))
			})
		})
	})

	Describe("SetPrivileged", func() {
		var (
			setErr error
		)

		JustBeforeEach(func() {
			setErr = repository.SetPrivileged(context.Background(), "some-volume", true)
		})

		Context("when the volume is found in the filesystem", func() {
			var fakeVolume *volumefakes.FakeFilesystemLiveVolume

			BeforeEach(func() {
				fakeVolume = new(volumefakes.FakeFilesystemLiveVolume)
				fakeVolume.HandleReturns("some-volume")
				fakeVolume.DataPathReturns("some-data-path")

				fakeFilesystem.LookupVolumeReturns(fakeVolume, true, nil)
			})

			Context("when setting privileged succeeds", func() {
				BeforeEach(func() {
					fakeVolume.StorePrivilegedReturns(nil)
				})

				It("succeeds", func() {
					Expect(setErr).ToNot(HaveOccurred())
				})
			})

			Context("when setting privileged fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeVolume.StorePrivilegedReturns(disaster)
				})

				It("returns the error", func() {
					Expect(setErr).To(Equal(disaster))
				})
			})
		})

		Context("when the volume is not found on the filesystem", func() {
			BeforeEach(func() {
				fakeFilesystem.LookupVolumeReturns(nil, false, nil)
			})

			It("returns ErrVolumeDoesNotExist", func() {
				Expect(setErr).To(Equal(volume.ErrVolumeDoesNotExist))
			})
		})

		Context("when looking up the volume on the filesystem fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeFilesystem.LookupVolumeReturns(nil, false, disaster)
			})

			It("returns the error", func() {
				Expect(setErr).To(Equal(disaster))
			})
		})
	})

	Describe("VolumeParent", func() {
		var (
			parent    volume.Volume
			found     bool
			parentErr error
		)

		JustBeforeEach(func() {
			parent, found, parentErr = repository.VolumeParent(context.Background(), "some-volume")
		})

		Context("when the volume is found in the filesystem", func() {
			var fakeVolume *volumefakes.FakeFilesystemLiveVolume

			BeforeEach(func() {
				fakeVolume = new(volumefakes.FakeFilesystemLiveVolume)
				fakeVolume.HandleReturns("some-volume")
				fakeVolume.DataPathReturns("some-data-path")
				fakeVolume.LoadPropertiesReturns(volume.Properties{"a": "a", "b": "b"}, nil)
				fakeVolume.LoadPrivilegedReturns(false, nil)

				fakeFilesystem.LookupVolumeReturns(fakeVolume, true, nil)
			})

			Context("when the volume has a parent", func() {
				var parentVolume *volumefakes.FakeFilesystemLiveVolume

				BeforeEach(func() {
					parentVolume = new(volumefakes.FakeFilesystemLiveVolume)
					parentVolume.HandleReturns("parent-volume")
					parentVolume.DataPathReturns("parent-data-path")
					parentVolume.LoadPropertiesReturns(volume.Properties{"parent": "property"}, nil)
					parentVolume.LoadPrivilegedReturns(true, nil)

					fakeVolume.ParentReturns(parentVolume, true, nil)
				})

				It("succeeds", func() {
					Expect(parentErr).ToNot(HaveOccurred())
				})

				It("found the child volume by looking for the right handle", func() {
					handle := fakeFilesystem.LookupVolumeArgsForCall(0)
					Expect(handle).To(Equal("some-volume"))
				})

				It("returns the parent volume and true", func() {
					Expect(found).To(BeTrue())
					Expect(parent).To(Equal(volume.Volume{
						Handle:     "parent-volume",
						Path:       "parent-data-path",
						Properties: volume.Properties{"parent": "property"},
						Privileged: true,
					}))
				})

				Context("when hydrating the parent volume fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						parentVolume.LoadPropertiesReturns(nil, disaster)
					})

					It("returns the special error", func() {
						Expect(parentErr).To(Equal(volume.ErrVolumeIsCorrupted))
					})
				})
			})

			Context("when the volume does not have a parent", func() {
				BeforeEach(func() {
					fakeVolume.ParentReturns(nil, false, nil)
				})

				It("succeeds", func() {
					Expect(parentErr).ToNot(HaveOccurred())
				})

				It("returns no volume and false", func() {
					Expect(found).To(BeFalse())
					Expect(parent).To(BeZero())
				})
			})

			Context("when getting the parent volume fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeVolume.ParentReturns(nil, false, disaster)
				})

				It("returns the error", func() {
					Expect(parentErr).To(Equal(disaster))
				})
			})
		})

		Context("when the volume is not found on the filesystem", func() {
			BeforeEach(func() {
				fakeFilesystem.LookupVolumeReturns(nil, false, nil)
			})

			It("returns ErrVolumeDoesNotExist", func() {
				Expect(parentErr).To(Equal(volume.ErrVolumeDoesNotExist))
			})
		})

		Context("when looking up the volume on the filesystem fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeFilesystem.LookupVolumeReturns(nil, false, disaster)
			})

			It("returns the error", func() {
				Expect(parentErr).To(Equal(disaster))
			})
		})
	})

	Describe("StreamP2pOut", func() {
		var (
			server             *httptest.Server
			streamErr          error
			serverCalled       bool
			serverResponseCode int
			serverReadBytes    []byte
			tempFile           *os.File
		)
		BeforeEach(func() {
			var err error
			tempFile, err = ioutil.TempFile("", "StreamP2pOutTest")
			Expect(err).ToNot(HaveOccurred())
		})
		AfterEach(func() {
			if server != nil {
				server.Close()
			}
			if tempFile != nil {
				os.Remove(tempFile.Name())
			}
		})

		JustBeforeEach(func() {
			serverCalled = false
			serverReadBytes = nil
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(serverResponseCode)
				serverCalled = true

				var err error
				serverReadBytes, err = ioutil.ReadAll(r.Body)
				Expect(err).ToNot(HaveOccurred())
			}))
			streamErr = repository.StreamP2pOut(context.Background(), "some-handle", filepath.Base(tempFile.Name()), volume.GzipEncoding, server.URL)
		})

		Context("when lookup volume fails", func() {
			BeforeEach(func() {
				fakeFilesystem.LookupVolumeReturns(nil, true, errors.New("some-error"))
			})
			It("should fail", func() {
				Expect(streamErr).To(HaveOccurred())
				Expect(streamErr).To(Equal(errors.New("some-error")))
			})
			It("should not http request", func() {
				Expect(serverCalled).To(BeFalse())
			})
		})

		Context("when volume not found", func() {
			BeforeEach(func() {
				fakeFilesystem.LookupVolumeReturns(nil, false, nil)
			})
			It("should fail", func() {
				Expect(streamErr).To(HaveOccurred())
				Expect(streamErr).To(Equal(volume.ErrVolumeDoesNotExist))
			})
			It("should not http request", func() {
				Expect(serverCalled).To(BeFalse())
			})
		})

		Context("when volume found", func() {
			var (
				fakeVolume *volumefakes.FakeFilesystemLiveVolume
			)
			BeforeEach(func() {
				tempFile.WriteString("hello-world")
				tempFile.Close()

				fakeVolume = new(volumefakes.FakeFilesystemLiveVolume)
				fakeVolume.DataPathReturns(filepath.Dir(tempFile.Name()))
				fakeFilesystem.LookupVolumeReturns(fakeVolume, true, nil)
			})

			Context("when load privileged fails", func() {
				BeforeEach(func() {
					fakeVolume.LoadPrivilegedReturns(false, errors.New("some-error"))
				})
				It("should fail", func() {
					Expect(streamErr).To(HaveOccurred())
					Expect(streamErr).To(Equal(errors.New("some-error")))
				})
				It("should not http request", func() {
					Expect(serverCalled).To(BeFalse())
				})
			})

			Context("when load privileged ok", func() {
				BeforeEach(func() {
					fakeVolume.LoadPrivilegedReturns(false, nil)
				})

				Context("remote returns error", func() {
					BeforeEach(func() {
						serverResponseCode = http.StatusInternalServerError
					})
					It("should fail", func() {
						Expect(streamErr).To(HaveOccurred())
						Expect(streamErr).To(Equal(fmt.Errorf("p2p streaming error %d", http.StatusInternalServerError)))
					})
					It("should http request", func() {
						Expect(serverCalled).To(BeTrue())
					})
				})

				Context("remote returns ok", func() {
					BeforeEach(func() {
						serverResponseCode = http.StatusNoContent
					})
					It("should fail", func() {
						Expect(streamErr).ToNot(HaveOccurred())
					})
					It("should http request", func() {
						Expect(serverCalled).To(BeTrue())
					})
					It("remote should receive bytes", func() {
						b := new(bytes.Buffer)
						tgzfs.Compress(b, filepath.Dir(tempFile.Name()), filepath.Base(tempFile.Name()))
						Expect(len(serverReadBytes)).To(Equal(len(b.Bytes())))
						n := len(serverReadBytes)
						Expect(serverReadBytes[:n]).To(Equal(b.Bytes()[:n]))
					})
				})
			})
		})
	})
})
