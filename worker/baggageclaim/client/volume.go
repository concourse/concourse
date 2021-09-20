package client

import (
	"context"
	"io"

	"github.com/concourse/concourse/worker/baggageclaim"
	"github.com/concourse/concourse/worker/baggageclaim/volume"
)

type clientVolume struct {
	handle string
	path   string

	bcClient *client
}

func (cv *clientVolume) Handle() string {
	return cv.handle
}

func (cv *clientVolume) Path() string {
	return cv.path
}

func (cv *clientVolume) Properties(ctx context.Context) (baggageclaim.VolumeProperties, error) {
	vr, found, err := cv.bcClient.getVolumeResponse(ctx, cv.handle)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, volume.ErrVolumeDoesNotExist
	}

	return vr.Properties, nil
}

func (cv *clientVolume) StreamIn(ctx context.Context, path string, encoding baggageclaim.Encoding, tarStream io.Reader) error {
	return cv.bcClient.streamIn(ctx, cv.handle, path, encoding, tarStream)
}

func (cv *clientVolume) StreamOut(ctx context.Context, path string, encoding baggageclaim.Encoding) (io.ReadCloser, error) {
	return cv.bcClient.streamOut(ctx, cv.handle, encoding, path)
}

func (cv *clientVolume) GetPrivileged(ctx context.Context) (bool, error) {
	return cv.bcClient.getPrivileged(ctx, cv.handle)
}

func (cv *clientVolume) SetPrivileged(ctx context.Context, privileged bool) error {
	return cv.bcClient.setPrivileged(ctx, cv.handle, privileged)
}

func (cv *clientVolume) Destroy(ctx context.Context) error {
	return cv.bcClient.destroy(ctx, cv.handle)
}

func (cv *clientVolume) SetProperty(ctx context.Context, name string, value string) error {
	return cv.bcClient.setProperty(ctx, cv.handle, name, value)
}

func (cv *clientVolume) GetStreamInP2pUrl(ctx context.Context, path string) (string, error) {
	return cv.bcClient.getStreamInP2pUrl(ctx, cv.handle, path)
}

func (cv *clientVolume) StreamP2pOut(ctx context.Context, path, url string, encoding baggageclaim.Encoding) error {
	return cv.bcClient.streamP2pOut(ctx, cv.handle, encoding, path, url)
}
