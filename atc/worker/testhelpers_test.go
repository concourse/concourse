package worker_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"reflect"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

type FakeVolumeFinder struct {
	Volumes map[string]worker.Volume
}

func (f FakeVolumeFinder) FindVolume(_ lager.Logger, teamID int, handle string) (worker.Volume, bool, error) {
	v, ok := f.Volumes[handle]
	return v, ok, nil
}

func newVolumeWithContent(content []byte) worker.Volume {
	fv := new(workerfakes.FakeVolume)
	fv.StreamOutStub = func(_ context.Context, _ string, _ baggageclaim.Encoding) (io.ReadCloser, error) {
		return noopCloser{bytes.NewReader(content)}, nil
	}
	return fv
}

type FakeDestination struct {
	Content map[string][]byte
}

func newFakeDestination() FakeDestination {
	return FakeDestination{Content: make(map[string][]byte)}
}

func (f FakeDestination) StreamIn(ctx context.Context, path string, enc baggageclaim.Encoding, tarReader io.Reader) error {
	content, err := ioutil.ReadAll(tarReader)
	if err != nil {
		return err
	}
	f.Content[path] = content
	return nil
}

func (f FakeDestination) GetStreamInP2pUrl(ctx context.Context, path string) (string, error) {
	panic("unimplemented")
}

func BeStreamableWithContent(content []byte) types.GomegaMatcher {
	return streamableWithContentMatcher{content}
}

type streamableWithContentMatcher struct {
	expected []byte
}

func (m streamableWithContentMatcher) Match(actual interface{}) (bool, error) {
	streamable, ok := actual.(worker.StreamableArtifactSource)
	if !ok {
		return false, errors.New("BeStreamableWithContent matcher expects a StreamableArtifactSource")
	}

	dst := newFakeDestination()

	err := streamable.StreamTo(context.Background(), dst)
	if err != nil {
		return false, err
	}

	return reflect.DeepEqual(dst.Content["."], m.expected), nil
}
func (m streamableWithContentMatcher) FailureMessage(actual interface{}) string {
	return format.Message(actual, "to stream the content", m.expected)
}
func (m streamableWithContentMatcher) NegatedFailureMessage(actual interface{}) string {
	return format.Message(actual, "not to stream the content", m.expected)
}

func ExistOnWorker(w worker.Worker) types.GomegaMatcher {
	return existOnWorkerMatcher{w}
}

type existOnWorkerMatcher struct {
	worker worker.Worker
}

func (m existOnWorkerMatcher) Match(actual interface{}) (bool, error) {
	source, ok := actual.(worker.ArtifactSource)
	if !ok {
		return false, errors.New("ExistOnWorker matcher expects an ArtifactSource")
	}

	_, found, err := source.ExistsOn(lagertest.NewTestLogger(""), m.worker)
	if err != nil {
		return false, err
	}

	return found, nil
}
func (m existOnWorkerMatcher) FailureMessage(actual interface{}) string {
	return format.Message(actual, "to exist on the worker", m.worker)
}
func (m existOnWorkerMatcher) NegatedFailureMessage(actual interface{}) string {
	return format.Message(actual, "not to exist on the worker", m.worker)
}

type noopCloser struct{ io.Reader }

func (noopCloser) Close() error { return nil }
