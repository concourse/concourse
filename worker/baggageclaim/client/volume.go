package client

import (
	"context"
	"io"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/worker/baggageclaim"
	"github.com/concourse/concourse/worker/baggageclaim/volume"
)

type clientVolume struct {
	// TODO: this would be much better off as an arg to each method
	logger lager.Logger

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

func (cv *clientVolume) Properties() (baggageclaim.VolumeProperties, error) {
	vr, found, err := cv.bcClient.getVolumeResponse(cv.logger, cv.handle)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, volume.ErrVolumeDoesNotExist
	}

	return vr.Properties, nil
}

func (cv *clientVolume) StreamIn(ctx context.Context, path string, encoding baggageclaim.Encoding, tarStream io.Reader) error {
	return cv.bcClient.streamIn(ctx, cv.logger, cv.handle, path, encoding, tarStream)
}

func (cv *clientVolume) StreamOut(ctx context.Context, path string, encoding baggageclaim.Encoding) (io.ReadCloser, error) {
	return cv.bcClient.streamOut(ctx, cv.logger, cv.handle, encoding, path)
}

func (cv *clientVolume) GetPrivileged() (bool, error) {
	return cv.bcClient.getPrivileged(cv.logger, cv.handle)
}

func (cv *clientVolume) SetPrivileged(privileged bool) error {
	return cv.bcClient.setPrivileged(cv.logger, cv.handle, privileged)
}

func (cv *clientVolume) Destroy() error {
	return cv.bcClient.destroy(cv.logger, cv.handle)
}

func (cv *clientVolume) SetProperty(name string, value string) error {
	return cv.bcClient.setProperty(cv.logger, cv.handle, name, value)
}

func (cv *clientVolume) GetStreamInP2pUrl(ctx context.Context, path string) (string, error) {
	return cv.bcClient.getStreamInP2pUrl(ctx, cv.logger, cv.handle, path)
}

func (cv *clientVolume) StreamP2pOut(ctx context.Context, path, url string, encoding baggageclaim.Encoding) error {
	return cv.bcClient.streamP2pOut(ctx, cv.logger, cv.handle, encoding, path, url)
}
