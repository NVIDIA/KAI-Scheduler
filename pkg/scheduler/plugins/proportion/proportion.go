// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package proportion

import (
	"math"

	commonconstants "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info/resources"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/reclaimer_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/metrics"
	cp "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/capacity_policy"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/queue_order"
	rec "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/reclaimable"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/resource_division"
	rs "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/resource_share"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/utils"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/scheduler_util"
)

const (
	mebibytes = 1000 * 1000
)

type proportionPlugin struct {
	totalResource       rs.ResourceQuantities
	queues              map[common_info.QueueID]*rs.QueueAttributes
	jobSimulationQueues map[common_info.QueueID]*rs.QueueAttributes
	// Arguments given for the plugin
	pluginArguments   map[string]string
	taskOrderFunc     common_info.LessFn
	reclaimablePlugin *rec.Reclaimable
}

func New(arguments map[string]string) framework.Plugin {
	return &proportionPlugin{
		totalResource:   rs.EmptyResourceQuantities(),
		queues:          map[common_info.QueueID]*rs.QueueAttributes{},
		pluginArguments: arguments,
	}
}

func (pp *proportionPlugin) Name() string {
	return "proportion"
}

func (pp *proportionPlugin) OnSessionOpen(ssn *framework.Session) {
	pp.calculateResourcesProportion(ssn)
	pp.taskOrderFunc = ssn.TaskOrderFn
	pp.reclaimablePlugin = rec.New(ssn.IsInferencePreemptible(), pp.taskOrderFunc)

	capacityPolicy := cp.New(pp.queues, ssn.IsInferencePreemptible())
	ssn.AddQueueOrderFn(pp.queueOrder)
	ssn.AddCanReclaimResourcesFn(pp.CanReclaimResourcesFn)
	ssn.AddReclaimableFn(pp.reclaimableFn)
	ssn.AddOnJobSolutionStartFn(pp.OnJobSolutionStartFn)
	ssn.AddIsNonPreemptibleJobOverQueueQuotaFns(capacityPolicy.IsNonPreemptibleJobOverQuota)
	ssn.AddIsJobOverCapacityFn(capacityPolicy.IsJobOverQueueCapacity)
	ssn.AddIsTaskAllocationOnNodeOverCapacityFn(capacityPolicy.IsTaskAllocationOnNodeOverCapacity)

	// Register event handlers.
	ssn.AddEventHandler(&framework.EventHandler{
		AllocateFunc:   pp.allocateHandlerFn(ssn),
		DeallocateFunc: pp.deallocateHandlerFn(ssn),
	})

	ssn.AddGetQueueAllocatedResourcesFn(pp.getQueueAllocatedResourceFn)
	ssn.AddGetQueueDeservedResourcesFn(pp.getQueueDeservedResourcesFn)
	ssn.AddGetQueueFairShareFn(pp.getQueueFairShareFn)
}

func (pp *proportionPlugin) OnSessionClose(*framework.Session) {
	pp.totalResource = nil
	pp.queues = nil
}

func (pp *proportionPlugin) OnJobSolutionStartFn() {
	pp.jobSimulationQueues = map[common_info.QueueID]*rs.QueueAttributes{}
	for queueId, queue := range pp.queues {
		pp.jobSimulationQueues[queueId] = queue.Clone()
	}
}

func (pp *proportionPlugin) CanReclaimResourcesFn(reclaimer *reclaimer_info.ReclaimerInfo) bool {
	return pp.reclaimablePlugin.CanReclaimResources(pp.queues, reclaimer)
}

func (pp *proportionPlugin) reclaimableFn(
	reclaimer *reclaimer_info.ReclaimerInfo,
	reclaimeeResourcesByQueue map[common_info.QueueID][]*resource_info.Resource,
) bool {
	return pp.reclaimablePlugin.Reclaimable(pp.jobSimulationQueues, reclaimer, reclaimeeResourcesByQueue)
}

func (pp *proportionPlugin) calculateResourcesProportion(ssn *framework.Session) {
	log.InfraLogger.V(6).Infof("Calculating resource proportion")

	pp.setTotalResources(ssn)
	pp.createQueueAttributes(ssn)
	log.InfraLogger.V(3).Infof("Total allocatable resources are <%v>, number of nodes: <%v>, number of "+
		"queues: <%v>", pp.totalResource.String(), len(ssn.Nodes), len(pp.queues))
}

func (pp *proportionPlugin) setTotalResources(ssn *framework.Session) {
	for _, node := range ssn.Nodes {
		pp.totalResource.Add(getNodeResources(ssn, node))
	}
}

func getNodeResources(ssn *framework.Session, node *node_info.NodeInfo) rs.ResourceQuantities {
	nodeResource := rs.EmptyResourceQuantities()

	if !scheduler_util.ValidateIsNodeReady(node.Node) {
		log.InfraLogger.V(2).Infof("Node <%v> is not ready, not counting resource for proportion calculations", node.Name)
		return nodeResource
	}

	_, found := node.Node.Labels[node_info.GpuWorkerNode]
	shouldIgnoreGPUs := ssn.IsRestrictNodeSchedulingEnabled() && !found
	if shouldIgnoreGPUs {
		nodeResource.Add(rs.NewResourceQuantities(node.Allocatable.Cpu(), node.Allocatable.Memory(), 0))
	} else {
		nodeResource.Add(utils.QuantifyResource(node.Allocatable))
		migEnabledGpus := 0
		for resource, qty := range node.Node.Status.Allocatable {
			if resource_info.IsMigResource(resource) {
				gpu, _, err := resources.ExtractGpuAndMemoryFromMigResourceName(string(resource))
				if err != nil {
					log.InfraLogger.Errorf("Failed to extract gpu and memory from mig resource %v: %v", resource, err)
					continue
				}
				migEnabledGpus += int(qty.Value()) * gpu
			}
		}
		nodeResource[rs.GpuResource] += float64(migEnabledGpus)
	}

	// Subtract resources of non-related pods
	schedulerName := ssn.GetSchedulerName()
	for _, podInfo := range node.PodInfos {
		if podInfo.Pod.Spec.SchedulerName != schedulerName &&
			pod_status.IsActiveUsedStatus(podInfo.Status) &&
			!podInfo.IsKaiUtilityPod() {
			log.InfraLogger.V(7).Infof("Pod %s/%s is scheduled by a different scheduler, marking resources as unallocatable "+
				"on node %s", podInfo.Namespace, podInfo.Name, node.Name)
			nodeResource.Sub(utils.QuantifyResourceRequirements(podInfo.ResReq))
		}
	}

	return nodeResource
}

func (pp *proportionPlugin) createQueueAttributes(ssn *framework.Session) {
	pp.createQueueResourceAttrs(ssn)
	pp.updateQueuesCurrentResourceUsage(ssn)
	pp.setFairShare()
}

func (pp *proportionPlugin) createQueueResourceAttrs(ssn *framework.Session) {
	for _, queue := range ssn.Queues {
		queueAttributes := &rs.QueueAttributes{
			UID:               queue.UID,
			Name:              queue.Name,
			ParentQueue:       queue.ParentQueue,
			ChildQueues:       queue.ChildQueues,
			CreationTimestamp: queue.CreationTimestamp,
			QueueResourceShare: rs.QueueResourceShare{
				GPU:    rs.ResourceShare{},
				CPU:    rs.ResourceShare{},
				Memory: rs.ResourceShare{},
			},
			Priority: queue.Priority,
		}
		deserved := queue.Resources.CPU.Quota
		limit := queue.Resources.CPU.Limit
		overQuotaWeight := queue.Resources.CPU.OverQuotaWeight
		queueAttributes.SetQuotaResources(rs.CpuResource, deserved, limit, overQuotaWeight)

		deserved = math.Max(commonconstants.UnlimitedResourceQuantity, queue.Resources.Memory.Quota*mebibytes)
		limit = math.Max(commonconstants.UnlimitedResourceQuantity, queue.Resources.Memory.Limit*mebibytes)
		overQuotaWeight = queue.Resources.Memory.OverQuotaWeight
		queueAttributes.SetQuotaResources(rs.MemoryResource, deserved, limit, overQuotaWeight)

		deserved = queue.Resources.GPU.Quota
		limit = queue.Resources.GPU.Limit
		overQuotaWeight = queue.Resources.GPU.OverQuotaWeight
		queueAttributes.SetQuotaResources(rs.GpuResource, deserved, limit, overQuotaWeight)

		pp.queues[queue.UID] = queueAttributes
		log.InfraLogger.V(7).Infof("Added queue attributes for queue <%s>", queue.Name)
	}
}

func (pp *proportionPlugin) updateQueuesCurrentResourceUsage(ssn *framework.Session) {
	for _, job := range ssn.PodGroupInfos {
		log.InfraLogger.V(7).Infof("Updateding queue consumed resources based on job <%s/%s>.",
			job.Namespace, job.Name)

		for status, tasks := range job.PodStatusIndex {
			if pod_status.AllocatedStatus(status) {
				for _, t := range tasks {
					resources := utils.QuantifyResourceRequirements(t.AcceptedResource)
					isPreemptible := job.IsPreemptibleJob(ssn.IsInferencePreemptible())
					pp.updateQueuesResourceUsageForAllocatedJob(job.Queue, resources, isPreemptible)
				}
			} else if status == pod_status.Pending {
				for _, t := range tasks {
					resources := utils.QuantifyResourceRequirements(t.ResReq)
					pp.updateQueuesResourceUsageForPendingJob(job.Queue, resources)
				}
			}
		}
	}
}

func (pp *proportionPlugin) updateQueuesResourceUsageForAllocatedJob(queueId common_info.QueueID,
	resourceQuantities rs.ResourceQuantities, preemptibleJob bool) {

	for queueAttributes, ok := pp.queues[queueId]; ok; queueAttributes, ok = pp.queues[queueAttributes.ParentQueue] {
		for _, resource := range rs.AllResources {
			qResourceShare := queueAttributes.ResourceShare(resource)
			resourceRequestedQuota := resourceQuantities[resource]

			qResourceShare.Allocated += resourceRequestedQuota
			qResourceShare.Request += resourceRequestedQuota
			if !preemptibleJob {
				qResourceShare.AllocatedNotPreemptible += resourceRequestedQuota
			}
		}
	}
}

func (pp *proportionPlugin) updateQueuesResourceUsageForPendingJob(queueId common_info.QueueID,
	resourceQuantities rs.ResourceQuantities) {

	for queueAttributes, ok := pp.queues[queueId]; ok; queueAttributes, ok = pp.queues[queueAttributes.ParentQueue] {
		for _, resource := range rs.AllResources {
			qResourceShare := queueAttributes.ResourceShare(resource)
			resourceRequestedQuota := resourceQuantities[resource]
			qResourceShare.Request += resourceRequestedQuota
		}
	}
}

func (pp *proportionPlugin) setFairShare() {
	topQueues := pp.getTopQueues()
	metrics.ResetQueueFairShare()
	pp.setFairShareForQueues(pp.totalResource, topQueues)
}

func (pp *proportionPlugin) setFairShareForQueues(totalResources rs.ResourceQuantities,
	queues map[common_info.QueueID]*rs.QueueAttributes) {

	if len(queues) == 0 {
		return
	}

	resource_division.SetResourcesShare(totalResources, queues)
	for _, queue := range queues {
		childQueues := pp.getChildQueues(queue)
		resources := queue.GetFairShare()
		pp.setFairShareForQueues(resources, childQueues)
	}
}

func (pp *proportionPlugin) getTopQueues() map[common_info.QueueID]*rs.QueueAttributes {
	topQueues := map[common_info.QueueID]*rs.QueueAttributes{}
	for _, queue := range pp.queues {
		if len(queue.ParentQueue) == 0 {
			topQueues[queue.UID] = queue
		}
	}
	return topQueues
}

func (pp *proportionPlugin) getChildQueues(parentQueue *rs.QueueAttributes) map[common_info.QueueID]*rs.QueueAttributes {
	childQueues := map[common_info.QueueID]*rs.QueueAttributes{}
	for _, queueId := range parentQueue.ChildQueues {
		childQueues[queueId] = pp.queues[queueId]
	}
	return childQueues
}

func (pp *proportionPlugin) allocateHandlerFn(ssn *framework.Session) func(event *framework.Event) {
	return func(event *framework.Event) {
		job := ssn.PodGroupInfos[event.Task.Job]
		isPreemptibleJob := job.IsPreemptibleJob(ssn.IsInferencePreemptible())
		taskResources := utils.QuantifyResourceRequirements(event.Task.AcceptedResource)

		for queue, ok := pp.queues[job.Queue]; ok; queue, ok = pp.queues[queue.ParentQueue] {
			for _, resource := range rs.AllResources {
				resourceShare := queue.ResourceShare(resource)
				resourceShare.Allocated += taskResources[resource]

				if !isPreemptibleJob {
					resourceShare.AllocatedNotPreemptible += taskResources[resource]
				}
			}
		}

		leafQueue := pp.queues[job.Queue]
		log.InfraLogger.V(7).Infof("Proportion AllocateFunc: job <%v/%v>, task resources <%v>, "+
			"queue: <%v>, queue allocated resources: <%v>",
			job.Namespace, job.Name, taskResources, leafQueue.Name, leafQueue.GetAllocatedShare())
	}
}

func (pp *proportionPlugin) deallocateHandlerFn(ssn *framework.Session) func(event *framework.Event) {
	return func(event *framework.Event) {
		job := ssn.PodGroupInfos[event.Task.Job]
		isPreemptibleJob := job.IsPreemptibleJob(ssn.IsInferencePreemptible())
		taskResources := utils.QuantifyResourceRequirements(event.Task.AcceptedResource)

		for queue, ok := pp.queues[job.Queue]; ok; queue, ok = pp.queues[queue.ParentQueue] {
			for _, resource := range rs.AllResources {
				resourceShare := queue.ResourceShare(resource)
				resourceShare.Allocated -= taskResources[resource]

				if !isPreemptibleJob {
					resourceShare.AllocatedNotPreemptible -= taskResources[resource]
				}
			}
		}

		leafQueue := pp.queues[job.Queue]
		log.InfraLogger.V(7).Infof("Proportion DeallocateFunc: job <%v/%v>, task resources <%v>, "+
			"queue: <%v>, queue allocated resources: <%v>",
			job.Namespace, job.Name, taskResources.String(), leafQueue.Name, leafQueue.GetAllocatedShare())
	}
}

// CompareQueueFn
func (pp *proportionPlugin) queueOrder(lQ, rQ *queue_info.QueueInfo, lJob, rJob *podgroup_info.PodGroupInfo, _, _ []*podgroup_info.PodGroupInfo) int {
	lQueueAttributes, found := pp.queues[lQ.UID]
	if !found {
		log.InfraLogger.Errorf("Failed to find queue: <%v>", lQ.Name)
		return 1
	}

	rQueueAttributes, found := pp.queues[rQ.UID]
	if !found {
		log.InfraLogger.Errorf("Failed to find queue: <%v>", rQ.Name)
		return -1
	}

	return queue_order.GetQueueOrderResult(lQueueAttributes, rQueueAttributes, lJob, rJob, pp.taskOrderFunc, pp.totalResource)
}

func (pp *proportionPlugin) getQueueDeservedResourcesFn(queue *queue_info.QueueInfo) *resource_info.ResourceRequirements {
	queueAttributes := pp.queues[queue.UID]
	return utils.ResourceRequirementsFromQuantities(queueAttributes.GetDeservedShare())
}

func (pp *proportionPlugin) getQueueFairShareFn(queue *queue_info.QueueInfo) *resource_info.ResourceRequirements {
	queueAttributes := pp.queues[queue.UID]
	return utils.ResourceRequirementsFromQuantities(queueAttributes.GetFairShare())
}

func (pp *proportionPlugin) getQueueAllocatedResourceFn(queue *queue_info.QueueInfo) *resource_info.ResourceRequirements {
	queueAttributes := pp.queues[queue.UID]
	return utils.ResourceRequirementsFromQuantities(queueAttributes.GetAllocatedShare())
}
