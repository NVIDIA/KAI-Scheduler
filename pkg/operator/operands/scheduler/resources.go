package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/slices"

	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	kaiConfigUtils "github.com/NVIDIA/KAI-scheduler/pkg/operator/config"
	"github.com/NVIDIA/KAI-scheduler/pkg/operator/operands/common"
)

const (
	invalidJobDepthMapError = "the scheduler's actions are %s. %s isn't one of them, making the queueDepthPerAction invalid"
	cpuWorkerNodeLabelKey   = "node-role.kubernetes.io/cpu-worker"
	gpuWorkerNodeLabelKey   = "node-role.kubernetes.io/gpu-worker"
	migWorkerNodeLabelKey   = "node-role.kubernetes.io/mig-enabled"
)

func deploymentForShard(
	ctx context.Context, readerClient client.Reader,
	kaiConfig *kaiv1.Config, shard *kaiv1.SchedulingShard,
) (client.Object, error) {
	shardDeploymentName := deploymentName(kaiConfig, shard)
	config := kaiConfig.Spec.Scheduler

	deployment, err := common.DeploymentForKAIConfig(ctx, readerClient, kaiConfig, config.Service, shardDeploymentName)
	if err != nil {
		return nil, err
	}
	cmObject, err := common.ObjectForKAIConfig(ctx, readerClient, &coreV1.ConfigMap{}, configMapName(kaiConfig, shard),
		kaiConfig.Spec.Namespace)
	if err != nil {
		return nil, err
	}
	schedulerConfig := cmObject.(*coreV1.ConfigMap)

	containerArgs, err := buildArgsList(
		shard, kaiConfig, configMountPath,
	)
	if err != nil {
		return nil, err
	}

	deployment.Spec.Replicas = config.Replicas
	deployment.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": shardDeploymentName,
		},
	}
	deployment.Spec.Strategy.Type = v1.RecreateDeploymentStrategyType
	deployment.Spec.Strategy.RollingUpdate = nil
	deployment.Spec.Template.ObjectMeta = metav1.ObjectMeta{
		Name: shardDeploymentName,
		Labels: map[string]string{
			"app": shardDeploymentName,
		},
		Annotations: map[string]string{
			"configMapVersion": schedulerConfig.ResourceVersion,
		},
	}
	deployment.Spec.Template.Spec.ServiceAccountName = *kaiConfig.Spec.Global.SchedulerName
	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
		{
			MountPath: configMountPath,
			Name:      "config",
			SubPath:   "config.yaml",
		},
	}
	deployment.Spec.Template.Spec.Containers[0].Env = []coreV1.EnvVar{
		{
			Name:  "GOGC",
			Value: fmt.Sprintf("%d", *config.GOGC),
		},
		{
			Name: "NAMESPACE",
			ValueFrom: &coreV1.EnvVarSource{
				FieldRef: &coreV1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
	}
	deployment.Spec.Template.Spec.Containers[0].Args = containerArgs
	deployment.Spec.Template.Spec.Volumes = []coreV1.Volume{
		{
			VolumeSource: coreV1.VolumeSource{
				ConfigMap: &coreV1.ConfigMapVolumeSource{
					LocalObjectReference: coreV1.LocalObjectReference{
						Name: configMapName(kaiConfig, shard),
					},
				},
			},
			Name: "config",
		},
	}

	return deployment, nil
}

func serviceAccountForKAIConfig(
	ctx context.Context, readerClient client.Reader,
	kaiConfig *kaiv1.Config,
) (*coreV1.ServiceAccount, error) {
	sa, err := common.ObjectForKAIConfig(ctx, readerClient, &coreV1.ServiceAccount{}, *kaiConfig.Spec.Global.SchedulerName,
		kaiConfig.Spec.Namespace)
	if err != nil {
		return nil, err
	}
	sa.(*coreV1.ServiceAccount).TypeMeta = metav1.TypeMeta{
		Kind:       "ServiceAccount",
		APIVersion: "v1",
	}
	return sa.(*coreV1.ServiceAccount), err
}

func configMapForShard(
	ctx context.Context, readerClient client.Reader,
	kaiConfig *kaiv1.Config, shard *kaiv1.SchedulingShard,
) (client.Object, error) {
	cmObject, err := common.ObjectForKAIConfig(ctx, readerClient, &coreV1.ConfigMap{}, configMapName(kaiConfig, shard),
		kaiConfig.Spec.Namespace)
	if err != nil {
		return nil, err
	}
	schedulerConfig := cmObject.(*coreV1.ConfigMap)
	schedulerConfig.TypeMeta = metav1.TypeMeta{
		Kind:       "ConfigMap",
		APIVersion: "v1",
	}
	placementArguments := calculatePlacementArguments(shard.Spec.Args.PlacementStrategy)
	innerConfig := config{}

	actions := []string{"allocate"}
	if placementArguments[gpuResource] != spreadStrategy && placementArguments[cpuResource] != spreadStrategy {
		actions = append(actions, "consolidation")
	}
	actions = append(actions, []string{"reclaim", "preempt", "stalegangeviction"}...)

	innerConfig.Actions = strings.Join(actions, ", ")

	innerConfig.Tiers = []tier{
		{
			Plugins: []plugin{
				{Name: "predicates"},
				{Name: "proportion"},
				{Name: "priority"},
				{Name: "nodeavailability"},
				{Name: "resourcetype"},
				{Name: "podaffinity"},
				{Name: "elastic"},
				{Name: "kubeflow"},
				{Name: "ray"},
				{Name: "taskorder"},
				{Name: "nominatednode"},
				{Name: "snapshot"},
				{Name: "dynamicresources"},
			},
		},
	}

	innerConfig.Tiers[0].Plugins = append(
		innerConfig.Tiers[0].Plugins,
		plugin{Name: fmt.Sprintf("gpu%s", strings.Replace(placementArguments[gpuResource], "bin", "", 1))},
		plugin{
			Name:      "nodeplacement",
			Arguments: placementArguments,
		},
	)

	if placementArguments[gpuResource] == binpackStrategy {
		innerConfig.Tiers[0].Plugins = append(
			innerConfig.Tiers[0].Plugins,
			plugin{Name: "gpusharingorder"},
		)
	}

	addMinRuntimePluginIfNeeded(&innerConfig.Tiers[0].Plugins, shard.Spec.Args)

	if len(shard.Spec.Args.QueueDepthPerAction) > 0 {
		if err = validateJobDepthMap(shard, innerConfig, actions); err != nil {
			return nil, err
		}
		// Set the validated map to the scheduler config
		innerConfig.QueueDepthPerAction = shard.Spec.Args.QueueDepthPerAction
	}

	data, marshalErr := yaml.Marshal(&innerConfig)
	if marshalErr != nil {
		return nil, marshalErr
	}
	schedulerConfig.Data = map[string]string{
		"config.yaml": string(data),
	}

	return schedulerConfig, nil
}

func validateJobDepthMap(shard *kaiv1.SchedulingShard, innerConfig config, actions []string) error {
	for actionToConfigure := range shard.Spec.Args.QueueDepthPerAction {
		if !slices.Contains(actions, actionToConfigure) {
			return fmt.Errorf(invalidJobDepthMapError, innerConfig.Actions, actionToConfigure)
		}
	}
	return nil
}

func serviceForShard(
	ctx context.Context, readerClient client.Reader,
	kaiConfig *kaiv1.Config, shard *kaiv1.SchedulingShard,
) (client.Object, error) {
	serviceName := fmt.Sprintf("%s-%s", *kaiConfig.Spec.Global.SchedulerName, shard.Name)
	serviceObj, err := common.ObjectForKAIConfig(ctx, readerClient, &coreV1.Service{}, serviceName,
		kaiConfig.Spec.Namespace)
	if err != nil {
		return nil, err
	}
	schedulerConfig := kaiConfig.Spec.Scheduler

	service := serviceObj.(*coreV1.Service)
	service.TypeMeta = metav1.TypeMeta{
		Kind:       "Service",
		APIVersion: "v1",
	}

	if service.Annotations == nil {
		service.Annotations = map[string]string{}
	}
	service.Annotations["prometheus.io/scrape"] = "true"

	service.Spec.ClusterIP = "None"
	service.Spec.Ports = []coreV1.ServicePort{
		{
			Name:       "http-metrics",
			Port:       int32(*schedulerConfig.SchedulerService.Port),
			Protocol:   coreV1.ProtocolTCP,
			TargetPort: intstr.FromInt(*schedulerConfig.SchedulerService.TargetPort),
		},
	}
	service.Spec.Selector = map[string]string{
		"app": serviceName,
	}
	service.Spec.SessionAffinity = coreV1.ServiceAffinityNone
	service.Spec.Type = *schedulerConfig.SchedulerService.Type

	return service, err
}

func buildArgsList(
	shard *kaiv1.SchedulingShard, kaiConfig *kaiv1.Config, configName string,
) ([]string, error) {
	args := []string{
		"--v", strconv.Itoa(*shard.Spec.Args.Verbosity),
		"--schedule-period", *shard.Spec.Args.DefaultSchedulerPeriod,
		"--scheduler-conf", configName,
		"--scheduler-name", *shard.Spec.Args.DefaultSchedulerName,
		"--namespace", kaiConfig.Spec.Namespace,
		"--nodepool-label-key", *kaiConfig.Spec.Global.NodePoolLabelKey,
		"--partition-label-value", shard.Spec.PartitionLabelValue,
		"--resource-reservation-app-label", *kaiConfig.Spec.Binder.ResourceReservation.AppLabel,
		"--cpu-worker-node-label-key", cpuWorkerNodeLabelKey,
		"--gpu-worker-node-label-key", gpuWorkerNodeLabelKey,
		"--mig-worker-node-label-key", migWorkerNodeLabelKey,
	}

	if kaiConfig.Spec.QueueController.MetricsNamespace != nil {
		args = append(args, "--metrics-namespace", *kaiConfig.Spec.QueueController.MetricsNamespace)
	}

	if shard.Spec.Args.RestrictNodeScheduling != nil && *shard.Spec.Args.RestrictNodeScheduling {
		args = append(args, "--restrict-node-scheduling")
	}

	if shard.Spec.Args.DetailedFitErrors != nil && *shard.Spec.Args.DetailedFitErrors {
		args = append(args, "--detailed-fit-errors")
	} else {
		args = append(args, "--detailed-fit-errors=false")
	}

	if *shard.Spec.Args.AdvancedCSIScheduling {
		args = append(args, "--schedule-csi-storage")
	}

	if shard.Spec.Args.MaxNumberConsolidationPreemptees != nil {
		args = append(args, "--max-consolidation-preemptees",
			strconv.Itoa(*shard.Spec.Args.MaxNumberConsolidationPreemptees))
	}

	if shard.Spec.Args.ClientQPS != nil {
		args = append(args, "--qps", strconv.Itoa(*shard.Spec.Args.ClientQPS))
	}

	if shard.Spec.Args.ClientBurst != nil {
		args = append(args, "--burst", strconv.Itoa(*shard.Spec.Args.ClientBurst))
	}

	if shard.Spec.Args.FullHierarchyFairness != nil {
		args = append(args, fmt.Sprintf("--full-hierarchy-fairness=%t", *shard.Spec.Args.FullHierarchyFairness))
	}

	if *shard.Spec.Args.UseSchedulingSignatures {
		args = append(args, "--use-scheduling-signatures=true")
	} else {
		args = append(args, "--use-scheduling-signatures=false")
	}

	if *shard.Spec.Args.Profiling {
		args = append(args, "--enable-profiler")
	}

	if shard.Spec.Args.Pyroscope != nil {
		args = append(args, "--pyroscope-address", shard.Spec.Args.Pyroscope.Address)
		if shard.Spec.Args.Pyroscope.MutexProfilerRate != nil {
			args = append(args, "--pyroscope-mutex-profiler-rate", strconv.Itoa(*shard.Spec.Args.Pyroscope.MutexProfilerRate))
		}
		if shard.Spec.Args.Pyroscope.BlockProfilerRate != nil {
			args = append(args, "--pyroscope-block-profiler-rate", strconv.Itoa(*shard.Spec.Args.Pyroscope.BlockProfilerRate))
		}
	}

	if *shard.Spec.Args.AllowConsolidatingReclaim {
		args = append(args, "--allow-consolidating-reclaim=true")
	} else {
		args = append(args, "--allow-consolidating-reclaim=false")
	}

	if shard.Spec.Args.NumOfStatusRecordingWorkers != nil {
		args = append(args, "--num-of-status-recording-workers",
			strconv.Itoa(*shard.Spec.Args.NumOfStatusRecordingWorkers))
	}

	if shard.Spec.Args.DefaultStalenessGracePeriod != nil {
		_, err := time.ParseDuration(*shard.Spec.Args.DefaultStalenessGracePeriod)
		if err != nil {
			return nil, fmt.Errorf("failed to parse staleness grace period: %w", err)
		}

		args = append(args, "--default-staleness-grace-period", *shard.Spec.Args.DefaultStalenessGracePeriod)
	}

	if featureGates := kaiConfigUtils.FeatureGatesArg(); featureGates != "" {
		args = append(args, featureGates)
	}
	schedulerConfig := kaiConfig.Spec.Scheduler
	if schedulerConfig.Replicas != nil && *schedulerConfig.Replicas > 1 {
		args = append(args, "--leader-elect")
	}

	return args, nil
}

func calculatePlacementArguments(placementStrategy *kaiv1.PlacementStrategy) map[string]string {
	return map[string]string{
		gpuResource: *placementStrategy.Gpu, cpuResource: *placementStrategy.Cpu,
	}
}

func addMinRuntimePluginIfNeeded(plugins *[]plugin, schedulingShardArgs *kaiv1.Args) {
	if schedulingShardArgs.PreemptMinRuntime == nil && schedulingShardArgs.ReclaimMinRuntime == nil {
		return
	}

	minRuntimePlugin := plugin{Name: "minruntime", Arguments: map[string]string{}}
	if schedulingShardArgs.PreemptMinRuntime != nil {
		minRuntimePlugin.Arguments["defaultPreemptMinRuntime"] = *schedulingShardArgs.PreemptMinRuntime
	}
	if schedulingShardArgs.ReclaimMinRuntime != nil {
		minRuntimePlugin.Arguments["defaultReclaimMinRuntime"] = *schedulingShardArgs.ReclaimMinRuntime
	}

	*plugins = append(*plugins, minRuntimePlugin)
}

func configMapName(config *kaiv1.Config, shard *kaiv1.SchedulingShard) string {
	return fmt.Sprintf("%s-%s", *config.Spec.Global.SchedulerName, shard.Name)
}

func deploymentName(config *kaiv1.Config, shard *kaiv1.SchedulingShard) string {
	return fmt.Sprintf("%s-%s", *config.Spec.Global.SchedulerName, shard.Name)
}
