// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package dynamicresources_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	. "go.uber.org/mock/gomock"
	"gopkg.in/h2non/gock.v1"
	resourceapi "k8s.io/api/resource/v1"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	featuregate "k8s.io/component-base/featuregate/testing"
	"k8s.io/kubernetes/pkg/features"

	schedulingv1alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v1alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/bindrequest_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/eviction_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/dra_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/jobs_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/nodes_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/tasks_fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStatementAllocateRollback_WithDRAClaims(t *testing.T) {
	test_utils.InitTestingInfrastructure()
	controller := NewController(t)
	defer controller.Finish()
	defer gock.Off()

	type testMetadata struct {
		name     string
		topology test_utils.TestTopologyBasic
		nodeName string
	}

	for i, test := range []testMetadata{
		{
			name: "Allocate then rollback pod with DRA claim",
			topology: test_utils.TestTopologyBasic{
				Queues: []test_utils.TestQueueBasic{
					{Name: "q-1"},
				},
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:      "job-test",
						Namespace: "test",
						QueueName: "q-1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								Name:               "task-1",
								State:              pod_status.Pending,
								ResourceClaimNames: []string{"gpu-claim"},
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 1,
					},
				},
				TestDRAObjects: dra_fake.TestDRAObjects{
					DeviceClasses: []string{"nvidia.com/gpu"},
					ResourceSlices: []*dra_fake.TestResourceSlice{
						{
							Name:            "node0-gpu",
							DeviceClassName: "nvidia.com/gpu",
							NodeName:        "node0",
							Count:           1,
						},
					},
					ResourceClaims: []*dra_fake.TestResourceClaim{
						{
							Name:            "gpu-claim",
							Namespace:       "test",
							DeviceClassName: "nvidia.com/gpu",
							Count:           1,
							Labels: map[string]string{
								constants.DefaultQueueLabel: "q-1",
							},
						},
					},
				},
			},
			nodeName: "node0",
		},
		{
			name: "Allocate then rollback pod with multiple DRA claims",
			topology: test_utils.TestTopologyBasic{
				Queues: []test_utils.TestQueueBasic{
					{Name: "q-1"},
				},
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:      "job-test",
						Namespace: "test",
						QueueName: "q-1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								Name:               "task-1",
								State:              pod_status.Pending,
								ResourceClaimNames: []string{"gpu-claim-1", "gpu-claim-2"},
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 2,
					},
				},
				TestDRAObjects: dra_fake.TestDRAObjects{
					DeviceClasses: []string{"nvidia.com/gpu"},
					ResourceSlices: []*dra_fake.TestResourceSlice{
						{
							Name:            "node0-gpu",
							DeviceClassName: "nvidia.com/gpu",
							NodeName:        "node0",
							Count:           2,
						},
					},
					ResourceClaims: []*dra_fake.TestResourceClaim{
						{
							Name:            "gpu-claim-1",
							Namespace:       "test",
							DeviceClassName: "nvidia.com/gpu",
							Count:           1,
							Labels: map[string]string{
								constants.DefaultQueueLabel: "q-1",
							},
						},
						{
							Name:            "gpu-claim-2",
							Namespace:       "test",
							DeviceClassName: "nvidia.com/gpu",
							Count:           1,
							Labels: map[string]string{
								constants.DefaultQueueLabel: "q-1",
							},
						},
					},
				},
			},
			nodeName: "node0",
		},
		{
			name: "Allocate then rollback pod where claim is already allocated to another pod (shared claim)",
			topology: test_utils.TestTopologyBasic{
				Queues: []test_utils.TestQueueBasic{
					{Name: "q-1"},
				},
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:      "job-other",
						Namespace: "test",
						QueueName: "q-1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								Name:               "other-task",
								State:              pod_status.Running,
								NodeName:           "node0",
								ResourceClaimNames: []string{"shared-gpu-claim"},
							},
						},
					},
					{
						Name:      "job-test",
						Namespace: "test",
						QueueName: "q-1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								Name:               "task-1",
								State:              pod_status.Pending,
								ResourceClaimNames: []string{"shared-gpu-claim"},
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 1,
					},
				},
				TestDRAObjects: dra_fake.TestDRAObjects{
					DeviceClasses: []string{"nvidia.com/gpu"},
					ResourceSlices: []*dra_fake.TestResourceSlice{
						{
							Name:            "node0-gpu",
							DeviceClassName: "nvidia.com/gpu",
							NodeName:        "node0",
							Count:           1,
						},
					},
					ResourceClaims: []*dra_fake.TestResourceClaim{
						{
							Name:            "shared-gpu-claim",
							Namespace:       "test",
							DeviceClassName: "nvidia.com/gpu",
							Count:           1,
							Labels: map[string]string{
								constants.DefaultQueueLabel: "q-1",
							},
							ClaimStatus: &resourceapi.ResourceClaimStatus{
								Allocation: &resourceapi.AllocationResult{
									Devices: resourceapi.DeviceAllocationResult{
										Results: []resourceapi.DeviceRequestAllocationResult{
											{
												Request: "gpu",
												Driver:  "gpu.resource.k8s.io",
												Pool:    "node0",
												Device:  "gpu-0",
											},
										},
									},
								},
								ReservedFor: []resourceapi.ResourceClaimConsumerReference{
									{
										Resource: "pods",
										Name:     "job-other-other-task",
										UID:      "other-pod-uid",
									},
								},
							},
						},
					},
				},
			},
			nodeName: "node0",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Logf("Running test number: %d, test name: %s", i, test.name)

			featuregate.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.DynamicResourceAllocation, true)
			ssn := test_utils.BuildSession(test.topology, controller)
			time.Sleep(1 * time.Millisecond)

			// Get the pending task from job-test
			var task *pod_info.PodInfo
			job := ssn.ClusterInfo.PodGroupInfos[common_info.PodGroupID("job-test")]
			assert.NotNil(t, job, "Job job-test should exist in session")
			for _, t := range job.GetAllPodsMap() {
				if t.Status == pod_status.Pending {
					task = t
					break
				}
			}
			assert.NotNil(t, task, "Should find pending task in job-test")

			stmt := ssn.Statement()
			cp := stmt.Checkpoint() // Take a checkpoint before allocation

			// Store initial state
			initialNodeName := task.NodeName
			initialStatus := task.Status
			initialClaimInfoCount := len(task.ResourceClaimInfo)
			expectedClaimCount := len(test.topology.TestDRAObjects.ResourceClaims)
			initialAllocations := make(map[string]*schedulingv1alpha2.ResourceClaimAllocation)
			for claimName, claimAlloc := range task.ResourceClaimInfo {
				if claimAlloc != nil {
					initialAllocations[claimName] = claimAlloc.DeepCopy()
				}
			}
			assert.Equal(t, expectedClaimCount, initialClaimInfoCount, "Task should have ResourceClaimInfo entries for each claim")

			err := stmt.Allocate(task, test.nodeName)
			assert.NoError(t, err, "Allocation should succeed")

			// Verify allocation happened
			assert.Equal(t, test.nodeName, task.NodeName, "Task should be allocated to node")
			assert.Equal(t, pod_status.Allocated, task.Status, "Task status should be Allocated")

			assert.Equal(t, initialClaimInfoCount, len(task.ResourceClaimInfo), "ResourceClaimInfo count should remain the same after allocation")
			for claimName, claimAlloc := range task.ResourceClaimInfo {
				assert.NotNil(t, claimAlloc, "Claim %s should exist", claimName)
				assert.NotNil(t, claimAlloc.Allocation, "Claim %s should have allocation details after allocation", claimName)
				assert.Greater(t, len(claimAlloc.Allocation.Devices.Results), 0, "Claim %s should have device results after allocation", claimName)
				for _, deviceResult := range claimAlloc.Allocation.Devices.Results {
					assert.NotEmpty(t, deviceResult.Device, "Claim %s should have allocated device name", claimName)
					assert.NotEmpty(t, deviceResult.Driver, "Claim %s should have driver specified", claimName)
				}
			}

			err = stmt.Rollback(cp)
			assert.NoError(t, err, "Rollback should succeed")

			// Verify rollback happened - task should be back to initial state
			assert.Equal(t, initialNodeName, task.NodeName, "Task should be back to initial node after rollback")
			assert.Equal(t, initialStatus, task.Status, "Task status should be back to initial status after rollback")

			assert.Equal(t, initialClaimInfoCount, len(task.ResourceClaimInfo), "ResourceClaimInfo count should remain the same after rollback")
			for claimName, currentAlloc := range task.ResourceClaimInfo {
				initialAlloc := initialAllocations[claimName]

				if initialAlloc == nil || initialAlloc.Allocation == nil {
					// Initial had no allocation
					if currentAlloc.Allocation != nil {
						assert.Equal(t, 0, len(currentAlloc.Allocation.Devices.Results),
							"Claim %s should have no device allocations after rollback (initial had none)", claimName)
					}
				} else {
					// Initial had allocation - verify exact match
					assert.NotNil(t, currentAlloc.Allocation, "Claim %s should have allocation after rollback", claimName)
					assert.Equal(t, len(initialAlloc.Allocation.Devices.Results), len(currentAlloc.Allocation.Devices.Results),
						"Claim %s should have same number of devices after rollback", claimName)

					// Compare each device allocation
					for i, initialDevice := range initialAlloc.Allocation.Devices.Results {
						currentDevice := currentAlloc.Allocation.Devices.Results[i]
						assert.Equal(t, initialDevice.Device, currentDevice.Device,
							"Claim %s device %d name should match after rollback", claimName, i)
						assert.Equal(t, initialDevice.Driver, currentDevice.Driver,
							"Claim %s device %d driver should match after rollback", claimName, i)
						assert.Equal(t, initialDevice.Pool, currentDevice.Pool,
							"Claim %s device %d pool should match after rollback", claimName, i)
						assert.Equal(t, initialDevice.Request, currentDevice.Request,
							"Claim %s device %d request should match after rollback", claimName, i)
					}
				}
			}
		})
	}
}

func TestStatementEvictUnevict_WithDRAClaims(t *testing.T) {
	test_utils.InitTestingInfrastructure()
	controller := NewController(t)
	defer controller.Finish()
	defer gock.Off()

	type testMetadata struct {
		name                string
		topology            test_utils.TestTopologyBasic
		bindRequest         *schedulingv1alpha2.BindRequest
		bindRequestNodeName string
	}

	for i, test := range []testMetadata{
		{
			name: "Evict then unevict running pod with DRA claim",
			topology: test_utils.TestTopologyBasic{
				Queues: []test_utils.TestQueueBasic{
					{Name: "q-1"},
				},
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:      "test-job",
						Namespace: "test",
						QueueName: "q-1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								Name:               "test-task",
								State:              pod_status.Running,
								NodeName:           "node0",
								ResourceClaimNames: []string{"gpu-claim"},
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 1,
					},
				},
				TestDRAObjects: dra_fake.TestDRAObjects{
					DeviceClasses: []string{"nvidia.com/gpu"},
					ResourceSlices: []*dra_fake.TestResourceSlice{
						{
							Name:            "node0-gpu",
							DeviceClassName: "nvidia.com/gpu",
							NodeName:        "node0",
							Count:           1,
						},
					},
					ResourceClaims: []*dra_fake.TestResourceClaim{
						{
							Name:            "gpu-claim",
							Namespace:       "test",
							DeviceClassName: "nvidia.com/gpu",
							Count:           1,
							Labels: map[string]string{
								constants.DefaultQueueLabel: "q-1",
							},
							ClaimStatus: &resourceapi.ResourceClaimStatus{
								Allocation: &resourceapi.AllocationResult{
									Devices: resourceapi.DeviceAllocationResult{
										Results: []resourceapi.DeviceRequestAllocationResult{
											{
												Request: "gpu",
												Driver:  "gpu.resource.k8s.io",
												Pool:    "node0",
												Device:  "gpu-0",
											},
										},
									},
								},
								ReservedFor: []resourceapi.ResourceClaimConsumerReference{
									{
										Resource: "pods",
										Name:     "job-running-task-running",
										UID:      "running-pod-uid",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Evict then unevict running pod with shared DRA claim (multiple consumers)",
			topology: test_utils.TestTopologyBasic{
				Queues: []test_utils.TestQueueBasic{
					{Name: "q-1"},
				},
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:      "job-other",
						Namespace: "test",
						QueueName: "q-1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								Name:               "task-other",
								State:              pod_status.Running,
								NodeName:           "node0",
								ResourceClaimNames: []string{"shared-gpu-claim"},
							},
						},
					},
					{
						Name:      "test-job",
						Namespace: "test",
						QueueName: "q-1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								Name:               "test-task",
								State:              pod_status.Running,
								NodeName:           "node0",
								ResourceClaimNames: []string{"shared-gpu-claim"},
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 1,
					},
				},
				TestDRAObjects: dra_fake.TestDRAObjects{
					DeviceClasses: []string{"nvidia.com/gpu"},
					ResourceSlices: []*dra_fake.TestResourceSlice{
						{
							Name:            "node0-gpu",
							DeviceClassName: "nvidia.com/gpu",
							NodeName:        "node0",
							Count:           1,
						},
					},
					ResourceClaims: []*dra_fake.TestResourceClaim{
						{
							Name:            "shared-gpu-claim",
							Namespace:       "test",
							DeviceClassName: "nvidia.com/gpu",
							Count:           1,
							Labels: map[string]string{
								constants.DefaultQueueLabel: "q-1",
							},
							ClaimStatus: &resourceapi.ResourceClaimStatus{
								Allocation: &resourceapi.AllocationResult{
									Devices: resourceapi.DeviceAllocationResult{
										Results: []resourceapi.DeviceRequestAllocationResult{
											{
												Request: "gpu",
												Driver:  "gpu.resource.k8s.io",
												Pool:    "node0",
												Device:  "gpu-0",
											},
										},
									},
								},
								ReservedFor: []resourceapi.ResourceClaimConsumerReference{
									{
										Resource: "pods",
										Name:     "job-other-task-other",
										UID:      "other-pod-uid",
									},
									{
										Resource: "pods",
										Name:     "job-running-task-running",
										UID:      "running-pod-uid",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Evict then unevict pod in Binding state with BindRequest and DRA claim",
			bindRequest: &schedulingv1alpha2.BindRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bind-request",
					Namespace: "test",
				},
				Spec: schedulingv1alpha2.BindRequestSpec{
					PodName:      "test-pod",
					SelectedNode: "node0",
					ResourceClaimAllocations: []schedulingv1alpha2.ResourceClaimAllocation{
						{
							Name: "gpu-claim",
							Allocation: &resourceapi.AllocationResult{
								Devices: resourceapi.DeviceAllocationResult{
									Results: []resourceapi.DeviceRequestAllocationResult{
										{
											Request: "gpu",
											Driver:  "gpu.resource.k8s.io",
											Pool:    "node0",
											Device:  "gpu-0",
										},
									},
								},
							},
						},
					},
				},
			},
			bindRequestNodeName: "node0",
			topology: test_utils.TestTopologyBasic{
				Queues: []test_utils.TestQueueBasic{
					{Name: "q-1"},
				},
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:      "test-job",
						Namespace: "test",
						QueueName: "q-1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								Name:               "test-task",
								State:              pod_status.Binding,
								NodeName:           "node0",
								ResourceClaimNames: []string{"gpu-claim"},
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 1,
					},
				},
				TestDRAObjects: dra_fake.TestDRAObjects{
					DeviceClasses: []string{"nvidia.com/gpu"},
					ResourceSlices: []*dra_fake.TestResourceSlice{
						{
							Name:            "node0-gpu",
							DeviceClassName: "nvidia.com/gpu",
							NodeName:        "node0",
							Count:           1,
						},
					},
					ResourceClaims: []*dra_fake.TestResourceClaim{
						{
							Name:            "gpu-claim",
							Namespace:       "test",
							DeviceClassName: "nvidia.com/gpu",
							Count:           1,
							Labels: map[string]string{
								constants.DefaultQueueLabel: "q-1",
							},
							ClaimStatus: &resourceapi.ResourceClaimStatus{
								Allocation: &resourceapi.AllocationResult{
									Devices: resourceapi.DeviceAllocationResult{
										Results: []resourceapi.DeviceRequestAllocationResult{
											{
												Request: "gpu",
												Driver:  "gpu.resource.k8s.io",
												Pool:    "node0",
												Device:  "gpu-0",
											},
										},
									},
								},
								ReservedFor: []resourceapi.ResourceClaimConsumerReference{
									{
										Resource: "pods",
										Name:     "job-binding-task-binding",
										UID:      "binding-pod-uid",
									},
								},
							},
						},
					},
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Logf("Running test number: %d, test name: %s", i, test.name)

			featuregate.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.DynamicResourceAllocation, true)
			ssn := test_utils.BuildSession(test.topology, controller)
			time.Sleep(1 * time.Millisecond)

			job := ssn.ClusterInfo.PodGroupInfos[common_info.PodGroupID("test-job")]
			task := job.GetAllPodsMap()[common_info.PodID("test-job-0")]

			// If this is a BindRequest test, recreate the PodInfo with a BindRequest
			if test.bindRequest != nil {
				bindRequestInfo := bindrequest_info.NewBindRequestInfo(test.bindRequest)

				// Recreate the PodInfo with the BindRequest
				newTask := pod_info.NewTaskInfoWithBindRequest(task.Pod, bindRequestInfo, ssn.ClusterInfo.ResourceClaims...)

				// Replace the task in the job's podSet
				for _, podSet := range job.PodSets {
					if _, exists := podSet.GetPodInfos()[task.UID]; exists {
						delete(podSet.GetPodInfos(), task.UID)
						podSet.GetPodInfos()[newTask.UID] = newTask
						break
					}
				}

				// Update our reference to point to the new task
				task = newTask
			}

			stmt := ssn.Statement()
			cp := stmt.Checkpoint()

			// Store initial state
			initialClaimInfoCount := len(task.ResourceClaimInfo)
			initialAllocations := make(map[string]*schedulingv1alpha2.ResourceClaimAllocation)
			for claimName, claimAlloc := range task.ResourceClaimInfo {
				if claimAlloc != nil {
					initialAllocations[claimName] = claimAlloc.DeepCopy()
				}
			}
			assert.Greater(t, initialClaimInfoCount, 0, "Task should have ResourceClaimInfo")
			initialNodeName := task.NodeName
			initialStatus := task.Status

			err := stmt.Evict(task, "test eviction", eviction_info.EvictionMetadata{
				EvictionGangSize: 1,
				Action:           "reclaim",
			})
			assert.NoError(t, err, "Eviction should succeed")

			// Verify eviction happened
			assert.Equal(t, pod_status.Releasing, task.Status, "Task status should be Releasing after eviction")
			assert.Equal(t, initialClaimInfoCount, len(task.ResourceClaimInfo), "ResourceClaimInfo count should remain constant after eviction")

			// Rollback (unevict)
			err = stmt.Rollback(cp)
			assert.NoError(t, err, "Rollback should succeed")

			// Verify uneviction happened - task should be back to initial state
			assert.Equal(t, initialNodeName, task.NodeName, "Task should be back to initial node after unevict")
			assert.Equal(t, initialStatus, task.Status, "Task status should be back to initial status after unevict")

			assert.Equal(t, initialClaimInfoCount, len(task.ResourceClaimInfo), "ResourceClaimInfo count should remain constant after unevict")
			for claimName, currentAlloc := range task.ResourceClaimInfo {
				initialAlloc := initialAllocations[claimName]
				assert.NotNil(t, initialAlloc, "Initial allocation for claim %s should exist", claimName)
				assert.NotNil(t, currentAlloc, "Current allocation for claim %s should exist", claimName)
				assert.NotNil(t, currentAlloc.Allocation, "Claim %s should have allocation after unevict", claimName)
				assert.Equal(t, len(initialAlloc.Allocation.Devices.Results), len(currentAlloc.Allocation.Devices.Results),
					"Claim %s should have same number of devices after unevict", claimName)

				for i, initialDevice := range initialAlloc.Allocation.Devices.Results {
					currentDevice := currentAlloc.Allocation.Devices.Results[i]
					assert.Equal(t, initialDevice.Device, currentDevice.Device,
						"Claim %s device %d name should match after unevict", claimName, i)
					assert.Equal(t, initialDevice.Driver, currentDevice.Driver,
						"Claim %s device %d driver should match after unevict", claimName, i)
					assert.Equal(t, initialDevice.Pool, currentDevice.Pool,
						"Claim %s device %d pool should match after unevict", claimName, i)
					assert.Equal(t, initialDevice.Request, currentDevice.Request,
						"Claim %s device %d request should match after unevict", claimName, i)
				}
			}
		})
	}
}

func TestStatementPipelineUnpipeline_WithDRAClaims(t *testing.T) {
	test_utils.InitTestingInfrastructure()
	controller := NewController(t)
	defer controller.Finish()
	defer gock.Off()

	type testMetadata struct {
		name     string
		topology test_utils.TestTopologyBasic
		nodeName string
	}

	for i, test := range []testMetadata{
		{
			name: "Pipeline then unpipeline pod with DRA claim",
			topology: test_utils.TestTopologyBasic{
				Queues: []test_utils.TestQueueBasic{
					{Name: "q-1"},
				},
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:      "job-pending",
						Namespace: "test",
						QueueName: "q-1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								Name:               "task-pending",
								State:              pod_status.Pending,
								ResourceClaimNames: []string{"gpu-claim"},
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 1,
					},
				},
				TestDRAObjects: dra_fake.TestDRAObjects{
					DeviceClasses: []string{"nvidia.com/gpu"},
					ResourceSlices: []*dra_fake.TestResourceSlice{
						{
							Name:            "node0-gpu",
							DeviceClassName: "nvidia.com/gpu",
							NodeName:        "node0",
							Count:           1,
						},
					},
					ResourceClaims: []*dra_fake.TestResourceClaim{
						{
							Name:            "gpu-claim",
							Namespace:       "test",
							DeviceClassName: "nvidia.com/gpu",
							Count:           1,
							Labels: map[string]string{
								constants.DefaultQueueLabel: "q-1",
							},
						},
					},
				},
			},
			nodeName: "node0",
		},
		{
			name: "Pipeline then unpipeline pod with multiple DRA claims",
			topology: test_utils.TestTopologyBasic{
				Queues: []test_utils.TestQueueBasic{
					{Name: "q-1"},
				},
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:      "job-pending",
						Namespace: "test",
						QueueName: "q-1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								Name:               "task-pending",
								State:              pod_status.Pending,
								ResourceClaimNames: []string{"gpu-claim-1", "gpu-claim-2"},
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 2,
					},
				},
				TestDRAObjects: dra_fake.TestDRAObjects{
					DeviceClasses: []string{"nvidia.com/gpu"},
					ResourceSlices: []*dra_fake.TestResourceSlice{
						{
							Name:            "node0-gpu",
							DeviceClassName: "nvidia.com/gpu",
							NodeName:        "node0",
							Count:           2,
						},
					},
					ResourceClaims: []*dra_fake.TestResourceClaim{
						{
							Name:            "gpu-claim-1",
							Namespace:       "test",
							DeviceClassName: "nvidia.com/gpu",
							Count:           1,
							Labels: map[string]string{
								constants.DefaultQueueLabel: "q-1",
							},
						},
						{
							Name:            "gpu-claim-2",
							Namespace:       "test",
							DeviceClassName: "nvidia.com/gpu",
							Count:           1,
							Labels: map[string]string{
								constants.DefaultQueueLabel: "q-1",
							},
						},
					},
				},
			},
			nodeName: "node0",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Logf("Running test number: %d, test name: %s", i, test.name)

			featuregate.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.DynamicResourceAllocation, true)
			ssn := test_utils.BuildSession(test.topology, controller)
			time.Sleep(1 * time.Millisecond)

			// Get the pending task
			var task *pod_info.PodInfo
			job := ssn.ClusterInfo.PodGroupInfos[common_info.PodGroupID("job-pending")]
			assert.NotNil(t, job, "Job job-pending should exist in session")
			for _, t := range job.GetAllPodsMap() {
				if t.Status == pod_status.Pending {
					task = t
					break
				}
			}
			assert.NotNil(t, task, "Should find pending task in job-pending")

			stmt := ssn.Statement()
			cp := stmt.Checkpoint()

			// Store initial state
			initialClaimInfoCount := len(task.ResourceClaimInfo)
			initialAllocations := make(map[string]*schedulingv1alpha2.ResourceClaimAllocation)
			for claimName, claimAlloc := range task.ResourceClaimInfo {
				if claimAlloc != nil {
					initialAllocations[claimName] = claimAlloc.DeepCopy()
				}
			}
			assert.Greater(t, initialClaimInfoCount, 0, "Pending task should have ResourceClaimInfo")
			initialNodeName := task.NodeName
			initialStatus := task.Status

			// Pipeline the task
			err := stmt.Pipeline(task, test.nodeName, false)
			assert.NoError(t, err, "Pipeline should succeed")

			// Verify pipeline happened
			assert.Equal(t, test.nodeName, task.NodeName, "Task should be pipelined to node")
			assert.Equal(t, pod_status.Pipelined, task.Status, "Task status should be Pipelined")
			assert.Equal(t, initialClaimInfoCount, len(task.ResourceClaimInfo), "ResourceClaimInfo count should remain constant after pipeline")

			// Verify each claim now has allocation details
			for claimName, claimAlloc := range task.ResourceClaimInfo {
				assert.NotNil(t, claimAlloc, "Claim %s should exist", claimName)
				assert.NotNil(t, claimAlloc.Allocation, "Claim %s should have allocation details after pipeline", claimName)
				assert.Greater(t, len(claimAlloc.Allocation.Devices.Results), 0, "Claim %s should have device results after pipeline", claimName)

				// Verify actual device allocation details
				for _, deviceResult := range claimAlloc.Allocation.Devices.Results {
					assert.NotEmpty(t, deviceResult.Device, "Claim %s should have allocated device name", claimName)
					assert.NotEmpty(t, deviceResult.Driver, "Claim %s should have driver specified", claimName)
					assert.NotEmpty(t, deviceResult.Pool, "Claim %s should have pool specified", claimName)
				}
			}

			// Rollback (unpipeline)
			err = stmt.Rollback(cp)
			assert.NoError(t, err, "Rollback should succeed")

			// Verify unpipeline happened - task should be back to initial state
			assert.Equal(t, initialNodeName, task.NodeName, "Task should be back to initial node after unpipeline")
			assert.Equal(t, initialStatus, task.Status, "Task status should be back to initial status after unpipeline")
			assert.Equal(t, initialClaimInfoCount, len(task.ResourceClaimInfo), "ResourceClaimInfo count should remain constant after unpipeline")

			// Verify each claim's allocation details are restored to exact initial state
			for claimName, currentAlloc := range task.ResourceClaimInfo {
				initialAlloc := initialAllocations[claimName]

				// Compare allocation content
				if initialAlloc == nil || initialAlloc.Allocation == nil {
					// Initial had no allocation
					if currentAlloc.Allocation != nil {
						assert.Equal(t, 0, len(currentAlloc.Allocation.Devices.Results),
							"Claim %s should have no device allocations after unpipeline (initial had none)", claimName)
					}
				} else {
					// Initial had allocation - verify exact match
					assert.NotNil(t, currentAlloc.Allocation, "Claim %s should have allocation after unpipeline", claimName)
					assert.Equal(t, len(initialAlloc.Allocation.Devices.Results), len(currentAlloc.Allocation.Devices.Results),
						"Claim %s should have same number of devices after unpipeline", claimName)

					// Compare each device allocation
					for i, initialDevice := range initialAlloc.Allocation.Devices.Results {
						currentDevice := currentAlloc.Allocation.Devices.Results[i]
						assert.Equal(t, initialDevice.Device, currentDevice.Device,
							"Claim %s device %d name should match after unpipeline", claimName, i)
						assert.Equal(t, initialDevice.Driver, currentDevice.Driver,
							"Claim %s device %d driver should match after unpipeline", claimName, i)
						assert.Equal(t, initialDevice.Pool, currentDevice.Pool,
							"Claim %s device %d pool should match after unpipeline", claimName, i)
						assert.Equal(t, initialDevice.Request, currentDevice.Request,
							"Claim %s device %d request should match after unpipeline", claimName, i)
					}
				}
			}
		})
	}
}
