// backend is responsible for interacting with a kubernetes cluster as a
// "container" (from Concourse point of view) provider.
//
package backend

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/garden"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/staging/src/k8s.io/client-go/tools/clientcmd"
)

type Backend struct {
	ns  string
	cfg *rest.Config
	cs  *kubernetes.Clientset
}

func New(namespace string) (backend *Backend, err error) {
	backend = &Backend{
		ns:  namespace,
		cfg: config,
	}

	switch {
	case config != "":
		cfg, err = clientcmd.BuildConfigFromFlags("", config)
		if err != nil {
			return
		}
	case inCluster:
		cfg, err = rest.InClusterConfig()
		if err != nil {
			err = fmt.Errorf("incluster cfg: %w", err)
			return
		}
	default:
		err = fmt.Errorf("incluster or config must be specified")
		return
	}

	backend.cs, err = kubernetes.NewForConfig(config)
	if err != nil {
		err = fmt.Errorf("k8s new for config: %w", err)
		return
	}

	return
}

// Containers lists pods matching a specific set of labels.
//
func (b *Backend) Containers(kvs map[string]string) (containers []*Container, err error) {
	podList, err := b.cs.CoreV1().Pods(b.ns).List(metav1.ListOptions{
		LabelSelector: labels.Set(kvs).String(),
	})
	if err != nil {
		err = fmt.Errorf("fetching pods: %w", err)
		return
	}

	pods := podList.Items

	for _, pod := range pods {
		handle, found := pod.Labels[LabelHandleKey]
		if !found {
			err = fmt.Errorf("found pod without concourse's handle label - that's weird")
			return
		}

		containers = append(containers, NewContainer(
			b.ns, handle, pod.Status.PodIP, b.cs, b.cfg,
		))
	}

	return
}

// Lookup returns the container with the specified handle.
//
func (b *Backend) Lookup(handle string) (container *Container, err error) {
	pod, err := b.cs.CoreV1().Pods(b.ns).Get(handle, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			err = fmt.Errorf("fetching pod: %w", err)
			return
		}

		err = garden.ContainerNotFoundError{handle}
		return
	}

	// TODO - perhaps wait for the pod to be ready, right here?
	// 	  this way we block (for a max period) until it's ready ... ?

	container = NewContainer(b.ns, handle, pod.Status.PodIP, b.cs, b.cfg)
	return
}

// Destroy gracefully terminates a container named "handle" in the configured
// namespace.
//
func (b *Backend) Destroy(handle string) (err error) {
	err = b.cs.CoreV1().Pods(b.ns).Delete(handle, &metav1.DeleteOptions{
		GracePeriodSeconds: int64Ref(10),
	})
	if err != nil {
		err = fmt.Errorf("delete: %w", err)
		return
	}

	return
}

// Creates a pod
//
func (b *Backend) Create(handle string, podDefinition *apiv1.Pod) (container *Container, err error) {
	_, err = b.cs.CoreV1().Pods(b.ns).Create(podDefinition)
	if err != nil {
		err = fmt.Errorf("pod creation: %w", err)
		return
	}

	err = b.waitForPod(context.TODO(), handle)

	// stream those things in?
	if err != nil {
		err = fmt.Errorf("wait for pod: %w", err)
		return
	}

	return b.Lookup(handle)
}

func (b *Backend) waitForPod(ctx context.Context, handle string) (err error) {
	watch, err := b.cs.CoreV1().Pods(b.ns).Watch(metav1.ListOptions{
		LabelSelector: LabelHandleKey + "=" + handle,
	})
	if err != nil {
		err = fmt.Errorf("pods watch: %w", err)
		return
	}

	statusC := make(chan struct{})

	go func() {
		for event := range watch.ResultChan() {
			p, ok := event.Object.(*apiv1.Pod)
			if !ok {
				// TODO show err
				return
			}

			if p.Status.Phase != apiv1.PodRunning {
				continue
			}

			close(statusC)
			return
		}
	}()

	// TODO re-sync on an interval (just because)

	select {
	case <-statusC:
	case <-ctx.Done():
		watch.Stop()
		err = ctx.Err()
	}

	return
}
