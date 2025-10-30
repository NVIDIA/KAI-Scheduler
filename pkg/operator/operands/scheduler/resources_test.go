// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/spf13/pflag"

	"github.com/NVIDIA/KAI-scheduler/cmd/scheduler/app/options"
	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	kaiv1qc "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/queue_controller"
	kaiv1scheduler "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/scheduler"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDeploymentForShard(t *testing.T) {
	tests := []struct {
		name        string
		config      *kaiv1.Config
		shard       *kaiv1.SchedulingShard
		expected    []string
		notExpected []string
	}{
		{
			name: "basic configuration",
			config: &kaiv1.Config{
				Spec: kaiv1.ConfigSpec{
					Global: &kaiv1.GlobalConfig{
						SchedulerName:    ptr.To("other-kai-scheduler"),
						NodePoolLabelKey: ptr.To("nodepool"),
					},
					Namespace: "default",
					Scheduler: &kaiv1scheduler.Scheduler{
						Replicas: ptr.To(int32(1)),
					},
				},
			},
			shard: &kaiv1.SchedulingShard{
				Spec: kaiv1.SchedulingShardSpec{
					PartitionLabelValue: "partition-1",
				},
			},
			expected: []string{
				fmt.Sprintf("--scheduler-conf=%s", configMountPath),
				"--scheduler-name=other-kai-scheduler",
				"--namespace=default",
				"--nodepool-label-key=nodepool",
				"--partition-label-value=partition-1",
			},
			notExpected: []string{"--leader-elect"},
		},
		{
			name: "with custom shard args",
			config: &kaiv1.Config{
				Spec: kaiv1.ConfigSpec{
					Global: &kaiv1.GlobalConfig{
						SchedulerName: ptr.To("other-kai-scheduler"),
					},
					Namespace: "default",
					Scheduler: &kaiv1scheduler.Scheduler{
						Replicas: ptr.To(int32(1)),
					},
				},
			},
			shard: &kaiv1.SchedulingShard{
				Spec: kaiv1.SchedulingShardSpec{
					Args: map[string]string{
						"v":                       "5",
						"enable-profiler":         "true",
						"full-hierarchy-fairness": "false",
					},
				},
			},
			expected: []string{
				fmt.Sprintf("--scheduler-conf=%s", configMountPath),
				"--scheduler-name=other-kai-scheduler",
				"--namespace=default",
				"--v=5",
				"--enable-profiler=true",
				"--full-hierarchy-fairness=false",
			},
		},
		{
			name: "with leader election enabled",
			config: &kaiv1.Config{
				Spec: kaiv1.ConfigSpec{
					Global: &kaiv1.GlobalConfig{
						SchedulerName: ptr.To("other-kai-scheduler"),
					},
					Namespace: "default",
					Scheduler: &kaiv1scheduler.Scheduler{
						Replicas: ptr.To(int32(2)),
					},
				},
			},
			shard: &kaiv1.SchedulingShard{},
			expected: []string{
				fmt.Sprintf("--scheduler-conf=%s", configMountPath),
				"--scheduler-name=other-kai-scheduler",
				"--namespace=default",
				"--leader-elect=true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := fake.NewClientBuilder().Build()
			tt.config.Spec.SetDefaultsWhereNeeded()

			s := NewSchedulerForShard(tt.shard)
			deployment, err := s.deploymentForShard(ctx, client, tt.config, tt.shard)
			require.NoError(t, err)
			assert.NotNil(t, deployment)

			deploy, ok := deployment.(*appsv1.Deployment)
			require.True(t, ok, "Expected *appsv1.Deployment")

			assert.Equal(t, deploymentName(tt.config, tt.shard), deploy.Name)
			assert.Equal(t, tt.config.Spec.Namespace, deploy.Namespace)

			container := deploy.Spec.Template.Spec.Containers[0]
			args := container.Args

			// Check expected args
			for _, expected := range tt.expected {
				assert.Contains(t, args, expected)
			}

			// Check not expected args
			for _, notExpected := range tt.notExpected {
				assert.NotContains(t, args, notExpected)
			}
		})
	}
}

func TestValidateJobDepthMap(t *testing.T) {
	tests := []struct {
		name        string
		shard       *kaiv1.SchedulingShard
		actions     []string
		expectError bool
	}{
		{
			name: "valid queue depth with allocate and reclaim",
			shard: &kaiv1.SchedulingShard{
				Spec: kaiv1.SchedulingShardSpec{
					QueueDepthPerAction: map[string]int{
						"allocate": 10,
						"reclaim":  5,
					},
				},
			},
			actions:     []string{"allocate", "reclaim", "preempt", "stalegangeviction"},
			expectError: false,
		},
		{
			name: "invalid queue depth with unknown action",
			shard: &kaiv1.SchedulingShard{
				Spec: kaiv1.SchedulingShardSpec{
					QueueDepthPerAction: map[string]int{
						"allocate": 10,
						"invalid":  5,
					},
				},
			},
			actions:     []string{"allocate", "preempt", "reclaim", "stalegangeviction"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			innerConfig := config{
				Actions: strings.Join(tt.actions, ", "),
			}

			err := validateJobDepthMap(tt.shard, innerConfig, tt.actions)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestBuildArgsList(t *testing.T) {
	tests := []struct {
		name        string
		config      *kaiv1.Config
		shard       *kaiv1.SchedulingShard
		expected    map[string]string
		notExpected []string
	}{
		{
			name: "basic args from config",
			config: &kaiv1.Config{
				Spec: kaiv1.ConfigSpec{
					Global: &kaiv1.GlobalConfig{
						SchedulerName:    ptr.To("test-scheduler"),
						NodePoolLabelKey: ptr.To("nodepool"),
					},
					Namespace: "kai-system",
					Scheduler: &kaiv1scheduler.Scheduler{
						Replicas: ptr.To(int32(2)),
					},
				},
			},
			shard: &kaiv1.SchedulingShard{
				Spec: kaiv1.SchedulingShardSpec{
					PartitionLabelValue: "prod",
				},
			},
			expected: map[string]string{
				"scheduler-conf":        "config.yaml",
				"scheduler-name":        "test-scheduler",
				"namespace":             "kai-system",
				"nodepool-label-key":    "nodepool",
				"partition-label-value": "prod",
				"leader-elect":          "true",
			},
			notExpected: []string{"metrics-namespace"},
		},
		{
			name: "with custom shard args overriding config",
			config: &kaiv1.Config{
				Spec: kaiv1.ConfigSpec{
					Global: &kaiv1.GlobalConfig{
						SchedulerName: ptr.To("test-scheduler"),
					},
					Namespace: "kai-system",
					QueueController: &kaiv1qc.QueueController{
						MetricsNamespace: ptr.To("monitoring"),
					},
					Scheduler: &kaiv1scheduler.Scheduler{
						Replicas: ptr.To(int32(1)),
					},
				},
			},
			shard: &kaiv1.SchedulingShard{
				Spec: kaiv1.SchedulingShardSpec{
					Args: map[string]string{
						"qps":       "100",
						"namespace": "override-ns",
					},
				},
			},
			expected: map[string]string{
				"scheduler-conf":    "config.yaml",
				"scheduler-name":    "test-scheduler",
				"namespace":         "override-ns",
				"qps":               "100",
				"metrics-namespace": "monitoring",
			},
			notExpected: []string{"leader-elect"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.Spec.SetDefaultsWhereNeeded()
			args, err := buildArgsList(tt.shard, tt.config, "config.yaml")
			require.NoError(t, err)

			// Create FlagSet and add server options
			so := options.NewServerOption()
			fs := pflag.NewFlagSet("test-args", pflag.ExitOnError)
			so.AddFlags(fs)

			// Parse the generated args
			err = fs.Parse(args)
			require.NoError(t, err)

			// Verify expected flags
			fs.Visit(func(flag *pflag.Flag) {
				if flag.Changed {
					expectedValue, ok := tt.expected[flag.Name]
					if ok && flag.Value.String() != expectedValue {
						t.Errorf("flag --%s: expected %q, got %q", flag.Name, expectedValue, flag.Value.String())
					}
				}
			})

			// Verify not expected flags
			for _, notFlag := range tt.notExpected {
				flag := fs.Lookup(notFlag)
				if flag != nil && flag.Changed {
					t.Errorf("forbidden flag set: --%s", notFlag)
				}
			}
		})
	}
}

func TestConfigMapForShard(t *testing.T) {
	tests := []struct {
		name        string
		config      *kaiv1.Config
		shard       *kaiv1.SchedulingShard
		expected    map[string]string // Only "config.yaml" key should be present
		expectedErr bool
	}{
		{
			name: "basic binpack strategy",
			config: &kaiv1.Config{
				Spec: kaiv1.ConfigSpec{
					Scheduler: &kaiv1scheduler.Scheduler{
						Replicas: ptr.To(int32(1)),
					},
				},
			},
			shard: &kaiv1.SchedulingShard{
				Spec: kaiv1.SchedulingShardSpec{
					PlacementStrategy: &kaiv1.PlacementStrategy{
						GPU: ptr.To(binpackStrategy),
						CPU: ptr.To(binpackStrategy),
					},
				},
			},
			expected: map[string]string{
				"config.yaml": `actions: allocate,consolidation,reclaim,preempt,stalegangeviction
tiers:
- plugins:
  - name: predicates
  - name: proportion
  - name: priority
  - name: nodeavailability
  - name: resourcetype
  - name: podaffinity
  - name: elastic
  - name: kubeflow
  - name: ray
  - name: subgrouporder
  - name: taskorder
  - name: nominatednode
  - name: dynamicresources
  - name: minruntime
  - name: topology
  - name: snapshot
  - name: gpupack
  - name: nodeplacement
    arguments:
      cpu: binpack
      gpu: binpack
  - name: gpusharingorder`,
			},
		},
		{
			name: "spread strategy with queue depth configuration",
			config: &kaiv1.Config{
				Spec: kaiv1.ConfigSpec{
					Scheduler: &kaiv1scheduler.Scheduler{
						Replicas: ptr.To(int32(1)),
					},
				},
			},
			shard: &kaiv1.SchedulingShard{
				Spec: kaiv1.SchedulingShardSpec{
					PlacementStrategy: &kaiv1.PlacementStrategy{
						GPU: ptr.To(spreadStrategy),
						CPU: ptr.To(binpackStrategy),
					},
					QueueDepthPerAction: map[string]int{
						"allocate": 20,
						"reclaim":  10,
					},
				},
			},
			expected: map[string]string{
				"config.yaml": `actions: allocate,reclaim,preempt,stalegangeviction
queueDepthPerAction:
  allocate: 20
  reclaim: 10
tiers:
- plugins:
  - name: predicates
  - name: proportion
  - name: priority
  - name: nodeavailability
  - name: resourcetype
  - name: podaffinity
  - name: elastic
  - name: kubeflow
  - name: ray
  - name: subgrouporder
  - name: taskorder
  - name: nominatednode
  - name: dynamicresources
  - name: minruntime
  - name: topology
  - name: snapshot
  - name: gpuspread
  - name: nodeplacement
    arguments:
      cpu: binpack
      gpu: spread`,
			},
		},
		{
			name: "invalid queue depth configuration",
			config: &kaiv1.Config{
				Spec: kaiv1.ConfigSpec{
					Scheduler: &kaiv1scheduler.Scheduler{
						Replicas: ptr.To(int32(1)),
					},
				},
			},
			shard: &kaiv1.SchedulingShard{
				Spec: kaiv1.SchedulingShardSpec{
					QueueDepthPerAction: map[string]int{
						"invalid-action": 5,
					},
				},
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := fake.NewClientBuilder().Build()
			tt.config.Spec.SetDefaultsWhereNeeded()
			tt.shard.Spec.SetDefaultsWhereNeeded()

			s := NewSchedulerForShard(tt.shard)
			cm, err := s.configMapForShard(ctx, client, tt.config, tt.shard)
			if !tt.expectedErr {
				require.NoError(t, err)
				assert.NotNil(t, cm)

				configMap, ok := cm.(*corev1.ConfigMap)
				require.True(t, ok, "Expected *corev1.ConfigMap")

				// Extract and verify config.yaml
				actualYAML, found := configMap.Data["config.yaml"]
				require.True(t, found, "ConfigMap missing config.yaml")

				// Unmarshal expected YAML from test case
				var expectedConfig config
				if _, ok := tt.expected["config.yaml"]; !ok {
					t.Fatal("Test case must provide expected YAML for config.yaml")
				}
				err = yaml.Unmarshal([]byte(tt.expected["config.yaml"]), &expectedConfig)
				require.NoError(t, err, "Failed to unmarshal expected config")

				// Unmarshal actual YAML from ConfigMap
				var actualConfig config
				err = yaml.Unmarshal([]byte(actualYAML), &actualConfig)
				require.NoError(t, err, "Failed to unmarshal actual config")

				// Compare the configuration structs
				assert.Equal(t, expectedConfig.Tiers, actualConfig.Tiers, "ConfigMap Tiers content mismatch")
				assert.Equal(t, expectedConfig.QueueDepthPerAction, actualConfig.QueueDepthPerAction, "ConfigMap QueueDepthPerAction content mismatch")
				// Trim and split actions
				expectedActions := make([]string, 0, len(expectedConfig.Actions))
				for _, action := range strings.Split(expectedConfig.Actions, ",") {
					expectedActions = append(expectedActions, strings.TrimSpace(action))
				}

				actualActions := make([]string, 0, len(actualConfig.Actions))
				for _, action := range strings.Split(actualConfig.Actions, ",") {
					actualActions = append(actualActions, strings.TrimSpace(action))
				}

			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestServiceForShard(t *testing.T) {
	tests := []struct {
		name         string
		config       *kaiv1.Config
		shard        *kaiv1.SchedulingShard
		expectedPort int32
	}{
		{
			name: "default port configuration",
			config: &kaiv1.Config{
				Spec: kaiv1.ConfigSpec{
					Scheduler: &kaiv1scheduler.Scheduler{
						SchedulerService: &kaiv1scheduler.Service{
							Port:       ptr.To(8080),
							TargetPort: ptr.To(8080),
							Type:       ptr.To(corev1.ServiceTypeClusterIP),
						},
					},
				},
			},
			shard: &kaiv1.SchedulingShard{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-shard",
				},
			},
			expectedPort: 8080,
		},
		{
			name: "custom metrics port configuration",
			config: &kaiv1.Config{
				Spec: kaiv1.ConfigSpec{
					Scheduler: &kaiv1scheduler.Scheduler{
						SchedulerService: &kaiv1scheduler.Service{
							Port:       ptr.To(80),
							TargetPort: ptr.To(80),
							Type:       ptr.To(corev1.ServiceTypeLoadBalancer),
						},
					},
				},
			},
			shard: &kaiv1.SchedulingShard{
				ObjectMeta: metav1.ObjectMeta{
					Name: "custom-shard",
				},
			},
			expectedPort: 80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := fake.NewClientBuilder().Build()
			tt.config.Spec.SetDefaultsWhereNeeded()
			tt.shard.Spec.SetDefaultsWhereNeeded()

			s := NewSchedulerForShard(tt.shard)
			service, err := s.serviceForShard(ctx, client, tt.config, tt.shard)
			require.NoError(t, err)
			assert.NotNil(t, service)

			svc, ok := service.(*corev1.Service)
			require.True(t, ok, "Expected *v1core.Service")

			assert.Equal(t, fmt.Sprintf("%s-%s", *tt.config.Spec.Global.SchedulerName, tt.shard.Name), svc.Name)
			assert.Equal(t, tt.config.Spec.Namespace, svc.Namespace)

			assert.Equal(t, tt.expectedPort, svc.Spec.Ports[0].Port)
			assert.Equal(t, tt.expectedPort, svc.Spec.Ports[0].TargetPort.IntVal)
		})
	}
}

func TestServiceAccountForScheduler(t *testing.T) {
	tests := []struct {
		name         string
		config       *kaiv1.Config
		expectedName string
	}{
		{
			name: "default scheduler name",
			config: &kaiv1.Config{
				Spec: kaiv1.ConfigSpec{
					Global: &kaiv1.GlobalConfig{
						SchedulerName: ptr.To("other-kai-scheduler"),
					},
				},
			},
			expectedName: "scheduler",
		},
		{
			name: "custom scheduler name",
			config: &kaiv1.Config{
				Spec: kaiv1.ConfigSpec{
					Global: &kaiv1.GlobalConfig{
						SchedulerName: ptr.To("custom-scheduler"),
					},
				},
			},
			expectedName: "scheduler",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := fake.NewClientBuilder().Build()
			tt.config.Spec.SetDefaultsWhereNeeded()

			s := &SchedulerForConfig{BaseResourceName: defaultResourceName}
			sa, err := s.serviceAccountForKAIConfig(ctx, client, tt.config)
			require.NoError(t, err)
			assert.NotNil(t, sa)

			assert.Equal(t, tt.expectedName, sa.GetName())
			assert.Equal(t, tt.config.Spec.Namespace, sa.GetNamespace())
		})
	}
}
