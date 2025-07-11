//go:build linux

package runtime_test

import (
	"github.com/concourse/concourse/worker/runtime"
	"github.com/concourse/concourse/worker/runtime/runtimefakes"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type IOManagerSuite struct {
	suite.Suite
	*require.Assertions

	ioManager runtime.IOManager
}

func (s *IOManagerSuite) SetupTest() {
	s.ioManager = runtime.NewIOManager()
}

func (s *IOManagerSuite) TestTrackingNewIOReaders() {
	id := "some-id"
	ioCreater := s.ioManager.Creator(id, cio.NullIO)
	expectedCIO, err := ioCreater(id)
	s.NoError(err)

	actualCIO, exists := s.ioManager.Get(id)
	s.True(exists)
	s.Equal(expectedCIO, actualCIO)
}

func (s *IOManagerSuite) TestClosingPreviousReaders() {
	id := "some-id"
	firstIO := &runtimefakes.FakeIO{}
	ioCreater := s.ioManager.Creator(id, func(id string) (cio.IO, error) {
		return firstIO, nil
	})
	_, err := ioCreater(id)
	s.NoError(err)

	secondIO := &runtimefakes.FakeIO{}
	ioAttach := s.ioManager.Attach(id, func(_ *cio.FIFOSet) (cio.IO, error) {
		s.Zero(firstIO.CancelCallCount(), "should only be called AFTER the new IO is attached")
		s.Zero(firstIO.CloseCallCount(), "should only be called AFTER the new IO is attached")
		return secondIO, nil
	})
	_, err = ioAttach(nil)
	s.NoError(err)

	s.Equal(1, firstIO.CancelCallCount(), "should have been called now that the new IO is attached")
	s.Equal(1, firstIO.CloseCallCount(), "should have been called now that the new IO is attached")

	actualIO, exists := s.ioManager.Get(id)
	s.True(exists)
	s.Equal(secondIO, actualIO, "IOManager should have the new IO")
}

func (s *IOManagerSuite) TestDeletingReader() {
	id := "some-id"
	ioCreater := s.ioManager.Creator(id, cio.NullIO)
	_, err := ioCreater(id)
	s.NoError(err)

	s.ioManager.Delete(id)

	actualCIO, exists := s.ioManager.Get(id)
	s.False(exists, "the IO should have been removed from the IOManager")
	s.Nil(actualCIO)
}
