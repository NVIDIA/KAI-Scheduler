// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package expectedruntime

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	enginev2alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	commonconstants "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
)

var _ = Describe("ExpectedRuntime Plugin", func() {
	var (
		plugin *expectedruntimePlugin
	)

	BeforeEach(func() {
		plugin = &expectedruntimePlugin{}
	})

	// Helper function to create a PodGroupInfo with annotations and last start timestamp
	createPodGroup := func(uid common_info.PodGroupID, expectedRuntime string, requeueNotBefore string, lastStartTime *time.Time, preemptible bool, activeTasksCount int) *podgroup_info.PodGroupInfo {
		// Create PodGroup CRD with annotations first
		annotations := make(map[string]string)
		if expectedRuntime != "" {
			annotations[commonconstants.ExpectedRuntimeAnnotation] = expectedRuntime
		}
		if requeueNotBefore != "" {
			annotations[commonconstants.RequeueNotBeforeAnnotation] = requeueNotBefore
		}

		preemptibility := enginev2alpha2.NonPreemptible
		if preemptible {
			preemptibility = enginev2alpha2.Preemptible
		}

		podGroup := &enginev2alpha2.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:        string(uid),
				Namespace:   "test",
				UID:         types.UID(uid),
				Annotations: annotations,
			},
			Spec: enginev2alpha2.PodGroupSpec{
				MinMember:      1,
				Queue:          "test-queue",
				Preemptibility: preemptibility,
			},
		}

		// Create PodGroupInfo and set PodGroup first
		// SetPodGroup will call setSubGroups which may reset PodSets, so we add pods after
		pg := podgroup_info.NewPodGroupInfo(uid)
		pg.SetPodGroup(podGroup)

		// Set Preemptibility manually (normally done by setPodGroupPriorityAndPreemptibility in ClusterInfo)
		pg.Preemptibility = preemptibility

		if lastStartTime != nil {
			pg.LastStartTimestamp = lastStartTime
		}

		// Add pods to the pod group using AddTaskInfo to properly initialize state
		// Must add after SetPodGroup because setSubGroups may reset PodSets
		// When PodGroup has no SubGroups, setSubGroups preserves existing default PodSet
		for i := 0; i < activeTasksCount; i++ {
			podID := common_info.PodID(fmt.Sprintf("%s-pod-%d", uid, i))
			podInfo := &pod_info.PodInfo{
				UID:    podID,
				Job:    uid,
				Status: pod_status.Running,
			}
			pg.AddTaskInfo(podInfo)
		}

		return pg
	}

	Describe("nominationFn", func() {
		var clusterInfo *api.ClusterInfo

		BeforeEach(func() {
			clusterInfo = api.NewClusterInfo()
		})

		Context("when job meets all eligibility criteria", func() {
			It("should nominate the job", func() {
				now := time.Now()
				startTime := now.Add(-2 * time.Hour) // Started 2 hours ago
				expectedRuntime := "1h"              // Expected runtime is 1 hour

				job := createPodGroup("job1", expectedRuntime, "", &startTime, true, 1)
				clusterInfo.PodGroupInfos[job.UID] = job

				// Verify job setup
				Expect(job.GetActiveAllocatedTasksCount()).To(Equal(1), "Job should have 1 active allocated task")
				Expect(job.IsPreemptibleJob()).To(BeTrue(), "Job should be preemptible")
				Expect(job.LastStartTimestamp).ToNot(BeNil(), "Job should have LastStartTimestamp")

				candidates := plugin.nominationFn(clusterInfo)

				Expect(candidates).To(HaveLen(1))
				Expect(candidates[0].UID).To(Equal(job.UID))
			})

			It("should nominate job at exact expected runtime boundary", func() {
				now := time.Now()
				startTime := now.Add(-1 * time.Hour) // Started exactly 1 hour ago
				expectedRuntime := "1h"              // Expected runtime is 1 hour

				job := createPodGroup("job1", expectedRuntime, "", &startTime, true, 1)
				clusterInfo.PodGroupInfos[job.UID] = job

				candidates := plugin.nominationFn(clusterInfo)

				Expect(candidates).To(HaveLen(1))
				Expect(candidates[0].UID).To(Equal(job.UID))
			})
		})

		Context("when job does not meet eligibility criteria", func() {
			It("should not nominate job without active tasks", func() {
				now := time.Now()
				startTime := now.Add(-2 * time.Hour)
				expectedRuntime := "1h"

				job := createPodGroup("job1", expectedRuntime, "", &startTime, true, 0) // 0 active tasks
				clusterInfo.PodGroupInfos[job.UID] = job

				candidates := plugin.nominationFn(clusterInfo)

				Expect(candidates).To(BeEmpty())
			})

			It("should not nominate non-preemptible job", func() {
				now := time.Now()
				startTime := now.Add(-2 * time.Hour)
				expectedRuntime := "1h"

				job := createPodGroup("job1", expectedRuntime, "", &startTime, false, 1) // non-preemptible
				clusterInfo.PodGroupInfos[job.UID] = job

				candidates := plugin.nominationFn(clusterInfo)

				Expect(candidates).To(BeEmpty())
			})

			It("should not nominate job without expected-runtime annotation", func() {
				now := time.Now()
				startTime := now.Add(-2 * time.Hour)

				job := createPodGroup("job1", "", "", &startTime, true, 1) // no expected-runtime
				clusterInfo.PodGroupInfos[job.UID] = job

				candidates := plugin.nominationFn(clusterInfo)

				Expect(candidates).To(BeEmpty())
			})

			It("should not nominate job with invalid expected-runtime", func() {
				now := time.Now()
				startTime := now.Add(-2 * time.Hour)

				job := createPodGroup("job1", "invalid-duration", "", &startTime, true, 1)
				clusterInfo.PodGroupInfos[job.UID] = job

				candidates := plugin.nominationFn(clusterInfo)

				Expect(candidates).To(BeEmpty())
			})

			It("should not nominate job that hasn't reached expected runtime", func() {
				now := time.Now()
				startTime := now.Add(-30 * time.Minute) // Started 30 minutes ago
				expectedRuntime := "1h"                 // Expected runtime is 1 hour

				job := createPodGroup("job1", expectedRuntime, "", &startTime, true, 1)
				clusterInfo.PodGroupInfos[job.UID] = job

				candidates := plugin.nominationFn(clusterInfo)

				Expect(candidates).To(BeEmpty())
			})

			It("should not nominate job without LastStartTimestamp", func() {
				expectedRuntime := "1h"

				job := createPodGroup("job1", expectedRuntime, "", nil, true, 1) // no last start time
				clusterInfo.PodGroupInfos[job.UID] = job

				candidates := plugin.nominationFn(clusterInfo)

				Expect(candidates).To(BeEmpty())
			})

			It("should not nominate job in cooldown period", func() {
				now := time.Now()
				startTime := now.Add(-2 * time.Hour)
				expectedRuntime := "1h"
				notBefore := now.Add(10 * time.Minute).Format(time.RFC3339) // Cooldown until 10 minutes from now

				job := createPodGroup("job1", expectedRuntime, notBefore, &startTime, true, 1)
				clusterInfo.PodGroupInfos[job.UID] = job

				candidates := plugin.nominationFn(clusterInfo)

				Expect(candidates).To(BeEmpty())
			})

			It("should nominate job after cooldown period expires", func() {
				now := time.Now()
				startTime := now.Add(-2 * time.Hour)
				expectedRuntime := "1h"
				notBefore := now.Add(-10 * time.Minute).Format(time.RFC3339) // Cooldown expired 10 minutes ago

				job := createPodGroup("job1", expectedRuntime, notBefore, &startTime, true, 1)
				clusterInfo.PodGroupInfos[job.UID] = job

				candidates := plugin.nominationFn(clusterInfo)

				Expect(candidates).To(HaveLen(1))
				Expect(candidates[0].UID).To(Equal(job.UID))
			})

			It("should not nominate job with clock skew (now < LastStartTimestamp)", func() {
				now := time.Now()
				startTime := now.Add(1 * time.Hour) // Start time in the future (clock skew)
				expectedRuntime := "1h"

				job := createPodGroup("job1", expectedRuntime, "", &startTime, true, 1)
				clusterInfo.PodGroupInfos[job.UID] = job

				candidates := plugin.nominationFn(clusterInfo)

				Expect(candidates).To(BeEmpty())
			})

			It("should not nominate job with invalid requeue-not-before annotation", func() {
				now := time.Now()
				startTime := now.Add(-2 * time.Hour)
				expectedRuntime := "1h"

				job := createPodGroup("job1", expectedRuntime, "invalid-timestamp", &startTime, true, 1)
				clusterInfo.PodGroupInfos[job.UID] = job

				candidates := plugin.nominationFn(clusterInfo)

				Expect(candidates).To(BeEmpty())
			})
		})

		Context("with multiple jobs", func() {
			It("should nominate only eligible jobs", func() {
				now := time.Now()

				// Eligible job
				startTime1 := now.Add(-2 * time.Hour)
				job1 := createPodGroup("job1", "1h", "", &startTime1, true, 1)
				clusterInfo.PodGroupInfos[job1.UID] = job1

				// Not eligible - hasn't reached expected runtime
				startTime2 := now.Add(-30 * time.Minute)
				job2 := createPodGroup("job2", "1h", "", &startTime2, true, 1)
				clusterInfo.PodGroupInfos[job2.UID] = job2

				// Not eligible - non-preemptible
				startTime3 := now.Add(-2 * time.Hour)
				job3 := createPodGroup("job3", "1h", "", &startTime3, false, 1)
				clusterInfo.PodGroupInfos[job3.UID] = job3

				candidates := plugin.nominationFn(clusterInfo)

				Expect(candidates).To(HaveLen(1))
				Expect(candidates[0].UID).To(Equal(job1.UID))
			})
		})
	})

	Describe("parseExpectedRuntime", func() {
		It("should parse valid duration string", func() {
			job := createPodGroup("job1", "2h", "", nil, true, 0)
			duration, err := plugin.parseExpectedRuntime(job)

			Expect(err).To(BeNil())
			Expect(duration).To(Equal(2 * time.Hour))
		})

		It("should parse minutes", func() {
			job := createPodGroup("job1", "30m", "", nil, true, 0)
			duration, err := plugin.parseExpectedRuntime(job)

			Expect(err).To(BeNil())
			Expect(duration).To(Equal(30 * time.Minute))
		})

		It("should return error for invalid duration", func() {
			job := createPodGroup("job1", "invalid", "", nil, true, 0)
			duration, err := plugin.parseExpectedRuntime(job)

			Expect(err).ToNot(BeNil())
			Expect(duration).To(Equal(time.Duration(0)))
		})

		It("should return error when annotation is missing", func() {
			job := createPodGroup("job1", "", "", nil, true, 0)
			duration, err := plugin.parseExpectedRuntime(job)

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("expected-runtime annotation not found"))
			Expect(duration).To(Equal(time.Duration(0)))
		})
	})

	Describe("parseRequeueNotBefore", func() {
		It("should parse valid RFC3339 timestamp", func() {
			now := time.Now()
			notBefore := now.Add(10 * time.Minute).Format(time.RFC3339)

			job := createPodGroup("job1", "", notBefore, nil, true, 0)
			timestamp, err := plugin.parseRequeueNotBefore(job)

			Expect(err).To(BeNil())
			Expect(timestamp).ToNot(BeNil())
			Expect(timestamp.Format(time.RFC3339)).To(Equal(notBefore))
		})

		It("should return nil when annotation is missing", func() {
			job := createPodGroup("job1", "", "", nil, true, 0)
			timestamp, err := plugin.parseRequeueNotBefore(job)

			Expect(err).To(BeNil())
			Expect(timestamp).To(BeNil())
		})

		It("should return error for invalid timestamp", func() {
			job := createPodGroup("job1", "", "invalid-timestamp", nil, true, 0)
			timestamp, err := plugin.parseRequeueNotBefore(job)

			Expect(err).ToNot(BeNil())
			Expect(timestamp).To(BeNil())
		})
	})

	Describe("isRuntimeExceeded", func() {
		It("should return true when runtime exceeds expected", func() {
			now := time.Now()
			startTime := now.Add(-2 * time.Hour)
			expectedRuntime := 1 * time.Hour

			job := createPodGroup("job1", "", "", &startTime, true, 0)
			result := plugin.isRuntimeExceeded(job, expectedRuntime, now)

			Expect(result).To(BeTrue())
		})

		It("should return true when runtime equals expected (boundary)", func() {
			now := time.Now()
			startTime := now.Add(-1 * time.Hour)
			expectedRuntime := 1 * time.Hour

			job := createPodGroup("job1", "", "", &startTime, true, 0)
			result := plugin.isRuntimeExceeded(job, expectedRuntime, now)

			Expect(result).To(BeTrue())
		})

		It("should return false when runtime is less than expected", func() {
			now := time.Now()
			startTime := now.Add(-30 * time.Minute)
			expectedRuntime := 1 * time.Hour

			job := createPodGroup("job1", "", "", &startTime, true, 0)
			result := plugin.isRuntimeExceeded(job, expectedRuntime, now)

			Expect(result).To(BeFalse())
		})

		It("should return false when LastStartTimestamp is missing", func() {
			now := time.Now()
			expectedRuntime := 1 * time.Hour

			job := createPodGroup("job1", "", "", nil, true, 0)
			result := plugin.isRuntimeExceeded(job, expectedRuntime, now)

			Expect(result).To(BeFalse())
		})

		It("should return false when clock skew detected", func() {
			now := time.Now()
			startTime := now.Add(1 * time.Hour) // Future time (clock skew)
			expectedRuntime := 1 * time.Hour

			job := createPodGroup("job1", "", "", &startTime, true, 0)
			result := plugin.isRuntimeExceeded(job, expectedRuntime, now)

			Expect(result).To(BeFalse())
		})
	})

	Describe("isInCooldown", func() {
		It("should return true when in cooldown", func() {
			now := time.Now()
			notBefore := now.Add(10 * time.Minute).Format(time.RFC3339)

			job := createPodGroup("job1", "", notBefore, nil, true, 0)
			result := plugin.isInCooldown(job, now)

			Expect(result).To(BeTrue())
		})

		It("should return false when cooldown expired", func() {
			now := time.Now()
			notBefore := now.Add(-10 * time.Minute).Format(time.RFC3339)

			job := createPodGroup("job1", "", notBefore, nil, true, 0)
			result := plugin.isInCooldown(job, now)

			Expect(result).To(BeFalse())
		})

		It("should return false when annotation is missing", func() {
			now := time.Now()

			job := createPodGroup("job1", "", "", nil, true, 0)
			result := plugin.isInCooldown(job, now)

			Expect(result).To(BeFalse())
		})

		It("should return true (conservative) when annotation is invalid", func() {
			now := time.Now()

			job := createPodGroup("job1", "", "invalid-timestamp", nil, true, 0)
			result := plugin.isInCooldown(job, now)

			Expect(result).To(BeTrue()) // Conservative: skip on error
		})
	})
})
