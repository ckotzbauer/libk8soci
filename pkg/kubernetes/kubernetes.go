package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"github.com/ckotzbauer/libk8soci/pkg/oci"
	"github.com/ckotzbauer/libk8soci/pkg/util"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type KubeClient struct {
	Client *kubernetes.Clientset
}

func NewClient() *KubeClient {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		logrus.WithError(err).Fatal("kubeconfig file could not be found!")
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		logrus.WithError(err).Fatal("Could not create Kubernetes client from config!")
	}

	return &KubeClient{Client: client}
}

func prepareLabelSelector(selector string) meta.ListOptions {
	listOptions := meta.ListOptions{}

	if len(selector) > 0 {
		listOptions.LabelSelector = util.Unescape(selector)
		logrus.Tracef("Applied labelSelector %v", listOptions.LabelSelector)
	}

	return listOptions
}

func (client *KubeClient) ListNamespaces(labelSelector string) ([]corev1.Namespace, error) {
	list, err := client.Client.CoreV1().Namespaces().List(context.Background(), prepareLabelSelector(labelSelector))

	if err != nil {
		return []corev1.Namespace{}, fmt.Errorf("failed to list namespaces: %w", err)
	}

	return list.Items, nil
}

func (client *KubeClient) listPods(namespace, labelSelector string) ([]corev1.Pod, error) {
	list, err := client.Client.CoreV1().Pods(namespace).List(context.Background(), prepareLabelSelector(labelSelector))

	if err != nil {
		return []corev1.Pod{}, fmt.Errorf("failed to list pods: %w", err)
	}

	return list.Items, nil
}

func (client *KubeClient) LoadImageInfos(namespaces []corev1.Namespace, podLabelSelector string) map[string]KubeImage {
	images := map[string]KubeImage{}

	for _, ns := range namespaces {
		pods, err := client.listPods(ns.Name, podLabelSelector)
		if err != nil {
			logrus.WithError(err).Errorf("failed to list pods for namespace: %s", ns.Name)
			continue
		}

		for _, pod := range pods {
			allImageCreds := []oci.KubeCreds{}

			statuses := []corev1.ContainerStatus{}
			statuses = append(statuses, pod.Status.ContainerStatuses...)
			statuses = append(statuses, pod.Status.InitContainerStatuses...)
			statuses = append(statuses, pod.Status.EphemeralContainerStatuses...)

			allImageCreds = client.LoadSecrets(pod.Namespace, pod.Spec.ImagePullSecrets)

			for _, c := range statuses {
				if c.ImageID != "" {
					imageIDSlice := strings.Split(c.ImageID, "://")
					trimmedImageID := imageIDSlice[len(imageIDSlice)-1]
					img, ok := images[trimmedImageID]
					if !ok {
						img = KubeImage{
							Image: oci.RegistryImage{Image: c.Image, ImageID: trimmedImageID, PullSecrets: allImageCreds},
							Pods:  []corev1.Pod{},
						}
					}

					img.Pods = append(img.Pods, pod)
					images[trimmedImageID] = img
				}
			}
		}
	}

	return images
}

func (client *KubeClient) LoadSecrets(namespace string, secrets []corev1.LocalObjectReference) []oci.KubeCreds {
	allImageCreds := []oci.KubeCreds{}

	for _, s := range secrets {
		secret, err := client.Client.CoreV1().Secrets(namespace).Get(context.Background(), s.Name, meta.GetOptions{})
		if err != nil {
			logrus.WithError(err).Errorf("Could not load secret: %s/%s", namespace, s.Name)
			continue
		}

		var creds []byte
		legacy := false
		name := secret.Name

		if secret.Type == corev1.SecretTypeDockerConfigJson {
			creds = secret.Data[corev1.DockerConfigJsonKey]
		} else if secret.Type == corev1.SecretTypeDockercfg {
			creds = secret.Data[corev1.DockerConfigKey]
			legacy = true
		} else {
			logrus.WithError(err).Errorf("invalid secret-type %s for pullSecret %s/%s", secret.Type, secret.Namespace, secret.Name)
		}

		if len(creds) > 0 {
			allImageCreds = append(allImageCreds, oci.KubeCreds{SecretName: name, SecretCredsData: creds, IsLegacySecret: legacy})
		}
	}

	return allImageCreds
}
