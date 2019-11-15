package backend_test

import (
	"errors"
	"testing"

	"github.com/concourse/concourse/worker/backend"
	"github.com/concourse/concourse/worker/backend/libcontainerd/libcontainerdfakes"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type BackendSuite struct {
	suite.Suite
	*require.Assertions

	backend backend.Backend
	client  *libcontainerdfakes.FakeClient
}

func (s *BackendSuite) SetupTest() {
	s.client = new(libcontainerdfakes.FakeClient)
	s.backend = backend.New(s.client)
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

func TestSuite(t *testing.T) {
	suite.Run(t, &BackendSuite{
		Assertions: require.New(t),
	})
}
