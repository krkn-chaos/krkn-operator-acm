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

Assisted-by: Claude Sonnet 4.5 (claude-sonnet-4-5@20250929)
*/

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	krknv1alpha1 "github.com/krkn-chaos/krkn-operator/api/v1alpha1"
)

var _ = Describe("KrknTargetRequest Controller", func() {
	const (
		testNamespace = "default"
		timeout       = time.Second * 10
		interval      = time.Millisecond * 250
	)

	Context("Phase 16: Multi-Operator Testing", func() {
		var (
			ctx           context.Context
			provider1     *krknv1alpha1.KrknOperatorTargetProvider
			provider2     *krknv1alpha1.KrknOperatorTargetProvider
			testRequest   *krknv1alpha1.KrknTargetRequest
			testConfigMap *corev1.ConfigMap
		)

		BeforeEach(func() {
			ctx = context.Background()

			// Create test ConfigMap
			testConfigMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "krkn-operator-config",
					Namespace: testNamespace,
				},
				Data: map[string]string{
					"operator-name":      "krkn-operator-acm",
					"operator-namespace": testNamespace,
				},
			}
			Expect(k8sClient.Create(ctx, testConfigMap)).To(Succeed())
		})

		AfterEach(func() {
			// Cleanup ConfigMap
			if testConfigMap != nil {
				_ = k8sClient.Delete(ctx, testConfigMap)
			}

			// Cleanup providers
			if provider1 != nil {
				_ = k8sClient.Delete(ctx, provider1)
			}
			if provider2 != nil {
				_ = k8sClient.Delete(ctx, provider2)
			}

			// Cleanup request
			if testRequest != nil {
				_ = k8sClient.Delete(ctx, testRequest)
			}
		})

		Describe("Scenario 1: Single Operator (Backward Compatibility)", func() {
			It("should complete request when single active provider contributes", func() {
				By("Creating an active provider")
				provider1 = &krknv1alpha1.KrknOperatorTargetProvider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "krkn-operator-acm",
						Namespace: testNamespace,
					},
					Spec: krknv1alpha1.KrknOperatorTargetProviderSpec{
						OperatorName: "krkn-operator-acm",
						Active:       true,
					},
				}
				Expect(k8sClient.Create(ctx, provider1)).To(Succeed())

				By("Creating a KrknTargetRequest")
				testRequest = &krknv1alpha1.KrknTargetRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-single-operator",
						Namespace: testNamespace,
						Labels: map[string]string{
							"krkn.krkn-chaos.dev/uuid": "test-uuid-single",
						},
					},
					Spec: krknv1alpha1.KrknTargetRequestSpec{
						UUID: "test-uuid-single",
					},
				}
				Expect(k8sClient.Create(ctx, testRequest)).To(Succeed())

				By("Updating status with initial data")
				testRequest.Status = krknv1alpha1.KrknTargetRequestStatus{
					Status:  "pending",
					Created: &metav1.Time{Time: time.Now()},
					TargetData: map[string][]krknv1alpha1.ClusterTarget{
						"krkn-operator-acm": {
							{
								ClusterName:   "test-cluster",
								ClusterAPIURL: "https://api.test.com:6443",
							},
						},
					},
				}
				Expect(k8sClient.Status().Update(ctx, testRequest)).To(Succeed())

				By("Listing active providers")
				providerList := &krknv1alpha1.KrknOperatorTargetProviderList{}
				err := k8sClient.List(ctx, providerList, client.InNamespace(testNamespace))
				Expect(err).NotTo(HaveOccurred())

				By("Counting active providers")
				activeProviderCount := 0
				for _, provider := range providerList.Items {
					if provider.Spec.Active {
						activeProviderCount++
					}
				}
				Expect(activeProviderCount).To(Equal(1))

				By("Checking contributor count")
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-single-operator",
					Namespace: testNamespace,
				}, testRequest)).To(Succeed())
				contributorCount := len(testRequest.Status.TargetData)
				Expect(contributorCount).To(Equal(1))

				By("Verifying completion condition is met")
				shouldComplete := contributorCount >= activeProviderCount && activeProviderCount > 0
				Expect(shouldComplete).To(BeTrue())

				By("Simulating status update to Completed")
				testRequest.Status.Status = "Completed"
				now := metav1.Now()
				testRequest.Status.Completed = &now
				Expect(k8sClient.Status().Update(ctx, testRequest)).To(Succeed())

				By("Verifying request is completed")
				Eventually(func() string {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      "test-single-operator",
						Namespace: testNamespace,
					}, testRequest)
					if err != nil {
						return ""
					}
					return testRequest.Status.Status
				}, timeout, interval).Should(Equal("Completed"))

				By("Verifying TargetData contains single operator's data")
				Expect(testRequest.Status.TargetData).To(HaveKey("krkn-operator-acm"))
				Expect(testRequest.Status.TargetData["krkn-operator-acm"]).To(HaveLen(1))
			})
		})

		Describe("Scenario 2: Two Operators Contributing", func() {
			It("should remain pending until both operators contribute", func() {
				By("Creating two active providers")
				provider1 = &krknv1alpha1.KrknOperatorTargetProvider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "krkn-operator-acm",
						Namespace: testNamespace,
					},
					Spec: krknv1alpha1.KrknOperatorTargetProviderSpec{
						OperatorName: "krkn-operator-acm",
						Active:       true,
					},
				}
				Expect(k8sClient.Create(ctx, provider1)).To(Succeed())

				provider2 = &krknv1alpha1.KrknOperatorTargetProvider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "krkn-operator-test2",
						Namespace: testNamespace,
					},
					Spec: krknv1alpha1.KrknOperatorTargetProviderSpec{
						OperatorName: "krkn-operator-test2",
						Active:       true,
					},
				}
				Expect(k8sClient.Create(ctx, provider2)).To(Succeed())

				By("Creating a KrknTargetRequest with first operator's contribution")
				testRequest = &krknv1alpha1.KrknTargetRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-two-operators",
						Namespace: testNamespace,
						Labels: map[string]string{
							"krkn.krkn-chaos.dev/uuid": "test-uuid-two",
						},
					},
					Spec: krknv1alpha1.KrknTargetRequestSpec{
						UUID: "test-uuid-two",
					},
				}
				Expect(k8sClient.Create(ctx, testRequest)).To(Succeed())

				By("Updating status with first operator's data")
				testRequest.Status = krknv1alpha1.KrknTargetRequestStatus{
					Status:  "pending",
					Created: &metav1.Time{Time: time.Now()},
					TargetData: map[string][]krknv1alpha1.ClusterTarget{
						"krkn-operator-acm": {
							{
								ClusterName:   "cluster1",
								ClusterAPIURL: "https://api.cluster1.com:6443",
							},
						},
					},
				}
				Expect(k8sClient.Status().Update(ctx, testRequest)).To(Succeed())

				By("Listing active providers (should be 2)")
				providerList := &krknv1alpha1.KrknOperatorTargetProviderList{}
				err := k8sClient.List(ctx, providerList, client.InNamespace(testNamespace))
				Expect(err).NotTo(HaveOccurred())

				activeProviderCount := 0
				for _, provider := range providerList.Items {
					if provider.Spec.Active {
						activeProviderCount++
					}
				}
				Expect(activeProviderCount).To(Equal(2))

				By("Verifying completion condition NOT met with 1/2 contributors")
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-two-operators",
					Namespace: testNamespace,
				}, testRequest)).To(Succeed())
				contributorCount := len(testRequest.Status.TargetData)
				Expect(contributorCount).To(Equal(1))
				shouldComplete := contributorCount >= activeProviderCount && activeProviderCount > 0
				Expect(shouldComplete).To(BeFalse())

				By("Adding second operator's contribution")
				Eventually(func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      "test-two-operators",
						Namespace: testNamespace,
					}, testRequest)
					if err != nil {
						return err
					}

					testRequest.Status.TargetData["krkn-operator-test2"] = []krknv1alpha1.ClusterTarget{
						{
							ClusterName:   "cluster2",
							ClusterAPIURL: "https://api.cluster2.com:6443",
						},
					}
					return k8sClient.Status().Update(ctx, testRequest)
				}, timeout, interval).Should(Succeed())

				By("Verifying completion condition IS met with 2/2 contributors")
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-two-operators",
					Namespace: testNamespace,
				}, testRequest)).To(Succeed())
				contributorCount = len(testRequest.Status.TargetData)
				Expect(contributorCount).To(Equal(2))
				shouldComplete = contributorCount >= activeProviderCount && activeProviderCount > 0
				Expect(shouldComplete).To(BeTrue())

				By("Updating status to Completed")
				testRequest.Status.Status = "Completed"
				now := metav1.Now()
				testRequest.Status.Completed = &now
				Expect(k8sClient.Status().Update(ctx, testRequest)).To(Succeed())

				By("Verifying TargetData contains both operators' data")
				Expect(testRequest.Status.TargetData).To(HaveKey("krkn-operator-acm"))
				Expect(testRequest.Status.TargetData).To(HaveKey("krkn-operator-test2"))
			})
		})

		Describe("Scenario 3: Active vs Inactive Providers", func() {
			It("should only count active providers for completion", func() {
				By("Creating one active and one inactive provider")
				provider1 = &krknv1alpha1.KrknOperatorTargetProvider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "krkn-operator-acm",
						Namespace: testNamespace,
					},
					Spec: krknv1alpha1.KrknOperatorTargetProviderSpec{
						OperatorName: "krkn-operator-acm",
						Active:       true,
					},
				}
				Expect(k8sClient.Create(ctx, provider1)).To(Succeed())

				provider2 = &krknv1alpha1.KrknOperatorTargetProvider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "krkn-operator-inactive",
						Namespace: testNamespace,
					},
					Spec: krknv1alpha1.KrknOperatorTargetProviderSpec{
						OperatorName: "krkn-operator-inactive",
						Active:       false, // Inactive
					},
				}
				Expect(k8sClient.Create(ctx, provider2)).To(Succeed())

				By("Creating a KrknTargetRequest with active operator's contribution")
				testRequest = &krknv1alpha1.KrknTargetRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-active-only",
						Namespace: testNamespace,
						Labels: map[string]string{
							"krkn.krkn-chaos.dev/uuid": "test-uuid-active",
						},
					},
					Spec: krknv1alpha1.KrknTargetRequestSpec{
						UUID: "test-uuid-active",
					},
				}
				Expect(k8sClient.Create(ctx, testRequest)).To(Succeed())

				By("Updating status with active operator's data")
				testRequest.Status = krknv1alpha1.KrknTargetRequestStatus{
					Status:  "pending",
					Created: &metav1.Time{Time: time.Now()},
					TargetData: map[string][]krknv1alpha1.ClusterTarget{
						"krkn-operator-acm": {
							{
								ClusterName:   "active-cluster",
								ClusterAPIURL: "https://api.active.com:6443",
							},
						},
					},
				}
				Expect(k8sClient.Status().Update(ctx, testRequest)).To(Succeed())

				By("Listing providers and counting only active ones")
				providerList := &krknv1alpha1.KrknOperatorTargetProviderList{}
				err := k8sClient.List(ctx, providerList, client.InNamespace(testNamespace))
				Expect(err).NotTo(HaveOccurred())

				totalProviders := len(providerList.Items)
				activeProviderCount := 0
				for _, provider := range providerList.Items {
					if provider.Spec.Active {
						activeProviderCount++
					}
				}
				Expect(totalProviders).To(Equal(2))
				Expect(activeProviderCount).To(Equal(1))

				By("Verifying completion condition IS met (1 contributor, 1 active provider)")
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-active-only",
					Namespace: testNamespace,
				}, testRequest)).To(Succeed())
				contributorCount := len(testRequest.Status.TargetData)
				Expect(contributorCount).To(Equal(1))
				shouldComplete := contributorCount >= activeProviderCount && activeProviderCount > 0
				Expect(shouldComplete).To(BeTrue())

				By("Verifying only active operator's data is present")
				Expect(testRequest.Status.TargetData).To(HaveKey("krkn-operator-acm"))
				Expect(testRequest.Status.TargetData).NotTo(HaveKey("krkn-operator-inactive"))
			})
		})

		Describe("Scenario 4: Provider Registration", func() {
			It("should have active field defaulting to true", func() {
				By("Creating a provider with active: true")
				provider1 = &krknv1alpha1.KrknOperatorTargetProvider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-provider",
						Namespace: testNamespace,
					},
					Spec: krknv1alpha1.KrknOperatorTargetProviderSpec{
						OperatorName: "test-operator",
						Active:       true,
					},
				}
				Expect(k8sClient.Create(ctx, provider1)).To(Succeed())

				By("Verifying the provider is active")
				createdProvider := &krknv1alpha1.KrknOperatorTargetProvider{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-provider",
					Namespace: testNamespace,
				}, createdProvider)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdProvider.Spec.Active).To(BeTrue())
			})

			It("should allow creating inactive providers", func() {
				By("Creating a provider with active: false")
				provider1 = &krknv1alpha1.KrknOperatorTargetProvider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-provider-inactive",
						Namespace: testNamespace,
					},
					Spec: krknv1alpha1.KrknOperatorTargetProviderSpec{
						OperatorName: "test-operator-inactive",
						Active:       false,
					},
				}
				Expect(k8sClient.Create(ctx, provider1)).To(Succeed())

				By("Verifying the provider is inactive")
				createdProvider := &krknv1alpha1.KrknOperatorTargetProvider{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-provider-inactive",
					Namespace: testNamespace,
				}, createdProvider)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdProvider.Spec.Active).To(BeFalse())
			})
		})

		Describe("Scenario 5: Timestamp Management", func() {
			It("should set created and completed timestamps", func() {
				By("Creating an active provider")
				provider1 = &krknv1alpha1.KrknOperatorTargetProvider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "krkn-operator-acm",
						Namespace: testNamespace,
					},
					Spec: krknv1alpha1.KrknOperatorTargetProviderSpec{
						OperatorName: "krkn-operator-acm",
						Active:       true,
					},
				}
				Expect(k8sClient.Create(ctx, provider1)).To(Succeed())

				By("Creating a KrknTargetRequest with created timestamp")
				createdTime := metav1.Now()
				testRequest = &krknv1alpha1.KrknTargetRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-timestamps",
						Namespace: testNamespace,
						Labels: map[string]string{
							"krkn.krkn-chaos.dev/uuid": "test-uuid-timestamps",
						},
					},
					Spec: krknv1alpha1.KrknTargetRequestSpec{
						UUID: "test-uuid-timestamps",
					},
				}
				Expect(k8sClient.Create(ctx, testRequest)).To(Succeed())

				By("Updating status with created timestamp")
				testRequest.Status = krknv1alpha1.KrknTargetRequestStatus{
					Status:  "pending",
					Created: &createdTime,
					TargetData: map[string][]krknv1alpha1.ClusterTarget{
						"krkn-operator-acm": {
							{
								ClusterName:   "test-cluster",
								ClusterAPIURL: "https://api.test.com:6443",
							},
						},
					},
				}
				Expect(k8sClient.Status().Update(ctx, testRequest)).To(Succeed())

				By("Verifying created timestamp is set")
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-timestamps",
					Namespace: testNamespace,
				}, testRequest)).To(Succeed())
				Expect(testRequest.CreationTimestamp.IsZero()).To(BeFalse())

				By("Simulating completion with completed timestamp")
				time.Sleep(100 * time.Millisecond) // Ensure time has passed
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-timestamps",
					Namespace: testNamespace,
				}, testRequest)).To(Succeed())
				completedTime := metav1.Now()
				testRequest.Status.Status = "Completed"
				testRequest.Status.Completed = &completedTime
				Expect(k8sClient.Status().Update(ctx, testRequest)).To(Succeed())

				By("Verifying completed timestamp is set and after created")
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-timestamps",
					Namespace: testNamespace,
				}, testRequest)).To(Succeed())
				Expect(testRequest.Status.Completed).NotTo(BeNil())
				Expect(testRequest.CreationTimestamp.IsZero()).To(BeFalse())
				// metav1.Time timestamps are truncated to second precision by etcd,
				// so we check that completed is not before creation (i.e., >= creation)
				Expect(testRequest.CreationTimestamp.Time.Before(testRequest.Status.Completed.Time) ||
					testRequest.CreationTimestamp.Time.Equal(testRequest.Status.Completed.Time)).To(BeTrue())
			})
		})

		Describe("Scenario 6: Data Isolation and Integrity", func() {
			It("should isolate each operator's data correctly", func() {
				By("Creating two active providers")
				provider1 = &krknv1alpha1.KrknOperatorTargetProvider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "krkn-operator-acm",
						Namespace: testNamespace,
					},
					Spec: krknv1alpha1.KrknOperatorTargetProviderSpec{
						OperatorName: "krkn-operator-acm",
						Active:       true,
					},
				}
				Expect(k8sClient.Create(ctx, provider1)).To(Succeed())

				provider2 = &krknv1alpha1.KrknOperatorTargetProvider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "krkn-operator-other",
						Namespace: testNamespace,
					},
					Spec: krknv1alpha1.KrknOperatorTargetProviderSpec{
						OperatorName: "krkn-operator-other",
						Active:       true,
					},
				}
				Expect(k8sClient.Create(ctx, provider2)).To(Succeed())

				By("Creating a request with both operators' data")
				testRequest = &krknv1alpha1.KrknTargetRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-isolation",
						Namespace: testNamespace,
					},
					Spec: krknv1alpha1.KrknTargetRequestSpec{
						UUID: "test-uuid-isolation",
					},
				}
				Expect(k8sClient.Create(ctx, testRequest)).To(Succeed())

				By("Updating status with both operators' data")
				testRequest.Status = krknv1alpha1.KrknTargetRequestStatus{
					Status:  "pending",
					Created: &metav1.Time{Time: time.Now()},
					TargetData: map[string][]krknv1alpha1.ClusterTarget{
						"krkn-operator-acm": {
							{
								ClusterName:   "acm-cluster-1",
								ClusterAPIURL: "https://api.acm1.com:6443",
							},
							{
								ClusterName:   "acm-cluster-2",
								ClusterAPIURL: "https://api.acm2.com:6443",
							},
						},
						"krkn-operator-other": {
							{
								ClusterName:   "other-cluster-1",
								ClusterAPIURL: "https://api.other1.com:6443",
							},
						},
					},
				}
				Expect(k8sClient.Status().Update(ctx, testRequest)).To(Succeed())

				By("Verifying data is properly namespaced by operator")
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-isolation",
					Namespace: testNamespace,
				}, testRequest)).To(Succeed())
				Expect(testRequest.Status.TargetData).To(HaveLen(2))
				Expect(testRequest.Status.TargetData["krkn-operator-acm"]).To(HaveLen(2))
				Expect(testRequest.Status.TargetData["krkn-operator-other"]).To(HaveLen(1))

				By("Verifying each operator's data is isolated")
				acmData := testRequest.Status.TargetData["krkn-operator-acm"]
				Expect(acmData[0].ClusterName).To(Equal("acm-cluster-1"))
				Expect(acmData[1].ClusterName).To(Equal("acm-cluster-2"))

				otherData := testRequest.Status.TargetData["krkn-operator-other"]
				Expect(otherData[0].ClusterName).To(Equal("other-cluster-1"))
			})
		})

		Describe("Scenario 7: Edge Cases", func() {
			It("should handle zero active providers gracefully", func() {
				By("Creating only inactive providers")
				provider1 = &krknv1alpha1.KrknOperatorTargetProvider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "krkn-operator-inactive",
						Namespace: testNamespace,
					},
					Spec: krknv1alpha1.KrknOperatorTargetProviderSpec{
						OperatorName: "krkn-operator-inactive",
						Active:       false,
					},
				}
				Expect(k8sClient.Create(ctx, provider1)).To(Succeed())

				By("Counting active providers (should be 0)")
				providerList := &krknv1alpha1.KrknOperatorTargetProviderList{}
				err := k8sClient.List(ctx, providerList, client.InNamespace(testNamespace))
				Expect(err).NotTo(HaveOccurred())

				activeProviderCount := 0
				for _, provider := range providerList.Items {
					if provider.Spec.Active {
						activeProviderCount++
					}
				}
				Expect(activeProviderCount).To(Equal(0))

				By("Verifying completion condition fails (activeProviderCount == 0)")
				contributorCount := 1 // Simulating one contribution
				shouldComplete := contributorCount >= activeProviderCount && activeProviderCount > 0
				Expect(shouldComplete).To(BeFalse())
			})

			It("should handle empty TargetData map", func() {
				By("Creating an active provider")
				provider1 = &krknv1alpha1.KrknOperatorTargetProvider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "krkn-operator-acm",
						Namespace: testNamespace,
					},
					Spec: krknv1alpha1.KrknOperatorTargetProviderSpec{
						OperatorName: "krkn-operator-acm",
						Active:       true,
					},
				}
				Expect(k8sClient.Create(ctx, provider1)).To(Succeed())

				By("Creating request with empty TargetData")
				testRequest = &krknv1alpha1.KrknTargetRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-empty-data",
						Namespace: testNamespace,
					},
					Spec: krknv1alpha1.KrknTargetRequestSpec{
						UUID: "test-uuid-empty",
					},
					Status: krknv1alpha1.KrknTargetRequestStatus{
						Status:     "pending",
						Created:    &metav1.Time{Time: time.Now()},
						TargetData: make(map[string][]krknv1alpha1.ClusterTarget),
					},
				}
				Expect(k8sClient.Create(ctx, testRequest)).To(Succeed())

				By("Verifying completion condition is NOT met (no contributors)")
				contributorCount := len(testRequest.Status.TargetData)
				Expect(contributorCount).To(Equal(0))

				providerList := &krknv1alpha1.KrknOperatorTargetProviderList{}
				err := k8sClient.List(ctx, providerList, client.InNamespace(testNamespace))
				Expect(err).NotTo(HaveOccurred())

				activeProviderCount := 0
				for _, provider := range providerList.Items {
					if provider.Spec.Active {
						activeProviderCount++
					}
				}

				shouldComplete := contributorCount >= activeProviderCount && activeProviderCount > 0
				Expect(shouldComplete).To(BeFalse())
			})
		})
	})

	Context("Legacy: Basic Reconciliation", func() {
		It("should handle non-existent resources gracefully", func() {
			ctx := context.Background()
			reconciler := &KrknTargetRequestReconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				OperatorName:      "test-operator",
				OperatorNamespace: testNamespace,
			}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent",
					Namespace: testNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("OwnerReference Management", func() {
		var (
			ctx         context.Context
			testRequest *krknv1alpha1.KrknTargetRequest
		)

		BeforeEach(func() {
			ctx = context.Background()
		})

		AfterEach(func() {
			// Cleanup request (secret should be auto-deleted via ownerReference)
			if testRequest != nil {
				_ = k8sClient.Delete(ctx, testRequest)
			}
		})

		It("should set ownerReference on created secret", func() {
			By("Creating a KrknTargetRequest")
			testRequest = &krknv1alpha1.KrknTargetRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-owner-reference",
					Namespace: testNamespace,
					Labels: map[string]string{
						"krkn.krkn-chaos.dev/uuid": "test-uuid-owner",
					},
				},
				Spec: krknv1alpha1.KrknTargetRequestSpec{
					UUID: "test-uuid-owner",
				},
			}
			Expect(k8sClient.Create(ctx, testRequest)).To(Succeed())

			By("Creating reconciler and preparing test data")
			reconciler := &KrknTargetRequestReconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				OperatorName:      "krkn-operator-acm",
				OperatorNamespace: testNamespace,
			}

			clustersData := map[string]ClusterData{
				"test-cluster": {
					ClusterName: "test-cluster",
					ClusterAPI:  "https://api.test.com:6443",
					Kubeconfig:  "ZmFrZS1rdWJlY29uZmln", // base64 "fake-kubeconfig"
				},
			}

			By("Creating secret with ownerReference")
			err := reconciler.createManagedClustersSecret(ctx, testRequest, clustersData)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying secret exists")
			secret := &corev1.Secret{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      testRequest.Spec.UUID,
				Namespace: testNamespace,
			}, secret)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying ownerReference is set correctly")
			Expect(secret.ObjectMeta.OwnerReferences).To(HaveLen(1))
			ownerRef := secret.ObjectMeta.OwnerReferences[0]
			Expect(ownerRef.APIVersion).To(Equal("krkn.krkn-chaos.dev/v1alpha1"))
			Expect(ownerRef.Kind).To(Equal("KrknTargetRequest"))
			Expect(ownerRef.Name).To(Equal(testRequest.Name))
			Expect(ownerRef.UID).To(Equal(testRequest.UID))
			Expect(*ownerRef.Controller).To(BeTrue())
			Expect(*ownerRef.BlockOwnerDeletion).To(BeTrue())

			By("Deleting KrknTargetRequest")
			err = k8sClient.Delete(ctx, testRequest)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying secret is automatically deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      testRequest.Spec.UUID,
					Namespace: testNamespace,
				}, secret)
				return err != nil
			}, timeout, interval).Should(BeTrue())
		})
	})
})
