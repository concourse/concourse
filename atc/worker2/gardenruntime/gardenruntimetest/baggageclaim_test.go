package gardenruntimetest

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/concourse/baggageclaim"
	"github.com/stretchr/testify/require"
)

func TestVolume_StreamInOut_Root(t *testing.T) {
	content := fstest.MapFS{
		"file1":        {Data: []byte("file 1 content")},
		"file2":        {Data: []byte("file 2 content")},
		"folder/file3": {Data: []byte("file 3 content")},
	}
	volume1 := NewVolume("volume1").WithContent(content)
	volume2 := NewVolume("volume2")

	ctx := context.Background()
	stream, err := volume1.StreamOut(ctx, ".", baggageclaim.GzipEncoding)
	require.NoError(t, err)

	err = volume2.StreamIn(ctx, ".", baggageclaim.GzipEncoding, stream)
	require.NoError(t, err)

	require.Equal(t, content, volume2.Content)
}

func TestVolume_StreamInOut_File(t *testing.T) {
	content := fstest.MapFS{
		"file1": {Data: []byte("file 1 content")},
	}
	volume1 := NewVolume("volume1").WithContent(content)
	volume2 := NewVolume("volume2")

	ctx := context.Background()
	stream, err := volume1.StreamOut(ctx, "file1", baggageclaim.GzipEncoding)
	require.NoError(t, err)

	err = volume2.StreamIn(ctx, ".", baggageclaim.GzipEncoding, stream)
	require.NoError(t, err)

	require.Equal(t, content, volume2.Content)
}
