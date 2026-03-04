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
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	kvstore "github.com/krkn-chaos/krkn-operator/pkg/configstore"
)

// ConfigMapReconciler reconciles the operator's configuration ConfigMap
type ConfigMapReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	ConfigMapName      string
	ConfigMapNamespace string
}

// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

// Reconcile syncs the ConfigMap data into the configstore singleton
func (r *ConfigMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get the configstore singleton
	store := kvstore.Get()

	// Fetch the ConfigMap
	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, req.NamespacedName, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("configMap not found, keeping existing configstore values",
				"name", req.Name,
				"namespace", req.Namespace)
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get ConfigMap")
		return ctrl.Result{}, err
	}

	logger.Info("syncing ConfigMap to configstore",
		"name", configMap.Name,
		"namespace", configMap.Namespace,
		"keys-count", len(configMap.Data))

	// Sync all values from ConfigMap to configstore
	for key, value := range configMap.Data {
		// If the key is "config.yaml", parse the content line by line
		if key == "config.yaml" {
			lines := strings.Split(value, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				// Split by comma to get KEY,VALUE
				parts := strings.SplitN(line, ",", 2)
				if len(parts) != 2 {
					logger.Info("skipping invalid line in config.yaml", "line", line)
					continue
				}
				configKey := strings.TrimSpace(parts[0])
				configValue := strings.TrimSpace(parts[1])

				oldValue, exists := store.GetValue(configKey)
				if !exists || oldValue != configValue {
					store.SetValue(configKey, configValue)
					logger.Info("updated config value from config.yaml",
						"key", configKey,
						"old", oldValue,
						"new", configValue,
						"is-new", !exists)
				}
			}
		} else {
			// For other keys, store as-is
			oldValue, exists := store.GetValue(key)
			if !exists || oldValue != value {
				store.SetValue(key, value)
				logger.Info("updated config value",
					"key", key,
					"old", oldValue,
					"new", value,
					"is-new", !exists)
			}
		}
	}

	return ctrl.Result{}, nil
}

// NewConfigMapFilter creates a predicate that only watches a specific ConfigMap
func NewConfigMapFilter(name, namespace string) predicate.Predicate {
	matches := func(objName, objNamespace string) bool {
		return objName == name && objNamespace == namespace
	}

	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return matches(e.Object.GetName(), e.Object.GetNamespace())
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return matches(e.ObjectNew.GetName(), e.ObjectNew.GetNamespace())
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return matches(e.Object.GetName(), e.Object.GetNamespace())
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return matches(e.Object.GetName(), e.Object.GetNamespace())
		},
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *ConfigMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		Named("configmap").
		WithEventFilter(NewConfigMapFilter(r.ConfigMapName, r.ConfigMapNamespace)).
		Complete(r)
}
