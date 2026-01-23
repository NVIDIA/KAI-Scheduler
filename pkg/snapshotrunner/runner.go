// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package snapshotrunner

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"runtime/pprof"
	"syscall"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	kaischedulerfake "github.com/NVIDIA/KAI-scheduler/pkg/apis/client/clientset/versioned/fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/conf_util"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/metrics"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins"
	snapshotplugin "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/snapshot"
)

// Options controls how a snapshot run is executed.
type Options struct {
	// Filename is the path to the zipped JSON snapshot file.
	Filename string

	// Verbosity controls the scheduler log verbosity. When set to 0, loggers are
	// not re-initialized by the runner and the caller is expected to have already
	// configured logging.
	Verbosity int

	// CPUProfile, when non-empty, enables CPU profiling and writes the profile
	// to the given path.
	CPUProfile string
}

// LoadSnapshot loads a snapshot from a zip file on disk.
func LoadSnapshot(filename string) (*snapshotplugin.Snapshot, error) {
	zipFile, err := zip.OpenReader(filename)
	if err != nil {
		return nil, err
	}
	defer zipFile.Close()

	for _, file := range zipFile.File {
		if file.Name == snapshotplugin.SnapshotFileName {
			jsonFile, err := file.Open()
			if err != nil {
				return nil, err
			}
			defer jsonFile.Close()

			var snapshot snapshotplugin.Snapshot
			err = json.NewDecoder(jsonFile).Decode(&snapshot)
			if err != nil {
				return nil, err
			}

			return &snapshot, nil
		}
	}

	return nil, os.ErrNotExist
}

// loadClientsWithSnapshot creates fake clients populated with the raw
// Kubernetes objects from the snapshot.
func loadClientsWithSnapshot(rawObjects *snapshotplugin.RawKubernetesObjects) (*fake.Clientset, *kaischedulerfake.Clientset) {
	kubeClient := fake.NewSimpleClientset()
	kaiClient := kaischedulerfake.NewSimpleClientset()

	for _, pod := range rawObjects.Pods {
		_, err := kubeClient.CoreV1().Pods(pod.Namespace).Create(context.TODO(), pod, v1.CreateOptions{})
		if err != nil {
			log.InfraLogger.Errorf("Failed to create pod: %v", err)
		}
	}

	for _, node := range rawObjects.Nodes {
		_, err := kubeClient.CoreV1().Nodes().Create(context.TODO(), node, v1.CreateOptions{})
		if err != nil {
			log.InfraLogger.Errorf("Failed to create node: %v", err)
		}
	}

	for _, bindRequest := range rawObjects.BindRequests {
		_, err := kaiClient.SchedulingV1alpha2().BindRequests(bindRequest.Namespace).Create(context.TODO(), bindRequest, v1.CreateOptions{})
		if err != nil {
			log.InfraLogger.Errorf("Failed to create bind request: %v", err)
		}
	}

	for _, podGroup := range rawObjects.PodGroups {
		_, err := kaiClient.SchedulingV2alpha2().PodGroups(podGroup.Namespace).Create(context.TODO(), podGroup, v1.CreateOptions{})
		if err != nil {
			log.InfraLogger.Errorf("Failed to create pod group: %v", err)
		}
	}

	for _, queue := range rawObjects.Queues {
		_, err := kaiClient.SchedulingV2().Queues(queue.Namespace).Create(context.TODO(), queue, v1.CreateOptions{})
		if err != nil {
			log.InfraLogger.Errorf("Failed to create queue: %v", err)
		}
	}

	for _, priorityClass := range rawObjects.PriorityClasses {
		_, err := kubeClient.SchedulingV1().PriorityClasses().Create(context.TODO(), priorityClass, v1.CreateOptions{})
		if err != nil {
			log.InfraLogger.Errorf("Failed to create priority class: %v", err)
		}
	}

	for _, configMap := range rawObjects.ConfigMaps {
		_, err := kubeClient.CoreV1().ConfigMaps(configMap.Namespace).Create(context.TODO(), configMap, v1.CreateOptions{})
		if err != nil {
			log.InfraLogger.Errorf("Failed to create config map: %v", err)
		}
	}

	for _, persistentVolumeClaim := range rawObjects.PersistentVolumeClaims {
		_, err := kubeClient.CoreV1().PersistentVolumeClaims(persistentVolumeClaim.Namespace).Create(context.TODO(), persistentVolumeClaim, v1.CreateOptions{})
		if err != nil {
			log.InfraLogger.Errorf("Failed to create persistent volume claim: %v", err)
		}
	}

	for _, csiStorageCapacity := range rawObjects.CSIStorageCapacities {
		_, err := kubeClient.StorageV1().CSIStorageCapacities(csiStorageCapacity.Namespace).Create(context.TODO(), csiStorageCapacity, v1.CreateOptions{})
		if err != nil {
			log.InfraLogger.Errorf("Failed to create CSI storage capacity: %v", err)
		}
	}

	for _, storageClass := range rawObjects.StorageClasses {
		_, err := kubeClient.StorageV1().StorageClasses().Create(context.TODO(), storageClass, v1.CreateOptions{})
		if err != nil {
			log.InfraLogger.Errorf("Failed to create storage class: %v", err)
		}
	}

	for _, csiDriver := range rawObjects.CSIDrivers {
		_, err := kubeClient.StorageV1().CSIDrivers().Create(context.TODO(), csiDriver, v1.CreateOptions{})
		if err != nil {
			log.InfraLogger.Errorf("Failed to create CSI driver: %v", err)
		}
	}

	for _, topology := range rawObjects.Topologies {
		_, err := kaiClient.KaiV1alpha1().Topologies().Create(context.TODO(), topology, v1.CreateOptions{})
		if err != nil {
			log.InfraLogger.Errorf("Failed to create topology: %v", err)
		}
	}

	for _, resourceClaim := range rawObjects.ResourceClaims {
		_, err := kubeClient.ResourceV1().ResourceClaims(resourceClaim.Namespace).Create(context.TODO(), resourceClaim, v1.CreateOptions{})
		if err != nil {
			log.InfraLogger.Errorf("Failed to create resource claim: %v", err)
		}
	}

	for _, resourceSlice := range rawObjects.ResourceSlices {
		_, err := kubeClient.ResourceV1().ResourceSlices().Create(context.TODO(), resourceSlice, v1.CreateOptions{})
		if err != nil {
			log.InfraLogger.Errorf("Failed to create resource slice: %v", err)
		}
	}

	for _, deviceClass := range rawObjects.DeviceClasses {
		_, err := kubeClient.ResourceV1().DeviceClasses().Create(context.TODO(), deviceClass, v1.CreateOptions{})
		if err != nil {
			log.InfraLogger.Errorf("Failed to create device class: %v", err)
		}
	}

	return kubeClient, kaiClient
}

// Run executes the scheduler actions defined in the snapshot configuration
// against the in-memory cluster recreated from the snapshot.
//
// The function mirrors the logic from the snapshot-tool binary but returns
// errors instead of exiting the process, making it suitable for use from
// tests and other Go code.
func Run(opts Options) error {
	if opts.Filename == "" {
		return errors.New("snapshot filename must be provided")
	}

	if opts.Verbosity > 0 {
		if err := log.InitLoggers(opts.Verbosity); err != nil {
			return err
		}
	}
	defer func() {
		syncErr := log.InfraLogger.Sync()
		if syncErr != nil && !errors.Is(syncErr, syscall.EINVAL) {
			// Best-effort log flush; use Stdout to avoid recursion.
			_, _ = os.Stdout.WriteString("Failed to write log: " + syncErr.Error())
		}
	}()
	log.InfraLogger.SetSessionID("snapshot-runner")

	snapshot, err := LoadSnapshot(opts.Filename)
	if err != nil {
		return err
	}

	actions.InitDefaultActions()
	plugins.InitDefaultPlugins()

	kubeClient, kaiClient := loadClientsWithSnapshot(snapshot.RawObjects)

	schedulerCacheParams := &cache.SchedulerCacheParams{
		KubeClient:                  kubeClient,
		KAISchedulerClient:          kaiClient,
		SchedulerName:               snapshot.SchedulerParams.SchedulerName,
		NodePoolParams:              snapshot.SchedulerParams.PartitionParams,
		RestrictNodeScheduling:      snapshot.SchedulerParams.RestrictSchedulingNodes,
		DetailedFitErrors:           snapshot.SchedulerParams.DetailedFitErrors,
		ScheduleCSIStorage:          snapshot.SchedulerParams.ScheduleCSIStorage,
		FullHierarchyFairness:       snapshot.SchedulerParams.FullHierarchyFairness,
		AllowConsolidatingReclaim:   snapshot.SchedulerParams.AllowConsolidatingReclaim,
		NumOfStatusRecordingWorkers: snapshot.SchedulerParams.NumOfStatusRecordingWorkers,
		DiscoveryClient:             kubeClient.Discovery(),
	}

	schedulerCache := cache.New(schedulerCacheParams)
	stopCh := make(chan struct{})
	defer close(stopCh)

	schedulerCache.Run(stopCh)
	schedulerCache.WaitForCacheSync(stopCh)

	if opts.CPUProfile != "" {
		f, err := os.Create(opts.CPUProfile)
		if err != nil {
			return err
		}

		if err := pprof.StartCPUProfile(f); err != nil {
			return err
		}
		defer pprof.StopCPUProfile()
	}

	ssn, err := framework.OpenSession(
		schedulerCache, snapshot.Config, snapshot.SchedulerParams, "", &http.ServeMux{},
	)
	if err != nil {
		return err
	}
	defer framework.CloseSession(ssn)

	acts, _ := conf_util.GetActionsFromConfig(snapshot.Config)
	for _, action := range acts {
		log.InfraLogger.SetAction(string(action.Name()))
		metrics.SetCurrentAction(string(action.Name()))
		actionStartTime := time.Now()
		action.Execute(ssn)
		metrics.UpdateActionDuration(string(action.Name()), metrics.Duration(actionStartTime))
	}

	return nil
}
