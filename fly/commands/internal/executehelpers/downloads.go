package executehelpers

import (
	"github.com/klauspost/compress/zstd"
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/concourse/go-archive/tarfs"
	"github.com/vbauerster/mpb/v4"
)

func Download(bar *mpb.Bar, team concourse.Team, artifactID int, path string) error {
	out, err := team.GetArtifact(artifactID)
	if err != nil {
		return err
	}

	defer out.Close()

	zstdReader, err := zstd.NewReader(bar.ProxyReader(out))
	if err != nil {
		return err
	}

	return tarfs.Extract(zstdReader, path)
}
