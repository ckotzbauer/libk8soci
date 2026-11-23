package kubernetes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	redisDigest  = "sha256:02419de7eddf55aa5bcf49efb74e88fa8d931b4d77c07eff8a6b2144472b6952"
	redisRef     = "docker.io/library/redis:7.4.2-alpine@" + redisDigest
	redisImageID = "docker.io/library/redis@" + redisDigest
	redisLocalID = "sha256:dc11e3ccde3c4defde0194f58c3ec2cf270ed7401560d82d8bd548284b82dfba"

	newDigest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
)

func TestExtractPodInfosImageFromSpec(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: meta.ObjectMeta{Name: "app", Namespace: "default"},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{Name: "init", Image: "busybox:1.37"}},
			Containers: []corev1.Container{
				{Name: "redis", Image: redisRef},
				{Name: "nginx", Image: "nginx:1.28.0-alpine"},
				// Patched in place to a new tag@digest that isn't running yet (pull pending).
				{Name: "stale", Image: "docker.io/library/redis:7.4.3-alpine@" + newDigest},
				{Name: "memcached", Image: "memcached@sha256:eeee"},
			},
			EphemeralContainers: []corev1.EphemeralContainer{
				{EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "debug", Image: "alpine:3.22"}},
			},
		},
		Status: corev1.PodStatus{
			InitContainerStatuses: []corev1.ContainerStatus{
				{Name: "init", Image: "docker.io/library/busybox:1.37", ImageID: "docker.io/library/busybox@sha256:aaaa"},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				// containerd reports the local image ID for images pulled by digest.
				{Name: "redis", Image: redisLocalID, ImageID: redisImageID},
				{Name: "nginx", Image: "docker.io/library/nginx:1.28.0-alpine", ImageID: "docker-pullable://docker.io/library/nginx@sha256:bbbb"},
				// Still the old container: the spec's digest differs from the running one.
				{Name: "stale", Image: redisLocalID, ImageID: redisImageID},
				{Name: "memcached", Image: "sha256:5d47", ImageID: "docker.io/library/memcached@sha256:eeee"},
				// Not in the spec: falls back to the status image.
				{Name: "orphan", Image: "docker.io/library/orphan:1", ImageID: "docker.io/library/orphan@sha256:cccc"},
				// No image ID yet: skipped, as before.
				{Name: "pending", Image: "docker.io/library/pending:1"},
			},
			EphemeralContainerStatuses: []corev1.ContainerStatus{
				{Name: "debug", Image: "docker.io/library/alpine:3.22", ImageID: "docker.io/library/alpine@sha256:dddd"},
			},
		},
	}

	info := (&KubeClient{}).ExtractPodInfos(pod)

	got := map[string][2]string{}
	for _, c := range info.Containers {
		got[c.Name] = [2]string{c.Image.Image, c.Image.ImageID}
	}

	assert.Equal(t, map[string][2]string{
		// The spec's digest is the running one: the spec reference, with its tag.
		"redis":     {redisRef, redisImageID},
		"memcached": {"memcached@sha256:eeee", "docker.io/library/memcached@sha256:eeee"},
		// The spec's digest isn't running: the status image.
		"stale": {redisLocalID, redisImageID},
		// Tag only: a tag can't be checked against the running digest, so the status image as before.
		"nginx":  {"docker.io/library/nginx:1.28.0-alpine", "docker.io/library/nginx@sha256:bbbb"},
		"orphan": {"docker.io/library/orphan:1", "docker.io/library/orphan@sha256:cccc"},
		"init":   {"docker.io/library/busybox:1.37", "docker.io/library/busybox@sha256:aaaa"},
		"debug":  {"docker.io/library/alpine:3.22", "docker.io/library/alpine@sha256:dddd"},
	}, got)
}
