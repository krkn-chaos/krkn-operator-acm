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

const (
	// OperatorName is the default name for this operator instance
	// Used for:
	// - KrknOperatorTargetProvider registration
	// - UpdateProviderConfig calls
	// - Setting target data in KrknTargetRequest status
	OperatorName = "krkn-operator-acm"

	// DefaultConfigMapName is the default name of the ConfigMap holding provider configuration
	DefaultConfigMapName = "krkn-operator-acm-config"

	// ACMDefaultSecret is the default secret name used for ACM managed cluster authentication
	ACMDefaultSecret = "application-manager"
)
