// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package supportedtypes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/aml"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/cronjobs"
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
	GetPodGroupMetadataFunc(metav1.GroupVersionKind) (plugins.GetPodGroupMetadataFunc, bool)
}

type supportedTypes map[metav1.GroupVersionKind]plugins.GetPodGroupMetadataFunc

func (s supportedTypes) GetPodGroupMetadataFunc(gvk metav1.GroupVersionKind) (plugins.GetPodGroupMetadataFunc, bool) {
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

func NewSupportedTypes(kubeClient client.Client, searchForLegacyPodGroups, gangScheduleKnative bool) SupportedTypes {

	inferenceServiceGrouper := knative.NewKnativeGrouper(kubeClient, gangScheduleKnative)
	cronJobGrouper := cronjobs.NewCronJobGrouper(kubeClient)
	rayGrouper := ray.NewRayGrouper(kubeClient)
	runaiJobGrouper := runaijob.NewRunaiJobGrouper(kubeClient, searchForLegacyPodGroups)
	k8sJobGrouper := job.NewK8sJobGrouper(kubeClient, searchForLegacyPodGroups)
	mpiGrouper := mpi.NewMpiGrouper(kubeClient)

	table := supportedTypes{
		{
			Group:   "apps",
			Version: "v1",
			Kind:    "Deployment",
		}: deployment.GetPodGroupMetadata,
		{
			Group:   "machinelearning.seldon.io",
			Version: "v1alpha2",
			Kind:    "SeldonDeployment",
		}: plugins.GetPodGroupMetadata,
		{
			Group:   "machinelearning.seldon.io",
			Version: "v1",
			Kind:    "SeldonDeployment",
		}: plugins.GetPodGroupMetadata,
		{
			Group:   "kubevirt.io",
			Version: "v1",
			Kind:    "VirtualMachineInstance",
		}: plugins.GetPodGroupMetadata,
		{
			Group:   "kubeflow.org",
			Version: "v1",
			Kind:    "TFJob",
		}: tensorflowlugin.GetPodGroupMetadata,
		{
			Group:   "kubeflow.org",
			Version: "v1",
			Kind:    "PyTorchJob",
		}: pytorchplugin.GetPodGroupMetadata,
		{
			Group:   "kubeflow.org",
			Version: "v1",
			Kind:    "XGBoostJob",
		}: xgboostplugin.GetPodGroupMetadata,
		{
			Group:   "kubeflow.org",
			Version: "v1",
			Kind:    "JAXJob",
		}: jaxplugin.GetPodGroupMetadata,
		{
			Group:   "kubeflow.org",
			Version: "v1",
			Kind:    "MPIJob",
		}: mpiGrouper.GetPodGroupMetadata,
		{
			Group:   "kubeflow.org",
			Version: "v2beta1",
			Kind:    "MPIJob",
		}: mpiGrouper.GetPodGroupMetadata,
		{
			Group:   "kubeflow.org",
			Version: "v1beta1",
			Kind:    "Notebook",
		}: notebookplugin.GetPodGroupMetadata,
		{
			Group:   "batch",
			Version: "v1",
			Kind:    "Job",
		}: k8sJobGrouper.GetPodGroupMetadata,
		{
			Group:   "apps",
			Version: "v1",
			Kind:    "StatefulSet",
		}: plugins.GetPodGroupMetadata,
		{
			Group:   "apps",
			Version: "v1",
			Kind:    "ReplicaSet",
		}: plugins.GetPodGroupMetadata,
		{
			Group:   "run.ai",
			Version: "v1",
			Kind:    "RunaiJob",
		}: runaiJobGrouper.GetPodGroupMetadata,
		{
			Group:   "",
			Version: "v1",
			Kind:    "Pod",
		}: podjob.GetPodGroupMetadata,
		{
			Group:   "amlarc.azureml.com",
			Version: "v1alpha1",
			Kind:    "AmlJob",
		}: aml.GetPodGroupMetadata,
		{
			Group:   "serving.knative.dev",
			Version: "v1",
			Kind:    "Service",
		}: inferenceServiceGrouper.GetPodGroupMetadata,
		{
			Group:   "batch",
			Version: "v1",
			Kind:    "CronJob",
		}: cronJobGrouper.GetPodGroupMetadata,
		{
			Group:   "workspace.devfile.io",
			Version: "v1alpha2",
			Kind:    "DevWorkspace",
		}: plugins.GetPodGroupMetadata,
		{
			Group:   "ray.io",
			Version: "v1alpha1",
			Kind:    "RayCluster",
		}: rayGrouper.GetPodGroupMetadataForRayCluster,
		{
			Group:   "ray.io",
			Version: "v1alpha1",
			Kind:    "RayJob",
		}: rayGrouper.GetPodGroupMetadataForRayJob,
		{
			Group:   "ray.io",
			Version: "v1alpha1",
			Kind:    "RayService",
		}: rayGrouper.GetPodGroupMetadataForRayService,
		{
			Group:   "ray.io",
			Version: "v1",
			Kind:    "RayCluster",
		}: rayGrouper.GetPodGroupMetadataForRayCluster,
		{
			Group:   "ray.io",
			Version: "v1",
			Kind:    "RayJob",
		}: rayGrouper.GetPodGroupMetadataForRayJob,
		{
			Group:   "ray.io",
			Version: "v1",
			Kind:    "RayService",
		}: rayGrouper.GetPodGroupMetadataForRayService,
		{
			Group:   "kubeflow.org",
			Version: "v1alpha1",
			Kind:    "ScheduledWorkflow",
		}: plugins.GetPodGroupMetadata,
		{
			Group:   "tekton.dev",
			Version: "v1",
			Kind:    "PipelineRun",
		}: plugins.GetPodGroupMetadata,
		{
			Group:   "tekton.dev",
			Version: "v1",
			Kind:    "TaskRun",
		}: plugins.GetPodGroupMetadata,
		{
			Group:   "egx.nvidia.io",
			Version: "v1",
			Kind:    "SPOTRequest",
		}: spotrequest.GetPodGroupMetadata,
	}

	skipTopOwnerGrouper := skiptopowner.NewSkipTopOwnerGrouper(kubeClient, table)
	table[metav1.GroupVersionKind{
		Group:   apiGroupArgo,
		Version: "v1alpha1",
		Kind:    "Workflow",
	}] = skipTopOwnerGrouper.GetPodGroupMetadata

	for _, kind := range []string{kindInferenceWorkload, kindTrainingWorkload, kindDistributedWorkload, kindInteractiveWorkload} {
		table[metav1.GroupVersionKind{
			Group:   apiGroupRunai,
			Version: "*",
			Kind:    kind,
		}] = skipTopOwnerGrouper.GetPodGroupMetadata
	}

	return table
}
