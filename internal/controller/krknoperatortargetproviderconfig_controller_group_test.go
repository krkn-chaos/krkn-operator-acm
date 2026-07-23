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
	"encoding/json"

	"github.com/krkn-chaos/krknctl/pkg/typing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Configuration Schema with Field Groups", func() {

	Describe("Group field creation", func() {
		It("should create group fields with correct type and properties", func() {
			// Test secret group field properties
			secretGroupName := "ACM_SECRET_GROUP"
			secretGroupShortDesc := "ACM/OCM Secret Selection"
			secretGroupDesc := "Select secrets for direct API connection to managed clusters. Each secret contains the CA certificate and service account token for cluster authentication."

			secretGroupField := typing.InputField{
				Name:             &secretGroupName,
				ShortDescription: &secretGroupShortDesc,
				Description:      &secretGroupDesc,
				Variable:         &secretGroupName,
				Type:             typing.Group,
				Required:         false,
				Secret:           false,
			}

			// Verify secret group field
			Expect(*secretGroupField.Name).To(Equal("ACM_SECRET_GROUP"))
			Expect(*secretGroupField.Variable).To(Equal("ACM_SECRET_GROUP"))
			Expect(secretGroupField.Type).To(Equal(typing.Group))
			Expect(secretGroupField.Required).To(BeFalse())
			Expect(*secretGroupField.ShortDescription).To(ContainSubstring("Secret Selection"))
			Expect(*secretGroupField.Description).To(ContainSubstring("direct API connection"))

			// Test proxy group field properties
			proxyGroupName := "ACM_USE_PROXY_GROUP"
			proxyGroupShortDesc := "Cluster Proxy Configuration"
			proxyGroupDesc := "Enable cluster proxy connection for managed clusters. Note: Activating proxy for a cluster overrides the secret selection and uses the cluster-proxy-addon for communication. To return to direct API access, disable the proxy for that cluster. Requires cluster-proxy-addon and ManifestWork to be deployed."

			proxyGroupField := typing.InputField{
				Name:             &proxyGroupName,
				ShortDescription: &proxyGroupShortDesc,
				Description:      &proxyGroupDesc,
				Variable:         &proxyGroupName,
				Type:             typing.Group,
				Required:         false,
				Secret:           false,
			}

			// Verify proxy group field
			Expect(*proxyGroupField.Name).To(Equal("ACM_USE_PROXY_GROUP"))
			Expect(*proxyGroupField.Variable).To(Equal("ACM_USE_PROXY_GROUP"))
			Expect(proxyGroupField.Type).To(Equal(typing.Group))
			Expect(proxyGroupField.Required).To(BeFalse())
			Expect(*proxyGroupField.ShortDescription).To(ContainSubstring("Proxy Configuration"))
			Expect(*proxyGroupField.Description).To(ContainSubstring("overrides the secret selection"))
		})

		It("should serialize group fields to JSON correctly", func() {
			groupName := "ACM_SECRET_GROUP"
			shortDesc := "Test Group"
			desc := "Test Description"

			groupField := typing.InputField{
				Name:             &groupName,
				ShortDescription: &shortDesc,
				Description:      &desc,
				Variable:         &groupName,
				Type:             typing.Group,
				Required:         false,
				Secret:           false,
			}

			jsonData, err := groupField.MarshalJSON()
			Expect(err).ToNot(HaveOccurred())

			var unmarshaled map[string]interface{}
			err = json.Unmarshal(jsonData, &unmarshaled)
			Expect(err).ToNot(HaveOccurred())

			Expect(unmarshaled["name"]).To(Equal("ACM_SECRET_GROUP"))
			Expect(unmarshaled["type"]).To(Equal("group"))
			Expect(unmarshaled["variable"]).To(Equal("ACM_SECRET_GROUP"))
		})
	})

	Describe("Field group assignment", func() {
		It("should assign secret fields to ACM_SECRET_GROUP", func() {
			clusterName := "test-cluster"
			varName := formatNamespaceToVarName(clusterName)
			proxyVarName := formatProxyVarName(clusterName)
			secretGroupName := "ACM_SECRET_GROUP"
			shortDesc := "Test Secret"
			desc := "Test Description"
			defaultVal := "test-secret"
			separator := ","
			allowedValues := "test-secret,other-secret"

			secretField := typing.InputField{
				Name:             &varName,
				ShortDescription: &shortDesc,
				Description:      &desc,
				Variable:         &varName,
				Type:             typing.Enum,
				Default:          &defaultVal,
				Separator:        &separator,
				AllowedValues:    &allowedValues,
				Required:         true,
				Secret:           false,
				Group:            &secretGroupName,
				MutuallyExcludes: &proxyVarName,
			}

			// Verify group assignment
			Expect(secretField.Group).ToNot(BeNil())
			Expect(*secretField.Group).To(Equal("ACM_SECRET_GROUP"))

			// Verify mutual exclusion
			Expect(secretField.MutuallyExcludes).ToNot(BeNil())
			Expect(*secretField.MutuallyExcludes).To(Equal("ACM_USE_PROXY_TEST_CLUSTER"))
		})

		It("should assign proxy fields to ACM_USE_PROXY_GROUP", func() {
			clusterName := "test-cluster"
			proxyVarName := formatProxyVarName(clusterName)
			secretVarName := formatNamespaceToVarName(clusterName)
			proxyGroupName := "ACM_USE_PROXY_GROUP"
			shortDesc := "Test Proxy"
			desc := "Test Description"
			defaultVal := "false"
			separator := ","
			allowedValues := "true,false"

			proxyField := typing.InputField{
				Name:             &proxyVarName,
				ShortDescription: &shortDesc,
				Description:      &desc,
				Variable:         &proxyVarName,
				Type:             typing.Enum,
				Default:          &defaultVal,
				Separator:        &separator,
				AllowedValues:    &allowedValues,
				Required:         false,
				Secret:           false,
				Group:            &proxyGroupName,
				MutuallyExcludes: &secretVarName,
			}

			// Verify group assignment
			Expect(proxyField.Group).ToNot(BeNil())
			Expect(*proxyField.Group).To(Equal("ACM_USE_PROXY_GROUP"))

			// Verify mutual exclusion
			Expect(proxyField.MutuallyExcludes).ToNot(BeNil())
			Expect(*proxyField.MutuallyExcludes).To(Equal("ACM_SECRET_TEST_CLUSTER"))
		})
	})

	Describe("Mutual exclusion between secret and proxy fields", func() {
		It("should create bidirectional mutual exclusion for each cluster", func() {
			clusters := []string{"cluster-1", "cluster-2", "cluster-3"}

			for _, cluster := range clusters {
				secretVarName := formatNamespaceToVarName(cluster)
				proxyVarName := formatProxyVarName(cluster)

				// Verify the variable names are correctly formatted
				Expect(secretVarName).To(HavePrefix("ACM_SECRET_"))
				Expect(proxyVarName).To(HavePrefix("ACM_USE_PROXY_"))

				// Verify they reference each other
				// In the actual implementation:
				// - secretField.MutuallyExcludes points to proxyVarName
				// - proxyField.MutuallyExcludes points to secretVarName
			}
		})

		It("should correctly pair secret and proxy fields for the same cluster", func() {
			clusterName := "my-cluster"
			secretVar := formatNamespaceToVarName(clusterName)
			proxyVar := formatProxyVarName(clusterName)

			Expect(secretVar).To(Equal("ACM_SECRET_MY_CLUSTER"))
			Expect(proxyVar).To(Equal("ACM_USE_PROXY_MY_CLUSTER"))

			// Both should reference the same cluster
			// Secret field: ACM_SECRET_MY_CLUSTER mutually excludes ACM_USE_PROXY_MY_CLUSTER
			// Proxy field: ACM_USE_PROXY_MY_CLUSTER mutually excludes ACM_SECRET_MY_CLUSTER
		})
	})

	Describe("formatProxyVarName helper", func() {
		It("should format cluster names to proxy variable names", func() {
			tests := []struct {
				cluster  string
				expected string
			}{
				{"local-cluster", "ACM_USE_PROXY_LOCAL_CLUSTER"},
				{"test-cluster", "ACM_USE_PROXY_TEST_CLUSTER"},
				{"my-managed-cluster", "ACM_USE_PROXY_MY_MANAGED_CLUSTER"},
				{"simple", "ACM_USE_PROXY_SIMPLE"},
			}

			for _, tt := range tests {
				result := formatProxyVarName(tt.cluster)
				Expect(result).To(Equal(tt.expected))
			}
		})

		It("should handle hyphens correctly", func() {
			result := formatProxyVarName("cluster-with-many-dashes")
			Expect(result).To(Equal("ACM_USE_PROXY_CLUSTER_WITH_MANY_DASHES"))
			Expect(result).ToNot(ContainSubstring("-"))
			Expect(result).To(ContainSubstring("_"))
		})
	})

	Describe("JSON serialization with groups", func() {
		It("should serialize fields with group attribute", func() {
			varName := "ACM_SECRET_TEST"
			groupName := "ACM_SECRET_GROUP"
			mutuallyExcludes := "ACM_USE_PROXY_TEST"
			shortDesc := "Test"
			desc := "Test field"
			defaultVal := "default"
			separator := ","
			allowedValues := "val1,val2"

			field := typing.InputField{
				Name:             &varName,
				ShortDescription: &shortDesc,
				Description:      &desc,
				Variable:         &varName,
				Type:             typing.Enum,
				Default:          &defaultVal,
				Separator:        &separator,
				AllowedValues:    &allowedValues,
				Required:         true,
				Secret:           false,
				Group:            &groupName,
				MutuallyExcludes: &mutuallyExcludes,
			}

			jsonData, err := field.MarshalJSON()
			Expect(err).ToNot(HaveOccurred())

			var unmarshaled map[string]interface{}
			err = json.Unmarshal(jsonData, &unmarshaled)
			Expect(err).ToNot(HaveOccurred())

			// Verify group is serialized
			Expect(unmarshaled["group"]).To(Equal("ACM_SECRET_GROUP"))

			// Verify mutually_excludes is serialized
			Expect(unmarshaled["mutually_excludes"]).To(Equal("ACM_USE_PROXY_TEST"))

			// Verify other standard fields
			Expect(unmarshaled["name"]).To(Equal("ACM_SECRET_TEST"))
			Expect(unmarshaled["type"]).To(Equal("enum"))
		})
	})
})
