package gardenruntimetest

import (
	"context"
	"testing"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc/runtime/runtimetest"
	"github.com/stretchr/testify/require"
)

func TestVolume_P2PStreamInOut_Root(t *testing.T) {
	content := runtimetest.VolumeContent{
		"file1":        {Data: []byte("file 1 content")},
		"file2":        {Data: []byte("file 2 content")},
		"folder/file3": {Data: []byte("file 3 content")},
	}
	volume1 := NewVolume("volume1").WithContent(content)
	volume2 := NewVolume("volume2")

	ctx := context.Background()

	url, err := volume2.GetStreamInP2pUrl(ctx, ".")
	require.NoError(t, err)

	err = volume1.StreamP2pOut(ctx, ".", url, baggageclaim.GzipEncoding)
	require.NoError(t, err)

	require.Equal(t, content, volume2.Content)
}
