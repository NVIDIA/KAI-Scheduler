package cleanup

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
)

// The ActionsCleanup is called to cleanup any remaining data created by the different actions that might persist after the session is closed.
func ActionsCleanup(ssn *framework.Session) {
	deallocatePipelinedPods(ssn)
}

func deallocatePipelinedPods(ssn *framework.Session) {
	stmt := ssn.Statement()
	for _, job := range ssn.ClusterInfo.PodGroupInfos {
		for _, task := range job.GetAllPodsMap() {
			if task.Status != pod_status.Pipelined {
				continue
			}
			if err := stmt.UnpipelineTask(task); err != nil {
				log.InfraLogger.Errorf("Failed to deallocate pipelined task <%v/%v>: %v",
					task.Namespace, task.Name, err)
			}
		}
	}
}
