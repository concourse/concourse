package executehelpers

import (
	"bufio"
	"bytes"
	"io"
	"os/exec"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/concourse/go-archive/tarfs"
	"github.com/klauspost/compress/zstd"
	"github.com/vbauerster/mpb/v4"
)

func Upload(bar *mpb.Bar, team concourse.Team, path string, includeIgnored bool, platform string) (atc.WorkerArtifact, error) {
	files := getFiles(path, includeIgnored)

	archiveStream, archiveWriter := io.Pipe()

	zstdWriter, err := zstd.NewWriter(archiveWriter)
	if err != nil {
		return atc.WorkerArtifact{}, err
	}

	go func() {
		err = tarfs.Compress(zstdWriter, path, files...)
		if err != nil {
			_ = zstdWriter.Close()
			archiveWriter.CloseWithError(err)
			return
		}

		archiveWriter.CloseWithError(zstdWriter.Close())
	}()

	return team.CreateArtifact(bar.ProxyReader(archiveStream), platform)
}

func getFiles(dir string, includeIgnored bool) []string {
	var files []string
	var err error

	if includeIgnored {
		files = []string{"."}
	} else {
		files, err = getGitFiles(dir)
		if err != nil {
			files = []string{"."}
		}
	}

	return files
}

func getGitFiles(dir string) ([]string, error) {
	tracked, err := gitLS(dir)
	if err != nil {
		return nil, err
	}

	deleted, err := gitLS(dir, "--deleted")
	if err != nil {
		return nil, err
	}

	existingFiles := difference(tracked, deleted)

	untracked, err := gitLS(dir, "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}

	return append(existingFiles, untracked...), nil
}

func gitLS(dir string, flags ...string) ([]string, error) {
	files := []string{}

	gitLS := exec.Command("git", append([]string{"ls-files", "-z"}, flags...)...)
	gitLS.Dir = dir

	gitOut, err := gitLS.StdoutPipe()
	if err != nil {
		return nil, err
	}

	outScan := bufio.NewScanner(gitOut)
	outScan.Split(scanNull)

	err = gitLS.Start()
	if err != nil {
		return nil, err
	}

	for outScan.Scan() {
		files = append(files, outScan.Text())
	}

	err = gitLS.Wait()
	if err != nil {
		return nil, err
	}

	return files, nil
}

func scanNull(data []byte, atEOF bool) (int, []byte, error) {
	// eof, no more data; terminate
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// look for terminating null byte
	if i := bytes.IndexByte(data, 0); i >= 0 {
		return i + 1, data[0:i], nil
	}

	// no final terminator; return what's left
	if atEOF {
		return len(data), data, nil
	}

	// request more data
	return 0, nil, nil
}

func difference(a, b []string) []string {
	mb := map[string]bool{}
	for _, x := range b {
		mb[x] = true
	}
	ab := []string{}
	for _, x := range a {
		if _, ok := mb[x]; !ok {
			ab = append(ab, x)
		}
	}
	return ab
}
