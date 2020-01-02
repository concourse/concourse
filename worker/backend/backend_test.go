package backend_test

import (
	"code.cloudfoundry.org/garden"
	"context"
	"errors"
	"github.com/concourse/concourse/worker/backend"
	"github.com/concourse/concourse/worker/backend/libcontainerd/libcontainerdfakes"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"syscall"
	"testing"
	"time"
)

type BackendSuite struct {
	suite.Suite
	*require.Assertions

	backend backend.Backend
	client  *libcontainerdfakes.FakeClient
}

const testNamespace = "test-namespace"

func (s *BackendSuite) SetupTest() {
	s.client = new(libcontainerdfakes.FakeClient)
	s.backend = backend.New(s.client, testNamespace)
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

			s.EqualError(err, "client error: error returning version")
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

func (s *BackendSuite) TestCreateSetsNamespace() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	s.client.NewContainerReturns(fakeContainer, nil)

	_, _ = s.backend.Create(minimumValidGdnSpec)
	s.Equal(1, s.client.NewContainerCallCount())

	ctx, _, _, _ := s.client.NewContainerArgsForCall(0)
	namespace, ok := namespaces.Namespace(ctx)
	s.True(ok)
	s.Equal(testNamespace, namespace)
}

func (s *BackendSuite) TestCreateContainerNewTaskFailure() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeContainer.NewTaskReturns(nil, errors.New("err"))

	s.client.NewContainerReturns(fakeContainer, nil)

	_, err := s.backend.Create(minimumValidGdnSpec)
	s.Error(err)

	s.Equal(1, fakeContainer.NewTaskCallCount())
}

func (s *BackendSuite) TestCreateContainerSetsHandle() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeContainer.IDReturns("handle")
	fakeContainer.NewTaskReturns(nil, nil)

	s.client.NewContainerReturns(fakeContainer, nil)
	cont, err := s.backend.Create(minimumValidGdnSpec)
	s.NoError(err)

	s.Equal("handle", cont.Handle())

}

func (s *BackendSuite) TestContainersWithContainerdFailure() {
	s.client.ContainersReturns(nil, errors.New("err"))

	_, err := s.backend.Containers(nil)
	s.Error(err)
	s.Equal(1, s.client.ContainersCallCount())
}

func (s *BackendSuite) TestContainersSetsNamespace() {
	_, _ = s.backend.Containers(nil)
	s.Equal(1, s.client.ContainersCallCount())

	ctx, _ := s.client.ContainersArgsForCall(0)
	namespace, ok := namespaces.Namespace(ctx)
	s.True(ok)
	s.Equal(testNamespace, namespace)
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
	s.ElementsMatch([]string{"foo=bar", "caz=zaz"}, labelSet)
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
	s.Equal("input validation error: handle is empty", err.Error())
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

func (s *BackendSuite) TestLookupCallGetContainerWithNamespace() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeContainer.IDReturns("handle")
	s.client.GetContainerReturns(fakeContainer, nil)

	_, _ = s.backend.Lookup("handle")
	s.Equal(1, s.client.GetContainerCallCount())

	ctx, _ := s.client.GetContainerArgsForCall(0)
	s.NotNil(ctx)

	namespace, found := namespaces.Namespace(ctx)
	s.True(found)
	s.Equal(testNamespace, namespace)
}

func (s *BackendSuite) TestLookupGetContainerError() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeContainer.IDReturns("handle")
	s.client.GetContainerReturns(fakeContainer, nil)

	s.client.GetContainerReturns(nil, errors.New("containerd-err"))

	_, err := s.backend.Lookup("handle")
	s.Error(err)
	s.EqualError(err, "client error: containerd-err")
}

func (s *BackendSuite) TestLookupGetContainerFails() {
	s.client.GetContainerReturns(nil, errors.New("err"))
	_, err := s.backend.Lookup("non-existent-handle")
	s.Error(err)
	s.EqualError(err, "client error: err")
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

func (s *BackendSuite) TestDestroySetsNamespace() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeTask := new(libcontainerdfakes.FakeTask)

	fakeTask.WaitStub = exitBeforeTimeout
	fakeContainer.TaskReturns(fakeTask, nil)
	s.client.GetContainerReturns(fakeContainer, nil)

	_ = s.backend.Destroy("some-handle")
	ctx, _ := s.client.DestroyArgsForCall(0)

	namespace, found := namespaces.Namespace(ctx)
	s.True(found)
	s.Equal(testNamespace, namespace)
}

func (s *BackendSuite) TestDestroyEmptyHandleError() {
	err := s.backend.Destroy("")
	s.EqualError(err, "input validation error: handle is empty")
	s.Equal(0, s.client.DestroyCallCount())
}

func (s *BackendSuite) TestDestroyNonEmptyHandle() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeTask := new(libcontainerdfakes.FakeTask)

	fakeTask.WaitStub = exitBeforeTimeout
	s.client.GetContainerReturns(fakeContainer, nil)
	fakeContainer.TaskReturns(fakeTask, nil)

	err := s.backend.Destroy("some-handle")
	s.NotEqual(err, backend.InputValidationError{})
	s.Equal(1, s.client.DestroyCallCount())
}

func (s *BackendSuite) TestDestroyLookupError() {
	s.client.GetContainerReturns(nil, errors.New("lookup-failed"))

	err := s.backend.Destroy("some-handle")
	s.EqualError(err, "client error: lookup-failed")
}

func (s *BackendSuite) TestDestroyGetTaskError() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeContainer.TaskReturns(nil, errors.New("task-error"))
	s.client.GetContainerReturns(fakeContainer, nil)

	err := s.backend.Destroy("some-handle")
	s.EqualError(err, "client error: task-error")
}

func (s *BackendSuite) TestDestroyGetTaskErrorNotFound() {
	// If a container is created without a task, it means that creation
	// did not complete successfully. These containers should be
	// deletable without error, for garbage collection.
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeContainer.TaskReturns(nil, errdefs.ErrNotFound)
	s.client.GetContainerReturns(fakeContainer, nil)

	err := s.backend.Destroy("some-handle")
	s.NoError(err)

	s.Equal(1, s.client.DestroyCallCount())
}

func (s *BackendSuite) TestDestroyTaskKillError() {
	fakeTask := new(libcontainerdfakes.FakeTask)
	fakeContainer := new(libcontainerdfakes.FakeContainer)

	fakeTask.KillReturns(errors.New("kill-error"))
	fakeContainer.TaskReturns(fakeTask, nil)
	s.client.GetContainerReturns(fakeContainer, nil)

	err := s.backend.Destroy("some-handle")
	s.EqualError(err, "client error: kill-error")

	s.Equal(1, fakeTask.KillCallCount())
	ctx, signal, _ := fakeTask.KillArgsForCall(0)
	s.Equal(syscall.SIGTERM, signal)

	namespace, found := namespaces.Namespace(ctx)
	s.True(found)
	s.Equal(testNamespace, namespace)
}

func (s *BackendSuite) TestDestroyTaskWaitError() {
	fakeTask := new(libcontainerdfakes.FakeTask)
	fakeContainer := new(libcontainerdfakes.FakeContainer)

	fakeTask.WaitReturns(nil, errors.New("wait-error"))
	fakeContainer.TaskReturns(fakeTask, nil)
	s.client.GetContainerReturns(fakeContainer, nil)

	err := s.backend.Destroy("some-handle")
	s.EqualError(err, "client error: wait-error")

	s.Equal(1, fakeTask.WaitCallCount())

	ctx := fakeTask.WaitArgsForCall(0)
	namespace, found := namespaces.Namespace(ctx)
	s.True(found)
	s.Equal(testNamespace, namespace)
}

func (s *BackendSuite) TestDestroyKillTaskSIGTERMFailedError() {
	// in this test case, the exit status is returned before the timeout but with an error,
	// indicating an edge case where SIGTERM did not successfully stop the process but isn't hanging.
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeTask := new(libcontainerdfakes.FakeTask)

	fakeTask.WaitStub = func(ctx context.Context) (<- chan containerd.ExitStatus, error) {
		c := make(chan containerd.ExitStatus, 1)
		go func() {
			es := containerd.NewExitStatus(0, time.Now(), errors.New("sigterm error"))
			c <- *es
			close(c)
		}()
		return c, nil
	}

	fakeContainer.TaskReturns(fakeTask, nil)
	s.client.GetContainerReturns(fakeContainer, nil)

	fakeTask.DeleteReturns(nil, nil)

	err := s.backend.Destroy("some-handle")

	s.Equal(1, fakeTask.KillCallCount())
	s.EqualError(err, "client error: sigterm error")
}

func (s *BackendSuite) TestDestroyKillTaskTimeoutError() {
	// so we don't have to wait 10 seconds for the default timeout
	s.backend = backend.NewWithTimeout(s.client, testNamespace, 10 * time.Millisecond)
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeTask := new(libcontainerdfakes.FakeTask)

	fakeContainer.TaskReturns(fakeTask, nil)
	exitChannel := make(chan containerd.ExitStatus) // this never returns
	fakeTask.WaitReturns(exitChannel, nil)
	s.client.GetContainerReturns(fakeContainer, nil)

	fakeTask.KillReturnsOnCall(0, nil)
	fakeTask.KillReturnsOnCall(1, errors.New("kill-again-error"))

	err := s.backend.Destroy("some-handle")

	s.Equal(2, fakeTask.KillCallCount())
	_, firstSignal, _ := fakeTask.KillArgsForCall(0)
	_, secondSignal, _ := fakeTask.KillArgsForCall(1)
	s.Equal(firstSignal, syscall.SIGTERM)
	s.Equal(secondSignal, syscall.SIGKILL)

	s.EqualError(err, "client error: kill-again-error")
}

func (s *BackendSuite) TestDestroyDeleteTaskError() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeTask := new(libcontainerdfakes.FakeTask)

	fakeTask.WaitStub = exitBeforeTimeout
	fakeContainer.TaskReturns(fakeTask, nil)
	s.client.GetContainerReturns(fakeContainer, nil)

	fakeTask.KillReturns(nil)
	fakeTask.DeleteReturns(nil, errors.New("task-delete-error"))

	err := s.backend.Destroy("some-handle")

	s.Equal(1, fakeTask.KillCallCount())
	s.Equal(1, fakeTask.DeleteCallCount())

	ctx, _ := fakeTask.DeleteArgsForCall(0)
	namespace, found := namespaces.Namespace(ctx)
	s.True(found)
	s.Equal(testNamespace, namespace)

	s.EqualError(err, "client error: task-delete-error")
}

func (s *BackendSuite) TestDestroyDeleteTaskFailedStatusError() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeTask := new(libcontainerdfakes.FakeTask)

	fakeTask.WaitStub = exitBeforeTimeout
	fakeContainer.TaskReturns(fakeTask, nil)
	s.client.GetContainerReturns(fakeContainer, nil)

	fakeTask.KillReturns(nil)
	deleteStatus := containerd.NewExitStatus(1, time.Now(), errors.New("delete failed"))
	fakeTask.DeleteReturns(deleteStatus, nil)

	err := s.backend.Destroy("some-handle")

	s.Equal(1, fakeTask.KillCallCount())
	s.Equal(1, fakeTask.DeleteCallCount())

	ctx, _ := fakeTask.DeleteArgsForCall(0)
	namespace, found := namespaces.Namespace(ctx)
	s.True(found)
	s.Equal(testNamespace, namespace)

	s.EqualError(err, "client error: delete failed")
}

func (s *BackendSuite) TestDestroyContainerError() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeTask := new(libcontainerdfakes.FakeTask)

	fakeTask.WaitStub = exitBeforeTimeout
	fakeContainer.TaskReturns(fakeTask, nil)
	s.client.GetContainerReturns(fakeContainer, nil)
	s.client.DestroyReturns(errors.New("destroy-error"))

	err := s.backend.Destroy("some-handle")

	s.Equal(1, fakeTask.KillCallCount()) // did not go down SIGKILL path
	s.Equal(1, fakeTask.DeleteCallCount())
	s.Equal(1, s.client.DestroyCallCount())
	s.EqualError(err, "client error: destroy-error")
}

func (s *BackendSuite) TestDestroyContainer() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeTask := new(libcontainerdfakes.FakeTask)

	fakeTask.WaitStub = exitBeforeTimeout
	fakeContainer.TaskReturns(fakeTask, nil)
	s.client.GetContainerReturns(fakeContainer, nil)

	err := s.backend.Destroy("some-handle")
	s.Equal(1, s.client.DestroyCallCount())
	s.NoError(err)
}

func (s *BackendSuite) TestStart() {
	err := s.backend.Start()
	s.NoError(err)
	s.Equal(1, s.client.InitCallCount())
}

func (s *BackendSuite) TestStartInitError() {
	s.client.InitReturns(errors.New("init failed"))
	err := s.backend.Start()
	s.EqualError(err, "client error: failed to initialize containerd client: init failed")
}

func (s *BackendSuite) TestStop() {
	s.backend.Stop()
	s.Equal(1, s.client.StopCallCount())
}

func TestSuite(t *testing.T) {
	suite.Run(t, &BackendSuite{
		Assertions: require.New(t),
	})
}

func exitBeforeTimeout(ctx context.Context) (<- chan containerd.ExitStatus, error) {
	c := make(chan containerd.ExitStatus, 1)
	go func() {
		es := containerd.NewExitStatus(0, time.Now(), nil)
		c <- *es
		close(c)
	}()
	return c, nil
}
