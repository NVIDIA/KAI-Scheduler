package rd

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
	"k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateResourceClaim(namespace, deviceClassName string, deviceCount int) *v1beta1.ResourceClaim {
	return &v1beta1.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        utils.GenerateRandomK8sName(10),
			Namespace:   namespace,
			Annotations: map[string]string{},
			Labels: map[string]string{
				constants.AppLabelName: "engine-e2e",
			},
		},
		Spec: v1beta1.ResourceClaimSpec{
			Devices: v1beta1.DeviceClaim{
				Requests: []v1beta1.DeviceRequest{
					{
						Name:            "req",
						DeviceClassName: deviceClassName,
						Selectors:       nil,
						AllocationMode:  v1beta1.DeviceAllocationModeExactCount,
						Count:           int64(deviceCount),
					},
				},
			},
		},
	}
}

func CreateResourceClaimTemplate(namespace, deviceClassName string, deviceCount int) *v1beta1.ResourceClaimTemplate {
	return &v1beta1.ResourceClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:        utils.GenerateRandomK8sName(10),
			Namespace:   namespace,
			Annotations: map[string]string{},
			Labels: map[string]string{
				constants.AppLabelName: "engine-e2e",
			},
		},
		Spec: v1beta1.ResourceClaimTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					constants.AppLabelName: "engine-e2e",
				},
			},
			Spec: v1beta1.ResourceClaimSpec{
				Devices: v1beta1.DeviceClaim{
					Requests: []v1beta1.DeviceRequest{
						{
							Name:            "req",
							DeviceClassName: deviceClassName,
							Selectors:       nil,
							AllocationMode:  v1beta1.DeviceAllocationModeExactCount,
							Count:           int64(deviceCount),
						},
					},
				},
			},
		},
	}
}
