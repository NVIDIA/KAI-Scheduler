package topology

import (
	"fmt"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/scores"
)

func (t *topologyPlugin) nodePreOrderFn(task *pod_info.PodInfo, nodes []*node_info.NodeInfo) error {
	job := t.getTaskJob(task)

	if t.jobNodeScores[job.UID] != nil || job.TopologyConstraint == nil || job.TopologyConstraint.PreferredLevel == "" {
		return nil
	}

	preferredLevel := job.TopologyConstraint.PreferredLevel

	domain, ok := t.nodeSetToDomain[job.TopologyConstraint.Topology][getNodeSetID(nodes)]
	if !ok {
		return fmt.Errorf("domain for node set %s not found", getNodeSetID(nodes))
	}

	t.jobNodeScores[job.UID] = calculateNodeScores(domain, DomainLevel(preferredLevel))

	return nil
}

func (t *topologyPlugin) nodeOrderFn(task *pod_info.PodInfo, node *node_info.NodeInfo) (float64, error) {
	job := t.getTaskJob(task)

	if job.TopologyConstraint == nil || job.TopologyConstraint.PreferredLevel == "" {
		return 0, nil
	}

	if t.jobNodeScores[job.UID] == nil {
		return 0, fmt.Errorf("node scores for job %s not found", task.Job)
	}

	return t.jobNodeScores[job.UID][node.Name], nil
}

func calculateNodeScores(domain *DomainInfo, preferredLevel DomainLevel) map[string]float64 {
	orderedPreferredLevelDomains := getLevelDomains(domain, preferredLevel)

	nodeScores := make(map[string]float64)
	for i, leafDomain := range orderedPreferredLevelDomains {
		for _, node := range leafDomain.Nodes {
			// Score nodes by their domain's order
			nodeScores[node.Name] = (float64(len(orderedPreferredLevelDomains)-i) / float64(len(orderedPreferredLevelDomains))) * scores.Topology
		}
	}

	return nodeScores
}

func getLevelDomains(root *DomainInfo, level DomainLevel) []*DomainInfo {
	if root.Level == level {
		return []*DomainInfo{root}
	}
	if len(root.Children) == 0 {
		return []*DomainInfo{}
	}

	levelDomains := []*DomainInfo{}
	for _, childDomain := range root.Children {
		levelDomains = append(levelDomains, getLevelDomains(childDomain, level)...)
	}
	return levelDomains
}

func (t *topologyPlugin) getTaskJob(task *pod_info.PodInfo) *podgroup_info.PodGroupInfo {
	return t.session.PodGroupInfos[task.Job]
}
