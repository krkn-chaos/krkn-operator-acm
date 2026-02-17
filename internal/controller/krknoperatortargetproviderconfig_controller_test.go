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
	"testing"

	kvstore "github.com/krkn-chaos/krkn-operator/pkg/configstore"
)

func TestGetDefaultSecret(t *testing.T) {
	// Get configstore singleton
	store := kvstore.Get()

	// Clean up before tests
	store.Delete("ACM_SECRET_LOCAL_CLUSTER")
	store.Delete("ACM_SECRET_MANAGED_CLUSTER_KRKN")

	tests := []struct {
		name              string
		namespace         string
		secrets           []string
		configstoreValue  string
		setConfigstore    bool
		expectedDefault   string
		description       string
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