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
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

func TestClusterAPITLSConfigUsesOnlyExplicitProxyCA(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test CA key: %v", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-proxy-ca"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	certificate, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, publicKey, privateKey)
	if err != nil {
		t.Fatalf("failed to generate test CA certificate: %v", err)
	}
	parsedCA, err := x509.ParseCertificate(certificate)
	if err != nil {
		t.Fatalf("failed to parse test CA certificate: %v", err)
	}

	caBundle := base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certificate,
	}))
	tlsConfig, err := clusterAPITLSConfig(caBundle)
	if err != nil {
		t.Fatalf("clusterAPITLSConfig() error = %v", err)
	}
	if tlsConfig.RootCAs == nil {
		t.Fatal("RootCAs is nil")
	}

	leafPublicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test leaf key: %v", err)
	}
	leafCertificate, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "proxy.example"},
		DNSNames:     []string{"proxy.example"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}, parsedCA, leafPublicKey, privateKey)
	if err != nil {
		t.Fatalf("failed to generate test leaf certificate: %v", err)
	}
	parsedLeaf, err := x509.ParseCertificate(leafCertificate)
	if err != nil {
		t.Fatalf("failed to parse test leaf certificate: %v", err)
	}
	if _, err := parsedLeaf.Verify(x509.VerifyOptions{
		Roots:         tlsConfig.RootCAs,
		DNSName:       "proxy.example",
		Intermediates: x509.NewCertPool(),
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		t.Fatalf("explicit proxy CA did not verify its leaf certificate: %v", err)
	}

	otherPublicKey, otherPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate unrelated CA key: %v", err)
	}
	otherCA, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber:          big.NewInt(3),
		Subject:               pkix.Name{CommonName: "unrelated-ca"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}, &x509.Certificate{
		SerialNumber:          big.NewInt(3),
		Subject:               pkix.Name{CommonName: "unrelated-ca"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}, otherPublicKey, otherPrivateKey)
	if err != nil {
		t.Fatalf("failed to generate unrelated CA certificate: %v", err)
	}
	otherParsedCA, err := x509.ParseCertificate(otherCA)
	if err != nil {
		t.Fatalf("failed to parse unrelated CA certificate: %v", err)
	}
	otherLeafKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate unrelated leaf key: %v", err)
	}
	otherLeaf, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber: big.NewInt(4),
		Subject:      pkix.Name{CommonName: "unrelated.example"},
		DNSNames:     []string{"unrelated.example"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}, otherParsedCA, otherLeafKey, otherPrivateKey)
	if err != nil {
		t.Fatalf("failed to generate unrelated leaf certificate: %v", err)
	}
	parsedOtherLeaf, err := x509.ParseCertificate(otherLeaf)
	if err != nil {
		t.Fatalf("failed to parse unrelated leaf certificate: %v", err)
	}
	if _, err := parsedOtherLeaf.Verify(x509.VerifyOptions{
		Roots:     tlsConfig.RootCAs,
		DNSName:   "unrelated.example",
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err == nil {
		t.Fatal("explicit proxy CA pool trusted an unrelated CA")
	}
}
