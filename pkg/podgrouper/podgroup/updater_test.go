// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package podgroup

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	enginev2alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
)

func Test_mergeTwoMaps(t *testing.T) {
	type args struct {
		org     map[string]string
		toMerge map[string]string
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			"Merge empty map",
			args{
				map[string]string{"A": "a"},
				map[string]string{},
			},
			map[string]string{"A": "a"},
		},
		{
			"Merge into empty map",
			args{
				map[string]string{},
				map[string]string{"A": "a"},
			},
			map[string]string{"A": "a"},
		},
		{
			"Merge two map - no common keys",
			args{
				map[string]string{"B": "b"},
				map[string]string{"A": "a"},
			},
			map[string]string{"A": "a", "B": "b"},
		},
		{
			"Merge two map - override common key",
			args{
				map[string]string{"B": "b", "A": "b"},
				map[string]string{"A": "a"},
			},
			map[string]string{"A": "a", "B": "b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeTwoMaps(tt.args.org, tt.args.toMerge); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeTwoMaps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_podGroupsEqual(t *testing.T) {
	type args struct {
		leftPodGroup  *enginev2alpha2.PodGroup
		rightPodGroup *enginev2alpha2.PodGroup
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"Two identical pod groups",
			args{
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{},
						Labels:      map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 1,
					},
				},
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{},
						Labels:      map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 1,
					},
				},
			},
			true,
		},
		{
			"Different owner reference",
			args{
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{},
						Labels:      map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 1,
					},
				},
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{},
						Labels:      map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o2",
								UID:  "32764823",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 1,
					},
				},
			},
			false,
		},
		{
			"Different spec",
			args{
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{},
						Labels:      map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 2,
					},
				},
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{},
						Labels:      map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 1,
					},
				},
			},
			false,
		},
		{
			"Different annotations",
			args{
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{"A": "a"},
						Labels:      map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 2,
					},
				},
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{},
						Labels:      map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 1,
					},
				},
			},
			false,
		},
		{
			"Different annotations - blocklisted",
			args{
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{pgWorkloadStatus: "Pending"},
						Labels:      map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 1,
					},
				},
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{pgWorkloadStatus: "Running"},
						Labels:      map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 1,
					},
				},
			},
			true,
		},
		{
			"Different labels",
			args{
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{},
						Labels:      map[string]string{"L2": "l2"},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 1,
					},
				},
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{},
						Labels:      map[string]string{"L1": "l1"},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 1,
					},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := podGroupsEqual(tt.args.leftPodGroup, tt.args.rightPodGroup); got != tt.want {
				t.Errorf("podGroupsEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_removeBlocklistedKeys(t *testing.T) {
	type args struct {
		inputAnnotations map[string]string
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			"No blockListed keys",
			args{
				map[string]string{
					"A": "a",
				},
			},
			map[string]string{
				"A": "a",
			},
		},
		{
			"Only blockListed keys",
			args{
				map[string]string{
					pgWorkloadStatus:            "Running",
					pgRunningPodsAnnotationName: "2",
				},
			},
			map[string]string{},
		},
		{
			"Mixed",
			args{
				map[string]string{
					pgWorkloadStatus:            "Running",
					pgRunningPodsAnnotationName: "2",
					"NotBlocklisted":            "abkjh",
				},
			},
			map[string]string{"NotBlocklisted": "abkjh"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removeBlocklistedKeys(tt.args.inputAnnotations); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("removeBlocklistedKeys() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_updatePodGroup(t *testing.T) {
	type args struct {
		oldPodGroup *enginev2alpha2.PodGroup
		newPodGroup *enginev2alpha2.PodGroup
	}
	tests := []struct {
		name                   string
		args                   args
		expectedPodGroupChange *enginev2alpha2.PodGroup
	}{
		{
			"Two identical pod groups",
			args{
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{},
						Labels:      map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 1,
					},
				},
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{},
						Labels:      map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 1,
					},
				},
			},
			&enginev2alpha2.PodGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "p1",
					Namespace:   "n1",
					Annotations: map[string]string{},
					Labels:      map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							Name: "o1",
							UID:  "81726381762",
						},
					},
				},
				Spec: enginev2alpha2.PodGroupSpec{
					MinMember: 1,
				},
			},
		},
		{
			"Different owner reference",
			args{
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{},
						Labels:      map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 1,
					},
				},
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{},
						Labels:      map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o2",
								UID:  "32764823",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 1,
					},
				},
			},
			&enginev2alpha2.PodGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "p1",
					Namespace:   "n1",
					Annotations: map[string]string{},
					Labels:      map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							Name: "o2",
							UID:  "32764823",
						},
					},
				},
				Spec: enginev2alpha2.PodGroupSpec{
					MinMember: 1,
				},
			},
		},
		{
			"Different spec",
			args{
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{},
						Labels:      map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 2,
					},
				},
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{},
						Labels:      map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 1,
					},
				},
			},
			&enginev2alpha2.PodGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "p1",
					Namespace:   "n1",
					Annotations: map[string]string{},
					Labels:      map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							Name: "o1",
							UID:  "81726381762",
						},
					},
				},
				Spec: enginev2alpha2.PodGroupSpec{
					MinMember: 2,
				},
			},
		},
		{
			"Different annotations",
			args{
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{"A": "a"},
						Labels:      map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 2,
					},
				},
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{"B": "b"},
						Labels:      map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 1,
					},
				},
			},
			&enginev2alpha2.PodGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "p1",
					Namespace:   "n1",
					Annotations: map[string]string{"A": "a", "B": "b"},
					Labels:      map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							Name: "o1",
							UID:  "81726381762",
						},
					},
				},
				Spec: enginev2alpha2.PodGroupSpec{
					MinMember: 1,
				},
			},
		},
		{
			"Different annotations - blocklisted",
			args{
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{pgWorkloadStatus: "Pending"},
						Labels:      map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 1,
					},
				},
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{pgWorkloadStatus: "Running"},
						Labels:      map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 1,
					},
				},
			},
			&enginev2alpha2.PodGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "p1",
					Namespace:   "n1",
					Annotations: map[string]string{pgWorkloadStatus: "Pending"},
					Labels:      map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							Name: "o1",
							UID:  "81726381762",
						},
					},
				},
				Spec: enginev2alpha2.PodGroupSpec{
					MinMember: 1,
				},
			},
		},
		{
			"Different labels",
			args{
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{},
						Labels:      map[string]string{"L2": "l2"},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 1,
					},
				},
				&enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "p1",
						Namespace:   "n1",
						Annotations: map[string]string{},
						Labels:      map[string]string{"L1": "l1"},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "o1",
								UID:  "81726381762",
							},
						},
					},
					Spec: enginev2alpha2.PodGroupSpec{
						MinMember: 1,
					},
				},
			},
			&enginev2alpha2.PodGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "p1",
					Namespace:   "n1",
					Annotations: map[string]string{},
					Labels:      map[string]string{"L1": "l1"},
					OwnerReferences: []metav1.OwnerReference{
						{
							Name: "o1",
							UID:  "81726381762",
						},
					},
				},
				Spec: enginev2alpha2.PodGroupSpec{
					MinMember: 1,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updatePodGroup(tt.args.oldPodGroup, tt.args.newPodGroup)
		})
	}
}
