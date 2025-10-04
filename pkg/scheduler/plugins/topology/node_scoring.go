package topology

import (
	"fmt"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/scores"
)

func (t *topologyPlugin) nodePreOrderFn(task *pod_info.PodInfo, nodes []*node_info.NodeInfo) error {
	job := t.session.PodGroupInfos[task.Job]

	domain, ok := t.nodeSetToDomain[job.TopologyConstraint.Topology][getNodeSetID(nodes)]
	if !ok {
		return fmt.Errorf("domain for node set %s not found", getNodeSetID(nodes))
	}

	t.jobNodeScores[job.UID] = calculateNodeScores(domain)

	return nil
}

func (t *topologyPlugin) nodeOrderFn(task *pod_info.PodInfo, node *node_info.NodeInfo) (float64, error) {
	return t.jobNodeScores[task.Job][node.Name], nil
}

func calculateNodeScores(domain *DomainInfo) map[string]float64 {
	orderedLeafDomains := getOrderedLeafDomains(domain)

	nodeScores := make(map[string]float64)
	for i, leafDomain := range orderedLeafDomains {
		for _, node := range leafDomain.Nodes {
			// Score nodes by their domain's order
			nodeScores[node.Name] = (float64(len(orderedLeafDomains)-i) / float64(len(orderedLeafDomains))) * scores.Topology
		}
	}

	return nodeScores
}

func getOrderedLeafDomains(domain *DomainInfo) []*DomainInfo {
	orderedLeafDomains := []*DomainInfo{}
	if len(domain.Children) == 0 {
		return append(orderedLeafDomains, domain)
	}

	for _, childDomain := range domain.Children {
		orderedLeafDomains = append(orderedLeafDomains, getOrderedLeafDomains(childDomain)...)
	}
	return orderedLeafDomains
}
