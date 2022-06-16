package kubernetes

import (
	"github.com/ckotzbauer/libk8soci/pkg/oci"
	corev1 "k8s.io/api/core/v1"
)

type KubeImage struct {
	Image oci.RegistryImage
	Pods  []corev1.Pod
}
