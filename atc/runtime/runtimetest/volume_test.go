package runtimetest

import (
	"context"
	"testing"

	"github.com/concourse/concourse/atc/compression"
	"github.com/stretchr/testify/require"
)

var gzipCompression = compression.NewGzipCompression()

func TestVolume_StreamInOut_Root(t *testing.T) {
	content := VolumeContent{
		"file1":        {Data: []byte("file 1 content")},
		"file2":        {Data: []byte("file 2 content")},
		"folder/file3": {Data: []byte("file 3 content")},
	}
	volume1 := NewVolume("volume1").WithContent(content)
	volume2 := NewVolume("volume2")

	ctx := context.Background()
	stream, err := volume1.StreamOut(ctx, ".", gzipCompression)
	require.NoError(t, err)

	err = volume2.StreamIn(ctx, ".", gzipCompression, stream)
	require.NoError(t, err)

	require.Equal(t, content, volume2.Content)
}

func TestVolume_StreamInOut_File(t *testing.T) {
	content := VolumeContent{
		"file1": {Data: []byte("file 1 content")},
	}
	volume1 := NewVolume("volume1").WithContent(content)
	volume2 := NewVolume("volume2")

	ctx := context.Background()
	stream, err := volume1.StreamOut(ctx, "file1", gzipCompression)
	require.NoError(t, err)

	err = volume2.StreamIn(ctx, ".", gzipCompression, stream)
	require.NoError(t, err)

	require.Equal(t, content, volume2.Content)
}
