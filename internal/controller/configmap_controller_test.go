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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kvstore "github.com/krkn-chaos/krkn-operator/pkg/configstore"
)

var _ = Describe("ConfigMap Controller", func() {
	const (
		ConfigMapName      = "test-config"
		ConfigMapNamespace = "default"
		timeout            = time.Second * 10
		interval           = time.Millisecond * 250
	)

	var (
		configMapReconciler *ConfigMapReconciler
		store               *kvstore.Store
	)

	BeforeEach(func() {
		// Initialize configstore
		store = kvstore.Get()

		// Create reconciler
		configMapReconciler = &ConfigMapReconciler{
			Client:             k8sClient,
			Scheme:             k8sClient.Scheme(),
			ConfigMapName:      ConfigMapName,
			ConfigMapNamespace: ConfigMapNamespace,
		}
	})

	AfterEach(func() {
		// Clean up ConfigMap if it exists
		ctx := context.Background()
		configMap := &corev1.ConfigMap{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      ConfigMapName,
			Namespace: ConfigMapNamespace,
		}, configMap)
		if err == nil {
			Expect(k8sClient.Delete(ctx, configMap)).To(Succeed())
		}

		// Clean up configstore
		snapshot := store.Snapshot()
		for key := range snapshot {
			store.Delete(key)
		}
	})

	Context("When reconciling a ConfigMap", func() {
		It("Should sync ConfigMap data to configstore", func() {
			ctx := context.Background()

			// Create a ConfigMap
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ConfigMapName,
					Namespace: ConfigMapNamespace,
				},
				Data: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			}
			Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

			// Reconcile
			result, err := configMapReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      ConfigMapName,
					Namespace: ConfigMapNamespace,
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			// Verify values are in configstore
			Eventually(func() bool {
				val1, ok1 := store.GetValue("key1")
				val2, ok2 := store.GetValue("key2")
				val3, ok3 := store.GetValue("key3")
				return ok1 && ok2 && ok3 &&
					val1 == "value1" &&
					val2 == "value2" &&
					val3 == "value3"
			}, timeout, interval).Should(BeTrue())
		})

		It("Should update configstore when ConfigMap is updated", func() {
			ctx := context.Background()

			// Create initial ConfigMap
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ConfigMapName,
					Namespace: ConfigMapNamespace,
				},
				Data: map[string]string{
					"key1": "initial-value",
				},
			}
			Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

			// First reconcile
			_, err := configMapReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      ConfigMapName,
					Namespace: ConfigMapNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify initial value
			Eventually(func() bool {
				val, ok := store.GetValue("key1")
				return ok && val == "initial-value"
			}, timeout, interval).Should(BeTrue())

			// Update ConfigMap
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      ConfigMapName,
				Namespace: ConfigMapNamespace,
			}, configMap)
			Expect(err).NotTo(HaveOccurred())

			configMap.Data["key1"] = "updated-value"
			Expect(k8sClient.Update(ctx, configMap)).To(Succeed())

			// Second reconcile
			_, err = configMapReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      ConfigMapName,
					Namespace: ConfigMapNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify updated value
			Eventually(func() bool {
				val, ok := store.GetValue("key1")
				return ok && val == "updated-value"
			}, timeout, interval).Should(BeTrue())
		})

		It("Should handle non-existent ConfigMap gracefully", func() {
			ctx := context.Background()

			// Reconcile non-existent ConfigMap
			result, err := configMapReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent",
					Namespace: ConfigMapNamespace,
				},
			})

			// Should not return error
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})

		It("Should handle empty ConfigMap", func() {
			ctx := context.Background()

			// Create empty ConfigMap
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ConfigMapName,
					Namespace: ConfigMapNamespace,
				},
				Data: map[string]string{},
			}
			Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

			// Reconcile
			result, err := configMapReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      ConfigMapName,
					Namespace: ConfigMapNamespace,
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})

		It("Should handle ConfigMap with empty string values", func() {
			ctx := context.Background()

			// Create ConfigMap with empty value
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ConfigMapName,
					Namespace: ConfigMapNamespace,
				},
				Data: map[string]string{
					"empty-key":  "",
					"normal-key": "normal-value",
				},
			}
			Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

			// Reconcile
			result, err := configMapReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      ConfigMapName,
					Namespace: ConfigMapNamespace,
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			// Verify values
			Eventually(func() bool {
				emptyVal, ok1 := store.GetValue("empty-key")
				normalVal, ok2 := store.GetValue("normal-key")
				return ok1 && ok2 &&
					emptyVal == "" &&
					normalVal == "normal-value"
			}, timeout, interval).Should(BeTrue())
		})

		It("Should add new keys to configstore when ConfigMap is updated with new keys", func() {
			ctx := context.Background()

			// Create initial ConfigMap
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ConfigMapName,
					Namespace: ConfigMapNamespace,
				},
				Data: map[string]string{
					"key1": "value1",
				},
			}
			Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

			// First reconcile
			_, err := configMapReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      ConfigMapName,
					Namespace: ConfigMapNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Update ConfigMap with new key
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      ConfigMapName,
				Namespace: ConfigMapNamespace,
			}, configMap)
			Expect(err).NotTo(HaveOccurred())

			configMap.Data["key2"] = "value2"
			Expect(k8sClient.Update(ctx, configMap)).To(Succeed())

			// Second reconcile
			_, err = configMapReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      ConfigMapName,
					Namespace: ConfigMapNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify both keys exist
			Eventually(func() bool {
				val1, ok1 := store.GetValue("key1")
				val2, ok2 := store.GetValue("key2")
				return ok1 && ok2 &&
					val1 == "value1" &&
					val2 == "value2"
			}, timeout, interval).Should(BeTrue())
		})
	})
})
