package kubernetes

import (
	"strings"

	log "github.com/sirupsen/logrus"
)

// PodArtifact represents an artifact that can be retrieved from a pod.
//
type PodArtifact struct {
	Pod    string
	Ip     string
	Handle string
}

func (a PodArtifact) String() string {
	return strings.Join([]string{
		a.Pod,
		a.Handle,
		a.Ip,
	}, ":")
}

func UnmarshalPodArtifact(str string) (a PodArtifact) {
	sess := log.WithFields(log.Fields{
		"str": str,
	})

	sess.Info("unmarshal")

	parts := strings.SplitN(str, ":", 3)
	a.Pod, a.Handle, a.Ip = parts[0], parts[1], parts[2]
	return
}
