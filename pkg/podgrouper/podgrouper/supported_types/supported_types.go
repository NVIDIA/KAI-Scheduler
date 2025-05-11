// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package supportedtypes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/aml"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/cronjobs"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/deployment"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/job"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/knative"
	jaxplugin "github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/kubeflow/jax"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/kubeflow/mpi"
	notebookplugin "github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/kubeflow/notebook"
	pytorchplugin "github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/kubeflow/pytorch"
	tensorflowlugin "github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/kubeflow/tensorflow"
	xgboostplugin "github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/kubeflow/xgboost"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/podjob"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/ray"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/runaijob"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/skiptopowner"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/spotrequest"
)

const (
	apiGroupArgo            = "argoproj.io"
	apiGroupRunai           = "run.ai"
	kindTrainingWorkload    = "TrainingWorkload"
	kindInteractiveWorkload = "InteractiveWorkload"
	kindDistributedWorkload = "DistributedWorkload"
	kindInferenceWorkload   = "InferenceWorkload"
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
// +kubebuilder:rbac:groups=run.ai,resources=trainingworkloads;interactiveworkloads;distributedworkloads;inferenceworkloads,verbs=get;list;watch

type SupportedTypes interface {
	GetPodGroupMetadataFunc(metav1.GroupVersionKind) (defaultgrouper.Grouper, bool)
}

type supportedTypes map[metav1.GroupVersionKind]defaultgrouper.Grouper

func (s supportedTypes) GetPodGroupMetadataFunc(gvk metav1.GroupVersionKind) (defaultgrouper.Grouper, bool) {
	if f, found := s[gvk]; found {
		return f, true
	}

	// search using wildcard version
	gvk.Version = "*"
	if f, found := s[gvk]; found {
		return f, true
	}
	return nil, false
}

func NewSupportedTypes(kubeClient client.Client, searchForLegacyPodGroups,
	gangScheduleKnative bool, queueLabelKey string) SupportedTypes {

	defaultGrouper := defaultgrouper.NewDefaultGrouper(queueLabelKey)
	mpiGrouper := mpi.NewMpiGrouper(kubeClient, queueLabelKey)
	rayClusterGrouper := ray.NewRayClusterGrouper(kubeClient, queueLabelKey)
	rayJobGrouper := ray.NewRayJobGrouper(kubeClient, queueLabelKey)
	rayServiceGrouper := ray.NewRayServiceGrouper(kubeClient, queueLabelKey)

	table := supportedTypes{
		{
			Group:   "apps",
			Version: "v1",
			Kind:    "Deployment",
		}: deployment.NewDeploymentGrouper(queueLabelKey),
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
		}: tensorflowlugin.NewTensorflowGrouper(queueLabelKey),
		{
			Group:   "kubeflow.org",
			Version: "v1",
			Kind:    "PyTorchJob",
		}: pytorchplugin.NewPytorchGrouper(queueLabelKey),
		{
			Group:   "kubeflow.org",
			Version: "v1",
			Kind:    "XGBoostJob",
		}: xgboostplugin.NewXGBoostGrouper(queueLabelKey),
		{
			Group:   "kubeflow.org",
			Version: "v1",
			Kind:    "JAXJob",
		}: jaxplugin.NewJaxGrouper(queueLabelKey),
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
		}: notebookplugin.NewNotebookGrouper(queueLabelKey),
		{
			Group:   "batch",
			Version: "v1",
			Kind:    "Job",
		}: job.NewK8sJobGrouper(kubeClient, queueLabelKey, searchForLegacyPodGroups),
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
		}: runaijob.NewRunaiJobGrouper(kubeClient, queueLabelKey, searchForLegacyPodGroups),
		{
			Group:   "",
			Version: "v1",
			Kind:    "Pod",
		}: podjob.NewPodJobGrouper(queueLabelKey),
		{
			Group:   "amlarc.azureml.com",
			Version: "v1alpha1",
			Kind:    "AmlJob",
		}: aml.NewAmlGrouper(queueLabelKey),
		{
			Group:   "serving.knative.dev",
			Version: "v1",
			Kind:    "Service",
		}: knative.NewKnativeGrouper(kubeClient, queueLabelKey, gangScheduleKnative),
		{
			Group:   "batch",
			Version: "v1",
			Kind:    "CronJob",
		}: cronjobs.NewCronJobGrouper(kubeClient, queueLabelKey),
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
		}: spotrequest.NewSpotRequestGrouper(queueLabelKey),
	}

	skipTopOwnerGrouper := skiptopowner.NewSkipTopOwnerGrouper(kubeClient, defaultGrouper, table)
	table[metav1.GroupVersionKind{
		Group:   apiGroupArgo,
		Version: "v1alpha1",
		Kind:    "Workflow",
	}] = skipTopOwnerGrouper

	for _, kind := range []string{kindInferenceWorkload, kindTrainingWorkload, kindDistributedWorkload, kindInteractiveWorkload} {
		table[metav1.GroupVersionKind{
			Group:   apiGroupRunai,
			Version: "*",
			Kind:    kind,
		}] = skipTopOwnerGrouper
	}

	return table
}
