package resource_division

import (
	consts "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	rs "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/resource_share"
	"knative.dev/pkg/ptr"
)

const (
	defaultRequest = 100000
)

type ResourceShareOverrides struct {
	Deserved                *float64
	FairShare               *float64
	MaxAllowed              *float64
	OverQuotaWeight         *float64
	Allocated               *float64
	AllocatedNotPreemptible *float64
	Request                 *float64
	AbsoluteUsage           *float64
	VacantAdjustedUsage     *float64
}

func (r *ResourceShareOverrides) ResourceShare() *rs.ResourceShare {
	rs := rs.ResourceShare{
		Deserved:                0,
		FairShare:               0,
		MaxAllowed:              consts.UnlimitedResourceQuantity,
		OverQuotaWeight:         1,
		Allocated:               0,
		AllocatedNotPreemptible: 0,
		Request:                 defaultRequest,
		AbsoluteUsage:           0,
	}
	if r.Deserved != nil {
		rs.Deserved = *r.Deserved
	}
	if r.FairShare != nil {
		rs.FairShare = *r.FairShare
	}
	if r.MaxAllowed != nil {
		rs.MaxAllowed = *r.MaxAllowed
	}
	if r.OverQuotaWeight != nil {
		rs.OverQuotaWeight = *r.OverQuotaWeight
	}
	if r.Allocated != nil {
		rs.Allocated = *r.Allocated
	}
	if r.AllocatedNotPreemptible != nil {
		rs.AllocatedNotPreemptible = *r.AllocatedNotPreemptible
	}
	if r.Request != nil {
		rs.Request = *r.Request
	}
	if r.AbsoluteUsage != nil {
		rs.AbsoluteUsage = *r.AbsoluteUsage
	}
	return &rs
}

type QueueResourceShareOverrides struct {
	GPU    *ResourceShareOverrides
	CPU    *ResourceShareOverrides
	Memory *ResourceShareOverrides
}

func (q QueueResourceShareOverrides) ResourceShare() rs.QueueResourceShare {
	if q.GPU == nil {
		q.GPU = &ResourceShareOverrides{}
	}
	if q.CPU == nil {
		q.CPU = &ResourceShareOverrides{
			Deserved: ptr.Float64(consts.UnlimitedResourceQuantity),
		}
	}
	if q.Memory == nil {
		q.Memory = &ResourceShareOverrides{
			Deserved: ptr.Float64(consts.UnlimitedResourceQuantity),
		}
	}
	rs := rs.QueueResourceShare{
		GPU:    *q.GPU.ResourceShare(),
		CPU:    *q.CPU.ResourceShare(),
		Memory: *q.Memory.ResourceShare(),
	}
	return rs
}
