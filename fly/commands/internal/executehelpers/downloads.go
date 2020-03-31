package executehelpers

import (
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/concourse/go-archive/tgzfs"
	"github.com/vbauerster/mpb/v4"
)

func Download(bar *mpb.Bar, team concourse.Team, artifactID int, path string) error {
	out, err := team.GetArtifact(artifactID)
	if err != nil {
		return err
	}

	defer out.Close()

	return tgzfs.Extract(bar.ProxyReader(out), path)
}
