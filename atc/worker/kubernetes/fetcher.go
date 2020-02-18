package kubernetes

import (
	"context"
	"fmt"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
)

func (k Kubernetes) fetchImageForContainer(
	containerSpec worker.ContainerSpec,
	worker db.Worker,
	container db.CreatingContainer,
) (imageUri string, err error) {
	spec := containerSpec.ImageSpec

	switch {
	case spec.ResourceType != "":
		// TODO handle custom resource types
		//
		imageUri, err = resourceTypeURI(spec.ResourceType, worker)
		if err != nil {
			return "", fmt.Errorf("resource type to uri: %w", err)
		}
	case spec.ImageURL != "": // rootfs_uri
		imageUri = spec.ImageURL
	case spec.ImageArtifact != nil:
		imageUri, err = imageArtifact(spec.ImageArtifact, worker)
		if err != nil {
			return "", fmt.Errorf("image artifact: %w", err)
		}
	case spec.ImageResource != nil:
		imageUri, err = k.imageResource(spec.ImageResource, worker, container, containerSpec.TeamID)
		if err != nil {
			return "", fmt.Errorf("image resource: %w", err)
		}
	case spec.ImageArtifactSource != nil:
		return "", fmt.Errorf("image artifact source not implemented")
	default:
		return "", fmt.Errorf("malformed imagespec")

	}

	return imageUri, nil
}

// TODO get resource factory w/ us
//
func (k Kubernetes) version() (err error) {
	// resourceType, found := i.customTypes.Lookup(i.imageResource.Type)
	// if found && resourceType.Version == nil {
	// 	err := i.ensureVersionOfType(ctx, logger, container, resourceType)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }

	// resourceSpec := worker.ContainerSpec{
	// 	ImageSpec: worker.ImageSpec{
	// 		ResourceType: i.imageResource.Type,
	// 	},
	// 	TeamID: i.teamID,
	// 	BindMounts: []worker.BindMountSource{
	// 		&worker.CertsVolumeMount{Logger: logger},
	// 	},
	// }

	// owner := db.NewImageCheckContainerOwner(container, i.teamID)

	// imageContainer, err := i.worker.FindOrCreateContainer(
	// 	ctx,
	// 	logger,
	// 	i.imageFetchingDelegate,
	// 	owner,
	// 	db.ContainerMetadata{
	// 		Type: db.ContainerTypeCheck,
	// 	},
	// 	resourceSpec,
	// 	i.customTypes,
	// )
	// if err != nil {
	// 	return nil, err
	// }

	// processSpec := runtime.ProcessSpec{
	// 	Path: "/opt/resource/check",
	// }
	// checkingResource := i.resourceFactory.NewResource(i.imageResource.Source, nil, i.imageResource.Version)
	// versions, err := checkingResource.Check(context.TODO(), processSpec, imageContainer)
	// if err != nil {
	// 	return nil, err
	// }

	// if len(versions) == 0 {
	// 	return nil, ErrImageUnavailable
	// }

	// return versions[0], nil
	return
}

func (k Kubernetes) imageResource(
	imageResource *worker.ImageResource,
	w db.Worker,
	container db.CreatingContainer,
	teamID int,
) (imageUri string, err error) {
	owner := db.NewImageGetContainerOwner(container, teamID)

	containerMetadata := db.ContainerMetadata{
		Type: db.ContainerTypeGet,
	}

	processSpec := runtime.ProcessSpec{
		Path: "/opt/resource/in",
		Args: []string{resource.ResourcesDir("get")},
		// StdoutWriter: step.delegate.Stdout(),	// TODO handle
		// StderrWriter: step.delegate.Stderr(),
	}

	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: imageResource.Type,
		},
		TeamID: teamID,
		Outputs: map[string]string{
			"resource": processSpec.Args[0],
		},
	}

	pod, err := k.findOrCreateContainer(
		owner,
		containerMetadata,
		containerSpec,
	)
	if err != nil {
		err = fmt.Errorf("find or create container: %w", err)
		return
	}

	res := k.rf.NewResource(
		imageResource.Source,
		imageResource.Params,
		map[string]string{
			"digest": "sha256:bc025862c3e8ec4a8754ea4756e33da6c41cba38330d7e324abd25c8e0b93300",
		}, // TODO version,
	)

	// TODO check result lol
	//
	_, err = res.Get(context.Background(), processSpec, pod)
	if err != nil {
		err = fmt.Errorf("get: %w", err)
		return
	}

	imageUri = pod.IP() + ":7788/concourse/resource"
	return
}

func imageArtifact(
	artifact runtime.Artifact,
	worker db.Worker,
) (imageUri string, err error) {
	artf := UnmarshalPodArtifact(artifact.ID())
	imageUri = artf.Ip + ":7788/concourse/" + artf.Handle
	return
}

func resourceTypeURI(resourceType string, worker db.Worker) (uri string, err error) {
	for _, wrt := range worker.ResourceTypes() {
		if wrt.Type == resourceType {
			uri = wrt.Image
			return
		}
	}

	err = fmt.Errorf("res type '%s' not found", resourceType)
	return
}
