package topology

import (
	"fmt"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
)

// Add a NodePreOrderFn to the session that calculates the topology tree for the job, and calculates min and max pod capacity for each domain level for that job.
// Add a NodeOrderFn to the session that scores nodes for task placement based on the topology tree (parent domain is higher priority than child domains)

func (t *topologyPlugin) nodePreOrderFn(task *pod_info.PodInfo, nodes []*node_info.NodeInfo) error {
	job := t.session.PodGroupInfos[task.Job]

	// Find the domain for the node set
	domain, ok := t.nodeSetToDomain[job.TopologyConstraint.Topology][getNodeSetID(nodes)]
	if !ok {
		return fmt.Errorf("domain for node set %s not found", getNodeSetID(nodes))
	}

	// Assuming the domain's tree is already sorted by the allocatable pods, traverse the leaves in-order

	return nil
}

func (t *topologyPlugin) nodeOrderFn(task *pod_info.PodInfo, node *node_info.NodeInfo) (float64, error) {
	return 0, nil
}
