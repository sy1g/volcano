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

	ginkgo.It("Should set queue from namespace annotation when podgroup uses default queue", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		// Create namespace with queue annotation
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testCtx.Namespace + "-with-queue",
				Annotations: map[string]string{
					schedulingv1beta1.QueueNameAnnotationKey: "namespace-queue",
				},
			},
		}
		_, err := testCtx.Kubeclient.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		defer func() {
			testCtx.Kubeclient.CoreV1().Namespaces().Delete(context.TODO(), ns.Name, metav1.DeleteOptions{})
		}()

		podgroup := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-podgroup-default-queue",
				Namespace: ns.Name,
			},
			Spec: schedulingv1beta1.PodGroupSpec{
				Queue:        schedulingv1beta1.DefaultQueue, // Using default queue
				MinMember:    1,
				MinResources: nil,
			},
		}

		createdPodGroup, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(ns.Name).Create(context.TODO(), podgroup, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(createdPodGroup.Spec.Queue).To(gomega.Equal("namespace-queue"))
	})

	ginkgo.It("Should not change queue when podgroup uses non-default queue", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		// Create namespace with queue annotation
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testCtx.Namespace + "-with-queue-custom",
				Annotations: map[string]string{
					schedulingv1beta1.QueueNameAnnotationKey: "namespace-queue",
				},
			},
		}
		_, err := testCtx.Kubeclient.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		defer func() {
			testCtx.Kubeclient.CoreV1().Namespaces().Delete(context.TODO(), ns.Name, metav1.DeleteOptions{})
		}()

		podgroup := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-podgroup-custom-queue",
				Namespace: ns.Name,
			},
			Spec: schedulingv1beta1.PodGroupSpec{
				Queue:        "custom-queue", // Using custom queue
				MinMember:    1,
				MinResources: nil,
			},
		}

		createdPodGroup, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(ns.Name).Create(context.TODO(), podgroup, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		// Should preserve the original custom queue
		gomega.Expect(createdPodGroup.Spec.Queue).To(gomega.Equal("custom-queue"))
	})

	ginkgo.It("Should not change queue when namespace has no queue annotation", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		// Create namespace without queue annotation
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testCtx.Namespace + "-no-queue",
			},
		}
		_, err := testCtx.Kubeclient.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		defer func() {
			testCtx.Kubeclient.CoreV1().Namespaces().Delete(context.TODO(), ns.Name, metav1.DeleteOptions{})
		}()

		podgroup := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-podgroup-no-annotation",
				Namespace: ns.Name,
			},
			Spec: schedulingv1beta1.PodGroupSpec{
				Queue:        schedulingv1beta1.DefaultQueue, // Using default queue
				MinMember:    1,
				MinResources: nil,
			},
		}

		createdPodGroup, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(ns.Name).Create(context.TODO(), podgroup, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		// Should remain as default queue since no namespace annotation exists
		gomega.Expect(createdPodGroup.Spec.Queue).To(gomega.Equal(schedulingv1beta1.DefaultQueue))
	})

	ginkgo.It("Should work with multiple podgroups in same namespace with queue annotation", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		// Create namespace with queue annotation
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testCtx.Namespace + "-multi-podgroups",
				Annotations: map[string]string{
					schedulingv1beta1.QueueNameAnnotationKey: "shared-queue",
				},
			},
		}
		_, err := testCtx.Kubeclient.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		defer func() {
			testCtx.Kubeclient.CoreV1().Namespaces().Delete(context.TODO(), ns.Name, metav1.DeleteOptions{})
		}()

		// Create first podgroup with default queue
		podgroup1 := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-podgroup-1",
				Namespace: ns.Name,
			},
			Spec: schedulingv1beta1.PodGroupSpec{
				Queue:        schedulingv1beta1.DefaultQueue,
				MinMember:    1,
				MinResources: nil,
			},
		}

		createdPodGroup1, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(ns.Name).Create(context.TODO(), podgroup1, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(createdPodGroup1.Spec.Queue).To(gomega.Equal("shared-queue"))

		// Create second podgroup with custom queue
		podgroup2 := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-podgroup-2",
				Namespace: ns.Name,
			},
			Spec: schedulingv1beta1.PodGroupSpec{
				Queue:        "custom-queue-2",
				MinMember:    2,
				MinResources: nil,
			},
		}

		createdPodGroup2, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(ns.Name).Create(context.TODO(), podgroup2, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		// Should preserve custom queue
		gomega.Expect(createdPodGroup2.Spec.Queue).To(gomega.Equal("custom-queue-2"))
	})

	ginkgo.It("Should handle empty queue annotation value", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		// Create namespace with empty queue annotation
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testCtx.Namespace + "-empty-queue",
				Annotations: map[string]string{
					schedulingv1beta1.QueueNameAnnotationKey: "",
				},
			},
		}
		_, err := testCtx.Kubeclient.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		defer func() {
			testCtx.Kubeclient.CoreV1().Namespaces().Delete(context.TODO(), ns.Name, metav1.DeleteOptions{})
		}()

		podgroup := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-podgroup-empty-annotation",
				Namespace: ns.Name,
			},
			Spec: schedulingv1beta1.PodGroupSpec{
				Queue:        schedulingv1beta1.DefaultQueue,
				MinMember:    1,
				MinResources: nil,
			},
		}

		createdPodGroup, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(ns.Name).Create(context.TODO(), podgroup, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		// Should set queue to empty string from annotation
		gomega.Expect(createdPodGroup.Spec.Queue).To(gomega.Equal(""))
	})

	ginkgo.It("Should preserve other podgroup fields when mutating queue", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		// Create namespace with queue annotation
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testCtx.Namespace + "-preserve-fields",
				Annotations: map[string]string{
					schedulingv1beta1.QueueNameAnnotationKey: "annotated-queue",
				},
			},
		}
		_, err := testCtx.Kubeclient.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		defer func() {
			testCtx.Kubeclient.CoreV1().Namespaces().Delete(context.TODO(), ns.Name, metav1.DeleteOptions{})
		}()

		podgroup := &schedulingv1beta1.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-podgroup-preserve",
				Namespace: ns.Name,
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
				// Add some resource requirements
				MinResources: &corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			},
		}

		createdPodGroup, err := testCtx.Vcclient.SchedulingV1beta1().PodGroups(ns.Name).Create(context.TODO(), podgroup, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// Should mutate queue
		gomega.Expect(createdPodGroup.Spec.Queue).To(gomega.Equal("annotated-queue"))

		// Should preserve all other fields
		gomega.Expect(createdPodGroup.Spec.MinMember).To(gomega.Equal(int32(5)))
		gomega.Expect(createdPodGroup.Labels["test-label"]).To(gomega.Equal("test-value"))
		gomega.Expect(createdPodGroup.Annotations["test-annotation"]).To(gomega.Equal("test-value"))
		gomega.Expect(createdPodGroup.Spec.MinResources).NotTo(gomega.BeNil())
		gomega.Expect((*createdPodGroup.Spec.MinResources)[corev1.ResourceCPU]).To(gomega.Equal(resource.MustParse("2")))
		gomega.Expect((*createdPodGroup.Spec.MinResources)[corev1.ResourceMemory]).To(gomega.Equal(resource.MustParse("4Gi")))
	})
})