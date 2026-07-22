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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	krknv1alpha1 "github.com/krkn-chaos/krkn-operator/api/v1alpha1"
	kvstore "github.com/krkn-chaos/krkn-operator/pkg/configstore"
)

func TestGetDefaultSecret(t *testing.T) {
	// Get configstore singleton
	store := kvstore.Get()

	// Clean up before tests
	store.Delete("ACM_SECRET_LOCAL_CLUSTER")
	store.Delete("ACM_SECRET_MANAGED_CLUSTER_KRKN")

	tests := []struct {
		name             string
		namespace        string
		secrets          []string
		configstoreValue string
		setConfigstore   bool
		expectedDefault  string
		description      string
	}{
		{
			name:            "Empty secrets list",
			namespace:       "test-cluster",
			secrets:         []string{},
			expectedDefault: "",
			description:     "Should return empty string for empty secrets list",
		},
		{
			name:      "ConfigStore value has priority",
			namespace: "local-cluster",
			secrets: []string{
				ACMDefaultSecret,
				"builder-dockercfg-xxx",
				"default-dockercfg-yyy",
			},
			configstoreValue: "builder-dockercfg-xxx",
			setConfigstore:   true,
			expectedDefault:  "builder-dockercfg-xxx",
			description:      "Should use configstore value when it exists and is valid",
		},
		{
			name:      "ConfigStore value not in list - fallback to ACMDefaultSecret",
			namespace: "local-cluster",
			secrets: []string{
				ACMDefaultSecret,
				"default-dockercfg-yyy",
			},
			configstoreValue: "non-existent-secret",
			setConfigstore:   true,
			expectedDefault:  ACMDefaultSecret,
			description:      "Should fallback to ACMDefaultSecret if configstore value is not in the list",
		},
		{
			name:      "No configstore - use ACMDefaultSecret",
			namespace: "local-cluster",
			secrets: []string{
				"builder-dockercfg-xxx",
				ACMDefaultSecret,
				"default-dockercfg-yyy",
			},
			setConfigstore:  false,
			expectedDefault: ACMDefaultSecret,
			description:     "Should use ACMDefaultSecret when no configstore value",
		},
		{
			name:      "No configstore, no ACMDefaultSecret - use first",
			namespace: "managed-cluster-krkn",
			secrets: []string{
				"builder-dockercfg-xxx",
				"default-dockercfg-yyy",
				"deployer-dockercfg-zzz",
			},
			setConfigstore:  false,
			expectedDefault: "builder-dockercfg-xxx",
			description:     "Should use first secret when no configstore and no ACMDefaultSecret",
		},
		{
			name:      "ConfigStore with empty value - fallback",
			namespace: "local-cluster",
			secrets: []string{
				ACMDefaultSecret,
				"default-dockercfg-yyy",
			},
			configstoreValue: "",
			setConfigstore:   true,
			expectedDefault:  ACMDefaultSecret,
			description:      "Should fallback when configstore value is empty string",
		},
		{
			name:      "ConfigStore value matches exactly",
			namespace: "managed-cluster-krkn",
			secrets: []string{
				"secret-a",
				"secret-b",
				"secret-c",
			},
			configstoreValue: "secret-b",
			setConfigstore:   true,
			expectedDefault:  "secret-b",
			description:      "Should use exact match from configstore",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup configstore
			varName := formatNamespaceToVarName(tt.namespace)
			store.Delete(varName) // Clean up first

			if tt.setConfigstore {
				store.SetValue(varName, tt.configstoreValue)
			}

			// Call function
			result := getDefaultSecret(tt.namespace, tt.secrets)

			// Verify result
			if result != tt.expectedDefault {
				t.Errorf("%s\ngetDefaultSecret(%q, %v) = %q, want %q",
					tt.description,
					tt.namespace,
					tt.secrets,
					result,
					tt.expectedDefault)
			}

			// Cleanup
			store.Delete(varName)
		})
	}
}

func TestFormatNamespaceToVarName(t *testing.T) {
	tests := []struct {
		namespace string
		expected  string
	}{
		{
			namespace: "local-cluster",
			expected:  "ACM_SECRET_LOCAL_CLUSTER",
		},
		{
			namespace: "managed-cluster-krkn",
			expected:  "ACM_SECRET_MANAGED_CLUSTER_KRKN",
		},
		{
			namespace: "my-test-namespace",
			expected:  "ACM_SECRET_MY_TEST_NAMESPACE",
		},
		{
			namespace: "simple",
			expected:  "ACM_SECRET_SIMPLE",
		},
		{
			namespace: "with-many-dashes-here",
			expected:  "ACM_SECRET_WITH_MANY_DASHES_HERE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			result := formatNamespaceToVarName(tt.namespace)
			if result != tt.expected {
				t.Errorf("formatNamespaceToVarName(%q) = %q, want %q",
					tt.namespace, result, tt.expected)
			}
		})
	}
}

func TestHasRequiredKeys(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string][]byte
		expected bool
	}{
		{
			name: "Has both ca.crt and token",
			data: map[string][]byte{
				"ca.crt": []byte("cert-data"),
				"token":  []byte("token-data"),
			},
			expected: true,
		},
		{
			name: "Has ca.crt, token, and extra keys",
			data: map[string][]byte{
				"ca.crt":    []byte("cert-data"),
				"token":     []byte("token-data"),
				"namespace": []byte("default"),
			},
			expected: true,
		},
		{
			name: "Missing token",
			data: map[string][]byte{
				"ca.crt": []byte("cert-data"),
			},
			expected: false,
		},
		{
			name: "Missing ca.crt",
			data: map[string][]byte{
				"token": []byte("token-data"),
			},
			expected: false,
		},
		{
			name:     "Empty data",
			data:     map[string][]byte{},
			expected: false,
		},
		{
			name: "Has neither key",
			data: map[string][]byte{
				"other-key": []byte("other-data"),
			},
			expected: false,
		},
		{
			name: "Has empty values but keys exist",
			data: map[string][]byte{
				"ca.crt": []byte(""),
				"token":  []byte(""),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasRequiredKeys(tt.data)
			if result != tt.expected {
				t.Errorf("hasRequiredKeys() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Ginkgo-style tests for additional coverage
var _ = Describe("KrknOperatorTargetProviderConfig Controller Helpers (Ginkgo)", func() {
	var (
		ctx        context.Context
		k8sClient  client.Client
		scheme     *runtime.Scheme
		reconciler *KrknOperatorTargetProviderConfigReconciler
		store      *kvstore.Store
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		k8sClient = fake.NewClientBuilder().WithScheme(scheme).Build()
		store = kvstore.Get()

		reconciler = &KrknOperatorTargetProviderConfigReconciler{
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

	Describe("getDefaultProxyMode", func() {
		It("should return false when not configured", func() {
			mode := getDefaultProxyMode("test-cluster")
			Expect(mode).To(Equal("false"))
		})

		It("should return true for 'true' value", func() {
			store.SetValue("ACM_USE_PROXY_TEST_CLUSTER", "true")
			mode := getDefaultProxyMode("test-cluster")
			Expect(mode).To(Equal("true"))
		})

		It("should return true for 'yes' value", func() {
			store.SetValue("ACM_USE_PROXY_TEST_CLUSTER", "yes")
			mode := getDefaultProxyMode("test-cluster")
			Expect(mode).To(Equal("true"))
		})

		It("should return true for '1' value", func() {
			store.SetValue("ACM_USE_PROXY_TEST_CLUSTER", "1")
			mode := getDefaultProxyMode("test-cluster")
			Expect(mode).To(Equal("true"))
		})

		It("should return false for 'false' value", func() {
			store.SetValue("ACM_USE_PROXY_TEST_CLUSTER", "false")
			mode := getDefaultProxyMode("test-cluster")
			Expect(mode).To(Equal("false"))
		})

		It("should return false for 'no' value", func() {
			store.SetValue("ACM_USE_PROXY_TEST_CLUSTER", "no")
			mode := getDefaultProxyMode("test-cluster")
			Expect(mode).To(Equal("false"))
		})

		It("should return false for '0' value", func() {
			store.SetValue("ACM_USE_PROXY_TEST_CLUSTER", "0")
			mode := getDefaultProxyMode("test-cluster")
			Expect(mode).To(Equal("false"))
		})

		It("should return false for empty string", func() {
			store.SetValue("ACM_USE_PROXY_TEST_CLUSTER", "")
			mode := getDefaultProxyMode("test-cluster")
			Expect(mode).To(Equal("false"))
		})

		It("should return false for invalid value", func() {
			store.SetValue("ACM_USE_PROXY_TEST_CLUSTER", "invalid")
			mode := getDefaultProxyMode("test-cluster")
			Expect(mode).To(Equal("false"))
		})

		It("should normalize values with whitespace", func() {
			store.SetValue("ACM_USE_PROXY_TEST_CLUSTER", "  TRUE  ")
			mode := getDefaultProxyMode("test-cluster")
			Expect(mode).To(Equal("true"))
		})
	})

	Describe("listSecretsInNamespace", func() {
		It("should return empty list when namespace has no secrets", func() {
			secrets, err := reconciler.listSecretsInNamespace(ctx, "empty-namespace")
			Expect(err).ToNot(HaveOccurred())
			Expect(secrets).To(BeEmpty())
		})

		It("should return only secrets with required keys", func() {
			// Create secret with both keys
			validSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-secret",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"ca.crt": []byte("test-ca"),
					"token":  []byte("test-token"),
				},
			}
			Expect(k8sClient.Create(ctx, validSecret)).To(Succeed())

			// Create secret with missing keys
			invalidSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-secret",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"other-key": []byte("test-data"),
				},
			}
			Expect(k8sClient.Create(ctx, invalidSecret)).To(Succeed())

			secrets, err := reconciler.listSecretsInNamespace(ctx, "test-namespace")
			Expect(err).ToNot(HaveOccurred())
			Expect(secrets).To(HaveLen(1))
			Expect(secrets[0]).To(Equal("valid-secret"))
		})

		It("should filter out secrets missing ca.crt", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-ca-secret",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"token": []byte("test-token"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			secrets, err := reconciler.listSecretsInNamespace(ctx, "test-namespace")
			Expect(err).ToNot(HaveOccurred())
			Expect(secrets).To(BeEmpty())
		})

		It("should filter out secrets missing token", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-token-secret",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"ca.crt": []byte("test-ca"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			secrets, err := reconciler.listSecretsInNamespace(ctx, "test-namespace")
			Expect(err).ToNot(HaveOccurred())
			Expect(secrets).To(BeEmpty())
		})
	})

	Describe("getConfigDataKeys", func() {
		It("should return empty slice for nil map", func() {
			keys := getConfigDataKeys(nil)
			Expect(keys).To(BeEmpty())
		})

		It("should return empty slice for empty map", func() {
			keys := getConfigDataKeys(map[string]krknv1alpha1.ProviderConfigData{})
			Expect(keys).To(BeEmpty())
		})

		It("should return all keys from map", func() {
			configData := map[string]krknv1alpha1.ProviderConfigData{
				"operator1": {},
				"operator2": {},
				"operator3": {},
			}
			keys := getConfigDataKeys(configData)
			Expect(keys).To(HaveLen(3))
			Expect(keys).To(ContainElements("operator1", "operator2", "operator3"))
		})
	})
})
