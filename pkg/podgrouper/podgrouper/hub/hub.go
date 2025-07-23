// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package pluginshub

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/aml"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/cronjobs"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/deployment"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/grouper"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/grove"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/job"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/knative"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/kubeflow"
	jaxplugin "github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/kubeflow/jax"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/kubeflow/mpi"
	notebookplugin "github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/kubeflow/notebook"
	pytorchplugin "github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/kubeflow/pytorch"
	tensorflowlugin "github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/kubeflow/tensorflow"
	xgboostplugin "github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/kubeflow/xgboost"
	leader_worker_set "github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/leaderworkerset"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/podjob"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/ray"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/runaijob"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/skiptopowner"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/spark"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/spotrequest"
)

const (
	apiGroupArgo                     = "argoproj.io"
	apiGroupRunai                    = "run.ai"
	kindTrainingWorkload             = "TrainingWorkload"
	kindInteractiveWorkload          = "InteractiveWorkload"
	kindDistributedWorkload          = "DistributedWorkload"
	kindInferenceWorkload            = "InferenceWorkload"
	kindDistributedInferenceWorkload = "DistributedInferenceWorkload"
)

// +kubebuilder:rbac:groups=apps,resources=replicasets;statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=replicasets/finalizers;statefulsets/finalizers,verbs=patch;update;create
// +kubebuilder:rbac:groups=machinelearning.seldon.io,resources=seldondeployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=machinelearning.seldon.io,resources=seldondeployments/finalizers,verbs=patch;update;create
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines;virtualmachineinstances,verbs=get;list;watch
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines/finalizers;virtualmachineinstances/finalizers,verbs=patch;update;create
// +kubebuilder:rbac:groups=workspace.devfile.io,resources=devworkspaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=workspace.devfile.io,resources=devworkspaces/finalizers,verbs=patch;update;create
// +kubebuilder:rbac:groups=argoproj.io,resources=workflows,verbs=get;list;watch
// +kubebuilder:rbac:groups=argoproj.io,resources=workflows/finalizers,verbs=patch;update;create
// +kubebuilder:rbac:groups=tekton.dev,resources=pipelineruns;taskruns,verbs=get;list;watch
// +kubebuilder:rbac:groups=tekton.dev,resources=pipelineruns/finalizers;taskruns/finalizers,verbs=patch;update;create
// +kubebuilder:rbac:groups=run.ai,resources=trainingworkloads;interactiveworkloads;distributedworkloads;inferenceworkloads;distributedinferenceworkloads,verbs=get;list;watch

type PluginsHub struct {
	defaultPlugin *defaultgrouper.DefaultGrouper
	customPlugins map[metav1.GroupVersionKind]grouper.Grouper
}

func (ph *PluginsHub) GetPodGrouperPlugin(gvk metav1.GroupVersionKind) grouper.Grouper {
	if f, found := ph.customPlugins[gvk]; found {
		return f
	}

	// search using wildcard version
	gvk.Version = "*"
	if f, found := ph.customPlugins[gvk]; found {
		return f
	}
	return ph.defaultPlugin
}

func NewPluginsHub(kubeClient client.Client, searchForLegacyPodGroups,
	gangScheduleKnative bool, queueLabelKey, nodePoolLabelKey string,
	defaultPrioritiesConfigMapName, defaultPrioritiesConfigMapNamespace string) *PluginsHub {
	defaultGrouper := defaultgrouper.NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, kubeClient)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams(defaultPrioritiesConfigMapName, defaultPrioritiesConfigMapNamespace)

	kubeFlowDistributedGrouper := kubeflow.NewKubeflowDistributedGrouper(defaultGrouper)
	mpiGrouper := mpi.NewMpiGrouper(kubeClient, kubeFlowDistributedGrouper)

	rayGrouper := ray.NewRayGrouper(kubeClient, defaultGrouper)
	rayClusterGrouper := ray.NewRayClusterGrouper(rayGrouper)
	rayJobGrouper := ray.NewRayJobGrouper(rayGrouper)
	rayServiceGrouper := ray.NewRayServiceGrouper(rayGrouper)

	sparkGrouper := spark.NewSparkGrouper(defaultGrouper)
	podJobGrouper := podjob.NewPodJobGrouper(defaultGrouper, sparkGrouper)

	groveGrouper := grove.NewGroveGrouper(kubeClient, defaultGrouper)

	table := map[metav1.GroupVersionKind]grouper.Grouper{
		{
			Group:   "apps",
			Version: "v1",
			Kind:    "Deployment",
		}: deployment.NewDeploymentGrouper(defaultGrouper),
		{
			Group:   "machinelearning.seldon.io",
			Version: "v1alpha2",
			Kind:    "SeldonDeployment",
		}: defaultGrouper,
		{
			Group:   "machinelearning.seldon.io",
			Version: "v1",
			Kind:    "SeldonDeployment",
		}: defaultGrouper,
		{
			Group:   "kubevirt.io",
			Version: "v1",
			Kind:    "VirtualMachineInstance",
		}: defaultGrouper,
		{
			Group:   "kubeflow.org",
			Version: "v1",
			Kind:    "TFJob",
		}: tensorflowlugin.NewTensorFlowGrouper(kubeFlowDistributedGrouper),
		{
			Group:   "kubeflow.org",
			Version: "v1",
			Kind:    "PyTorchJob",
		}: pytorchplugin.NewPyTorchGrouper(kubeFlowDistributedGrouper),
		{
			Group:   "kubeflow.org",
			Version: "v1",
			Kind:    "XGBoostJob",
		}: xgboostplugin.NewXGBoostGrouper(kubeFlowDistributedGrouper),
		{
			Group:   "kubeflow.org",
			Version: "v1",
			Kind:    "JAXJob",
		}: jaxplugin.NewJaxGrouper(kubeFlowDistributedGrouper),
		{
			Group:   "kubeflow.org",
			Version: "v1",
			Kind:    "MPIJob",
		}: mpiGrouper,
		{
			Group:   "kubeflow.org",
			Version: "v2beta1",
			Kind:    "MPIJob",
		}: mpiGrouper,
		{
			Group:   "kubeflow.org",
			Version: "v1beta1",
			Kind:    "Notebook",
		}: notebookplugin.NewNotebookGrouper(defaultGrouper),
		{
			Group:   "batch",
			Version: "v1",
			Kind:    "Job",
		}: job.NewK8sJobGrouper(kubeClient, defaultGrouper, searchForLegacyPodGroups),
		{
			Group:   "apps",
			Version: "v1",
			Kind:    "StatefulSet",
		}: defaultGrouper,
		{
			Group:   "apps",
			Version: "v1",
			Kind:    "ReplicaSet",
		}: defaultGrouper,
		{
			Group:   "run.ai",
			Version: "v1",
			Kind:    "RunaiJob",
		}: runaijob.NewRunaiJobGrouper(kubeClient, defaultGrouper, searchForLegacyPodGroups),
		{
			Group:   "",
			Version: "v1",
			Kind:    "Pod",
		}: podJobGrouper,
		{
			Group:   "amlarc.azureml.com",
			Version: "v1alpha1",
			Kind:    "AmlJob",
		}: aml.NewAmlGrouper(defaultGrouper),
		{
			Group:   "serving.knative.dev",
			Version: "v1",
			Kind:    "Service",
		}: knative.NewKnativeGrouper(kubeClient, defaultGrouper, gangScheduleKnative),
		{
			Group:   "batch",
			Version: "v1",
			Kind:    "CronJob",
		}: cronjobs.NewCronJobGrouper(kubeClient, defaultGrouper),
		{
			Group:   "workspace.devfile.io",
			Version: "v1alpha2",
			Kind:    "DevWorkspace",
		}: defaultGrouper,
		{
			Group:   "ray.io",
			Version: "v1alpha1",
			Kind:    "RayCluster",
		}: rayClusterGrouper,
		{
			Group:   "ray.io",
			Version: "v1alpha1",
			Kind:    "RayJob",
		}: rayJobGrouper,
		{
			Group:   "ray.io",
			Version: "v1alpha1",
			Kind:    "RayService",
		}: rayServiceGrouper,
		{
			Group:   "ray.io",
			Version: "v1",
			Kind:    "RayCluster",
		}: rayClusterGrouper,
		{
			Group:   "ray.io",
			Version: "v1",
			Kind:    "RayJob",
		}: rayJobGrouper,
		{
			Group:   "ray.io",
			Version: "v1",
			Kind:    "RayService",
		}: rayServiceGrouper,
		{
			Group:   "kubeflow.org",
			Version: "v1alpha1",
			Kind:    "ScheduledWorkflow",
		}: defaultGrouper,
		{
			Group:   "tekton.dev",
			Version: "v1",
			Kind:    "PipelineRun",
		}: defaultGrouper,
		{
			Group:   "tekton.dev",
			Version: "v1",
			Kind:    "TaskRun",
		}: defaultGrouper,
		{
			Group:   "egx.nvidia.io",
			Version: "v1",
			Kind:    "SPOTRequest",
		}: spotrequest.NewSpotRequestGrouper(defaultGrouper),
		{
			Group:   "leaderworkerset.x-k8s.io",
			Version: "v1",
			Kind:    "LeaderWorkerSet",
		}: leader_worker_set.NewLwsGrouper(defaultGrouper),
		{
			Group:   "grove.io",
			Version: "v1alpha1",
			Kind:    "PodGangSet",
		}: groveGrouper,
	}

	skipTopOwnerGrouper := skiptopowner.NewSkipTopOwnerGrouper(kubeClient, defaultGrouper, table)
	table[metav1.GroupVersionKind{
		Group:   apiGroupArgo,
		Version: "v1alpha1",
		Kind:    "Workflow",
	}] = skipTopOwnerGrouper

	for _, kind := range []string{
		kindInferenceWorkload,
		kindTrainingWorkload,
		kindDistributedWorkload,
		kindInteractiveWorkload,
		kindDistributedInferenceWorkload,
	} {
		table[metav1.GroupVersionKind{
			Group:   apiGroupRunai,
			Version: "*",
			Kind:    kind,
		}] = skipTopOwnerGrouper
	}

	return &PluginsHub{
		defaultPlugin: defaultGrouper,
		customPlugins: table,
	}
}
