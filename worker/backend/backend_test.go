package backend_test

import (
	"errors"
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

func (s *BackendSuite) TestDestroySetsNamespace() {
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
	err := s.backend.Destroy("some-handle")
	s.NotEqual(err, backend.InputValidationError{})
	s.Equal(1, s.client.DestroyCallCount())
}

func (s *BackendSuite) TestDestroyContainerError() {
	s.client.DestroyReturns(errors.New("random"))

	err := s.backend.Destroy("some-handle")
	s.Equal(1, s.client.DestroyCallCount())
	s.Error(err)
}

func (s *BackendSuite) TestDestroyContainer() {
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
