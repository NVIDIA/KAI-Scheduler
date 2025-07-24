/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package podhooks

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("KaiAdmission Webhook", func() {
	var (
		defaulter *podMutator
	)

	BeforeEach(func() {
		Expect(defaulter).NotTo(BeNil(), "Expected defaulter to be initialized")
	})

	AfterEach(func() {
	})

	Context("When creating KaiAdmission under Defaulting Webhook", func() {

		It("Should apply defaults when a required field is empty", func() {
			By("simulating a scenario where defaults should be applied")
			ctx := context.Background()
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name: "test-container",
						},
					},
				},
			}
			// Simulate a scenario where a default is needed: e.g., missing label "foo"
			if pod.Labels == nil {
				pod.Labels = map[string]string{}
			}
			delete(pod.Labels, "foo")
			By("calling the Default method to apply defaults")
			defaulter.Default(ctx, pod)
			By("checking that the default values are set")
			Expect(pod.Labels).To(HaveKeyWithValue("foo", "default_value"))
		})
	})
})
