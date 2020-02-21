package backend

import (
	"fmt"
	"path/filepath"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	baggageclaimContainerName = "baggageclaim"
	baggageclaimImage         = "cirocosta/baggageclaim@sha256:00a4ab8137388073723047811d6149be12035435eff9e5bb47684a0a2a3cefdc"
	baggageclaimVolumeName    = "baggageclaim"
	baggageclaimVolumesPath   = "/vols"

	beltloaderContainerName = "beltloader"
	beltloaderImage         = baggageclaimImage

	initCommand    = "/init"
	initVolumeName = "init"
	initMountPath  = "/init"
	initSubPath    = "init"

	MainContainerName = "main"

	// TODO labels for ... build, pipeline, team, get, etc  info
	LabelHandleKey                         = "org.concourse-ci.handle"
	LabelConcourseKey, LabelConcourseValue = "org.concourse-ci", "yes"
	Subdomain                              = "container"
)

type PodMutationOpt func(*apiv1.Pod)
type ContainerMutationOpt func(*apiv1.Container)

// Pod converts the spec to a pod.
//
func Pod(opts ...PodMutationOpt) (pod *apiv1.Pod) {
	pod = new(apiv1.Pod)

	for _, opt := range opts {
		opt(pod)
	}

	return
}

// WithBase
//
func WithBase(handle string) PodMutationOpt {
	return func(pod *apiv1.Pod) {
		meta := metav1.ObjectMeta{
			Name: handle,
			Labels: map[string]string{
				LabelHandleKey:    handle,
				LabelConcourseKey: LabelConcourseValue,
			},
		}

		spec := apiv1.PodSpec{
			Hostname:  handle,
			Subdomain: Subdomain,
		}

		pod.ObjectMeta = meta
		pod.Spec = spec
	}
}

// WithInputs
//                         name   dest
func WithInputs(inputs map[string]string) ContainerMutationOpt {
	// the signature and effect is pretty much the same
	return WithOutputs(inputs)
}

// WithOutputs
//                           name   dest
func WithOutputs(outputs map[string]string) ContainerMutationOpt {
	return func(container *apiv1.Container) {
		mounts := make([]apiv1.VolumeMount, 0, len(outputs))

		for name, dest := range outputs {
			mounts = append(mounts,
				apiv1.VolumeMount{
					Name:      baggageclaimVolumeName,
					MountPath: dest,
					SubPath:   filepath.Join("live", name, "volume"),
				},
			)
		}

		container.VolumeMounts = append(
			container.VolumeMounts, mounts...,
		)
	}
}

func WithDir(dir string) ContainerMutationOpt {
	return func(container *apiv1.Container) {
		container.WorkingDir = dir
	}
}

func WithEnv(env []string) ContainerMutationOpt {
	return func(container *apiv1.Container) {
		for _, rawEnvVar := range env {
			parts := strings.SplitN(rawEnvVar, "=", 2)
			k, v := parts[0], parts[1]

			container.Env = append(container.Env, apiv1.EnvVar{
				Name: k, Value: v,
			})
		}
	}
}

// WithInputsFetcher
//
// ps.: no-op  it `inputs` is empty.
//
//                                uri    dst
func WithInputsFetcher(inputs map[string]string, opts ...ContainerMutationOpt) PodMutationOpt {
	return func(pod *apiv1.Pod) {
		if len(inputs) == 0 {
			return
		}

		command := []string{
			"beltloader",
		}

		for src, dst := range inputs {
			command = append(command,
				fmt.Sprintf("--uri=src=%s,dst=%s", src, dst),
			)
		}

		container := apiv1.Container{
			Name:    beltloaderContainerName,
			Image:   beltloaderImage,
			Command: command,
		}

		for _, opt := range opts {
			opt(&container)
		}

		pod.Spec.InitContainers = append(
			pod.Spec.InitContainers,
			container,
		)
	}
}

// WithBaggageclaim
//
func WithBaggageclaim(opts ...ContainerMutationOpt) PodMutationOpt {
	return func(pod *apiv1.Pod) {
		container := apiv1.Container{
			Name:  baggageclaimContainerName,
			Image: baggageclaimImage,
			Command: []string{
				"baggageclaim",
				"--volumes=" + baggageclaimVolumesPath,
				"--driver=naive",
				"--bind-ip=0.0.0.0",
			},
		}

		for _, opt := range opts {
			opt(&container)
		}

		WithBaggageclaimVolumeMount()(&container)

		pod.Spec.Containers = append(
			pod.Spec.Containers, container,
		)

		WithBaggageclaimVolume()(pod)
	}
}

func WithBaggageclaimVolumeMount() ContainerMutationOpt {
	return func(container *apiv1.Container) {
		mount := apiv1.VolumeMount{
			Name:      baggageclaimVolumeName,
			MountPath: baggageclaimVolumesPath,
		}

		container.VolumeMounts = append(
			container.VolumeMounts, mount,
		)
	}
}

// WithBaggageclaimVolume
//
func WithBaggageclaimVolume() PodMutationOpt {
	return func(pod *apiv1.Pod) {
		volume := apiv1.Volume{
			Name: baggageclaimVolumeName,
			VolumeSource: apiv1.VolumeSource{
				EmptyDir: &apiv1.EmptyDirVolumeSource{},
			},
		}

		pod.Spec.Volumes = append(
			pod.Spec.Volumes, volume,
		)
	}
}

// WithMain adds the `main` container to the pod.
//
// The main container is what brings the desired functionality, e.g., in a `get`
// step, it's what's based of the resource type, and what ultimately executes
// `/opt/resource/in` from that image.
//
// ps.: the `main` container brings its own volume that carries its `init`
//      executable.
//
func WithMain(image string, opts ...ContainerMutationOpt) PodMutationOpt {
	return func(pod *apiv1.Pod) {
		container := apiv1.Container{
			Name:    MainContainerName,
			Image:   image,
			Command: []string{initCommand},
		}

		withInitVolumeMount()(&container)

		for _, opt := range opts {
			opt(&container)
		}

		pod.Spec.Containers = append(
			pod.Spec.Containers, container,
		)

		withInitVolume()(pod)
	}
}

func withInitVolumeMount() ContainerMutationOpt {
	return func(container *apiv1.Container) {
		mount := apiv1.VolumeMount{
			Name:      initVolumeName,
			MountPath: initMountPath,
			SubPath:   initSubPath,
		}

		container.VolumeMounts = append(
			container.VolumeMounts, mount,
		)
	}

}

func withInitVolume() PodMutationOpt {
	return func(pod *apiv1.Pod) {
		volume := apiv1.Volume{
			Name: initVolumeName,
			VolumeSource: apiv1.VolumeSource{
				ConfigMap: &apiv1.ConfigMapVolumeSource{
					LocalObjectReference: apiv1.LocalObjectReference{initSubPath},
					DefaultMode:          int32Ref(0777),
				},
			},
		}

		pod.Spec.Volumes = append(
			pod.Spec.Volumes, volume,
		)
	}
}

func int32Ref(i int32) *int32 {
	return &i
}

func int64Ref(i int64) *int64 {
	return &i
}
