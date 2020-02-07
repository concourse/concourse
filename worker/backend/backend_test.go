package backend_test

import (
	"errors"
	"testing"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/worker/backend"
	"github.com/concourse/concourse/worker/backend/backendfakes"
	"github.com/concourse/concourse/worker/backend/libcontainerd/libcontainerdfakes"
	"github.com/containerd/containerd"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type BackendSuite struct {
	suite.Suite
	*require.Assertions

	backend backend.Backend
	client  *libcontainerdfakes.FakeClient
	network *backendfakes.FakeNetwork
	system  *backendfakes.FakeUserNamespace
	killer  *backendfakes.FakeKiller
}

func (s *BackendSuite) SetupTest() {
	s.client = new(libcontainerdfakes.FakeClient)
	s.killer = new(backendfakes.FakeKiller)
	s.network = new(backendfakes.FakeNetwork)
	s.system = new(backendfakes.FakeUserNamespace)

	var err error
	s.backend, err = backend.New(s.client,
		backend.WithKiller(s.killer),
		backend.WithNetwork(s.network),
		backend.WithUserNamespace(s.system),
	)
	s.NoError(err)
}

func (s *BackendSuite) TestNew() {
	_, err := backend.New(nil)
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
	s.EqualError(errors.Unwrap(err), expectedErr.Error())

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

// func (s *BackendSuite) TestDestroyGracefullyStopErrors() {
// 	fakeContainer := new(libcontainerdfakes.FakeContainer)

// 	s.client.GetContainerReturns(fakeContainer, nil)
// 	s.containerStopper.GracefullyStopReturns(errors.New("gracefully-stop-failed"))

// 	err := s.backend.Destroy("some-handle")

// 	s.Equal(1, s.containerStopper.GracefullyStopCallCount())
// 	s.EqualError(errors.Unwrap(err), "gracefully-stop-failed")
// }

// func (s *BackendSuite) TestDestroyContainerDeleteError() {
// 	fakeContainer := new(libcontainerdfakes.FakeContainer)
// 	fakeContainer.DeleteReturns(errors.New("destroy-error"))

// 	s.client.GetContainerReturns(fakeContainer, nil)

// 	err := s.backend.Destroy("some-handle")

// 	s.Equal(1, s.containerStopper.GracefullyStopCallCount())
// 	s.Equal(1, fakeContainer.DeleteCallCount())
// 	s.EqualError(errors.Unwrap(err), "destroy-error")
// }

// func (s *BackendSuite) TestDestroy() {
// 	fakeContainer := new(libcontainerdfakes.FakeContainer)

// 	s.client.GetContainerReturns(fakeContainer, nil)

// 	err := s.backend.Destroy("some-handle")
// 	s.NoError(err)

// 	s.Equal(1, s.client.GetContainerCallCount())
// 	s.Equal(1, s.containerStopper.GracefullyStopCallCount())
// 	s.Equal(1, fakeContainer.DeleteCallCount())
// }

func (s *BackendSuite) TestStart() {
	err := s.backend.Start()
	s.NoError(err)
	s.Equal(1, s.client.InitCallCount())
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
