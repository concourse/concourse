package worker_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

type content map[string][]byte

type file struct {
	name    string
	content []byte
}

// implements os.FileInfo
func (f file) Name() string       { return f.name }
func (f file) Size() int64        { return int64(len(f.content)) }
func (f file) Mode() os.FileMode  { return 0777 }
func (f file) ModTime() time.Time { return time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC) }
func (f file) IsDir() bool        { return false }
func (f file) Sys() interface{}   { return nil }

func tarGzContent(files ...file) []byte {
	buf := new(bytes.Buffer)
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	for _, file := range files {
		header, _ := tar.FileInfoHeader(file, file.name)
		tw.WriteHeader(header)
		tw.Write(file.content)
	}
	tw.Close()
	gw.Close()

	return buf.Bytes()
}

type FakeVolumeFinder struct {
	Volumes map[string]worker.Volume
}

func (f FakeVolumeFinder) FindVolume(_ lager.Logger, teamID int, handle string) (worker.Volume, bool, error) {
	v, ok := f.Volumes[handle]
	return v, ok, nil
}

func newVolumeWithContent(content content) worker.Volume {
	fv := new(workerfakes.FakeVolume)
	fv.StreamOutStub = func(_ context.Context, path string, _ baggageclaim.Encoding) (io.ReadCloser, error) {
		return noopCloser{bytes.NewReader(content[path])}, nil
	}
	return fv
}

type FakeDestination struct {
	Content content
}

func newFakeDestination() FakeDestination {
	return FakeDestination{Content: make(content)}
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

func (f FakeDestination) InitializeResourceCache(cache db.UsedResourceCache) error {
	return nil
}

func BeStreamableWithContent(content content) types.GomegaMatcher {
	return streamableWithContentMatcher{content}
}

type streamableWithContentMatcher struct {
	expected content
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

	return reflect.DeepEqual(dst.Content, m.expected), nil
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
