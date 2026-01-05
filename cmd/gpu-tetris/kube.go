package main

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func BuildRestConfig(kubeconfigPath string) (*rest.Config, error) {
	// Prefer explicit kubeconfig when provided.
	if kubeconfigPath != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("build config from kubeconfig %q: %w", kubeconfigPath, err)
		}
		return cfg, nil
	}

	// Try in-cluster first (works when running as a pod).
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}

	// Fall back to standard kubeconfig resolution (~/.kube/config, KUBECONFIG).
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	cfg, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kube client config: %w", err)
	}
	return cfg, nil
}

func NewKubeClient(kubeconfigPath string) (kubernetes.Interface, error) {
	cfg, err := BuildRestConfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}
