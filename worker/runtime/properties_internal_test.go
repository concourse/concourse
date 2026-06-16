//go:build linux

package runtime

import (
	"strings"
	"testing"
	"unicode/utf8"

	"code.cloudfoundry.org/garden"
	"github.com/stretchr/testify/require"
)

// Chunking a long value must split on rune boundaries, else a multi-byte rune
// is bisected and the chunk is invalid UTF-8, which containerd rejects.
func TestPropertiesToLabels_DoesNotSplitMultiByteRunes(t *testing.T) {
	// 9000 bytes of em dashes (3 bytes each): over the 4096-byte label limit,
	// and 4096 isn't a multiple of 3 so a byte-wise split must bisect a rune.
	value := strings.Repeat("—", 3000)

	labels, err := propertiesToLabels(garden.Properties{"prop": value})
	require.NoError(t, err)
	require.Greater(t, len(labels), 1, "value should have been split into multiple chunks")

	for key, chunk := range labels {
		require.True(t, utf8.ValidString(chunk),
			"chunk %q is not valid UTF-8 - a multi-byte rune was split across the boundary", key)
	}

	// The chunks must still reassemble to the exact original value.
	require.Equal(t,
		garden.Properties{"prop": value},
		labelsToProperties(labels),
	)
}
