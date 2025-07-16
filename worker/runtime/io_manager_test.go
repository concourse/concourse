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
	containerId := "cid"
	taskID := "tid"
	ioCreater := s.ioManager.Creator(containerId, taskID, cio.NullIO)
	expectedCIO, err := ioCreater("some-other-id")
	s.NoError(err)

	actualCIO, exists := s.ioManager.Get(containerId, taskID)
	s.True(exists)
	s.Equal(expectedCIO, actualCIO)
}

func (s *IOManagerSuite) TestClosingPreviousReaders() {
	containerId := "cid"
	taskID := "tid"
	firstIO := &runtimefakes.FakeIO{}
	ioCreater := s.ioManager.Creator(containerId, taskID, func(id string) (cio.IO, error) {
		return firstIO, nil
	})
	_, err := ioCreater("some-other-id")
	s.NoError(err)

	secondIO := &runtimefakes.FakeIO{}
	ioAttach := s.ioManager.Attach(containerId, taskID, func(_ *cio.FIFOSet) (cio.IO, error) {
		s.Zero(firstIO.CancelCallCount(), "should only be called AFTER the new IO is attached")
		s.Zero(firstIO.CloseCallCount(), "should never be called")
		return secondIO, nil
	})
	_, err = ioAttach(nil)
	s.NoError(err)

	s.Equal(1, firstIO.CancelCallCount(), "should have been called now that the new IO is attached")
	s.Zero(firstIO.CloseCallCount(), "should never be called")

	actualIO, exists := s.ioManager.Get(containerId, taskID)
	s.True(exists)
	s.Equal(secondIO, actualIO, "IOManager should have the new IO")
	s.NotEqual(actualIO, firstIO, "the previous IO should never be re-used")
}

func (s *IOManagerSuite) TestAttachWhenNoPreviousIOExists() {
	containerId := "cid"
	taskID := "tid"
	fakeIO := &runtimefakes.FakeIO{}

	ioAttach := s.ioManager.Attach(containerId, taskID, func(_ *cio.FIFOSet) (cio.IO, error) {
		return fakeIO, nil
	})
	_, err := ioAttach(nil)
	s.NoError(err)

	actualIO, exists := s.ioManager.Get(containerId, taskID)
	s.True(exists)
	s.Equal(fakeIO, actualIO, "IOManager should have the new IO")
}

func (s *IOManagerSuite) TestDeletingReader() {
	containerId := "cid"
	taskID := "tid"
	ioCreater := s.ioManager.Creator(containerId, taskID, cio.NullIO)
	_, err := ioCreater("some-other-id")
	s.NoError(err)

	s.ioManager.Delete(containerId)

	actualCIO, exists := s.ioManager.Get(containerId, taskID)
	s.False(exists, "the IO should have been removed from the IOManager")
	s.Nil(actualCIO)
}
