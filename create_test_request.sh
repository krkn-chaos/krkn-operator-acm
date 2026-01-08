#!/bin/bash
# Helper script to create a test KrknTargetRequest
# Generated-by: Claude Sonnet 4.5 (claude-sonnet-4-5@20250929)

export KUBECONFIG=/Users/tsebasti/Scrap/acm-hub-krkn/auth/kubeconfig

# Generate or use provided UUID
UUID="${1:-test-$(date +%s)}"

# Operator namespace - change this if your operator runs in a different namespace
NAMESPACE="${2:-krkn-operator-system}"

echo "Creating KrknTargetRequest with UUID: $UUID in namespace: $NAMESPACE"
echo ""

kubectl apply -f - <<EOF
apiVersion: krkn.krkn-chaos.dev/v1alpha1
kind: KrknTargetRequest
metadata:
  name: request-${UUID}
  namespace: ${NAMESPACE}
spec:
  uuid: "${UUID}"
EOF

echo ""
echo "✅ Request created!"
echo ""
echo "Useful commands:"
echo "  # Watch all requests in operator namespace"
echo "  kubectl get ktr -n ${NAMESPACE} -w"
echo ""
echo "  # Get this specific request by UUID"
echo "  kubectl get ktr -n ${NAMESPACE} -l krkn.krkn-chaos.dev/uuid=${UUID}"
echo ""
echo "  # Get the request details"
echo "  kubectl get ktr request-${UUID} -n ${NAMESPACE} -o yaml"
echo ""
echo "  # Get the secret with cluster data (multi-operator format)"
echo "  kubectl get secret ${UUID} -n ${NAMESPACE} -o jsonpath='{.data.managed-clusters}' | base64 -d | jq ."
echo ""
echo "  # Get cluster data for a specific operator"
echo "  kubectl get secret ${UUID} -n ${NAMESPACE} -o jsonpath='{.data.managed-clusters}' | base64 -d | jq '.\"krkn-operator-acm\"'"
echo ""
