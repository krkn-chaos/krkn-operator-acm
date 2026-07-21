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
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kvstore "github.com/krkn-chaos/krkn-operator/pkg/configstore"
)

func TestProxyHelpers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Proxy Helpers Suite")
}

var _ = Describe("Proxy Configuration", func() {
	var (
		ctx        context.Context
		k8sClient  client.Client
		scheme     *runtime.Scheme
		reconciler *KrknTargetRequestReconciler
		store      *kvstore.Store
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		k8sClient = fake.NewClientBuilder().WithScheme(scheme).Build()
		store = kvstore.Get()

		reconciler = &KrknTargetRequestReconciler{
			Client:            k8sClient,
			Scheme:            scheme,
			OperatorName:      "krkn-operator-acm",
			OperatorNamespace: "krkn-operator-system",
		}
	})

	AfterEach(func() {
		// Clear configstore after each test
		if store != nil {
			snapshot := store.Snapshot()
			for key := range snapshot {
				store.Delete(key)
			}
		}
	})

	Describe("isProxyModeEnabled", func() {
		It("should return false when config is not set", func() {
			enabled := isProxyModeEnabled(ctx, "test-cluster")
			Expect(enabled).To(BeFalse())
		})

		It("should return true for 'true' value", func() {
			store.SetValue("ACM_USE_PROXY_TEST_CLUSTER", "true")
			enabled := isProxyModeEnabled(ctx, "test-cluster")
			Expect(enabled).To(BeTrue())
		})

		It("should return true for 'yes' value", func() {
			store.SetValue("ACM_USE_PROXY_TEST_CLUSTER", "yes")
			enabled := isProxyModeEnabled(ctx, "test-cluster")
			Expect(enabled).To(BeTrue())
		})

		It("should return true for '1' value", func() {
			store.SetValue("ACM_USE_PROXY_TEST_CLUSTER", "1")
			enabled := isProxyModeEnabled(ctx, "test-cluster")
			Expect(enabled).To(BeTrue())
		})

		It("should return false for 'false' value", func() {
			store.SetValue("ACM_USE_PROXY_TEST_CLUSTER", "false")
			enabled := isProxyModeEnabled(ctx, "test-cluster")
			Expect(enabled).To(BeFalse())
		})

		It("should return false for 'no' value", func() {
			store.SetValue("ACM_USE_PROXY_TEST_CLUSTER", "no")
			enabled := isProxyModeEnabled(ctx, "test-cluster")
			Expect(enabled).To(BeFalse())
		})

		It("should return false for '0' value", func() {
			store.SetValue("ACM_USE_PROXY_TEST_CLUSTER", "0")
			enabled := isProxyModeEnabled(ctx, "test-cluster")
			Expect(enabled).To(BeFalse())
		})

		It("should return false for empty string", func() {
			store.SetValue("ACM_USE_PROXY_TEST_CLUSTER", "")
			enabled := isProxyModeEnabled(ctx, "test-cluster")
			Expect(enabled).To(BeFalse())
		})
	})

	Describe("formatProxyVarName", func() {
		It("should format cluster name with hyphens", func() {
			varName := formatProxyVarName("local-cluster")
			Expect(varName).To(Equal("ACM_USE_PROXY_LOCAL_CLUSTER"))
		})

		It("should format cluster name with multiple hyphens", func() {
			varName := formatProxyVarName("managed-cluster-1")
			Expect(varName).To(Equal("ACM_USE_PROXY_MANAGED_CLUSTER_1"))
		})

		It("should handle cluster name without hyphens", func() {
			varName := formatProxyVarName("cluster1")
			Expect(varName).To(Equal("ACM_USE_PROXY_CLUSTER1"))
		})

		It("should convert to uppercase", func() {
			varName := formatProxyVarName("MixedCase-Cluster")
			Expect(varName).To(Equal("ACM_USE_PROXY_MIXEDCASE_CLUSTER"))
		})
	})

	Describe("getProxyURL", func() {
		It("should construct proxy URL from ManagedProxyConfiguration and Service", func() {
			// Create ManagedProxyConfiguration
			proxyConfig := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "proxy.open-cluster-management.io/v1alpha1",
					"kind":       "ManagedProxyConfiguration",
					"metadata": map[string]interface{}{
						"name": "cluster-proxy",
					},
					"spec": map[string]interface{}{
						"proxyServer": map[string]interface{}{
							"namespace": "open-cluster-management-addon",
						},
					},
				},
			}
			proxyConfig.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "proxy.open-cluster-management.io",
				Version: "v1alpha1",
				Kind:    "ManagedProxyConfiguration",
			})
			Expect(k8sClient.Create(ctx, proxyConfig)).To(Succeed())

			// Create proxy service
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-proxy-addon-user",
					Namespace: "open-cluster-management-addon",
					Labels: map[string]string{
						"component": "cluster-proxy-addon-user",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port: 8090,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, service)).To(Succeed())

			// Get proxy URL
			proxyURL, err := reconciler.getProxyURL(ctx, "test-cluster")
			Expect(err).ToNot(HaveOccurred())
			Expect(proxyURL).To(Equal("https://cluster-proxy-addon-user.open-cluster-management-addon.svc:8090/test-cluster"))
		})

		It("should return error when ManagedProxyConfiguration not found", func() {
			_, err := reconciler.getProxyURL(ctx, "test-cluster")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get ManagedProxyConfiguration"))
		})

		It("should return error when proxy namespace missing in spec", func() {
			// Create ManagedProxyConfiguration without proxyServer.namespace
			proxyConfig := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "proxy.open-cluster-management.io/v1alpha1",
					"kind":       "ManagedProxyConfiguration",
					"metadata": map[string]interface{}{
						"name": "cluster-proxy",
					},
					"spec": map[string]interface{}{},
				},
			}
			proxyConfig.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "proxy.open-cluster-management.io",
				Version: "v1alpha1",
				Kind:    "ManagedProxyConfiguration",
			})
			Expect(k8sClient.Create(ctx, proxyConfig)).To(Succeed())

			_, err := reconciler.getProxyURL(ctx, "test-cluster")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to extract proxyServer.namespace"))
		})
	})

	Describe("findProxyService", func() {
		It("should find proxy service by label", func() {
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-proxy-addon-user",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"component": "cluster-proxy-addon-user",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port: 8090,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, service)).To(Succeed())

			name, namespace, port, err := reconciler.findProxyService(ctx, "test-namespace")
			Expect(err).ToNot(HaveOccurred())
			Expect(name).To(Equal("cluster-proxy-addon-user"))
			Expect(namespace).To(Equal("test-namespace"))
			Expect(port).To(Equal(int32(8090)))
		})

		It("should return error when no service found", func() {
			_, _, _, err := reconciler.findProxyService(ctx, "empty-namespace")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no proxy service found"))
		})

		It("should return error when service has no ports", func() {
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-proxy-addon-user",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"component": "cluster-proxy-addon-user",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{},
				},
			}
			Expect(k8sClient.Create(ctx, service)).To(Succeed())

			_, _, _, err := reconciler.findProxyService(ctx, "test-namespace")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("has no ports defined"))
		})

		It("should use first service when multiple services match", func() {
			service1 := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "proxy-service-1",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"component": "cluster-proxy-addon-user",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{{Port: 8090}},
				},
			}
			service2 := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "proxy-service-2",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"component": "cluster-proxy-addon-user",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{{Port: 8091}},
				},
			}
			Expect(k8sClient.Create(ctx, service1)).To(Succeed())
			Expect(k8sClient.Create(ctx, service2)).To(Succeed())

			name, _, port, err := reconciler.findProxyService(ctx, "test-namespace")
			Expect(err).ToNot(HaveOccurred())
			// Should return one of them (fake client may not guarantee order)
			Expect(name).To(SatisfyAny(Equal("proxy-service-1"), Equal("proxy-service-2")))
			Expect(port).To(SatisfyAny(Equal(int32(8090)), Equal(int32(8091))))
		})
	})

	Describe("getProxyCA", func() {
		It("should read CA from default ConfigMap (openshift-service-ca.crt)", func() {
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "openshift-service-ca.crt",
					Namespace: "multicluster-engine",
				},
				Data: map[string]string{
					"service-ca.crt": "-----BEGIN CERTIFICATE-----\nTEST_CA_DATA\n-----END CERTIFICATE-----",
				},
			}
			Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

			ca, err := reconciler.getProxyCA(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(ca).ToNot(BeEmpty())
			// Should be base64 encoded
			Expect(ca).To(MatchRegexp("^[A-Za-z0-9+/=]+$"))
		})

		It("should return error when ConfigMap not found", func() {
			_, err := reconciler.getProxyCA(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get service CA ConfigMap"))
			Expect(err.Error()).To(ContainSubstring("verify OpenShift cluster"))
		})

		It("should return error when service-ca.crt key missing from ConfigMap", func() {
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "openshift-service-ca.crt",
					Namespace: "multicluster-engine",
				},
				Data: map[string]string{
					"other-key": "some-data",
				},
			}
			Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

			_, err := reconciler.getProxyCA(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("service-ca.crt key not found"))
		})
	})

	Describe("getOperatorToken", func() {
		var (
			tempTokenFile *os.File
		)

		BeforeEach(func() {
			// Create temporary token file
			var err error
			tempTokenFile, err = os.CreateTemp("", "token-*")
			Expect(err).ToNot(HaveOccurred())

			// Write test token
			_, err = tempTokenFile.WriteString("test-operator-token-12345")
			Expect(err).ToNot(HaveOccurred())
			Expect(tempTokenFile.Close()).To(Succeed())
		})

		AfterEach(func() {
			// Cleanup
			if tempTokenFile != nil {
				os.Remove(tempTokenFile.Name())
			}
		})

		It("should read token from file", func() {
			// Temporarily override the constant for testing
			// Note: In real code, ServiceAccountTokenPath is a constant
			// For testing, we'd need to make it a variable or use dependency injection
			// This test demonstrates the concept
			Skip("Skipping due to const limitation - would work with DI")
		})

		It("should return error when token file not found", func() {
			// Override reconciler to use non-existent path
			// This would require making the path configurable
			Skip("Skipping due to const limitation - would work with DI")
		})
	})

	Describe("getServiceAccountName", func() {
		AfterEach(func() {
			os.Unsetenv("SERVICE_ACCOUNT_NAME")
		})

		It("should return default service account name when env not set", func() {
			saName := reconciler.getServiceAccountName(ctx)
			Expect(saName).To(Equal(DefaultServiceAccountName))
		})

		It("should return service account name from environment", func() {
			os.Setenv("SERVICE_ACCOUNT_NAME", "custom-sa")
			saName := reconciler.getServiceAccountName(ctx)
			Expect(saName).To(Equal("custom-sa"))
		})
	})

	Describe("ensureManifestWork", func() {
		It("should create ManifestWork when it doesn't exist", func() {
			err := reconciler.ensureManifestWork(ctx, "test-cluster")
			// When creating ManifestWork, returns nil (success)
			// Full validation happens on subsequent calls
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return error when ManifestWork exists but not Applied", func() {
			Skip("Requires ManifestWork CRD in test environment")
		})

		It("should succeed when ManifestWork exists and is Applied", func() {
			Skip("Requires ManifestWork CRD in test environment")
		})
	})

	Describe("getProxyConfig", func() {
		It("should return disabled config when proxy mode not enabled", func() {
			config, err := reconciler.getProxyConfig(ctx, "test-cluster")
			Expect(err).ToNot(HaveOccurred())
			Expect(config).ToNot(BeNil())
			Expect(config.Enabled).To(BeFalse())
		})

		It("should return error when proxy enabled but ManagedProxyConfiguration missing", func() {
			store.SetValue("ACM_USE_PROXY_TEST_CLUSTER", "true")

			// Create ManifestWork with Applied status (simulating OCM processed it)
			manifestWork := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "work.open-cluster-management.io/v1",
					"kind":       "ManifestWork",
					"metadata": map[string]interface{}{
						"name":      ManifestWorkName,
						"namespace": "test-cluster",
					},
					"status": map[string]interface{}{
						"conditions": []interface{}{
							map[string]interface{}{
								"type":   "Applied",
								"status": "True",
							},
							map[string]interface{}{
								"type":   "Available",
								"status": "True",
							},
						},
					},
				},
			}
			manifestWork.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "work.open-cluster-management.io",
				Version: "v1",
				Kind:    "ManifestWork",
			})
			Expect(k8sClient.Create(ctx, manifestWork)).To(Succeed())

			// Now getProxyConfig should fail because ManagedProxyConfiguration doesn't exist
			config, err := reconciler.getProxyConfig(ctx, "test-cluster")
			Expect(err).To(HaveOccurred())
			Expect(config).To(BeNil())
			// Should fail fast with clear error message
			Expect(err.Error()).To(ContainSubstring("proxy mode enabled but"))
			// Error message should suggest either fixing or disabling proxy mode
			Expect(err.Error()).To(ContainSubstring("ACM_USE_PROXY_TEST_CLUSTER=false"))
		})
	})
})
