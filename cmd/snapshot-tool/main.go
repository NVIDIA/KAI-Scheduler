// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/zip"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8sframework "k8s.io/kubernetes/pkg/scheduler/framework"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/eviction_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/conf"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/conf_util"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	k8splugins "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/k8s_internal/plugins"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/metrics"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/snapshot"
)

type mockCache struct {
	cache.Cache
	snapshot *api.ClusterInfo
}

func (m *mockCache) Snapshot() (*api.ClusterInfo, error) {
	return m.snapshot, nil
}

func (m *mockCache) Run(stopCh <-chan struct{}) {
	// No-op
}

func (m *mockCache) WaitForCacheSync(stopCh <-chan struct{}) {
	// No-op
}

func (m *mockCache) KubeClient() kubernetes.Interface {
	return fake.NewSimpleClientset()
}

func (m *mockCache) KubeInformerFactory() informers.SharedInformerFactory {
	// No-op
	return nil
}

func (m *mockCache) SnapshotSharedLister() k8sframework.NodeInfoLister {
	// No-op
	return nil
}

func (m *mockCache) InternalK8sPlugins() *k8splugins.K8sPlugins {
	// No-op
	return nil
}

func (m *mockCache) WaitForWorkers(stopCh <-chan struct{}) {
	// No-op
}

func (m *mockCache) Bind(podInfo *pod_info.PodInfo, hostname string) error {
	log.InfraLogger.V(1).Infow("Bind", "pod name", podInfo.Name, "namespace", podInfo.Namespace, "node", hostname)
	return nil
}

func (m *mockCache) Evict(ssnPod *v1.Pod, job *podgroup_info.PodGroupInfo, evictionMetadata eviction_info.EvictionMetadata, message string) error {
	log.InfraLogger.V(1).Infow(
		"Evict",
		"pod name", ssnPod.Name,
		"namespace", ssnPod.Namespace,
		"node", ssnPod.Spec.NodeSelector,
		"job", job.Name,
		"evictionMetadata", evictionMetadata,
		"message", message)
	return nil
}

func (m *mockCache) RecordJobStatusEvent(job *podgroup_info.PodGroupInfo) error {
	return nil
}

func (m *mockCache) TaskPipelined(task *pod_info.PodInfo, message string) {
	// No-op
}

func loadSnapshot(filename string) (*snapshot.Snapshot, error) {
	zipFile, err := zip.OpenReader(filename)
	if err != nil {
		return nil, err
	}
	defer zipFile.Close()

	for _, file := range zipFile.File {
		if file.Name == snapshot.SnapshotFileName {
			jsonFile, err := file.Open()
			if err != nil {
				return nil, err
			}
			defer jsonFile.Close()

			var snapshot snapshot.Snapshot
			err = json.NewDecoder(jsonFile).Decode(&snapshot)
			if err != nil {
				return nil, err
			}

			return &snapshot, nil
		}
	}

	return nil, os.ErrNotExist
}

func main() {
	filename := flag.String("filename", "", "location of the zipped JSON file")
	flag.Parse()

	snapshot, err := loadSnapshot(*filename)
	if err != nil {
		log.InfraLogger.Fatalf(err.Error(), err)
	}

	actions.InitDefaultActions()
	plugins.InitDefaultPlugins()

	mockCache := &mockCache{snapshot: snapshot.Snapshot}

	ssn, err := framework.OpenSession(
		mockCache, &conf.SchedulerConfiguration{}, snapshot.SchedulerParams, "", &http.ServeMux{},
	)
	if err != nil {
		log.InfraLogger.Fatalf(err.Error(), err)
	}
	defer framework.CloseSession(ssn)

	actions, _ := conf_util.GetActionsFromConfig(snapshot.Config)
	for _, action := range actions {
		log.InfraLogger.SetAction(string(action.Name()))
		metrics.SetCurrentAction(string(action.Name()))
		actionStartTime := time.Now()
		action.Execute(ssn)
		metrics.UpdateActionDuration(string(action.Name()), metrics.Duration(actionStartTime))
	}
}
