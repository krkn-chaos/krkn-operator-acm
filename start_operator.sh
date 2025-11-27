#!/bin/bash
# Start the krkn-operator-acm
# Generated-by: Claude Sonnet 4.5 (claude-sonnet-4-5@20250929)

export KUBECONFIG=/Users/tsebasti/Scrap/acm-hub-krkn/auth/kubeconfig

echo "========================================="
echo "Starting krkn-operator-acm"
echo "========================================="
echo "Kubeconfig: $KUBECONFIG"
echo ""
echo "The operator will start and watch for KrknTargetRequest resources."
echo "Press Ctrl+C to stop"
echo "========================================="
echo ""

# Run the operator using the prebuilt binary if it exists, otherwise use go run
if [ -f "./bin/manager" ]; then
    echo "Using prebuilt binary: ./bin/manager"
    ./bin/manager
else
    echo "Building and running with go run..."
    go run ./cmd/main.go
fi