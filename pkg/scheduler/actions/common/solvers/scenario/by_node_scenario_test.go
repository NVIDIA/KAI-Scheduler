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

func TestPodByNodeScenario_VictimsTasksFromNodes(t *testing.T) {
	type fields struct {
		session               *framework.Session
		pendingTasksAsJob     *podgroup_info.PodGroupInfo
		potentialVictimsTasks []*pod_info.PodInfo
		recordedVictimsJobs   []*podgroup_info.PodGroupInfo
	}
	type args struct {
		tasks     []*pod_info.PodInfo
		nodeNames []string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []*pod_info.PodInfo
	}{
		{
			name: "Single potential job, 2 pods",
			fields: fields{
				session: &framework.Session{
					PodGroupInfos: map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
						"pg1": podgroup_info.NewPodGroupInfo("pg1",
							pod_info.NewTaskInfo(&v1.Pod{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "name1",
									Namespace: "n1",
									Annotations: map[string]string{
										commonconstants.PodGroupAnnotationForPod: "pg1",
									},
								},
								Spec: v1.PodSpec{
									NodeName: "node1",
								},
								Status: v1.PodStatus{
									Phase: v1.PodRunning,
								},
							}),
							pod_info.NewTaskInfo(&v1.Pod{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "name2",
									Namespace: "n1",
									Annotations: map[string]string{
										commonconstants.PodGroupAnnotationForPod: "pg1",
									},
								},
								Spec: v1.PodSpec{
									NodeName: "node1",
								},
								Status: v1.PodStatus{
									Phase: v1.PodRunning,
								},
							}),
						),
						"pg2": podgroup_info.NewPodGroupInfo("pg2", pod_info.NewTaskInfo(&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "name3",
								Namespace: "n1",
								Annotations: map[string]string{
									commonconstants.PodGroupAnnotationForPod: "pg2",
								},
							},
							Spec: v1.PodSpec{
								NodeName: "node1",
							},
							Status: v1.PodStatus{
								Phase: v1.PodRunning,
							},
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
						Spec: v1.PodSpec{
							NodeName: "node1",
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					}),
					pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name2",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg1",
							},
						},
						Spec: v1.PodSpec{
							NodeName: "node1",
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					}),
				},
			},
			args: args{
				tasks:     make([]*pod_info.PodInfo, 0),
				nodeNames: []string{"node1"},
			},
			want: []*pod_info.PodInfo{
				pod_info.NewTaskInfo(&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name1",
						Namespace: "n1",
						Annotations: map[string]string{
							commonconstants.PodGroupAnnotationForPod: "pg1",
						},
					},
					Spec: v1.PodSpec{
						NodeName: "node1",
					},
					Status: v1.PodStatus{
						Phase: v1.PodRunning,
					},
				}),
				pod_info.NewTaskInfo(&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name2",
						Namespace: "n1",
						Annotations: map[string]string{
							commonconstants.PodGroupAnnotationForPod: "pg1",
						},
					},
					Spec: v1.PodSpec{
						NodeName: "node1",
					},
					Status: v1.PodStatus{
						Phase: v1.PodRunning,
					},
				}),
			},
		},
		{
			name: "No pods on node",
			fields: fields{
				session: &framework.Session{
					PodGroupInfos: map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
						"pg1": podgroup_info.NewPodGroupInfo("pg1",
							pod_info.NewTaskInfo(&v1.Pod{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "name1",
									Namespace: "n1",
									Annotations: map[string]string{
										commonconstants.PodGroupAnnotationForPod: "pg1",
									},
								},
								Spec: v1.PodSpec{
									NodeName: "node1",
								},
								Status: v1.PodStatus{
									Phase: v1.PodRunning,
								},
							}),
							pod_info.NewTaskInfo(&v1.Pod{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "name2",
									Namespace: "n1",
									Annotations: map[string]string{
										commonconstants.PodGroupAnnotationForPod: "pg1",
									},
								},
								Spec: v1.PodSpec{
									NodeName: "node1",
								},
								Status: v1.PodStatus{
									Phase: v1.PodRunning,
								},
							}),
						),
						"pg2": podgroup_info.NewPodGroupInfo("pg2", pod_info.NewTaskInfo(&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "name3",
								Namespace: "n1",
								Annotations: map[string]string{
									commonconstants.PodGroupAnnotationForPod: "pg2",
								},
							},
							Spec: v1.PodSpec{
								NodeName: "node1",
							},
							Status: v1.PodStatus{
								Phase: v1.PodRunning,
							},
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
						Spec: v1.PodSpec{
							NodeName: "node1",
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					}),
					pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name2",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg1",
							},
						},
						Spec: v1.PodSpec{
							NodeName: "node1",
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					}),
				},
			},
			args: args{
				tasks:     make([]*pod_info.PodInfo, 0),
				nodeNames: []string{"node2"},
			},
			want: nil,
		},
		{
			name: "Single potential job, return pods from same job on different nodes",
			fields: fields{
				session: &framework.Session{
					PodGroupInfos: map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
						"pg1": podgroup_info.NewPodGroupInfo("pg1",
							pod_info.NewTaskInfo(&v1.Pod{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "name1",
									Namespace: "n1",
									Annotations: map[string]string{
										commonconstants.PodGroupAnnotationForPod: "pg1",
									},
								},
								Spec: v1.PodSpec{
									NodeName: "node1",
								},
								Status: v1.PodStatus{
									Phase: v1.PodRunning,
								},
							}),
							pod_info.NewTaskInfo(&v1.Pod{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "name2",
									Namespace: "n1",
									Annotations: map[string]string{
										commonconstants.PodGroupAnnotationForPod: "pg1",
									},
								},
								Spec: v1.PodSpec{
									NodeName: "node2",
								},
								Status: v1.PodStatus{
									Phase: v1.PodRunning,
								},
							}),
						),
						"pg2": podgroup_info.NewPodGroupInfo("pg2", pod_info.NewTaskInfo(&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "name3",
								Namespace: "n1",
								Annotations: map[string]string{
									commonconstants.PodGroupAnnotationForPod: "pg2",
								},
							},
							Spec: v1.PodSpec{
								NodeName: "node1",
							},
							Status: v1.PodStatus{
								Phase: v1.PodRunning,
							},
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
						Spec: v1.PodSpec{
							NodeName: "node1",
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					}),
					pod_info.NewTaskInfo(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "name2",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg1",
							},
						},
						Spec: v1.PodSpec{
							NodeName: "node2",
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					}),
				},
			},
			args: args{
				tasks:     make([]*pod_info.PodInfo, 0),
				nodeNames: []string{"node1"},
			},
			want: []*pod_info.PodInfo{
				pod_info.NewTaskInfo(&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name1",
						Namespace: "n1",
						Annotations: map[string]string{
							commonconstants.PodGroupAnnotationForPod: "pg1",
						},
					},
					Spec: v1.PodSpec{
						NodeName: "node1",
					},
					Status: v1.PodStatus{
						Phase: v1.PodRunning,
					},
				}),
				pod_info.NewTaskInfo(&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name2",
						Namespace: "n1",
						Annotations: map[string]string{
							commonconstants.PodGroupAnnotationForPod: "pg1",
						},
					},
					Spec: v1.PodSpec{
						NodeName: "node2",
					},
					Status: v1.PodStatus{
						Phase: v1.PodRunning,
					},
				}),
			},
		},
		{
			name: "Single potential job, return pods from same job on different nodes - get tasks from AddPotentialVictimsTasks",
			fields: fields{
				session: &framework.Session{
					PodGroupInfos: map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
						"pg1": podgroup_info.NewPodGroupInfo("pg1",
							pod_info.NewTaskInfo(&v1.Pod{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "name1",
									Namespace: "n1",
									Annotations: map[string]string{
										commonconstants.PodGroupAnnotationForPod: "pg1",
									},
								},
								Spec: v1.PodSpec{
									NodeName: "node1",
								},
								Status: v1.PodStatus{
									Phase: v1.PodRunning,
								},
							}),
							pod_info.NewTaskInfo(&v1.Pod{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "name2",
									Namespace: "n1",
									Annotations: map[string]string{
										commonconstants.PodGroupAnnotationForPod: "pg1",
									},
								},
								Spec: v1.PodSpec{
									NodeName: "node2",
								},
								Status: v1.PodStatus{
									Phase: v1.PodRunning,
								},
							}),
						),
						"pg2": podgroup_info.NewPodGroupInfo("pg2", pod_info.NewTaskInfo(&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "name3",
								Namespace: "n1",
								Annotations: map[string]string{
									commonconstants.PodGroupAnnotationForPod: "pg2",
								},
							},
							Spec: v1.PodSpec{
								NodeName: "node1",
							},
							Status: v1.PodStatus{
								Phase: v1.PodRunning,
							},
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
						Spec: v1.PodSpec{
							NodeName: "node1",
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
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
						Spec: v1.PodSpec{
							NodeName: "node2",
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					}),
				},
				nodeNames: []string{"node1"},
			},
			want: []*pod_info.PodInfo{
				pod_info.NewTaskInfo(&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name1",
						Namespace: "n1",
						Annotations: map[string]string{
							commonconstants.PodGroupAnnotationForPod: "pg1",
						},
					},
					Spec: v1.PodSpec{
						NodeName: "node1",
					},
					Status: v1.PodStatus{
						Phase: v1.PodRunning,
					},
				}),
				pod_info.NewTaskInfo(&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name2",
						Namespace: "n1",
						Annotations: map[string]string{
							commonconstants.PodGroupAnnotationForPod: "pg1",
						},
					},
					Spec: v1.PodSpec{
						NodeName: "node2",
					},
					Status: v1.PodStatus{
						Phase: v1.PodRunning,
					},
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bns := NewByNodeScenario(tt.fields.session, tt.fields.pendingTasksAsJob, tt.fields.pendingTasksAsJob, tt.fields.potentialVictimsTasks,
				tt.fields.recordedVictimsJobs)
			if tt.args.tasks != nil {
				bns.AddPotentialVictimsTasks(tt.args.tasks)
			}
			if got := bns.VictimsTasksFromNodes(tt.args.nodeNames); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("VictimsTasksFromNodes() = %v, want %v", got, tt.want)
			}
		})
	}
}
