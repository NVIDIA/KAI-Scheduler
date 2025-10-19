// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package topology

import (
	"fmt"
	"math"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info/subgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/scores"
)

func (t *topologyPlugin) nodePreOrderFn(task *pod_info.PodInfo, nodes []*node_info.NodeInfo) error {
	// GuyTodo: Remove function
	return nil

	sgi, err := t.getTaskSubGroupInfo(task)
	if err != nil {
		return err
	}

	// GuyDebug: Print debug log
	fmt.Printf("Calculating node scores for task %s/%s in sub-group %s on %d nodes\n",
		task.Namespace, task.Name, sgi.GetName(), len(nodes))

	// GuyTodo: Calculate whether the subgroup has a preferred topology level (consider hierarchy of levels)

	// GuyTodo: Make sure the job Topology is the same as the task's Topology (validate with Hagay we don't allow different topologies in a job)
	topologyConstraint := sgi.GetTopologyConstraint()
	if topologyConstraint == nil || topologyConstraint.PreferredLevel == "" {
		return nil
	}

	// GuyDebug: Print debug log
	fmt.Printf("Calculating node scores for task %s/%s in sub-group %s with topology constraint %+v on %d nodes\n",
		task.Namespace, task.Name, sgi.GetName(), topologyConstraint, len(nodes))

	domain, ok := t.nodeSetToDomain[topologyConstraint.Topology][getNodeSetID(nodes)]
	if !ok || domain == nil {
		return fmt.Errorf("domain for node set %s not found", getNodeSetID(nodes))
	}

	// Avoid recalculating node scores for the same domain
	if t.subGroupDomainNodeScores[sgi.GetName()].domainID == domain.ID {
		return nil
	}

	t.subGroupDomainNodeScores[sgi.GetName()] = domainNodeScores{
		domainID:   domain.ID,
		nodeScores: calculateNodeScores(domain, DomainLevel(topologyConstraint.PreferredLevel)),
	}

	return nil
}

func (t *topologyPlugin) nodeOrderFn(task *pod_info.PodInfo, node *node_info.NodeInfo) (float64, error) {
	taskSubGroupInfo, err := t.getTaskSubGroupInfo(task)
	if err != nil {
		return 0, fmt.Errorf("failed to get sub-group info for task %s/%s: %v", task.Namespace, task.Name, err)
	}

	relevantNodeScores := t.getRelevantNodeScores(taskSubGroupInfo)
	if relevantNodeScores == nil {
		// Sub-group or any of its ancestors has not defined any node scores
		return 0, nil
	}

	score, ok := relevantNodeScores[node.Name]
	if !ok {
		return 0, fmt.Errorf("node %s not found in relevant node scores for sub-group %s", node.Name, taskSubGroupInfo.GetName())
	}

	return score, nil
}

func calculateNodeScores(domain *DomainInfo, preferredLevel DomainLevel) map[string]float64 {
	orderedPreferredLevelDomains := getLevelDomains(domain, preferredLevel)

	nodeScores := make(map[string]float64)
	for i, leafDomain := range orderedPreferredLevelDomains {
		for _, node := range leafDomain.Nodes {
			// Score nodes by their domain's order
			score := (float64(i+1) / float64(len(orderedPreferredLevelDomains))) * 10
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

func (t *topologyPlugin) getTaskSubGroupInfo(task *pod_info.PodInfo) (*subgroup_info.SubGroupInfo, error) {
	job := t.session.PodGroupInfos[task.Job]
	if job == nil {
		return nil, fmt.Errorf("job %s not found for task %s/%s", task.Job, task.Namespace, task.Name)
	}

	sgName := t.getTaskSubGroupName(task)
	sg, found := job.GetSubGroups()[sgName]
	if !found {
		return nil, fmt.Errorf("sub-group %s not found in job %s", sgName, job.Name)
	}

	return &sg.SubGroupInfo, nil
}

// GuyTodo: Move to common location (podgroup_info?)
func (t *topologyPlugin) getTaskSubGroupName(task *pod_info.PodInfo) string {
	sgName := podgroup_info.DefaultSubGroup
	if task.SubGroupName != "" {
		sgName = task.SubGroupName
	}
	return sgName
}

func (t *topologyPlugin) getRelevantNodeScores(sgi *subgroup_info.SubGroupInfo) map[string]float64 {
	if scores, ok := t.subGroupNodeScores[sgi.GetName()]; ok {
		return scores
	}

	if sgi.GetParent() == nil {
		return nil
	}

	return t.getRelevantNodeScores(&sgi.GetParent().SubGroupInfo)
}
