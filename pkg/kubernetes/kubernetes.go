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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

type KubeClient struct {
	Client *kubernetes.Clientset
}

func DefaultConfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	return config, nil
}

func NewClient() *KubeClient {
	config, err := DefaultConfig()
	if err != nil {
		logrus.WithError(err).Fatal("kubeconfig file could not be found!")
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		logrus.WithError(err).Fatal("Could not create Kubernetes client from config!")
	}

	return &KubeClient{Client: client}
}

func prepareLabelSelector(listOptions meta.ListOptions, selector string) meta.ListOptions {
	if len(selector) > 0 {
		listOptions.LabelSelector = util.Unescape(selector)
		logrus.Tracef("Applied labelSelector %v", listOptions.LabelSelector)
	}

	return listOptions
}

func (client *KubeClient) ListNamespaces(labelSelector string) ([]corev1.Namespace, error) {
	list, err := client.Client.CoreV1().Namespaces().List(context.Background(), prepareLabelSelector(meta.ListOptions{}, labelSelector))

	if err != nil {
		return []corev1.Namespace{}, fmt.Errorf("failed to list namespaces: %w", err)
	}

	return list.Items, nil
}

func (client *KubeClient) listPods(namespace, labelSelector string) ([]corev1.Pod, error) {
	list, err := client.Client.CoreV1().Pods(namespace).List(context.Background(), prepareLabelSelector(meta.ListOptions{}, labelSelector))

	if err != nil {
		return []corev1.Pod{}, fmt.Errorf("failed to list pods: %w", err)
	}

	return list.Items, nil
}

func (client *KubeClient) LoadPodInfos(namespaces []corev1.Namespace, podLabelSelector string) []PodInfo {
	podInfos := []PodInfo{}

	for _, ns := range namespaces {
		pods, err := client.listPods(ns.Name, podLabelSelector)
		if err != nil {
			logrus.WithError(err).Errorf("failed to list pods for namespace: %s", ns.Name)
			continue
		}

		for _, pod := range pods {
			podInfos = append(podInfos, PodInfo{
				Containers:   client.ExtractContainerInfos(pod),
				PodName:      pod.Name,
				PodNamespace: pod.Namespace,
				Annotations:  pod.Annotations,
			})
		}
	}

	return podInfos
}

func (client *KubeClient) ExtractContainerInfos(pod corev1.Pod) []ContainerInfo {
	statuses := []corev1.ContainerStatus{}
	statuses = append(statuses, pod.Status.ContainerStatuses...)
	statuses = append(statuses, pod.Status.InitContainerStatuses...)
	statuses = append(statuses, pod.Status.EphemeralContainerStatuses...)

	allImageCreds := client.LoadSecrets(pod.Namespace, pod.Spec.ImagePullSecrets)
	containers := make([]ContainerInfo, 0)

	for _, c := range statuses {
		if c.ImageID != "" {
			imageIDSlice := strings.Split(c.ImageID, "://")
			trimmedImageID := imageIDSlice[len(imageIDSlice)-1]
			containers = append(containers, ContainerInfo{
				Image: oci.RegistryImage{
					Image:       c.Image,
					ImageID:     trimmedImageID,
					PullSecrets: allImageCreds,
				},
				Name: c.Name,
			})
		}
	}

	return containers
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

func (client *KubeClient) CreatePodInformer(labelSelector string) cache.SharedIndexInformer {
	ctx := context.Background()
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options meta.ListOptions) (runtime.Object, error) {
				return client.Client.CoreV1().Pods(meta.NamespaceAll).List(ctx, prepareLabelSelector(options, labelSelector))
			},
			WatchFunc: func(options meta.ListOptions) (watch.Interface, error) {
				return client.Client.CoreV1().Pods(meta.NamespaceAll).Watch(ctx, prepareLabelSelector(options, labelSelector))
			},
		},
		&corev1.Pod{},
		0,
		cache.Indexers{})
}
