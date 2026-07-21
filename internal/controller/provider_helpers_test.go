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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	krknv1alpha1 "github.com/krkn-chaos/krkn-operator/api/v1alpha1"
)

var _ = Describe("Provider Helpers", func() {
	var (
		ctx       context.Context
		k8sClient client.Client
		scheme    *runtime.Scheme
		logger    = log.Log.WithName("provider-helpers-test")
	)

	BeforeEach(func() {
		ctx = context.Background()
		ctx = log.IntoContext(ctx, logger)
		scheme = runtime.NewScheme()
		Expect(krknv1alpha1.AddToScheme(scheme)).To(Succeed())
		k8sClient = fake.NewClientBuilder().WithScheme(scheme).Build()
	})

	Describe("checkProviderActive", func() {
		It("should return false when provider does not exist", func() {
			_, shouldSkip, result, err := checkProviderActive(ctx, k8sClient, logger, "test-operator", "test-namespace")
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldSkip).To(BeTrue())
			Expect(result.Requeue).To(BeFalse())
		})

		It("should return false when provider exists and is active", func() {
			// Create active provider
			provider := &krknv1alpha1.KrknOperatorTargetProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-operator",
					Namespace: "test-namespace",
				},
				Spec: krknv1alpha1.KrknOperatorTargetProviderSpec{
					OperatorName: "test-operator",
					Active:       true,
				},
			}
			Expect(k8sClient.Create(ctx, provider)).To(Succeed())

			_, shouldSkip, result, err := checkProviderActive(ctx, k8sClient, logger, "test-operator", "test-namespace")
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldSkip).To(BeFalse())
			Expect(result.Requeue).To(BeFalse())
		})

		It("should return true when provider exists but is inactive", func() {
			// Create inactive provider
			provider := &krknv1alpha1.KrknOperatorTargetProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-operator",
					Namespace: "test-namespace",
				},
				Spec: krknv1alpha1.KrknOperatorTargetProviderSpec{
					OperatorName: "test-operator",
					Active:       false,
				},
			}
			Expect(k8sClient.Create(ctx, provider)).To(Succeed())

			_, shouldSkip, result, err := checkProviderActive(ctx, k8sClient, logger, "test-operator", "test-namespace")
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldSkip).To(BeTrue())
			Expect(result.Requeue).To(BeFalse())
		})
	})

	Describe("countActiveProviders", func() {
		It("should return 0 for empty provider list", func() {
			providerList := &krknv1alpha1.KrknOperatorTargetProviderList{}
			count := countActiveProviders(providerList)
			Expect(count).To(Equal(0))
		})

		It("should count only active providers", func() {
			providerList := &krknv1alpha1.KrknOperatorTargetProviderList{
				Items: []krknv1alpha1.KrknOperatorTargetProvider{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "provider-1"},
						Spec:       krknv1alpha1.KrknOperatorTargetProviderSpec{OperatorName: "provider-1", Active: true},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "provider-2"},
						Spec:       krknv1alpha1.KrknOperatorTargetProviderSpec{OperatorName: "provider-2", Active: false},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "provider-3"},
						Spec:       krknv1alpha1.KrknOperatorTargetProviderSpec{OperatorName: "provider-3", Active: true},
					},
				},
			}
			count := countActiveProviders(providerList)
			Expect(count).To(Equal(2))
		})

		It("should return 0 when all providers are inactive", func() {
			providerList := &krknv1alpha1.KrknOperatorTargetProviderList{
				Items: []krknv1alpha1.KrknOperatorTargetProvider{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "provider-1"},
						Spec:       krknv1alpha1.KrknOperatorTargetProviderSpec{OperatorName: "provider-1", Active: false},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "provider-2"},
						Spec:       krknv1alpha1.KrknOperatorTargetProviderSpec{OperatorName: "provider-2", Active: false},
					},
				},
			}
			count := countActiveProviders(providerList)
			Expect(count).To(Equal(0))
		})
	})

	Describe("shouldMarkAsCompleted", func() {
		It("should return true when active count matches contributor count", func() {
			Expect(shouldMarkAsCompleted(2, 2)).To(BeTrue())
		})

		It("should return false when contributor count is less than active count", func() {
			Expect(shouldMarkAsCompleted(3, 2)).To(BeFalse())
		})

		It("should return true when contributor count is greater than active count", func() {
			// This shouldn't happen in practice, but contributorCount >= activeProviderCount returns true
			Expect(shouldMarkAsCompleted(2, 3)).To(BeTrue())
		})

		It("should return false when active count is zero", func() {
			// Edge case: no active providers
			Expect(shouldMarkAsCompleted(0, 0)).To(BeFalse())
		})

		It("should return true for single active provider", func() {
			Expect(shouldMarkAsCompleted(1, 1)).To(BeTrue())
		})
	})
})
