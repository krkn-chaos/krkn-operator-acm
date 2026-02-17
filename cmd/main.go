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

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"os"
	"path/filepath"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/krkn-chaos/krkn-operator-acm/internal/controller"
	krknv1alpha1 "github.com/krkn-chaos/krkn-operator/api/v1alpha1"
	"github.com/krkn-chaos/krkn-operator/pkg/provider"
	// +kubebuilder:scaffold:imports
)

// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=krkn.krkn-chaos.dev,resources=krknoperatortargetproviders,verbs=create;get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=krkn.krkn-chaos.dev,resources=krknoperatortargetproviders/status,verbs=get;update;patch

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(krknv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// OperatorConfig holds the configuration for the operator
type OperatorConfig struct {
	OperatorName      string
	OperatorNamespace string
}

// getOperatorConfig reads the operator configuration from the ConfigMap or environment variables
func getOperatorConfig(ctx context.Context, k8sClient client.Client, defaultNamespace string) (*OperatorConfig, error) {
	config := &OperatorConfig{
		OperatorName:      "krkn-operator-acm", // Default
		OperatorNamespace: defaultNamespace,    // From environment or default
	}

	// Try to read from environment variables first (takes precedence)
	if envName := os.Getenv("OPERATOR_NAME"); envName != "" {
		config.OperatorName = envName
		setupLog.Info("Using operator name from environment", "operator-name", envName)
	}

	if envNamespace := os.Getenv("POD_NAMESPACE"); envNamespace != "" {
		config.OperatorNamespace = envNamespace
		setupLog.Info("Using operator namespace from environment", "operator-namespace", envNamespace)
	}

	// Try to read from ConfigMap (if environment variables not set)
	configMap := &corev1.ConfigMap{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      "krkn-operator-config",
		Namespace: config.OperatorNamespace,
	}, configMap)

	if err != nil {
		if errors.IsNotFound(err) {
			// ConfigMap not found - use defaults/environment
			setupLog.Info("ConfigMap not found, using defaults/environment",
				"operator-name", config.OperatorName,
				"operator-namespace", config.OperatorNamespace)
			return config, nil
		}
		return nil, err
	}

	// Override with ConfigMap values if not set by environment
	if os.Getenv("OPERATOR_NAME") == "" {
		if operatorName, ok := configMap.Data["operator-name"]; ok && operatorName != "" {
			config.OperatorName = operatorName
			setupLog.Info("Loaded operator name from ConfigMap", "operator-name", operatorName)
		}
	}

	if os.Getenv("POD_NAMESPACE") == "" {
		if operatorNamespace, ok := configMap.Data["operator-namespace"]; ok && operatorNamespace != "" {
			config.OperatorNamespace = operatorNamespace
			setupLog.Info("Loaded operator namespace from ConfigMap", "operator-namespace", operatorNamespace)
		}
	}

	return config, nil
}

// ConfigLoaderRunnable loads operator configuration on startup
type ConfigLoaderRunnable struct {
	Client                      client.Client
	DefaultNamespace            string
	TargetRequestReconciler     *controller.KrknTargetRequestReconciler
	ProviderConfigReconciler    *controller.KrknOperatorTargetProviderConfigReconciler
}

// Start implements the Runnable interface
func (r *ConfigLoaderRunnable) Start(ctx context.Context) error {
	setupLog.Info("Loading operator configuration...")

	// Load configuration from ConfigMap/environment
	config, err := getOperatorConfig(ctx, r.Client, r.DefaultNamespace)
	if err != nil {
		setupLog.Error(err, "FATAL: Failed to load operator configuration")
		return err
	}

	setupLog.Info("Configuration loaded",
		"operator-name", config.OperatorName,
		"operator-namespace", config.OperatorNamespace)

	// Update reconcilers with loaded config
	r.TargetRequestReconciler.OperatorName = config.OperatorName
	r.TargetRequestReconciler.OperatorNamespace = config.OperatorNamespace

	if r.ProviderConfigReconciler != nil {
		r.ProviderConfigReconciler.OperatorName = config.OperatorName
		r.ProviderConfigReconciler.OperatorNamespace = config.OperatorNamespace
	}

	// Keep running to satisfy the Runnable interface
	<-ctx.Done()
	return nil
}

// nolint:gocyclo
func main() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Create watchers for metrics and webhooks certificates
	var metricsCertWatcher, webhookCertWatcher *certwatcher.CertWatcher

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		var err error
		webhookCertWatcher, err = certwatcher.New(
			filepath.Join(webhookCertPath, webhookCertName),
			filepath.Join(webhookCertPath, webhookCertKey),
		)
		if err != nil {
			setupLog.Error(err, "Failed to initialize webhook certificate watcher")
			os.Exit(1)
		}

		webhookTLSOpts = append(webhookTLSOpts, func(config *tls.Config) {
			config.GetCertificate = webhookCertWatcher.GetCertificate
		})
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: webhookTLSOpts,
	})

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		var err error
		metricsCertWatcher, err = certwatcher.New(
			filepath.Join(metricsCertPath, metricsCertName),
			filepath.Join(metricsCertPath, metricsCertKey),
		)
		if err != nil {
			setupLog.Error(err, "to initialize metrics certificate watcher", "error", err)
			os.Exit(1)
		}

		metricsServerOptions.TLSOpts = append(metricsServerOptions.TLSOpts, func(config *tls.Config) {
			config.GetCertificate = metricsCertWatcher.GetCertificate
		})
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "9021e13d.krkn-chaos.dev",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Determine default namespace from environment
	defaultNamespace := os.Getenv("POD_NAMESPACE")
	if defaultNamespace == "" {
		defaultNamespace = "krkn-operator-system" // Default namespace
	}

	// Determine operator name from environment (will be loaded from ConfigMap if not set)
	operatorName := os.Getenv("OPERATOR_NAME")
	if operatorName == "" {
		operatorName = "krkn-operator-acm" // Default
	}

	// Create reconciler with placeholder values (will be updated by ConfigLoaderRunnable)
	targetRequestReconciler := &controller.KrknTargetRequestReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		OperatorName:      operatorName,     // Placeholder
		OperatorNamespace: defaultNamespace, // Placeholder
	}

	// Setup controller
	if err := targetRequestReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KrknTargetRequest")
		os.Exit(1)
	}

	// Create KrknOperatorTargetProviderConfig reconciler
	providerConfigReconciler := &controller.KrknOperatorTargetProviderConfigReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		OperatorName:      operatorName,     // Placeholder
		OperatorNamespace: defaultNamespace, // Placeholder
	}

	// Setup controller
	if err := providerConfigReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KrknOperatorTargetProviderConfig")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	// Add config loader runnable
	// This runs after the manager cache starts and loads config from ConfigMap
	if err := mgr.Add(&ConfigLoaderRunnable{
		Client:                   mgr.GetClient(),
		DefaultNamespace:         defaultNamespace,
		TargetRequestReconciler:  targetRequestReconciler,
		ProviderConfigReconciler: providerConfigReconciler,
	}); err != nil {
		setupLog.Error(err, "unable to add config loader runnable")
		os.Exit(1)
	}

	// Add provider registration using the standard krkn-operator package
	// This handles automatic registration, heartbeat, and deactivation on shutdown
	providerReg := provider.NewProviderRegistrationWithConfig(
		mgr.GetClient(),
		provider.Config{
			ProviderName:      operatorName,
			HeartbeatInterval: 30 * time.Second,
			Namespace:         defaultNamespace,
		},
	)
	if err := mgr.Add(providerReg); err != nil {
		setupLog.Error(err, "unable to add provider registration")
		os.Exit(1)
	}
	setupLog.Info("Provider registration configured", "operator-name", operatorName, "namespace", defaultNamespace)

	if metricsCertWatcher != nil {
		setupLog.Info("Adding metrics certificate watcher to manager")
		if err := mgr.Add(metricsCertWatcher); err != nil {
			setupLog.Error(err, "unable to add metrics certificate watcher to manager")
			os.Exit(1)
		}
	}

	if webhookCertWatcher != nil {
		setupLog.Info("Adding webhook certificate watcher to manager")
		if err := mgr.Add(webhookCertWatcher); err != nil {
			setupLog.Error(err, "unable to add webhook certificate watcher to manager")
			os.Exit(1)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
