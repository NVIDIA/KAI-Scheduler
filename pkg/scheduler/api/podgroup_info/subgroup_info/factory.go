// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package subgroup_info

import (
	"fmt"
	"strings"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/topology_info"
)

const RootSubGroupSetName = ""

func FromPodGroup(podGroup *v2alpha2.PodGroup) (*SubGroupSet, error) {
	allSubGroups := map[string]*v2alpha2.SubGroup{}
	children := map[string][]string{}

	for _, subGroup := range podGroup.Spec.SubGroups {
		if _, found := allSubGroups[subGroup.Name]; found {
			return nil, fmt.Errorf("subgroup <%s> already exists", subGroup.Name)
		}
		allSubGroups[subGroup.Name] = &subGroup
		parentName := formatParentName(subGroup.Parent)
		children[parentName] = append(children[parentName], subGroup.Name)
	}

	var topologyConstraint *topology_info.TopologyConstraintInfo
	if podGroup.Spec.TopologyConstraint.Topology != "" {
		topologyConstraint = &topology_info.TopologyConstraintInfo{
			Topology:       podGroup.Spec.TopologyConstraint.Topology,
			RequiredLevel:  podGroup.Spec.TopologyConstraint.RequiredTopologyLevel,
			PreferredLevel: podGroup.Spec.TopologyConstraint.PreferredTopologyLevel,
		}
	}
	root := NewSubGroupSet(RootSubGroupSetName, topologyConstraint)
	subGroupSets := map[string]*SubGroupSet{
		RootSubGroupSetName: root,
	}
	podSets := map[string]*PodSet{}

	for name, subGroup := range allSubGroups {
		var topologyConstrainInfo *topology_info.TopologyConstraintInfo
		if subGroup.TopologyConstraint != nil {
			topologyConstrainInfo = &topology_info.TopologyConstraintInfo{
				Topology:       subGroup.TopologyConstraint.Topology,
				RequiredLevel:  subGroup.TopologyConstraint.RequiredTopologyLevel,
				PreferredLevel: subGroup.TopologyConstraint.PreferredTopologyLevel,
			}
		}
		_, hasChildren := children[name]
		if hasChildren {
			subGroupSets[name] = NewSubGroupSet(name, topologyConstrainInfo)
		} else {
			podSets[name] = NewPodSet(name, subGroup.MinMember, topologyConstrainInfo)
		}
	}

	for name, podSet := range podSets {
		subGroup := allSubGroups[name]
		if err := addPodSetToParent(podSet, subGroup.Parent, subGroupSets); err != nil {
			return nil, err
		}
	}

	for name, subGroupSet := range subGroupSets {
		if name == RootSubGroupSetName {
			continue
		}

		subGroup := allSubGroups[name]
		if err := addSubGroupSetToParent(subGroupSet, subGroup.Parent, subGroupSets); err != nil {
			return nil, err
		}
	}

	return root, nil
}

func addSubGroupSetToParent(subGroupSet *SubGroupSet, parentName *string, subGroupSets map[string]*SubGroupSet) error {
	parent := formatParentName(parentName)
	parentSubGroupSet, found := subGroupSets[parent]
	if !found {
		return fmt.Errorf("parent subgroup <%s> of <%s> not found", parent, subGroupSet.GetName())
	}

	parentSubGroupSet.AddSubGroup(subGroupSet)
	return nil
}

func addPodSetToParent(podSet *PodSet, parentName *string, subGroupSets map[string]*SubGroupSet) error {
	parent := formatParentName(parentName)
	parentSubGroupSet, found := subGroupSets[parent]
	if !found {
		return fmt.Errorf("parent subgroup <%s> of <%s> not found", parent, podSet.GetName())
	}

	parentSubGroupSet.AddPodSet(podSet)
	return nil
}

func formatParentName(parentName *string) string {
	if parentName == nil {
		return RootSubGroupSetName
	}
	return strings.ToLower(*parentName)
}
