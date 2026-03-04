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

	"github.com/go-logr/logr"
	krknv1alpha1 "github.com/krkn-chaos/krkn-operator/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// checkProviderActive verifies if the operator's own provider is active.
// Returns:
// - providerList: the list of all providers in the namespace (for reuse in counting)
// - shouldSkip: true if reconcile should be skipped
// - result: the ctrl.Result to return if shouldSkip is true
// - err: any error encountered
func checkProviderActive(
	ctx context.Context,
	c client.Client,
	logger logr.Logger,
	operatorName string,
	operatorNamespace string,
) (*krknv1alpha1.KrknOperatorTargetProviderList, bool, ctrl.Result, error) {
	// List all providers in the operator's namespace
	providerList := &krknv1alpha1.KrknOperatorTargetProviderList{}
	err := c.List(ctx, providerList, client.InNamespace(operatorNamespace))
	if err != nil {
		logger.Error(err, "failed to list KrknOperatorTargetProviders", "namespace", operatorNamespace)
		return nil, true, ctrl.Result{}, err
	}

	// Find this operator's provider
	var thisProvider *krknv1alpha1.KrknOperatorTargetProvider
	for i := range providerList.Items {
		if providerList.Items[i].Spec.OperatorName == operatorName {
			thisProvider = &providerList.Items[i]
			break
		}
	}

	// Check if provider exists
	if thisProvider == nil {
		logger.Info("provider not found, skipping reconcile", "provider-name", operatorName)
		return providerList, true, ctrl.Result{}, nil
	}

	// Check if provider is active
	if !thisProvider.Spec.Active {
		logger.Info("provider is not active, skipping reconcile", "provider-name", operatorName)
		return providerList, true, ctrl.Result{}, nil
	}

	// Provider is active, continue reconcile
	return providerList, false, ctrl.Result{}, nil
}

// countActiveProviders counts the number of active providers in a provider list
func countActiveProviders(providerList *krknv1alpha1.KrknOperatorTargetProviderList) int {
	activeCount := 0
	for _, provider := range providerList.Items {
		if provider.Spec.Active {
			activeCount++
		}
	}
	return activeCount
}

// shouldMarkAsCompleted determines if a CR should be marked as completed
// based on the number of active providers and contributors
func shouldMarkAsCompleted(activeProviderCount int, contributorCount int) bool {
	return activeProviderCount > 0 && contributorCount >= activeProviderCount
}
