package backend_test

import (
	"errors"
	"github.com/containerd/containerd/errdefs"
	"syscall"
	"testing"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/worker/backend"
	"github.com/concourse/concourse/worker/backend/libcontainerd/libcontainerdfakes"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
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
			versionReturn: errors.New("errr"),
		},
	} {
		s.T().Run(tc.desc, func(t *testing.T) {
			s.client.VersionReturns(tc.versionReturn)

			err := s.backend.Ping()
			if tc.succeeds {
				s.NoError(err)
				return
			}

			s.Error(err)
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
	s.Equal(err, backend.InputValidationError{})
}

func (s *BackendSuite) TestLookupCallGetContainerWithHandle() {
	_, _ = s.backend.Lookup("handle")
	s.Equal(1, s.client.GetContainerCallCount())

	_, handle := s.client.GetContainerArgsForCall(0)
	s.Equal("handle", handle)
}

func (s *BackendSuite) TestLookupCallGetContainerWithNamespace() {
	_, _ = s.backend.Lookup("handle")
	s.Equal(1, s.client.GetContainerCallCount())

	ctx, _ := s.client.GetContainerArgsForCall(0)
	s.NotNil(ctx)

	namespace, found := namespaces.Namespace(ctx)
	s.True(found)
	s.Equal(testNamespace, namespace)
}

func (s *BackendSuite) TestLookupGetContainerError() {
	s.client.GetContainerReturns(nil, errors.New("containerd-err"))

	_, err := s.backend.Lookup("handle")
	s.Error(err)
	s.Equal(err, errors.New("containerd-err"))
}

func (s *BackendSuite) TestLookupGetContainerFails() {
	s.client.GetContainerReturns(nil, errors.New("err"))
	_, err := s.backend.Lookup("non-existent-handle")
	s.Error(err)
	s.Equal(err, errors.New("err"))
}

func (s *BackendSuite) TestLookupGetContainer() {
	s.client.GetContainerReturns(new(libcontainerdfakes.FakeContainer), nil)
	container, err := s.backend.Lookup("non-existent-handle")
	s.NoError(err)
	s.NotNil(container)
}

func (s *BackendSuite) TestDestroySetsNamespace() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeTask := new(libcontainerdfakes.FakeTask)

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
	s.Equal(err, backend.InputValidationError{})
	s.Equal(0, s.client.DestroyCallCount())
}

func (s *BackendSuite) TestDestroyNonEmptyHandle() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeTask := new(libcontainerdfakes.FakeTask)

	s.client.GetContainerReturns(fakeContainer, nil)
	fakeContainer.TaskReturns(fakeTask, nil)


	err := s.backend.Destroy("some-handle")
	s.NotEqual(err, backend.InputValidationError{})
	s.Equal(1, s.client.DestroyCallCount())
}

func (s *BackendSuite) TestDestroyLookupError() {
	s.client.GetContainerReturns(nil, errors.New("lookup-failed"))

	err := s.backend.Destroy("some-handle")
	s.Equal(err, errors.New("lookup-failed"))
}

func (s *BackendSuite) TestDestroyGetTaskError() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeContainer.TaskReturns(nil, errors.New("task-error"))
	s.client.GetContainerReturns(fakeContainer, nil)

	err := s.backend.Destroy("some-handle")
	s.Equal(err, errors.New("task-error"))
}

func (s *BackendSuite) TestDestroyGetTaskErrorNotFound() {
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
	s.EqualError(err, "kill-error")

	s.Equal(1, fakeTask.KillCallCount())
	ctx, signal, _ := fakeTask.KillArgsForCall(0)
	s.Equal(syscall.SIGTERM, signal)

	namespace, found := namespaces.Namespace(ctx)
	s.True(found)
	s.Equal(testNamespace, namespace)
}

func (s *BackendSuite) TestDestroyWaitError() {
	fakeTask := new(libcontainerdfakes.FakeTask)
	fakeContainer := new(libcontainerdfakes.FakeContainer)

	fakeTask.WaitReturns(nil, errors.New("wait-error"))
	fakeContainer.TaskReturns(fakeTask, nil)
	s.client.GetContainerReturns(fakeContainer, nil)

	err := s.backend.Destroy("some-handle")
	s.EqualError(err, "wait-error")

	s.Equal(1, fakeTask.WaitCallCount())

	ctx := fakeTask.WaitArgsForCall(0)
	namespace, found := namespaces.Namespace(ctx)
	s.True(found)
	s.Equal(testNamespace, namespace)
}

func (s *BackendSuite) TestDestroyKillTaskTimeoutError() {
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
	s.EqualError(err, "kill-again-error")
}

func (s *BackendSuite) TestDestroyDeleteTaskError() { // todo: this is still going down the context timeout path
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeTask := new(libcontainerdfakes.FakeTask)

	fakeContainer.TaskReturns(fakeTask, nil)
	exitChannel := make(chan containerd.ExitStatus) // this never returns
	fakeTask.WaitReturns(exitChannel, nil)
	s.client.GetContainerReturns(fakeContainer, nil)

	fakeTask.KillReturns(nil)
	fakeTask.DeleteReturns(nil, errors.New("task-delete-error"))

	err := s.backend.Destroy("some-handle")

	s.Equal(2, fakeTask.KillCallCount())
	s.Equal(1, fakeTask.DeleteCallCount())

	ctx, _ := fakeTask.DeleteArgsForCall(0)
	namespace, found := namespaces.Namespace(ctx)
	s.True(found)
	s.Equal(testNamespace, namespace)

	s.EqualError(err, "task-delete-error")
}

func (s *BackendSuite) TestDestroyDeleteTaskExitsNonzeroError() {
}

//func (s *BackendSuite) TestDestroyContainerError() {
//	fakeContainer := new(libcontainerdfakes.FakeContainer)
//	fakeTask := new(libcontainerdfakes.FakeTask)
//
//	exitChannel := make(chan containerd.ExitStatus)
//	fakeTask.WaitReturns(exitChannel, nil)
//	fakeContainer.TaskReturns(fakeTask, nil)
//	s.client.GetContainerReturns(fakeContainer, nil)
//	s.client.DestroyReturns(errors.New("random"))
//
//	// TODO: get the exitChannel to return before the timeout
//	//s.Equal(1, fakeTask.KillCallCount())
//
//	s.Equal(1, s.client.DestroyCallCount())
//	//s.Error(err)
//}

// TODO: send down successful channel exitStatus path
func (s *BackendSuite) TestDestroyContainer() {
	fakeContainer := new(libcontainerdfakes.FakeContainer)
	fakeTask := new(libcontainerdfakes.FakeTask)

	fakeContainer.TaskReturns(fakeTask, nil)
	s.client.GetContainerReturns(fakeContainer, nil)

	err := s.backend.Destroy("some-handle")
	s.Equal(1, s.client.DestroyCallCount())
	s.NoError(err)
}

func (s *BackendSuite) TestStart() {
	s.backend.Start()
	s.Equal(1, s.client.InitCallCount())
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
