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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	schedulingv1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"
	"volcano.sh/volcano/test/e2e/util"
)

var _ = ginkgo.Describe("PodGroup Mutating Webhook E2E Test", func() {

	ginkgo.It("Should not modify podgroup with custom queue", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		customQueue := "custom-queue"
		podgroup := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "custom-queue-podgroup",
				Namespace: testCtx.Namespace,
			},
			Spec: schedulingv1beta1.PodGroupSpec{
				Queue:        customQueue,
				MinMember:    1,
				MinResources: nil,
			},
		}

		createdPodGroup, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(testCtx.Namespace).Create(context.TODO(), podgroup, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(createdPodGroup.Spec.Queue).To(gomega.Equal(customQueue))
	})

	ginkgo.It("Should set queue from namespace annotation when podgroup uses default queue", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		// First, update the namespace with queue annotation
		namespaceQueueName := "namespace-queue"
		ns, err := testCtx.Kubeclient.CoreV1().Namespaces().Get(context.TODO(), testCtx.Namespace, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		if ns.Annotations == nil {
			ns.Annotations = make(map[string]string)
		}
		ns.Annotations[schedulingv1beta1.QueueNameAnnotationKey] = namespaceQueueName

		_, err = testCtx.Kubeclient.CoreV1().Namespaces().Update(context.TODO(), ns, metav1.UpdateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// Create podgroup with default queue
		podgroup := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default-queue-podgroup",
				Namespace: testCtx.Namespace,
			},
			Spec: schedulingv1beta1.PodGroupSpec{
				Queue:        schedulingv1beta1.DefaultQueue,
				MinMember:    1,
				MinResources: nil,
			},
		}

		createdPodGroup, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(testCtx.Namespace).Create(context.TODO(), podgroup, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(createdPodGroup.Spec.Queue).To(gomega.Equal(namespaceQueueName))
	})

	ginkgo.It("Should keep default queue when namespace has no queue annotation", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		// Ensure namespace has no queue annotation
		ns, err := testCtx.Kubeclient.CoreV1().Namespaces().Get(context.TODO(), testCtx.Namespace, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		if ns.Annotations != nil {
			delete(ns.Annotations, schedulingv1beta1.QueueNameAnnotationKey)
			_, err = testCtx.Kubeclient.CoreV1().Namespaces().Update(context.TODO(), ns, metav1.UpdateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		}

		// Create podgroup with default queue
		podgroup := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "no-annotation-podgroup",
				Namespace: testCtx.Namespace,
			},
			Spec: schedulingv1beta1.PodGroupSpec{
				Queue:        schedulingv1beta1.DefaultQueue,
				MinMember:    1,
				MinResources: nil,
			},
		}

		createdPodGroup, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(testCtx.Namespace).Create(context.TODO(), podgroup, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(createdPodGroup.Spec.Queue).To(gomega.Equal(schedulingv1beta1.DefaultQueue))
	})

	ginkgo.It("Should handle empty queue annotation in namespace", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		// Set empty queue annotation in namespace
		ns, err := testCtx.Kubeclient.CoreV1().Namespaces().Get(context.TODO(), testCtx.Namespace, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		if ns.Annotations == nil {
			ns.Annotations = make(map[string]string)
		}
		ns.Annotations[schedulingv1beta1.QueueNameAnnotationKey] = ""

		_, err = testCtx.Kubeclient.CoreV1().Namespaces().Update(context.TODO(), ns, metav1.UpdateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// Create podgroup with default queue
		podgroup := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "empty-annotation-podgroup",
				Namespace: testCtx.Namespace,
			},
			Spec: schedulingv1beta1.PodGroupSpec{
				Queue:        schedulingv1beta1.DefaultQueue,
				MinMember:    1,
				MinResources: nil,
			},
		}

		createdPodGroup, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(testCtx.Namespace).Create(context.TODO(), podgroup, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		// Even with empty annotation value, it should be set (this tests the mutation logic)
		gomega.Expect(createdPodGroup.Spec.Queue).To(gomega.Equal(""))
	})

	ginkgo.It("Should handle podgroup with empty queue field", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		// Set queue annotation in namespace
		namespaceQueueName := "namespace-queue-for-empty"
		ns, err := testCtx.Kubeclient.CoreV1().Namespaces().Get(context.TODO(), testCtx.Namespace, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		if ns.Annotations == nil {
			ns.Annotations = make(map[string]string)
		}
		ns.Annotations[schedulingv1beta1.QueueNameAnnotationKey] = namespaceQueueName

		_, err = testCtx.Kubeclient.CoreV1().Namespaces().Update(context.TODO(), ns, metav1.UpdateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// Create podgroup with empty queue (which should be treated as default)
		podgroup := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "empty-queue-podgroup",
				Namespace: testCtx.Namespace,
			},
			Spec: schedulingv1beta1.PodGroupSpec{
				Queue:        "", // Empty queue, should be treated as default
				MinMember:    1,
				MinResources: nil,
			},
		}

		createdPodGroup, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(testCtx.Namespace).Create(context.TODO(), podgroup, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		// Empty queue should not trigger mutation (only default queue does)
		gomega.Expect(createdPodGroup.Spec.Queue).To(gomega.Equal(""))
	})

	ginkgo.It("Should preserve other podgroup fields during mutation", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		// Set queue annotation in namespace
		namespaceQueueName := "namespace-queue-preserve"
		ns, err := testCtx.Kubeclient.CoreV1().Namespaces().Get(context.TODO(), testCtx.Namespace, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		if ns.Annotations == nil {
			ns.Annotations = make(map[string]string)
		}
		ns.Annotations[schedulingv1beta1.QueueNameAnnotationKey] = namespaceQueueName

		_, err = testCtx.Kubeclient.CoreV1().Namespaces().Update(context.TODO(), ns, metav1.UpdateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// Create podgroup with default queue and other fields
		podgroup := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "preserve-fields-podgroup",
				Namespace: testCtx.Namespace,
				Labels: map[string]string{
					"test-label": "test-value",
				},
				Annotations: map[string]string{
					"test-annotation": "test-value",
				},
			},
			Spec: schedulingv1beta1.PodGroupSpec{
				Queue:     schedulingv1beta1.DefaultQueue,
				MinMember: 5,
				MinResources: &corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			},
		}

		createdPodGroup, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(testCtx.Namespace).Create(context.TODO(), podgroup, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// Queue should be mutated
		gomega.Expect(createdPodGroup.Spec.Queue).To(gomega.Equal(namespaceQueueName))

		// Other fields should be preserved
		gomega.Expect(createdPodGroup.Spec.MinMember).To(gomega.Equal(int32(5)))
		gomega.Expect(createdPodGroup.Spec.MinResources).NotTo(gomega.BeNil())
		gomega.Expect((*createdPodGroup.Spec.MinResources)[corev1.ResourceCPU]).To(gomega.Equal(resource.MustParse("2")))
		gomega.Expect((*createdPodGroup.Spec.MinResources)[corev1.ResourceMemory]).To(gomega.Equal(resource.MustParse("4Gi")))
		gomega.Expect(createdPodGroup.Labels["test-label"]).To(gomega.Equal("test-value"))
		gomega.Expect(createdPodGroup.Annotations["test-annotation"]).To(gomega.Equal("test-value"))
	})

	ginkgo.It("Should handle multiple podgroups in same namespace", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		// Set queue annotation in namespace
		namespaceQueueName := "namespace-queue-multiple"
		ns, err := testCtx.Kubeclient.CoreV1().Namespaces().Get(context.TODO(), testCtx.Namespace, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		if ns.Annotations == nil {
			ns.Annotations = make(map[string]string)
		}
		ns.Annotations[schedulingv1beta1.QueueNameAnnotationKey] = namespaceQueueName

		_, err = testCtx.Kubeclient.CoreV1().Namespaces().Update(context.TODO(), ns, metav1.UpdateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// Create multiple podgroups with different queue configurations
		podgroup1 := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "multiple-1-default",
				Namespace: testCtx.Namespace,
			},
			Spec: schedulingv1beta1.PodGroupSpec{
				Queue:        schedulingv1beta1.DefaultQueue,
				MinMember:    1,
				MinResources: nil,
			},
		}

		podgroup2 := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "multiple-2-custom",
				Namespace: testCtx.Namespace,
			},
			Spec: schedulingv1beta1.PodGroupSpec{
				Queue:        "custom-queue",
				MinMember:    1,
				MinResources: nil,
			},
		}

		createdPodGroup1, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(testCtx.Namespace).Create(context.TODO(), podgroup1, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		createdPodGroup2, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(testCtx.Namespace).Create(context.TODO(), podgroup2, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// First podgroup should be mutated to use namespace queue
		gomega.Expect(createdPodGroup1.Spec.Queue).To(gomega.Equal(namespaceQueueName))

		// Second podgroup should keep its custom queue
		gomega.Expect(createdPodGroup2.Spec.Queue).To(gomega.Equal("custom-queue"))
	})
})
