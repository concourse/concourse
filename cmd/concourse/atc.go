package main

import (
	"fmt"
	"os/exec"
	"os/user"
	"path/filepath"
	"syscall"

	"github.com/vito/concourse-bin/bindata"
)

func (cmd *ConcourseCommand) atc() (*exec.Cmd, error) {
	err := bindata.RestoreAssets(cmd.WorkDir, "atc")
	if err != nil {
		return nil, err
	}

	u, err := user.Lookup(cmd.User)
	if err != nil {
		return nil, err
	}

	atcBin := filepath.Join(cmd.WorkDir, "atc", "atc")

	args := []string{
		"--peer-url", "http://" + cmd.PeerIP + ":8080",
		"--worker-garden-url", "http://" + cmd.PeerIP + ":7777",
	}

	for _, t := range []string{"time", "git", "docker-image", "archive"} {
		args = append(args, "--worker-resource", t+":docker:///concourse/"+t+"-resource")
	}

	args = append(args, cmd.ATCArgs...)

	atc := exec.Command(atcBin, args...)

	cred := &syscall.Credential{}

	_, err = fmt.Sscanf(u.Uid, "%d", &cred.Uid)
	if err != nil {
		return nil, err
	}

	_, err = fmt.Sscanf(u.Gid, "%d", &cred.Gid)
	if err != nil {
		return nil, err
	}

	atc.SysProcAttr = &syscall.SysProcAttr{
		Credential: cred,
	}

	return atc, nil
}
