package executehelpers

import (
	"os"

	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/concourse/go-archive/tgzfs"
)

func Download(team concourse.Team, artifactID int, path string) error {
	pb := progress(path+":", os.Stdout)

	pb.Start()
	defer pb.Finish()

	out, err := team.GetArtifact(artifactID)
	if err != nil {
		return err
	}

	defer out.Close()

	return tgzfs.Extract(pb.NewProxyReader(out), path)
}
