// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package subgroup_info

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
)

type wantGroup struct {
	Name    string
	Groups  []*wantGroup
	PodSets []*wantPodSet
}

type wantPodSet struct {
	Name      string
	MinMember int32
}

func checkGroupStructure(t *testing.T, got *SubGroupSet, want *wantGroup) {
	if got == nil {
		t.Fatalf("expected SubGroupSet %q, got nil", want.Name)
	}
	if got.GetName() != want.Name {
		t.Fatalf("expected SubGroupSet name %q, got %q", want.Name, got.GetName())
	}
	// Check PodSets
	if len(got.podSets) != len(want.PodSets) {
		t.Errorf("SubGroupSet %q: expected %d podSets, got %d", want.Name, len(want.PodSets), len(got.podSets))
	} else {
		used := make([]bool, len(got.podSets))
		for _, wp := range want.PodSets {
			found := false
			for i, gp := range got.podSets {
				if used[i] {
					continue
				}
				if gp.GetName() == wp.Name && gp.GetMinAvailable() == wp.MinMember {
					used[i] = true
					found = true
					break
				}
			}
			if !found {
				t.Errorf("SubGroupSet %q: expected podSet %q with MinMember %d not found", want.Name, wp.Name, wp.MinMember)
			}
		}
	}
	// Check children groups (SubGroupSets)
	if len(got.groups) != len(want.Groups) {
		t.Errorf("SubGroupSet %q: expected %d child groups, got %d", want.Name, len(want.Groups), len(got.groups))
		return
	}
	for _, wantChild := range want.Groups {
		found := false
		for _, gotChild := range got.groups {
			if gotChild.GetName() == wantChild.Name {
				checkGroupStructure(t, gotChild, wantChild)
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SubGroupSet %q: expected group child %q not found among %v", want.Name, wantChild.Name, childrenNames(got.groups))
		}
	}
}

func childrenNames(groups []*SubGroupSet) []string {
	names := make([]string, 0, len(groups))
	for _, g := range groups {
		names = append(names, g.GetName())
	}
	return names
}

func TestFromPodGroup_FullTree(t *testing.T) {
	tests := []struct {
		name     string
		podGroup *v2alpha2.PodGroup
		want     *wantGroup
		wantErr  string // substring match
	}{
		{
			name: "simple two-level",
			podGroup: &v2alpha2.PodGroup{
				ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "jobA"},
				Spec: v2alpha2.PodGroupSpec{
					SubGroups: []v2alpha2.SubGroup{
						{Name: "sg1", Parent: nil},
						{Name: "sg2", Parent: ptr.To("sg1"), MinMember: 3},
					},
				},
			},
			want: &wantGroup{
				Name: "ns1/jobA",
				Groups: []*wantGroup{
					{
						Name:    "sg1",
						Groups:  nil,
						PodSets: []*wantPodSet{{Name: "sg2", MinMember: 3}},
					},
				},
				PodSets: nil,
			},
		},
		{
			name: "hierarchy with one leaf",
			podGroup: &v2alpha2.PodGroup{
				ObjectMeta: metav1.ObjectMeta{Namespace: "ns2", Name: "jobB"},
				Spec: v2alpha2.PodGroupSpec{
					SubGroups: []v2alpha2.SubGroup{
						{Name: "rootchild", Parent: nil},
						{Name: "middle", Parent: ptr.To("rootchild")},
						{Name: "leaf", Parent: ptr.To("middle"), MinMember: 5},
					},
				},
			},
			want: &wantGroup{
				Name: "ns2/jobB",
				Groups: []*wantGroup{
					{
						Name: "rootchild",
						Groups: []*wantGroup{
							{
								Name:    "middle",
								Groups:  nil,
								PodSets: []*wantPodSet{{Name: "leaf", MinMember: 5}},
							},
						},
						PodSets: nil,
					},
				},
				PodSets: nil,
			},
		},
		{
			name: "empty subgroups",
			podGroup: &v2alpha2.PodGroup{
				ObjectMeta: metav1.ObjectMeta{Namespace: "ns3", Name: "empty"},
				Spec:       v2alpha2.PodGroupSpec{SubGroups: nil},
			},
			want: &wantGroup{
				Name:    "ns3/empty",
				Groups:  nil,
				PodSets: nil,
			},
		},
		{
			name: "parent not found",
			podGroup: &v2alpha2.PodGroup{
				ObjectMeta: metav1.ObjectMeta{Namespace: "ns4", Name: "bad"},
				Spec: v2alpha2.PodGroupSpec{
					SubGroups: []v2alpha2.SubGroup{
						{Name: "sg1", Parent: ptr.To("nonexistent"), MinMember: 2},
					},
				},
			},
			want:    nil,
			wantErr: "parent subgroup <nonexistent> of <sg1> not found",
		},
		{
			name: "subgroup set parent not found",
			podGroup: &v2alpha2.PodGroup{
				ObjectMeta: metav1.ObjectMeta{Namespace: "ns5", Name: "bad"},
				Spec: v2alpha2.PodGroupSpec{
					SubGroups: []v2alpha2.SubGroup{
						{Name: "p1", Parent: nil},
						{Name: "c1", Parent: ptr.To("p1")},
						{Name: "c2", Parent: ptr.To("no_such_set"), MinMember: 1},
					},
				},
			},
			want:    nil,
			wantErr: "parent subgroup <no_such_set> of <c2> not found",
		},
		{
			name: "duplicate subgroup names",
			podGroup: &v2alpha2.PodGroup{
				ObjectMeta: metav1.ObjectMeta{Namespace: "ns6", Name: "dup"},
				Spec: v2alpha2.PodGroupSpec{
					SubGroups: []v2alpha2.SubGroup{
						{Name: "sg", Parent: nil},
						{Name: "sg", Parent: ptr.To("sg"), MinMember: 1},
					},
				},
			},
			want:    nil,
			wantErr: "subgroup <sg> already exists",
		},
		{
			name: "parent of deep child not found",
			podGroup: &v2alpha2.PodGroup{
				ObjectMeta: metav1.ObjectMeta{Namespace: "ns-missing", Name: "missingparent"},
				Spec: v2alpha2.PodGroupSpec{
					SubGroups: []v2alpha2.SubGroup{
						{Name: "root", Parent: nil},
						{Name: "c1", Parent: ptr.To("root")},
						{Name: "c2", Parent: ptr.To("doesnotexist"), MinMember: 4},
					},
				},
			},
			want:    nil,
			wantErr: "parent subgroup <doesnotexist> of <c2> not found",
		},
		{
			name: "intermediate subgroup parent not found",
			podGroup: &v2alpha2.PodGroup{
				ObjectMeta: metav1.ObjectMeta{Namespace: "ns-intermediate", Name: "missingintermediate"},
				Spec: v2alpha2.PodGroupSpec{
					SubGroups: []v2alpha2.SubGroup{
						{Name: "parent", Parent: nil},
						{Name: "mid", Parent: ptr.To("no_such_parent")},
						{Name: "leaf", Parent: ptr.To("mid"), MinMember: 1},
					},
				},
			},
			want:    nil,
			wantErr: "parent subgroup <no_such_parent> of <mid> not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, err := FromPodGroup(tt.podGroup)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if got, want := err.Error(), tt.wantErr; !strings.Contains(got, want) {
					t.Errorf("expected error containing %q, got %q", want, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.want == nil && root != nil {
				t.Fatalf("expected no SubGroupSet, got one: %#v", root)
			}
			if tt.want != nil {
				checkGroupStructure(t, root, tt.want)
			}
		})
	}
}
