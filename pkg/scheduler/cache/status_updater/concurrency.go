// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package status_updater

import (
	"context"
	"strconv"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	enginev2alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	commonconstants "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/utils"
)

func (su *defaultStatusUpdater) Run(stopCh <-chan struct{}) {
	for i := 0; i < su.numberOfWorkers; i++ {
		go su.updateWorker(stopCh)
	}
	go su.queueBufferWorker(stopCh)
}

func (su *defaultStatusUpdater) SyncPodGroupsWithPendingUpdates(podGroups []*enginev2alpha2.PodGroup) {
	usedKeys := make(map[updatePayloadKey]bool, len(podGroups))
	for i := range podGroups {
		key := su.keyForPodGroupPayload(podGroups[i].Name, podGroups[i].Namespace, podGroups[i].UID)
		usedKeys[key] = true
		pgLatestUpdate, inFlightUpdateFound, appliedUpdateFound := su.getLatestPgUpdate(key)
		if !inFlightUpdateFound && !appliedUpdateFound {
			continue
		}
		podGroup := pgLatestUpdate.object.(*enginev2alpha2.PodGroup)
		podGroupMatchesUpdate := su.syncPodGroup(podGroup, podGroups[i])
		// Delete the inflight update if it was applied + the pod group in the lister matches the inFlight
		if podGroupMatchesUpdate && appliedUpdateFound {
			su.appliedPodGroupUpdates.Delete(key)
		}
	}

	// Cleanup podGroups that don't comeup anymore
	su.cleanUpdatesForNonSeenPodGroups(usedKeys)
}

func (su *defaultStatusUpdater) getLatestPgUpdate(key updatePayloadKey) (*inflightUpdate, bool, bool) {
	inflightPgUpdate, inFlightUpdateFound := su.inFlightPodGroups.Load(key)
	appliedPgUpdate, appliedUpdateFound := su.appliedPodGroupUpdates.Load(key)
	var pgLatestUpdate *inflightUpdate
	if inFlightUpdateFound {
		pgLatestUpdate = inflightPgUpdate.(*inflightUpdate)
	} else if appliedUpdateFound {
		pgLatestUpdate = appliedPgUpdate.(*inflightUpdate)
	}
	return pgLatestUpdate, inFlightUpdateFound, appliedUpdateFound
}

func (su *defaultStatusUpdater) cleanUpdatesForNonSeenPodGroups(usedKeys map[updatePayloadKey]bool) {
	su.inFlightPodGroups.Range(func(key any, _ any) bool {
		if _, found := usedKeys[key.(updatePayloadKey)]; !found {
			su.inFlightPodGroups.Delete(key)
		}
		return true
	})
	su.appliedPodGroupUpdates.Range(func(key any, _ any) bool {
		if _, found := usedKeys[key.(updatePayloadKey)]; !found {
			su.appliedPodGroupUpdates.Delete(key)
		}
		return true
	})
}

func (su *defaultStatusUpdater) syncPodGroup(inFlightPodGroup, snapshotPodGroup *enginev2alpha2.PodGroup) bool {
	staleTimeStampUpdated := false
	if snapshotPodGroup.Annotations[commonconstants.StalePodgroupTimeStamp] == inFlightPodGroup.Annotations[commonconstants.StalePodgroupTimeStamp] {
		staleTimeStampUpdated = true
	} else {
		if snapshotPodGroup.Annotations == nil {
			snapshotPodGroup.Annotations = make(map[string]string)
		}
		snapshotPodGroup.Annotations[commonconstants.StalePodgroupTimeStamp] = inFlightPodGroup.Annotations[commonconstants.StalePodgroupTimeStamp]
	}
	lastStartTimestampUpdated := false
	if snapshotPodGroup.Annotations[commonconstants.LastStartTimeStamp] == inFlightPodGroup.Annotations[commonconstants.LastStartTimeStamp] {
		lastStartTimestampUpdated = true
	} else {
		if snapshotPodGroup.Annotations == nil {
			snapshotPodGroup.Annotations = make(map[string]string)
		}
		snapshotPodGroup.Annotations[commonconstants.LastStartTimeStamp] = inFlightPodGroup.Annotations[commonconstants.LastStartTimeStamp]
	}

	updatedSchedulingCondition := false
	lastSchedulingCondition := utils.GetLastSchedulingCondition(inFlightPodGroup)
	currentLastSchedulingCondition := utils.GetLastSchedulingCondition(snapshotPodGroup)
	if currentLastSchedulingCondition != nil && lastSchedulingCondition != nil {
		currentID, currentErr := strconv.Atoi(currentLastSchedulingCondition.TransitionID)
		lastID, lastErr := strconv.Atoi(lastSchedulingCondition.TransitionID)
		if currentErr == nil && lastErr == nil && (lastID <= currentID || currentID == 0) {
			updatedSchedulingCondition = true
		}
	}
	if !updatedSchedulingCondition {
		snapshotPodGroup.Status.SchedulingConditions = inFlightPodGroup.Status.SchedulingConditions
	}
	return lastStartTimestampUpdated && staleTimeStampUpdated && updatedSchedulingCondition
}

func (su *defaultStatusUpdater) keyForPodGroupPayload(name, namespace string, uid types.UID) updatePayloadKey {
	return updatePayloadKey(types.NamespacedName{Name: name, Namespace: namespace}.String() + "_" + string(uid))
}

func (su *defaultStatusUpdater) keyForPodStatusPayload(name, namespace string, uid types.UID) updatePayloadKey {
	return updatePayloadKey(types.NamespacedName{Name: name, Namespace: namespace}.String() + "_" + string(uid) + "-Status")
}

func (su *defaultStatusUpdater) keyForPodLabelsPayload(name, namespace string, uid types.UID) updatePayloadKey {
	return updatePayloadKey(types.NamespacedName{Name: name, Namespace: namespace}.String() + "_" + string(uid) + "-Labels")
}

func (su *defaultStatusUpdater) processPayload(ctx context.Context, payload *updatePayload) {
	updateData, found := su.loadInflighUpdate(payload)
	if !found {
		return
	}

	switch payload.objectType {
	case podType:
		su.updatePod(ctx, payload.key, updateData.patchData, updateData.subResources, updateData.object)
	case podGroupType:
		su.updatePodGroup(ctx, payload.key, updateData)
	}
}

func (su *defaultStatusUpdater) loadInflighUpdate(payload *updatePayload) (*inflightUpdate, bool) {
	var data any
	var found bool
	switch {
	case payload.objectType == podType:
		data, found = su.inFlightPods.Load(payload.key)
	case payload.objectType == podGroupType:
		data, found = su.inFlightPodGroups.Load(payload.key)
	}

	if !found {
		return nil, false
	}
	return data.(*inflightUpdate), true
}

func (su *defaultStatusUpdater) updatePod(
	ctx context.Context, key updatePayloadKey, patchData []byte, subResources []string, object runtime.Object,
) {
	pod := object.(*v1.Pod)
	_, err := su.kubeClient.CoreV1().Pods(pod.Namespace).Patch(
		ctx, pod.Name, types.StrategicMergePatchType, patchData, metav1.PatchOptions{}, subResources...,
	)
	if err != nil {
		log.StatusUpdaterLogger.Errorf("Failed to update pod %s/%s: %v", pod.Namespace, pod.Name, err)
	} else {
		su.inFlightPods.Delete(key)
	}
}

// +kubebuilder:rbac:groups="scheduling.run.ai",resources=podgroups,verbs=update;patch
// +kubebuilder:rbac:groups="scheduling.run.ai",resources=podgroups/status,verbs=create;delete;update;patch;get;list;watch

func (su *defaultStatusUpdater) updatePodGroup(
	ctx context.Context, key updatePayloadKey, updateData *inflightUpdate,
) {
	podGroup := updateData.object.(*enginev2alpha2.PodGroup)

	var err error
	if updateData.updateStatus {
		_, err = su.kubeaischedClient.SchedulingV2alpha2().PodGroups(podGroup.Namespace).UpdateStatus(
			ctx, podGroup, metav1.UpdateOptions{},
		)
	}
	if len(updateData.patchData) > 0 {
		_, err = su.kubeaischedClient.SchedulingV2alpha2().PodGroups(podGroup.Namespace).Patch(
			ctx, podGroup.Name, types.JSONPatchType, updateData.patchData, metav1.PatchOptions{}, updateData.subResources...,
		)
	}
	if err != nil {
		log.StatusUpdaterLogger.Errorf("Failed to update pod group %s/%s: %v", podGroup.Namespace, podGroup.Name, err)
	} else {
		// Move the update to the applied cache
		su.appliedPodGroupUpdates.Store(key, updateData)
		su.inFlightPodGroups.Delete(key)
	}
}

func (su *defaultStatusUpdater) updateInFlightObject(key updatePayloadKey, objectType string, object *inflightUpdate) {
	switch objectType {
	case podType:
		su.inFlightPods.Store(key, object)
	case podGroupType:
		su.inFlightPodGroups.Store(key, object)
	}
}

func (su *defaultStatusUpdater) pushToUpdateQueue(payload *updatePayload, update *inflightUpdate) {
	su.updateInFlightObject(payload.key, payload.objectType, update)
	su.updateQueueIn <- payload
}

func (su *defaultStatusUpdater) queueBufferWorker(stopCh <-chan struct{}) {
	for {
		if len(su.updateQueueBuffer) == 0 {
			select {
			case <-stopCh:
				return
			case payload := <-su.updateQueueIn:
				su.updateQueueBuffer = append(su.updateQueueBuffer, payload)
			}
		} else {
			select {
			case <-stopCh:
				return
			case payload := <-su.updateQueueIn:
				su.updateQueueBuffer = append(su.updateQueueBuffer, payload)
			case su.updateQueueOut <- su.updateQueueBuffer[0]:
				su.updateQueueBuffer = su.updateQueueBuffer[1:]
			}
		}
	}
}

func (su *defaultStatusUpdater) updateWorker(stopCh <-chan struct{}) {
	ctx, cancel := context.WithCancel(context.Background())
	for {
		select {
		case <-stopCh:
			cancel()
			return
		case payload := <-su.updateQueueOut:
			su.processPayload(ctx, payload)
		}
	}
}
