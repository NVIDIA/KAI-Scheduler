// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package reflectjoborder

import (
	"fmt"
	"hash"
	"hash/fnv"
	"maps"
	"slices"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
)

// computeFingerprint returns a 64-bit FNV-1a hash over session state that affects job ordering.
func computeFingerprint(ssn *framework.Session) uint64 {
	h := fnv.New64a()

	// PodGroupInfos: UID, Priority, Queue, CreationTimestamp (tie-breaker in JobOrderFn),
	// IsReadyForScheduling, PodStatusIndex counts
	for _, id := range slices.Sorted(maps.Keys(ssn.ClusterInfo.PodGroupInfos)) {
		pg := ssn.ClusterInfo.PodGroupInfos[id]
		fmt.Fprintf(h, "pg\x00%s\x00%d\x00%s\x00%d\x00%t\x00",
			pg.UID, pg.Priority, pg.Queue, pg.CreationTimestamp.UnixNano(), pg.IsReadyForScheduling())
		for _, status := range slices.Sorted(maps.Keys(pg.PodStatusIndex)) {
			fmt.Fprintf(h, "ps\x00%d\x00%d\x00", status, len(pg.PodStatusIndex[status]))
		}
	}

	// QueueInfos: UID, Priority, ParentQueue, ChildQueues, CreationTimestamp
	// (tie-breaker in prioritizeBasedOnCreationTime), resource quotas
	for _, id := range slices.Sorted(maps.Keys(ssn.ClusterInfo.Queues)) {
		q := ssn.ClusterInfo.Queues[id]
		fmt.Fprintf(h, "q\x00%s\x00%d\x00%s\x00%d\x00",
			q.UID, q.Priority, q.ParentQueue, q.CreationTimestamp.UnixNano())
		for _, cid := range slices.Sorted(slices.Values(q.ChildQueues)) {
			fmt.Fprintf(h, "cq\x00%s\x00", cid)
		}
		hashResourceQuota(h, q.Resources)
	}

	// QueueResourceUsage
	for _, qid := range slices.Sorted(maps.Keys(ssn.ClusterInfo.QueueResourceUsage.Queues)) {
		usage := ssn.ClusterInfo.QueueResourceUsage.Queues[qid]
		fmt.Fprintf(h, "qu\x00%s\x00", qid)
		for _, rn := range slices.Sorted(maps.Keys(usage)) {
			fmt.Fprintf(h, "%s\x00%v\x00", rn, usage[rn])
		}
	}

	// Nodes: name + GPU capacity (affects DRF)
	for _, name := range slices.Sorted(maps.Keys(ssn.ClusterInfo.Nodes)) {
		fmt.Fprintf(h, "n\x00%s\x00%v\x00", name, ssn.ClusterInfo.Nodes[name].Allocatable.GPUs())
	}

	// JobsDepth config
	fmt.Fprintf(h, "jd\x00%d", ssn.GetJobsDepth(framework.Allocate))

	return h.Sum64()
}

func hashResourceQuota(h hash.Hash64, rq queue_info.QueueQuota) {
	fmt.Fprintf(h, "rq\x00%v\x00%v\x00%v\x00%v\x00%v\x00%v\x00%v\x00%v\x00%v\x00",
		rq.GPU.Quota, rq.GPU.Limit, rq.GPU.OverQuotaWeight,
		rq.CPU.Quota, rq.CPU.Limit, rq.CPU.OverQuotaWeight,
		rq.Memory.Quota, rq.Memory.Limit, rq.Memory.OverQuotaWeight)
}
