// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package scenario

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	commonconstants "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
)

func TestPodSimpleScenario_AddPotentialVictimsTasks(t *testing.T) {
	type fields struct {
		session               *framework.Session
		pendingTasksAsJob     *podgroup_info.PodGroupInfo
		potentialVictimsTasks []*pod_info.PodInfo
		recordedVictimsJobs   []*podgroup_info.PodGroupInfo
	}
	type args struct {
		tasks []*pod_info.PodInfo
	}
	type expected struct {
		potentialVictimsTasks []*pod_info.PodInfo
		victimsJobsTaskGroups map[common_info.PodGroupID][]*podgroup_info.PodGroupInfo
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		expected expected
	}{
		{
			name: "victims in ctor",
			fields: fields{
				session: &framework.Session{
					PodGroupInfos: map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
						"pg1": podgroup_info.NewPodGroupInfo("pg1"),
					},
				},
				pendingTasksAsJob: podgroup_info.NewPodGroupInfo("123"),
				potentialVictimsTasks: []*pod_info.PodInfo{
					pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name1",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg1",
							},
						},
						Spec: v1.PodSpec{},
					}),
				},
				recordedVictimsJobs: []*podgroup_info.PodGroupInfo{},
			},
			args: args{
				tasks: make([]*pod_info.PodInfo, 0),
			},
			expected: expected{
				potentialVictimsTasks: []*pod_info.PodInfo{
					pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name1",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg1",
							},
						},
						Spec: v1.PodSpec{},
					}),
				},
				victimsJobsTaskGroups: map[common_info.PodGroupID][]*podgroup_info.PodGroupInfo{
					"pg1": {
						podgroup_info.NewPodGroupInfo("pg1"),
					},
				},
			},
		},
		{
			name: "victims in ctor and AddPotentialVictimsTasks",
			fields: fields{
				session: &framework.Session{
					PodGroupInfos: map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
						"pg1": podgroup_info.NewPodGroupInfo("pg1"),
						"pg2": podgroup_info.NewPodGroupInfo("pg2"),
					},
				},
				pendingTasksAsJob: podgroup_info.NewPodGroupInfo("123"),
				potentialVictimsTasks: []*pod_info.PodInfo{
					pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name1",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg1",
							},
						},
						Spec: v1.PodSpec{},
					}),
				},
				recordedVictimsJobs: []*podgroup_info.PodGroupInfo{},
			},
			args: args{
				tasks: []*pod_info.PodInfo{
					pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name2",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg2",
							},
						},
						Spec: v1.PodSpec{},
					}),
				},
			},
			expected: expected{
				potentialVictimsTasks: []*pod_info.PodInfo{
					pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name1",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg1",
							},
						},
						Spec: v1.PodSpec{},
					}),
					pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name2",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg2",
							},
						},
						Spec: v1.PodSpec{},
					}),
				},
				victimsJobsTaskGroups: map[common_info.PodGroupID][]*podgroup_info.PodGroupInfo{
					"pg1": {
						podgroup_info.NewPodGroupInfo("pg1"),
					},
					"pg2": {
						podgroup_info.NewPodGroupInfo("pg2"),
					},
				},
			},
		},
		{
			name: "recorded victims update victimsJobsTaskGroups field",
			fields: fields{
				session: &framework.Session{
					PodGroupInfos: map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
						"pg1": podgroup_info.NewPodGroupInfo("pg1"),
						"pg2": podgroup_info.NewPodGroupInfo("pg2"),
					},
				},
				pendingTasksAsJob: podgroup_info.NewPodGroupInfo("123"),
				potentialVictimsTasks: []*pod_info.PodInfo{
					pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name1",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg1",
							},
						},
						Spec: v1.PodSpec{},
					}),
				},
				recordedVictimsJobs: []*podgroup_info.PodGroupInfo{
					podgroup_info.NewPodGroupInfo("pg2", pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name2",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg2",
							},
						},
						Spec: v1.PodSpec{},
					})),
				},
			},
			args: args{
				tasks: make([]*pod_info.PodInfo, 0),
			},
			expected: expected{
				potentialVictimsTasks: []*pod_info.PodInfo{
					pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name1",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg1",
							},
						},
						Spec: v1.PodSpec{},
					}),
				},
				victimsJobsTaskGroups: map[common_info.PodGroupID][]*podgroup_info.PodGroupInfo{
					"pg1": {
						podgroup_info.NewPodGroupInfo("pg1"),
					},
					"pg2": {
						podgroup_info.NewPodGroupInfo("pg2"),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewBaseScenario(
				tt.fields.session, tt.fields.pendingTasksAsJob, tt.fields.pendingTasksAsJob, tt.fields.potentialVictimsTasks,
				tt.fields.recordedVictimsJobs)
			s.AddPotentialVictimsTasks(tt.args.tasks)
			if !reflect.DeepEqual(s.potentialVictimsTasks, tt.expected.potentialVictimsTasks) {
				t.Errorf("potentialVictimsTasks = %v, want %v", s.potentialVictimsTasks,
					tt.expected.potentialVictimsTasks)
			}
			for jobId := range s.victimsJobsTaskGroups {
				if !reflect.DeepEqual(
					len(s.victimsJobsTaskGroups[jobId]), len(tt.expected.victimsJobsTaskGroups[jobId])) {
					t.Errorf(
						"victimsJobsTaskGroups-job id %v has different amount of jobs under the Id= %v, want %v",
						jobId, s.victimsJobsTaskGroups, tt.expected.victimsJobsTaskGroups)
				}
			}

		})
	}
}

func TestPodSimpleScenario_GetVictimJobRepresentativeById(t *testing.T) {
	type fields struct {
		session               *framework.Session
		pendingTasksAsJob     *podgroup_info.PodGroupInfo
		potentialVictimsTasks []*pod_info.PodInfo
		recordedVictimsJobs   []*podgroup_info.PodGroupInfo
	}
	type args struct {
		victimPodInfo *pod_info.PodInfo
		tasks         []*pod_info.PodInfo
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *podgroup_info.PodGroupInfo
	}{
		{
			name: "Single task for job",
			fields: fields{
				session: &framework.Session{
					PodGroupInfos: map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
						"pg1": podgroup_info.NewPodGroupInfo("pg1", pod_info.NewTaskInfo(&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "name1",
								Namespace: "n1",
								Annotations: map[string]string{
									commonconstants.PodGroupAnnotationForPod: "pg1",
								},
							},
							Spec: v1.PodSpec{},
						})),
						"pg2": podgroup_info.NewPodGroupInfo("pg2", pod_info.NewTaskInfo(&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "name2",
								Namespace: "n1",
								Annotations: map[string]string{
									commonconstants.PodGroupAnnotationForPod: "pg2",
								},
							},
							Spec: v1.PodSpec{},
						})),
					},
				},
				pendingTasksAsJob: podgroup_info.NewPodGroupInfo("123"),
				potentialVictimsTasks: []*pod_info.PodInfo{
					pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name1",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg1",
							},
						},
						Spec: v1.PodSpec{},
					}),
				},
				recordedVictimsJobs: []*podgroup_info.PodGroupInfo{
					podgroup_info.NewPodGroupInfo("pg2", pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name2",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg2",
							},
						},
						Spec: v1.PodSpec{},
					})),
				},
			},
			args: args{
				victimPodInfo: pod_info.NewTaskInfo(&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name1",
						Namespace: "n1",
						Annotations: map[string]string{
							commonconstants.PodGroupAnnotationForPod: "pg1",
						},
					},
					Spec: v1.PodSpec{},
				}),
				tasks: make([]*pod_info.PodInfo, 0),
			},
			want: podgroup_info.NewPodGroupInfo("pg1", pod_info.NewTaskInfo(&v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name1",
					Namespace: "n1",
					Annotations: map[string]string{
						commonconstants.PodGroupAnnotationForPod: "pg1",
					},
				},
				Spec: v1.PodSpec{},
			})),
		},
		{
			name: "Task not given to the scenario",
			fields: fields{
				session: &framework.Session{
					PodGroupInfos: map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
						"pg1": podgroup_info.NewPodGroupInfo("pg1", pod_info.NewTaskInfo(&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "name1",
								Namespace: "n1",
								Annotations: map[string]string{
									commonconstants.PodGroupAnnotationForPod: "pg1",
								},
							},
							Spec: v1.PodSpec{},
						})),
						"pg2": podgroup_info.NewPodGroupInfo("pg2", pod_info.NewTaskInfo(&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "name2",
								Namespace: "n1",
								Annotations: map[string]string{
									commonconstants.PodGroupAnnotationForPod: "pg2",
								},
							},
							Spec: v1.PodSpec{},
						})),
					},
				},
				pendingTasksAsJob: podgroup_info.NewPodGroupInfo("123"),
				potentialVictimsTasks: []*pod_info.PodInfo{
					pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name1",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg1",
							},
						},
						Spec: v1.PodSpec{},
					}),
				},
				recordedVictimsJobs: []*podgroup_info.PodGroupInfo{
					podgroup_info.NewPodGroupInfo("pg2", pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name2",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg2",
							},
						},
						Spec: v1.PodSpec{},
					})),
				},
			},
			args: args{
				victimPodInfo: pod_info.NewTaskInfo(&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name3",
						Namespace: "n1",
						Annotations: map[string]string{
							commonconstants.PodGroupAnnotationForPod: "pg3",
						},
					},
					Spec: v1.PodSpec{},
				}),
				tasks: make([]*pod_info.PodInfo, 0),
			},
			want: nil,
		},
		{
			name: "For elastic job, get only a single-task job representor",
			fields: fields{
				session: &framework.Session{
					PodGroupInfos: map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
						"pg1": podgroup_info.NewPodGroupInfo("pg1", pod_info.NewTaskInfo(&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "name1",
								Namespace: "n1",
								Annotations: map[string]string{
									commonconstants.PodGroupAnnotationForPod: "pg1",
								},
							},
							Spec: v1.PodSpec{},
						})),
						"pg2": podgroup_info.NewPodGroupInfo("pg2", pod_info.NewTaskInfo(&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "name2",
								Namespace: "n1",
								Annotations: map[string]string{
									commonconstants.PodGroupAnnotationForPod: "pg2",
								},
							},
							Spec: v1.PodSpec{},
						})),
					},
				},
				pendingTasksAsJob: podgroup_info.NewPodGroupInfo("123"),
				potentialVictimsTasks: []*pod_info.PodInfo{
					pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name1",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg1",
							},
						},
						Spec: v1.PodSpec{},
					}),
				},
				recordedVictimsJobs: []*podgroup_info.PodGroupInfo{
					podgroup_info.NewPodGroupInfo("pg2", pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name2",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg2",
							},
						},
						Spec: v1.PodSpec{},
					})),
				},
			},
			args: args{
				victimPodInfo: pod_info.NewTaskInfo(&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name1",
						Namespace: "n1",
						Annotations: map[string]string{
							commonconstants.PodGroupAnnotationForPod: "pg1",
						},
					},
					Spec: v1.PodSpec{},
				}),
				tasks: []*pod_info.PodInfo{
					pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name1.2",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg1",
							},
						},
						Spec: v1.PodSpec{},
					}),
				},
			},
			want: podgroup_info.NewPodGroupInfo("pg1", pod_info.NewTaskInfo(&v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name1",
					Namespace: "n1",
					Annotations: map[string]string{
						commonconstants.PodGroupAnnotationForPod: "pg1",
					},
				},
				Spec: v1.PodSpec{},
			})),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewBaseScenario(
				tt.fields.session, tt.fields.pendingTasksAsJob, tt.fields.pendingTasksAsJob, tt.fields.potentialVictimsTasks,
				tt.fields.recordedVictimsJobs)
			s.AddPotentialVictimsTasks(tt.args.tasks)
			if got := s.GetVictimJobRepresentativeById(tt.args.victimPodInfo); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetVictimJobRepresentativeById() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPodSimpleScenario_LatestPotentialVictim(t *testing.T) {
	type fields struct {
		session               *framework.Session
		pendingTasksAsJob     *podgroup_info.PodGroupInfo
		potentialVictimsTasks []*pod_info.PodInfo
	}
	type args struct {
		tasks []*pod_info.PodInfo
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *podgroup_info.PodGroupInfo
	}{
		{
			name: "tasks only in ctor",
			fields: fields{
				session: &framework.Session{
					PodGroupInfos: map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
						"pg1": podgroup_info.NewPodGroupInfo("pg1", pod_info.NewTaskInfo(&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "name1",
								Namespace: "n1",
								Annotations: map[string]string{
									commonconstants.PodGroupAnnotationForPod: "pg1",
								},
							},
							Spec: v1.PodSpec{},
						})),
						"pg2": podgroup_info.NewPodGroupInfo("pg2", pod_info.NewTaskInfo(&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "name2",
								Namespace: "n1",
								Annotations: map[string]string{
									commonconstants.PodGroupAnnotationForPod: "pg2",
								},
							},
							Spec: v1.PodSpec{},
						})),
					},
				},
				pendingTasksAsJob: podgroup_info.NewPodGroupInfo("123"),
				potentialVictimsTasks: []*pod_info.PodInfo{
					pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name1",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg1",
							},
						},
						Spec: v1.PodSpec{},
					}),
				},
			},
			args: args{},
			want: podgroup_info.NewPodGroupInfo("pg1", pod_info.NewTaskInfo(&v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name1",
					Namespace: "n1",
					Annotations: map[string]string{
						commonconstants.PodGroupAnnotationForPod: "pg1",
					},
				},
				Spec: v1.PodSpec{},
			})),
		},
		{
			name: "return latest task from AddPotentialVictimsTasks",
			fields: fields{
				session: &framework.Session{
					PodGroupInfos: map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
						"pg1": podgroup_info.NewPodGroupInfo("pg1", pod_info.NewTaskInfo(&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "name1",
								Namespace: "n1",
								Annotations: map[string]string{
									commonconstants.PodGroupAnnotationForPod: "pg1",
								},
							},
							Spec: v1.PodSpec{},
						}), pod_info.NewTaskInfo(&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "name2",
								Namespace: "n1",
								Annotations: map[string]string{
									commonconstants.PodGroupAnnotationForPod: "pg1",
								},
							},
							Spec: v1.PodSpec{},
						})),
						"pg2": podgroup_info.NewPodGroupInfo("pg2", pod_info.NewTaskInfo(&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "name2",
								Namespace: "n1",
								Annotations: map[string]string{
									commonconstants.PodGroupAnnotationForPod: "pg2",
								},
							},
							Spec: v1.PodSpec{},
						})),
					},
				},
				pendingTasksAsJob: podgroup_info.NewPodGroupInfo("123"),
				potentialVictimsTasks: []*pod_info.PodInfo{
					pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name1",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg1",
							},
						},
						Spec: v1.PodSpec{},
					}),
				},
			},
			args: args{
				tasks: []*pod_info.PodInfo{
					pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name2",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg1",
							},
						},
						Spec: v1.PodSpec{},
					}),
				},
			},
			want: podgroup_info.NewPodGroupInfo("pg1", pod_info.NewTaskInfo(&v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name1",
					Namespace: "n1",
					Annotations: map[string]string{
						commonconstants.PodGroupAnnotationForPod: "pg1",
					},
				},
				Spec: v1.PodSpec{},
			}), pod_info.NewTaskInfo(&v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name2",
					Namespace: "n1",
					Annotations: map[string]string{
						commonconstants.PodGroupAnnotationForPod: "pg1",
					},
				},
				Spec: v1.PodSpec{},
			})),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewBaseScenario(
				tt.fields.session, tt.fields.pendingTasksAsJob, tt.fields.pendingTasksAsJob, tt.fields.potentialVictimsTasks,
				[]*podgroup_info.PodGroupInfo{})
			if tt.args.tasks != nil {
				s.AddPotentialVictimsTasks(tt.args.tasks)
			}
			if got := s.LatestPotentialVictim(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LatestPotentialVictim() = %v, want %v", got, tt.want)
			}
		})
	}
}
