/*
Copyright 2025 The Volcano Authors.

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

package admission

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	schedulingv1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"
	"volcano.sh/volcano/test/e2e/util"
)

var _ = ginkgo.Describe("PodGroup Mutating Webhook E2E Test", func() {

	ginkgo.It("Should set queue from namespace annotation when PodGroup uses default queue", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		// First, add queue annotation to the namespace
		namespace, err := testCtx.Kubeclient.CoreV1().Namespaces().Get(context.TODO(), testCtx.Namespace, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		if namespace.Annotations == nil {
			namespace.Annotations = make(map[string]string)
		}
		namespace.Annotations[schedulingv1beta1.QueueNameAnnotationKey] = "custom-queue"

		_, err = testCtx.Kubeclient.CoreV1().Namespaces().Update(context.TODO(), namespace, metav1.UpdateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// Create PodGroup with default queue
		podGroup := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default-queue-podgroup",
				Namespace: testCtx.Namespace,
			},
			Spec: schedulingv1beta1.PodGroupSpec{
				Queue: schedulingv1beta1.DefaultQueue, // Using default queue
			},
		}

		createdPodGroup, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(testCtx.Namespace).Create(context.TODO(), podGroup, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(createdPodGroup.Spec.Queue).To(gomega.Equal("custom-queue"))
	})

	ginkgo.It("Should not modify queue when PodGroup uses non-default queue", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		// Add queue annotation to the namespace
		namespace, err := testCtx.Kubeclient.CoreV1().Namespaces().Get(context.TODO(), testCtx.Namespace, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		if namespace.Annotations == nil {
			namespace.Annotations = make(map[string]string)
		}
		namespace.Annotations[schedulingv1beta1.QueueNameAnnotationKey] = "namespace-queue"

		_, err = testCtx.Kubeclient.CoreV1().Namespaces().Update(context.TODO(), namespace, metav1.UpdateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// Create PodGroup with explicitly specified queue
		podGroup := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "custom-queue-podgroup",
				Namespace: testCtx.Namespace,
			},
			Spec: schedulingv1beta1.PodGroupSpec{
				Queue: "explicit-queue", // Non-default queue
			},
		}

		createdPodGroup, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(testCtx.Namespace).Create(context.TODO(), podGroup, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		// Should keep the explicitly specified queue, not change to namespace annotation
		gomega.Expect(createdPodGroup.Spec.Queue).To(gomega.Equal("explicit-queue"))
	})

	ginkgo.It("Should not modify queue when namespace has no queue annotation", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		// Ensure namespace has no queue annotation
		namespace, err := testCtx.Kubeclient.CoreV1().Namespaces().Get(context.TODO(), testCtx.Namespace, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		if namespace.Annotations != nil {
			delete(namespace.Annotations, schedulingv1beta1.QueueNameAnnotationKey)
		}

		_, err = testCtx.Kubeclient.CoreV1().Namespaces().Update(context.TODO(), namespace, metav1.UpdateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// Create PodGroup with default queue
		podGroup := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "no-annotation-podgroup",
				Namespace: testCtx.Namespace,
			},
			Spec: schedulingv1beta1.PodGroupSpec{
				Queue: schedulingv1beta1.DefaultQueue,
			},
		}

		createdPodGroup, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(testCtx.Namespace).Create(context.TODO(), podGroup, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		// Should remain as default queue since no namespace annotation exists
		gomega.Expect(createdPodGroup.Spec.Queue).To(gomega.Equal(schedulingv1beta1.DefaultQueue))
	})

	ginkgo.It("Should handle empty queue annotation value correctly", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		// Set empty queue annotation
		namespace, err := testCtx.Kubeclient.CoreV1().Namespaces().Get(context.TODO(), testCtx.Namespace, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		if namespace.Annotations == nil {
			namespace.Annotations = make(map[string]string)
		}
		namespace.Annotations[schedulingv1beta1.QueueNameAnnotationKey] = "" // Empty value

		_, err = testCtx.Kubeclient.CoreV1().Namespaces().Update(context.TODO(), namespace, metav1.UpdateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// Create PodGroup with default queue
		podGroup := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "empty-annotation-podgroup",
				Namespace: testCtx.Namespace,
			},
			Spec: schedulingv1beta1.PodGroupSpec{
				Queue: schedulingv1beta1.DefaultQueue,
			},
		}

		createdPodGroup, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(testCtx.Namespace).Create(context.TODO(), podGroup, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		// Should be set to empty string from annotation
		gomega.Expect(createdPodGroup.Spec.Queue).To(gomega.Equal(""))
	})

	ginkgo.It("Should preserve other PodGroup fields when mutating queue", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		// Add queue annotation to namespace
		namespace, err := testCtx.Kubeclient.CoreV1().Namespaces().Get(context.TODO(), testCtx.Namespace, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		if namespace.Annotations == nil {
			namespace.Annotations = make(map[string]string)
		}
		namespace.Annotations[schedulingv1beta1.QueueNameAnnotationKey] = "annotated-queue"

		_, err = testCtx.Kubeclient.CoreV1().Namespaces().Update(context.TODO(), namespace, metav1.UpdateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// Create PodGroup with other fields set
		minMember := int32(3)
		podGroup := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "preserve-fields-podgroup",
				Namespace: testCtx.Namespace,
				Labels: map[string]string{
					"test-label": "test-value",
				},
			},
			Spec: schedulingv1beta1.PodGroupSpec{
				Queue:          schedulingv1beta1.DefaultQueue, // Will be mutated
				MinMember:      minMember,
				PriorityClassName: "high-priority",
			},
		}

		createdPodGroup, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(testCtx.Namespace).Create(context.TODO(), podGroup, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// Queue should be mutated
		gomega.Expect(createdPodGroup.Spec.Queue).To(gomega.Equal("annotated-queue"))
		// Other fields should be preserved
		gomega.Expect(createdPodGroup.Spec.MinMember).To(gomega.Equal(int32(3)))
		gomega.Expect(createdPodGroup.Spec.PriorityClassName).To(gomega.Equal("high-priority"))
		gomega.Expect(createdPodGroup.ObjectMeta.Labels["test-label"]).To(gomega.Equal("test-value"))
	})

	ginkgo.It("Should handle namespace with multiple annotations correctly", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		// Add multiple annotations including queue annotation
		namespace, err := testCtx.Kubeclient.CoreV1().Namespaces().Get(context.TODO(), testCtx.Namespace, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		if namespace.Annotations == nil {
			namespace.Annotations = make(map[string]string)
		}
		namespace.Annotations["custom-annotation"] = "custom-value"
		namespace.Annotations["another-annotation"] = "another-value"
		namespace.Annotations[schedulingv1beta1.QueueNameAnnotationKey] = "multi-annotation-queue"

		_, err = testCtx.Kubeclient.CoreV1().Namespaces().Update(context.TODO(), namespace, metav1.UpdateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// Create PodGroup with default queue
		podGroup := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "multi-annotation-podgroup",
				Namespace: testCtx.Namespace,
			},
			Spec: schedulingv1beta1.PodGroupSpec{
				Queue: schedulingv1beta1.DefaultQueue,
			},
		}

		createdPodGroup, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(testCtx.Namespace).Create(context.TODO(), podGroup, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		// Should correctly pick up queue annotation value
		gomega.Expect(createdPodGroup.Spec.Queue).To(gomega.Equal("multi-annotation-queue"))
	})
})