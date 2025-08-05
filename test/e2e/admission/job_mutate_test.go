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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"volcano.sh/apis/pkg/apis/batch/v1alpha1"
	"volcano.sh/volcano/test/e2e/util"
)

const (
	defaultQueue     = "default"
	defaultMaxRetry  = int32(3)
	defaultTaskSpec0 = "default0"
	defaultTaskSpec1 = "default1"
)

var _ = ginkgo.Describe("Job Mutating Webhook E2E Test", func() {

	ginkgo.It("Should add default queue when not specified", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		job := &v1alpha1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default-queue-job",
				Namespace: testCtx.Namespace,
			},
			Spec: v1alpha1.JobSpec{
				MinAvailable: 1,
				// Queue not specified
				Tasks: []v1alpha1.TaskSpec{
					{
						Name:     "task-1",
						Replicas: 1,
						Template: createPodTemplateForMutation(),
					},
				},
			},
		}

		createdJob, err := testCtx.Vcclient.BatchV1alpha1().Jobs(testCtx.Namespace).Create(context.TODO(), job, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(createdJob.Spec.Queue).To(gomega.Equal(defaultQueue))
	})

	ginkgo.It("Should add default scheduler name when not specified", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		job := &v1alpha1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default-scheduler-job",
				Namespace: testCtx.Namespace,
			},
			Spec: v1alpha1.JobSpec{
				MinAvailable: 1,
				Queue:        "default",
				// SchedulerName not specified
				Tasks: []v1alpha1.TaskSpec{
					{
						Name:     "task-1",
						Replicas: 1,
						Template: createPodTemplateForMutation(),
					},
				},
			},
		}

		createdJob, err := testCtx.Vcclient.BatchV1alpha1().Jobs(testCtx.Namespace).Create(context.TODO(), job, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(createdJob.Spec.SchedulerName).NotTo(gomega.BeEmpty())
	})

	ginkgo.It("Should add default maxRetry when not specified", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		job := &v1alpha1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default-max-retry-job",
				Namespace: testCtx.Namespace,
			},
			Spec: v1alpha1.JobSpec{
				MinAvailable: 1,
				Queue:        "default",
				// MaxRetry not specified (defaults to 0)
				Tasks: []v1alpha1.TaskSpec{
					{
						Name:     "task-1",
						Replicas: 1,
						Template: createPodTemplateForMutation(),
					},
				},
			},
		}

		createdJob, err := testCtx.Vcclient.BatchV1alpha1().Jobs(testCtx.Namespace).Create(context.TODO(), job, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(createdJob.Spec.MaxRetry).To(gomega.Equal(defaultMaxRetry))
	})

	ginkgo.It("Should calculate and set default minAvailable when not specified", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		job := &v1alpha1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default-min-available-job",
				Namespace: testCtx.Namespace,
			},
			Spec: v1alpha1.JobSpec{
				// MinAvailable not specified (defaults to 0)
				Queue: "default",
				Tasks: []v1alpha1.TaskSpec{
					{
						Name:     "task-1",
						Replicas: 2,
						Template: createPodTemplateForMutation(),
					},
					{
						Name:     "task-2",
						Replicas: 3,
						Template: createPodTemplateForMutation(),
					},
				},
			},
		}

		createdJob, err := testCtx.Vcclient.BatchV1alpha1().Jobs(testCtx.Namespace).Create(context.TODO(), job, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		// Should calculate minAvailable as sum of all task replicas (2 + 3 = 5)
		gomega.Expect(createdJob.Spec.MinAvailable).To(gomega.Equal(int32(5)))
	})

	ginkgo.It("Should calculate minAvailable considering task-level minAvailable when specified", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		taskMinAvailable := int32(1)
		job := &v1alpha1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task-min-available-job",
				Namespace: testCtx.Namespace,
			},
			Spec: v1alpha1.JobSpec{
				// MinAvailable not specified (defaults to 0)
				Queue: "default",
				Tasks: []v1alpha1.TaskSpec{
					{
						Name:         "task-1",
						Replicas:     2,
						MinAvailable: &taskMinAvailable, // Specified as 1
						Template:     createPodTemplateForMutation(),
					},
					{
						Name:     "task-2",
						Replicas: 3,
						// MinAvailable not specified, should use replicas (3)
						Template: createPodTemplateForMutation(),
					},
				},
			},
		}

		createdJob, err := testCtx.Vcclient.BatchV1alpha1().Jobs(testCtx.Namespace).Create(context.TODO(), job, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		// Should calculate minAvailable as sum: 1 (task-1 minAvailable) + 3 (task-2 replicas) = 4
		gomega.Expect(createdJob.Spec.MinAvailable).To(gomega.Equal(int32(4)))
	})

	ginkgo.It("Should generate default task names when not specified", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		job := &v1alpha1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default-task-name-job",
				Namespace: testCtx.Namespace,
			},
			Spec: v1alpha1.JobSpec{
				MinAvailable: 1,
				Queue:        "default",
				Tasks: []v1alpha1.TaskSpec{
					{
						// Name not specified
						Replicas: 1,
						Template: createPodTemplateForMutation(),
					},
					{
						// Name not specified
						Replicas: 1,
						Template: createPodTemplateForMutation(),
					},
				},
			},
		}

		createdJob, err := testCtx.Vcclient.BatchV1alpha1().Jobs(testCtx.Namespace).Create(context.TODO(), job, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(createdJob.Spec.Tasks[0].Name).To(gomega.Equal(defaultTaskSpec0))
		gomega.Expect(createdJob.Spec.Tasks[1].Name).To(gomega.Equal(defaultTaskSpec1))
	})

	ginkgo.It("Should set DNS policy for hostNetwork pods", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		job := &v1alpha1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "host-network-dns-job",
				Namespace: testCtx.Namespace,
			},
			Spec: v1alpha1.JobSpec{
				MinAvailable: 1,
				Queue:        "default",
				Tasks: []v1alpha1.TaskSpec{
					{
						Name:     "host-network-task",
						Replicas: 1,
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{"name": "test"},
							},
							Spec: corev1.PodSpec{
								HostNetwork: true,
								// DNSPolicy not specified
								Containers: []corev1.Container{
									{
										Name:  "fake-name",
										Image: "busybox:1.24",
									},
								},
							},
						},
					},
				},
			},
		}

		createdJob, err := testCtx.Vcclient.BatchV1alpha1().Jobs(testCtx.Namespace).Create(context.TODO(), job, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(createdJob.Spec.Tasks[0].Template.Spec.DNSPolicy).To(gomega.Equal(corev1.DNSClusterFirstWithHostNet))
	})

	ginkgo.It("Should set default task-level minAvailable when not specified", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		job := &v1alpha1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default-task-min-available-job",
				Namespace: testCtx.Namespace,
			},
			Spec: v1alpha1.JobSpec{
				MinAvailable: 1,
				Queue:        "default",
				Tasks: []v1alpha1.TaskSpec{
					{
						Name:     "task-1",
						Replicas: 3,
						// MinAvailable not specified
						Template: createPodTemplateForMutation(),
					},
				},
			},
		}

		createdJob, err := testCtx.Vcclient.BatchV1alpha1().Jobs(testCtx.Namespace).Create(context.TODO(), job, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(*createdJob.Spec.Tasks[0].MinAvailable).To(gomega.Equal(int32(3))) // Should equal replicas
	})

	ginkgo.It("Should set default task-level maxRetry when not specified", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		job := &v1alpha1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default-task-max-retry-job",
				Namespace: testCtx.Namespace,
			},
			Spec: v1alpha1.JobSpec{
				MinAvailable: 1,
				Queue:        "default",
				Tasks: []v1alpha1.TaskSpec{
					{
						Name:     "task-1",
						Replicas: 1,
						// MaxRetry not specified (defaults to 0)
						Template: createPodTemplateForMutation(),
					},
				},
			},
		}

		createdJob, err := testCtx.Vcclient.BatchV1alpha1().Jobs(testCtx.Namespace).Create(context.TODO(), job, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(createdJob.Spec.Tasks[0].MaxRetry).To(gomega.Equal(defaultMaxRetry))
	})

	ginkgo.It("Should add svc plugin for tensorflow plugin", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		job := &v1alpha1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tensorflow-plugin-job",
				Namespace: testCtx.Namespace,
			},
			Spec: v1alpha1.JobSpec{
				MinAvailable: 1,
				Queue:        "default",
				Plugins: map[string][]string{
					"tensorflow": {},
					// svc plugin not specified, should be added automatically
				},
				Tasks: []v1alpha1.TaskSpec{
					{
						Name:     "task-1",
						Replicas: 1,
						Template: createPodTemplateForMutation(),
					},
				},
			},
		}

		createdJob, err := testCtx.Vcclient.BatchV1alpha1().Jobs(testCtx.Namespace).Create(context.TODO(), job, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(createdJob.Spec.Plugins).To(gomega.HaveKey("svc"))
		gomega.Expect(createdJob.Spec.Plugins).To(gomega.HaveKey("tensorflow"))
	})

	ginkgo.It("Should add svc plugin for mpi plugin", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		job := &v1alpha1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mpi-plugin-job",
				Namespace: testCtx.Namespace,
			},
			Spec: v1alpha1.JobSpec{
				MinAvailable: 1,
				Queue:        "default",
				Plugins: map[string][]string{
					"mpi": {},
					// svc plugin not specified, should be added automatically
				},
				Tasks: []v1alpha1.TaskSpec{
					{
						Name:     "task-1",
						Replicas: 1,
						Template: createPodTemplateForMutation(),
					},
				},
			},
		}

		createdJob, err := testCtx.Vcclient.BatchV1alpha1().Jobs(testCtx.Namespace).Create(context.TODO(), job, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(createdJob.Spec.Plugins).To(gomega.HaveKey("svc"))
		gomega.Expect(createdJob.Spec.Plugins).To(gomega.HaveKey("mpi"))
	})

	ginkgo.It("Should add svc plugin for pytorch plugin", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		job := &v1alpha1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pytorch-plugin-job",
				Namespace: testCtx.Namespace,
			},
			Spec: v1alpha1.JobSpec{
				MinAvailable: 1,
				Queue:        "default",
				Plugins: map[string][]string{
					"pytorch": {},
					// svc plugin not specified, should be added automatically
				},
				Tasks: []v1alpha1.TaskSpec{
					{
						Name:     "task-1",
						Replicas: 1,
						Template: createPodTemplateForMutation(),
					},
				},
			},
		}

		createdJob, err := testCtx.Vcclient.BatchV1alpha1().Jobs(testCtx.Namespace).Create(context.TODO(), job, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(createdJob.Spec.Plugins).To(gomega.HaveKey("svc"))
		gomega.Expect(createdJob.Spec.Plugins).To(gomega.HaveKey("pytorch"))
	})

	ginkgo.It("Should add ssh plugin for mpi plugin", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		job := &v1alpha1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mpi-ssh-plugin-job",
				Namespace: testCtx.Namespace,
			},
			Spec: v1alpha1.JobSpec{
				MinAvailable: 1,
				Queue:        "default",
				Plugins: map[string][]string{
					"mpi": {},
					// ssh plugin not specified, should be added automatically
				},
				Tasks: []v1alpha1.TaskSpec{
					{
						Name:     "task-1",
						Replicas: 1,
						Template: createPodTemplateForMutation(),
					},
				},
			},
		}

		createdJob, err := testCtx.Vcclient.BatchV1alpha1().Jobs(testCtx.Namespace).Create(context.TODO(), job, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(createdJob.Spec.Plugins).To(gomega.HaveKey("ssh"))
		gomega.Expect(createdJob.Spec.Plugins).To(gomega.HaveKey("mpi"))
		gomega.Expect(createdJob.Spec.Plugins).To(gomega.HaveKey("svc")) // Should also have svc plugin
	})

	ginkgo.It("Should not override existing plugins", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		job := &v1alpha1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "existing-plugins-job",
				Namespace: testCtx.Namespace,
			},
			Spec: v1alpha1.JobSpec{
				MinAvailable: 1,
				Queue:        "default",
				Plugins: map[string][]string{
					"tensorflow": {},
					"svc":        {"--enable-networking"}, // Already specified with arguments
				},
				Tasks: []v1alpha1.TaskSpec{
					{
						Name:     "task-1",
						Replicas: 1,
						Template: createPodTemplateForMutation(),
					},
				},
			},
		}

		createdJob, err := testCtx.Vcclient.BatchV1alpha1().Jobs(testCtx.Namespace).Create(context.TODO(), job, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(createdJob.Spec.Plugins).To(gomega.HaveKey("svc"))
		gomega.Expect(createdJob.Spec.Plugins).To(gomega.HaveKey("tensorflow"))
		// Should preserve existing svc plugin arguments
		gomega.Expect(createdJob.Spec.Plugins["svc"]).To(gomega.ContainElement("--enable-networking"))
	})

	ginkgo.It("Should not add plugins when no distributed framework plugins are used", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		job := &v1alpha1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "no-framework-plugins-job",
				Namespace: testCtx.Namespace,
			},
			Spec: v1alpha1.JobSpec{
				MinAvailable: 1,
				Queue:        "default",
				Plugins: map[string][]string{
					"gang": {}, // Only gang plugin, no distributed framework
				},
				Tasks: []v1alpha1.TaskSpec{
					{
						Name:     "task-1",
						Replicas: 1,
						Template: createPodTemplateForMutation(),
					},
				},
			},
		}

		createdJob, err := testCtx.Vcclient.BatchV1alpha1().Jobs(testCtx.Namespace).Create(context.TODO(), job, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(createdJob.Spec.Plugins).To(gomega.HaveKey("gang"))
		// Should not automatically add svc or ssh plugins
		gomega.Expect(createdJob.Spec.Plugins).NotTo(gomega.HaveKey("svc"))
		gomega.Expect(createdJob.Spec.Plugins).NotTo(gomega.HaveKey("ssh"))
	})

	ginkgo.It("Should preserve existing field values when specified", func() {
		testCtx := util.InitTestContext(util.Options{})
		defer util.CleanupTestContext(testCtx)

		customMinAvailable := int32(2)
		job := &v1alpha1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "preserve-values-job",
				Namespace: testCtx.Namespace,
			},
			Spec: v1alpha1.JobSpec{
				MinAvailable:  2,              // Explicitly specified
				Queue:         "custom-queue", // Explicitly specified
				SchedulerName: "custom-sched", // Explicitly specified
				MaxRetry:      10,             // Explicitly specified
				Tasks: []v1alpha1.TaskSpec{
					{
						Name:         "custom-task", // Explicitly specified
						Replicas:     3,
						MinAvailable: &customMinAvailable, // Explicitly specified
						MaxRetry:     5,                   // Explicitly specified
						Template:     createPodTemplateForMutation(),
					},
				},
			},
		}

		createdJob, err := testCtx.Vcclient.BatchV1alpha1().Jobs(testCtx.Namespace).Create(context.TODO(), job, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// Should preserve all explicitly specified values
		gomega.Expect(createdJob.Spec.MinAvailable).To(gomega.Equal(int32(2)))
		gomega.Expect(createdJob.Spec.Queue).To(gomega.Equal("custom-queue"))
		gomega.Expect(createdJob.Spec.SchedulerName).To(gomega.Equal("custom-sched"))
		gomega.Expect(createdJob.Spec.MaxRetry).To(gomega.Equal(int32(10)))
		gomega.Expect(createdJob.Spec.Tasks[0].Name).To(gomega.Equal("custom-task"))
		gomega.Expect(*createdJob.Spec.Tasks[0].MinAvailable).To(gomega.Equal(int32(2)))
		gomega.Expect(createdJob.Spec.Tasks[0].MaxRetry).To(gomega.Equal(int32(5)))
	})
})

// Helper function to create a basic pod template for job mutation tests
func createPodTemplateForMutation() corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"name": "test"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "fake-name",
					Image: "busybox:1.24",
				},
			},
		},
	}
}
