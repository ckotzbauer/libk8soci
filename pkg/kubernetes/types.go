package kubernetes

import (
	"github.com/ckotzbauer/libk8soci/pkg/oci"
	corev1 "k8s.io/api/core/v1"
)

type ContainerInfo struct {
	Image oci.RegistryImage
	Name  string
}

type PodInfo struct {
	Containers      []ContainerInfo
	PodName         string
	PodNamespace    string
	Annotations     map[string]string
	PullSecretNames []corev1.LocalObjectReference
}
