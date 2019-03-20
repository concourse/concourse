package executehelpers

import (
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/concourse/go-archive/tgzfs"
	"github.com/vbauerster/mpb/v4"
	"github.com/vbauerster/mpb/v4/decor"
)

func Download(team concourse.Team, artifactID int, path string) error {
	progress := mpb.New()

	bar := progress.AddBar(
		0,
		mpb.PrependDecorators(decor.Name("downloading to "+path)),
		mpb.AppendDecorators(decor.CountersKibiByte("%.1f")),
	)

	defer func() {
		bar.SetTotal(bar.Current(), true)
		progress.Wait()
	}()

	out, err := team.GetArtifact(artifactID)
	if err != nil {
		return err
	}

	defer out.Close()

	return tgzfs.Extract(bar.ProxyReader(out), path)
}
