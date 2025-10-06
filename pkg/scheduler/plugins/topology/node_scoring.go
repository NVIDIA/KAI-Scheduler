package topology

import (
	"fmt"
	"math"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/scores"
)

func (t *topologyPlugin) nodePreOrderFn(task *pod_info.PodInfo, nodes []*node_info.NodeInfo) error {
	job := t.getTaskJob(task)
	if job.TopologyConstraint == nil || job.TopologyConstraint.PreferredLevel == "" {
		return nil
	}

	domain, ok := t.nodeSetToDomain[job.TopologyConstraint.Topology][getNodeSetID(nodes)]
	if !ok {
		return fmt.Errorf("domain for node set %s not found", getNodeSetID(nodes))
	}

	// Avoid recalculating node scores for the same domain
	if t.jobDomainNodeScores[job.UID].domainID == domain.ID {
		return nil
	}

	t.jobDomainNodeScores[job.UID] = domainNodeScores{
		domainID:   domain.ID,
		nodeScores: calculateNodeScores(domain, DomainLevel(job.TopologyConstraint.PreferredLevel)),
	}

	return nil
}

func (t *topologyPlugin) nodeOrderFn(task *pod_info.PodInfo, node *node_info.NodeInfo) (float64, error) {
	job := t.getTaskJob(task)
	if job.TopologyConstraint == nil || job.TopologyConstraint.PreferredLevel == "" {
		return 0, nil
	}

	if t.jobDomainNodeScores[job.UID].domainID == "" {
		return 0, fmt.Errorf("node scores for job %s not found", task.Job)
	}

	return t.jobDomainNodeScores[job.UID].nodeScores[node.Name], nil
}

func calculateNodeScores(domain *DomainInfo, preferredLevel DomainLevel) map[string]float64 {
	orderedPreferredLevelDomains := getLevelDomains(domain, preferredLevel)

	nodeScores := make(map[string]float64)
	for i, leafDomain := range orderedPreferredLevelDomains {
		for _, node := range leafDomain.Nodes {
			// Score nodes by their domain's order
			score := (float64(len(orderedPreferredLevelDomains)-i) / float64(len(orderedPreferredLevelDomains))) * 10
			// Round down the score to the nearest integer to prevent interference between plugins
			// (ensures topology scores remain higher than other lower-priority plugin scores)
			normalizedScore := math.Floor(score) * scores.Topology
			nodeScores[node.Name] = normalizedScore
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
