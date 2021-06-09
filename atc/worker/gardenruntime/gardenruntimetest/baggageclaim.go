package gardenruntimetest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/runtime/runtimetest"
	"github.com/concourse/concourse/worker/baggageclaim"
)

type Baggageclaim struct {
	Volumes []*Volume
}

func (b *Baggageclaim) FindVolume(handle string) (*Volume, int, bool) {
	for i, v := range b.Volumes {
		if v.handle == handle {
			return v, i, true
		}
	}
	return nil, 0, false
}

func (b *Baggageclaim) FilteredVolumes(pred func(*Volume) bool) []*Volume {
	var filtered []*Volume
	for _, v := range b.Volumes {
		if pred(v) {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

func (b *Baggageclaim) AddVolume(volume *Volume) *Volume {
	_, i, ok := b.FindVolume(volume.handle)
	if ok {
		b.Volumes[i] = volume
		return volume
	}
	b.Volumes = append(b.Volumes, volume)
	return volume
}

func (b *Baggageclaim) CreateVolume(_ lager.Logger, handle string, spec baggageclaim.VolumeSpec) (baggageclaim.Volume, error) {
	volume := b.AddVolume(NewVolume(handle).WithSpec(spec))
	return volume, nil
}

func (b *Baggageclaim) ListVolumes(_ lager.Logger, filter baggageclaim.VolumeProperties) (baggageclaim.Volumes, error) {
	filteredVolumes := b.FilteredVolumes(func(v *Volume) bool {
		return matchesFilter(v.Spec.Properties, filter)
	})
	bcVolumes := make([]baggageclaim.Volume, len(filteredVolumes))
	for i, vol := range filteredVolumes {
		bcVolumes[i] = vol
	}
	return bcVolumes, nil
}

func (b *Baggageclaim) LookupVolume(_ lager.Logger, handle string) (baggageclaim.Volume, bool, error) {
	v, _, ok := b.FindVolume(handle)
	return v, ok, nil
}

func (b *Baggageclaim) DestroyVolumes(logger lager.Logger, handles []string) error {
	for _, handle := range handles {
		b.DestroyVolume(logger, handle)
	}
	return nil
}

func (b *Baggageclaim) DestroyVolume(_ lager.Logger, handle string) error {
	b.Volumes = b.FilteredVolumes(func(v *Volume) bool {
		return v.handle != handle
	})
	return nil
}

func matchesFilter(properties map[string]string, filter map[string]string) bool {
	for k, v := range filter {
		if properties[k] != v {
			return false
		}
	}
	return true
}

func NewVolume(handle string) *Volume {
	return &Volume{
		handle: handle,
		path:   fmt.Sprintf("/volumes/%s", handle),
		Spec: baggageclaim.VolumeSpec{
			Strategy:   baggageclaim.EmptyStrategy{},
			Properties: baggageclaim.VolumeProperties{},
		},
		Content: runtimetest.VolumeContent{},
	}
}

type Volume struct {
	handle string
	path   string
	Spec   baggageclaim.VolumeSpec

	Content runtimetest.VolumeContent
}

func (v Volume) WithContent(content runtimetest.VolumeContent) *Volume {
	v.Content = content
	return &v
}

func (v Volume) WithSpec(spec baggageclaim.VolumeSpec) *Volume {
	v.Spec = spec
	return &v
}

func (v Volume) Handle() string { return v.handle }
func (v Volume) Path() string   { return v.path }

func (v Volume) SetProperty(key, value string) error {
	v.Spec.Properties[key] = value
	return nil
}
func (v Volume) Properties() (baggageclaim.VolumeProperties, error) {
	return v.Spec.Properties, nil
}

func (v *Volume) SetPrivileged(p bool) error {
	v.Spec.Privileged = p
	return nil
}
func (v Volume) GetPrivileged() (bool, error) { return v.Spec.Privileged, nil }

func (v Volume) StreamIn(ctx context.Context, path string, encoding baggageclaim.Encoding, tarStream io.Reader) error {
	return v.Content.StreamIn(ctx, path, encoding, tarStream)
}

func (v Volume) StreamOut(ctx context.Context, path string, encoding baggageclaim.Encoding) (io.ReadCloser, error) {
	return v.Content.StreamOut(ctx, path, encoding)
}

func (v Volume) GetStreamInP2pUrl(_ context.Context, path string) (string, error) {
	closeCh := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(closeCh)
		path := strings.TrimPrefix(r.URL.Path, "/")
		if err := v.StreamIn(r.Context(), path, baggageclaim.GzipEncoding, r.Body); err != nil {
			panic(err)
		}
	}))
	go func() {
		<-closeCh
		server.Close()
	}()
	return server.URL + "/" + path, nil
}

func (v Volume) StreamP2pOut(ctx context.Context, path string, streamInURL string, encoding baggageclaim.Encoding) error {
	stream, err := v.StreamOut(ctx, path, encoding)
	if err != nil {
		return err
	}
	defer stream.Close()
	_, err = http.Post(streamInURL, "application/gzip", stream)
	return err
}

func (v Volume) Destroy() error {
	return nil
}
