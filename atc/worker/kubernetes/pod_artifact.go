package kubernetes

import "strings"

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
	parts := strings.SplitN(str, ":", 3)
	a.Pod, a.Handle, a.Ip = parts[0], parts[1], parts[2]
	return
}
