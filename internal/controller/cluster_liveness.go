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
*/

package controller

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const clusterLivenessTimeout = 2 * time.Second

var clusterHealthEndpoints = []string{"/readyz", "/livez", "/healthz"}

type clusterLivenessResult struct {
	Online  bool
	Healthy bool
}

// checkClusterAPILiveness probes the API's unauthenticated health endpoints.
// A kubeconfig or cluster credential is not needed to determine whether the
// configured endpoint can serve requests. Any HTTP response confirms that the
// endpoint is reachable; a successful health endpoint response is required for
// the liveness result to be healthy.
func checkClusterAPILiveness(ctx context.Context, apiURL, caBundle string) (clusterLivenessResult, error) {
	if ctx == nil {
		return clusterLivenessResult{}, fmt.Errorf("context must not be nil")
	}

	baseURL, err := url.Parse(apiURL)
	if err != nil || baseURL.Host == "" || (baseURL.Scheme != "https" && baseURL.Scheme != "http") {
		return clusterLivenessResult{}, fmt.Errorf("invalid cluster API URL %q", apiURL)
	}

	var tlsConfig *tls.Config
	if baseURL.Scheme == "https" {
		tlsConfig, err = clusterAPITLSConfig(caBundle)
		if err != nil {
			return clusterLivenessResult{}, fmt.Errorf("failed to configure cluster API TLS: %w", err)
		}
	}

	probeCtx, cancel := context.WithTimeout(ctx, clusterLivenessTimeout)
	defer cancel()

	transport := &http.Transport{
		Proxy:             http.ProxyFromEnvironment,
		TLSClientConfig:   tlsConfig,
		ForceAttemptHTTP2: true,
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return checkClusterAPILivenessWithClient(probeCtx, baseURL, client)
}

func checkClusterAPILivenessWithClient(
	ctx context.Context,
	baseURL *url.URL,
	client *http.Client,
) (clusterLivenessResult, error) {
	if ctx == nil {
		return clusterLivenessResult{}, fmt.Errorf("context must not be nil")
	}
	if baseURL == nil || baseURL.Host == "" || (baseURL.Scheme != "https" && baseURL.Scheme != "http") {
		return clusterLivenessResult{}, fmt.Errorf("invalid cluster API URL %v", baseURL)
	}
	result := clusterLivenessResult{}

	for _, healthPath := range clusterHealthEndpoints {
		endpointURL := *baseURL
		endpointURL.Path = strings.TrimRight(baseURL.Path, "/") + "/" + strings.TrimLeft(healthPath, "/")
		endpointURL.RawPath = ""
		endpointURL.RawQuery = ""
		endpointURL.Fragment = ""

		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpointURL.String(), nil)
		if err != nil {
			return result, fmt.Errorf("failed to create cluster API health request: %w", err)
		}

		response, err := client.Do(request)
		if err != nil {
			return result, fmt.Errorf("cluster API health check failed: %w", err)
		}
		result.Online = true
		statusCode := response.StatusCode
		if err := response.Body.Close(); err != nil {
			return result, fmt.Errorf("failed to close cluster API health response: %w", err)
		}

		if statusCode == http.StatusNotFound || statusCode == http.StatusMethodNotAllowed {
			continue
		}
		if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
			return result, fmt.Errorf("cluster API health endpoint %s returned HTTP %s", healthPath, response.Status)
		}
		result.Healthy = true
		return result, nil
	}

	return result, fmt.Errorf("cluster API does not support health endpoints %s", strings.Join(clusterHealthEndpoints, ", "))
}

func clusterAPITLSConfig(caBundle string) (*tls.Config, error) {
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("failed to load system root certificates: %w", err)
	}
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	if caBundle != "" {
		caPEM, err := base64.StdEncoding.DecodeString(caBundle)
		if err != nil {
			// ManagedCluster client configs serialize CA byte slices as base64,
			// while proxy CA data may already be provided as PEM text.
			caPEM = []byte(caBundle)
		}
		if !rootCAs.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("CA bundle contains no valid certificates")
		}
	}

	return &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: rootCAs}, nil
}
