package api

import "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"

type UsageDBConfig struct {
	ClientType string `yaml:"clientType" json:"clientType"`
}

type Interface interface {
	GetResourceUsage() (*queue_info.ClusterUsage, error)
}
