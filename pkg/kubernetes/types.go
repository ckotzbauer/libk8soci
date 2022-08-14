package kubernetes

import (
	"github.com/ckotzbauer/libk8soci/pkg/oci"
)

type ContainerInfo struct {
	Image oci.RegistryImage
	Name  string
}

type PodInfo struct {
	Containers   []ContainerInfo
	PodName      string
	PodNamespace string
	Annotations  map[string]string
}
