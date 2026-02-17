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

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	krknv1alpha1 "github.com/krkn-chaos/krkn-operator/api/v1alpha1"
	kvstore "github.com/krkn-chaos/krkn-operator/pkg/configstore"
)

func TestGetConfiguredSecretName(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	_ = krknv1alpha1.AddToScheme(scheme)

	reconciler := &KrknTargetRequestReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
		Scheme: scheme,
	}

	store := kvstore.Get()

	// Clean up before tests
	store.Delete("ACM_SECRET_LOCAL_CLUSTER")
	store.Delete("ACM_SECRET_MANAGED_CLUSTER_KRKN")
	store.Delete("ACM_SECRET_TEST_CLUSTER")

	tests := []struct {
		name                string
		clusterName         string
		configuredSecret    string
		setInConfigstore    bool
		expectedSecretName  string
	}{
		{
			name:               "ConfigStore has custom secret",
			clusterName:        "local-cluster",
			configuredSecret:   "custom-secret",
			setInConfigstore:   true,
			expectedSecretName: "custom-secret",
		},
		{
			name:               "ConfigStore not set - use default",
			clusterName:        "managed-cluster-krkn",
			setInConfigstore:   false,
			expectedSecretName: ACMDefaultSecret,
		},
		{
			name:               "ConfigStore has empty value - use default",
			clusterName:        "test-cluster",
			configuredSecret:   "",
			setInConfigstore:   true,
			expectedSecretName: ACMDefaultSecret,
		},
		{
			name:               "ConfigStore has application-manager explicitly",
			clusterName:        "local-cluster",
			configuredSecret:   "application-manager",
			setInConfigstore:   true,
			expectedSecretName: "application-manager",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup configstore
			varName := formatNamespaceToVarName(tt.clusterName)
			store.Delete(varName)

			if tt.setInConfigstore {
				store.SetValue(varName, tt.configuredSecret)
			}

			// Test
			result := reconciler.getConfiguredSecretName(tt.clusterName)

			// Verify
			if result != tt.expectedSecretName {
				t.Errorf("getConfiguredSecretName(%q) = %q, want %q",
					tt.clusterName, result, tt.expectedSecretName)
			}

			// Cleanup
			store.Delete(varName)
		})
	}
}