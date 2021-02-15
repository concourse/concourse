package runtime_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"github.com/concourse/concourse/worker/runtime"
	"github.com/concourse/concourse/worker/runtime/libcontainerd/libcontainerdfakes"
	"github.com/concourse/concourse/worker/runtime/runtimefakes"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type BackendSuite struct {
	suite.Suite
	*require.Assertions

	backend runtime.GardenBackend
	client  *libcontainerdfakes.FakeClient
	network *runtimefakes.FakeNetwork
	userns  *runtimefakes.FakeUserNamespace
	killer  *runtimefakes.FakeKiller
}

func (s *BackendSuite) SetupTest() {
	s.client = new(libcontainerdfakes.FakeClient)
	s.killer = new(runtimefakes.FakeKiller)
	s.network = new(runtimefakes.FakeNetwork)
	s.userns = new(runtimefakes.FakeUserNamespace)

	var err error
	s.backend, err = runtime.NewGardenBackend(s.client,
		runtime.WithKiller(s.killer),
		runtime.WithNetwork(s.network),
		runtime.WithUserNamespace(s.userns),
	)
	s.NoError(err)
}

func (s *BackendSuite) TestNew() {
	_, err := runtime.NewGardenBackend(nil)
	s.EqualError(err, "nil client")
}

func (s *BackendSuite) TestPing() {
	for _, tc := range []struct {
		desc          string
		versionReturn error
		succeeds      bool
	}{
		{
			desc:          "fail from containerd version service",
			succeeds:      true,
			versionReturn: nil,
		},
		{
			desc:          "ok from containerd's version service",
			succeeds:      false,
			versionReturn: errors.New("error returning version"),
		},
	} {
		s.T().Run(tc.desc, func(t *testing.T) {
			s.client.VersionReturns(tc.versionReturn)

			err := s.backend.Ping()
			if tc.succeeds {
				s.NoError(err)
				return
			}

			s.EqualError(errors.Unwrap(err), "error returning version")
		})
	}
}

var (
	invalidGdnSpec      = garden.ContainerSpec{}
	minimumValidGdnSpec = garden.ContainerSpec{
		Handle: "handle", RootFSPath: "raw:///rootfs",
	}
)

func (s *BackendSuite) TestCreateWithInvalidSpec() {
	_, err := s.backend.Create(invalidGdnSpec)

	s.Error(err)
	s.Equal(0, s.client.NewContainerCallCount())
}

func (s *BackendSuite) TestCreateWithNewContainerFailure() {
	s.client.NewContainerReturns(nil, errors.New("err"))

	_, err := s.backend.Create(minimumValidGdnSpec)
	s.Error(err)

	s.Equal(1, s.client.NewContainerCallCount())
}

func (s *BackendSuite) TestCreateContainerNewTaskFailure() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)

	expectedErr := errors.New("task-err")
	fakeContainer.NewTaskReturns(nil, expectedErr)

	s.client.NewContainerReturns(fakeContainer, nil)

	_, err := s.backend.Create(minimumValidGdnSpec)
	s.EqualError(errors.Unwrap(errors.Unwrap(err)), expectedErr.Error())

	s.Equal(1, fakeContainer.NewTaskCallCount())
}

func (s *BackendSuite) TestCreateContainerTaskStartFailure() {
	fakeTask := new(libcontainerdfakes.FakeTask)
	fakeContainer := new(libcontainerdfakes.FakeContainer)

	s.client.NewContainerReturns(fakeContainer, nil)
	fakeContainer.NewTaskReturns(fakeTask, nil)
	fakeTask.StartReturns(errors.New("start-err"))

	_, err := s.backend.Create(minimumValidGdnSpec)
	s.Error(err)

	s.EqualError(errors.Unwrap(err), "start-err")
}

func (s *BackendSuite) TestCreateContainerSetsHandle() {
	fakeTask := new(libcontainerdfakes.FakeTask)
	fakeContainer := new(libcontainerdfakes.FakeContainer)

	fakeContainer.IDReturns("handle")
	fakeContainer.NewTaskReturns(fakeTask, nil)

	s.client.NewContainerReturns(fakeContainer, nil)
	cont, err := s.backend.Create(minimumValidGdnSpec)
	s.NoError(err)

	s.Equal("handle", cont.Handle())
}

func (s *BackendSuite) TestCreateMaxContainersReached() {
	backend, err := runtime.NewGardenBackend(s.client,
		runtime.WithKiller(s.killer),
		runtime.WithNetwork(s.network),
		runtime.WithUserNamespace(s.userns),
		runtime.WithMaxContainers(1),
		runtime.WithRequestTimeout(1*time.Second),
	)
	s.NoError(err)

	fakeTask := new(libcontainerdfakes.FakeTask)
	fakeContainer := new(libcontainerdfakes.FakeContainer)

	fakeContainer.NewTaskReturns(fakeTask, nil)
	s.client.NewContainerReturns(fakeContainer, nil)

	s.client.ContainersReturns([]containerd.Container{fakeContainer}, nil)
	_, err = backend.Create(minimumValidGdnSpec)
	s.Error(err)
	s.Contains(err.Error(), "max containers reached")
}

func (s *BackendSuite) TestCreateMaxContainersReachedConcurrent() {
	fakeTask := new(libcontainerdfakes.FakeTask)
	fakeContainer := new(libcontainerdfakes.FakeContainer)

	fakeContainer.NewTaskReturns(fakeTask, nil)

	s.client.NewContainerStub = func(context context.Context, str string, strings map[string]string, spec *specs.Spec) (container containerd.Container, e error) {
		s.client.ContainersReturns([]containerd.Container{fakeContainer}, nil)
		return fakeContainer, nil
	}

	backend, err := runtime.NewGardenBackend(s.client,
		runtime.WithKiller(s.killer),
		runtime.WithNetwork(s.network),
		runtime.WithUserNamespace(s.userns),
		runtime.WithMaxContainers(1),
		runtime.WithRequestTimeout(1*time.Second),
	)
	s.NoError(err)

	numberOfRequests := 10
	requestErrors := make(chan error, numberOfRequests)
	wg := sync.WaitGroup{}
	wg.Add(numberOfRequests)

	for i := 0; i < numberOfRequests; i++ {
		go func() {
			_, err := backend.Create(minimumValidGdnSpec)
			if err != nil {
				requestErrors <- err
			}
			wg.Done()
		}()
	}
	wg.Wait()
	close(requestErrors)

	s.Len(requestErrors, numberOfRequests-1)
	s.Equal(s.client.NewContainerCallCount(), 1)
	for err := range requestErrors {
		s.Contains(err.Error(), "max containers reached")
	}
}

func (s *BackendSuite) TestCreateContainerLockTimeout() {
	fakeTask := new(libcontainerdfakes.FakeTask)
	fakeContainer := new(libcontainerdfakes.FakeContainer)

	fakeContainer.IDReturns("handle")
	fakeContainer.NewTaskReturns(fakeTask, nil)

	s.client.NewContainerStub = func(context context.Context, str string, strings map[string]string, spec *specs.Spec) (container containerd.Container, e error) {
		s.client.ContainersReturns([]containerd.Container{fakeContainer}, nil)
		time.Sleep(500 * time.Millisecond)
		return fakeContainer, nil
	}

	numberOfRequests := 10

	backend, err := runtime.NewGardenBackend(s.client,
		runtime.WithKiller(s.killer),
		runtime.WithNetwork(s.network),
		runtime.WithUserNamespace(s.userns),
		runtime.WithRequestTimeout(10*time.Millisecond),
		runtime.WithMaxContainers(numberOfRequests),
	)
	s.NoError(err)

	requestErrors := make(chan error, numberOfRequests)
	wg := sync.WaitGroup{}
	wg.Add(numberOfRequests)

	for i := 0; i < numberOfRequests; i++ {
		go func() {
			_, err := backend.Create(minimumValidGdnSpec)
			if err != nil {
				requestErrors <- err
			}
			wg.Done()
		}()
	}
	wg.Wait()
	close(requestErrors)

	s.Len(requestErrors, numberOfRequests-1)
	for err := range requestErrors {
		s.Contains(err.Error(), "acquiring create container lock")
	}
}
func (s *BackendSuite) TestContainersWithContainerdFailure() {
	s.client.ContainersReturns(nil, errors.New("err"))

	_, err := s.backend.Containers(nil)
	s.Error(err)
	s.Equal(1, s.client.ContainersCallCount())
}

func (s *BackendSuite) TestContainersWithInvalidPropertyFilters() {
	for _, tc := range []struct {
		desc   string
		filter map[string]string
	}{
		{
			desc: "empty key",
			filter: map[string]string{
				"": "bar",
			},
		},
		{
			desc: "empty value",
			filter: map[string]string{
				"foo": "",
			},
		},
	} {
		s.T().Run(tc.desc, func(t *testing.T) {
			_, err := s.backend.Containers(tc.filter)

			s.Error(err)
			s.Equal(0, s.client.ContainersCallCount())
		})
	}
}

func (s *BackendSuite) TestContainersWithProperProperties() {
	_, _ = s.backend.Containers(map[string]string{"foo": "bar", "caz": "zaz"})
	s.Equal(1, s.client.ContainersCallCount())

	_, labelSet := s.client.ContainersArgsForCall(0)
	s.ElementsMatch([]string{"labels.foo==bar", "labels.caz==zaz"}, labelSet)
}

func (s *BackendSuite) TestContainersConversion() {
	fakeContainer1 := new(libcontainerdfakes.FakeContainer)
	fakeContainer2 := new(libcontainerdfakes.FakeContainer)

	s.client.ContainersReturns([]containerd.Container{
		fakeContainer1, fakeContainer2,
	}, nil)

	containers, err := s.backend.Containers(nil)
	s.NoError(err)
	s.Equal(1, s.client.ContainersCallCount())
	s.Len(containers, 2)
}

func (s *BackendSuite) TestLookupEmptyHandleError() {
	_, err := s.backend.Lookup("")
	s.Equal("empty handle", err.Error())
}

func (s *BackendSuite) TestLookupCallGetContainerWithHandle() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeContainer.IDReturns("handle")
	s.client.GetContainerReturns(fakeContainer, nil)

	_, _ = s.backend.Lookup("handle")
	s.Equal(1, s.client.GetContainerCallCount())

	_, handle := s.client.GetContainerArgsForCall(0)
	s.Equal("handle", handle)
}

func (s *BackendSuite) TestLookupGetContainerError() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeContainer.IDReturns("handle")
	s.client.GetContainerReturns(fakeContainer, nil)

	s.client.GetContainerReturns(nil, errors.New("containerd-err"))

	_, err := s.backend.Lookup("handle")
	s.Error(err)
	s.EqualError(errors.Unwrap(err), "containerd-err")
}

func (s *BackendSuite) TestLookupGetContainerFails() {
	s.client.GetContainerReturns(nil, errors.New("err"))
	_, err := s.backend.Lookup("non-existent-handle")
	s.Error(err)
	s.EqualError(errors.Unwrap(err), "err")
}

func (s *BackendSuite) TestLookupGetNoContainerReturned() {
	s.client.GetContainerReturns(nil, errors.New("not found"))
	container, err := s.backend.Lookup("non-existent-handle")
	s.Error(err)
	s.Nil(container)
}

func (s *BackendSuite) TestLookupGetContainer() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeContainer.IDReturns("handle")
	s.client.GetContainerReturns(fakeContainer, nil)
	container, err := s.backend.Lookup("handle")
	s.NoError(err)
	s.NotNil(container)
	s.Equal("handle", container.Handle())
}

func (s *BackendSuite) TestDestroyEmptyHandleError() {
	err := s.backend.Destroy("")
	s.EqualError(err, "empty handle")
}

func (s *BackendSuite) TestDestroyGetContainerError() {
	s.client.GetContainerReturns(nil, errors.New("get-container-failed"))

	err := s.backend.Destroy("some-handle")
	s.EqualError(errors.Unwrap(err), "get-container-failed")
}

func (s *BackendSuite) TestDestroyGetTaskError() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)

	s.client.GetContainerReturns(fakeContainer, nil)

	expectedError := errors.New("get-task-failed")
	fakeContainer.TaskReturns(nil, expectedError)

	err := s.backend.Destroy("some handle")
	s.True(errors.Is(err, expectedError))
}

func (s *BackendSuite) TestDestroyGetTaskErrorNotFoundAndDeleteFails() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)

	s.client.GetContainerReturns(fakeContainer, nil)
	fakeContainer.TaskReturns(nil, errdefs.ErrNotFound)

	expectedError := errors.New("delete-container-failed")
	fakeContainer.DeleteReturns(expectedError)

	err := s.backend.Destroy("some handle")
	s.True(errors.Is(err, expectedError))
}

func (s *BackendSuite) TestDestroyGetTaskErrorNotFoundAndDeleteSucceeds() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)

	s.client.GetContainerReturns(fakeContainer, nil)
	fakeContainer.TaskReturns(nil, errdefs.ErrNotFound)

	err := s.backend.Destroy("some handle")

	s.Equal(1, fakeContainer.DeleteCallCount())
	s.NoError(err)
}

func (s *BackendSuite) TestDestroyKillTaskFails() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeTask := new(libcontainerdfakes.FakeTask)

	s.client.GetContainerReturns(fakeContainer, nil)
	fakeContainer.TaskReturns(fakeTask, nil)

	expectedError := errors.New("kill-task-failed")
	s.killer.KillReturns(expectedError)

	err := s.backend.Destroy("some handle")
	s.True(errors.Is(err, expectedError))
	_, _, behaviour := s.killer.KillArgsForCall(0)
	s.Equal(runtime.KillGracefully, behaviour)
}

func (s *BackendSuite) TestDestroyRemoveNetworkFails() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeTask := new(libcontainerdfakes.FakeTask)

	s.client.GetContainerReturns(fakeContainer, nil)
	fakeContainer.TaskReturns(fakeTask, nil)

	expectedError := errors.New("remove-network-failed")
	s.network.RemoveReturns(expectedError)

	err := s.backend.Destroy("some handle")
	s.True(errors.Is(err, expectedError))
}

func (s *BackendSuite) TestDestroyDeleteTaskFails() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeTask := new(libcontainerdfakes.FakeTask)

	s.client.GetContainerReturns(fakeContainer, nil)
	fakeContainer.TaskReturns(fakeTask, nil)

	expectedError := errors.New("delete-task-failed")
	fakeTask.DeleteReturns(nil, expectedError)

	err := s.backend.Destroy("some handle")
	s.True(errors.Is(err, expectedError))
}

func (s *BackendSuite) TestDestroyContainerDeleteFailsAndDeleteTaskSucceeds() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeTask := new(libcontainerdfakes.FakeTask)

	s.client.GetContainerReturns(fakeContainer, nil)
	fakeContainer.TaskReturns(fakeTask, nil)

	expectedError := errors.New("delete-container-failed")
	fakeContainer.DeleteReturns(expectedError)

	err := s.backend.Destroy("some handle")
	s.True(errors.Is(err, expectedError))
}

func (s *BackendSuite) TestDestroySucceeds() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeTask := new(libcontainerdfakes.FakeTask)
	s.client.GetContainerReturns(fakeContainer, nil)
	fakeContainer.TaskReturns(fakeTask, nil)

	err := s.backend.Destroy("some handle")
	s.NoError(err)
}

func (s *BackendSuite) TestStartInitsClientAndSetsUpRestrictedNetworks() {
	err := s.backend.Start()
	s.NoError(err)
	s.Equal(1, s.client.InitCallCount())
	s.Equal(1, s.network.SetupRestrictedNetworksCallCount())
}

func (s *BackendSuite) TestStartInitError() {
	s.client.InitReturns(errors.New("init failed"))
	err := s.backend.Start()
	s.EqualError(errors.Unwrap(err), "init failed")
}

func (s *BackendSuite) TestStop() {
	s.backend.Stop()
	s.Equal(1, s.client.StopCallCount())
}

func (s *BackendSuite) TestGraceTimeGetPropertyFails() {
	fakeContainer := new(gardenfakes.FakeContainer)
	fakeContainer.PropertyReturns("", errors.New("error"))
	result := s.backend.GraceTime(fakeContainer)
	s.Equal(time.Duration(0), result)
}

func (s *BackendSuite) TestGraceTimeInvalidInteger() {
	fakeContainer := new(gardenfakes.FakeContainer)
	fakeContainer.PropertyReturns("not a number", nil)
	result := s.backend.GraceTime(fakeContainer)
	s.Equal(time.Duration(0), result)
}

func (s *BackendSuite) TestGraceTimeReturnsDuration() {
	fakeContainer := new(gardenfakes.FakeContainer)
	fakeContainer.PropertyReturns("123", nil)
	result := s.backend.GraceTime(fakeContainer)
	s.Equal(time.Duration(123), result)
}
