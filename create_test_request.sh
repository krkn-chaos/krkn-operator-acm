#!/bin/bash
# Helper script to create a test KrknTargetRequest
# Generated-by: Claude Sonnet 4.5 (claude-sonnet-4-5@20250929)

export KUBECONFIG=/Users/tsebasti/Scrap/acm-hub-krkn/auth/kubeconfig

# Generate or use provided UUID
UUID="${1:-test-$(date +%s)}"

echo "Creating KrknTargetRequest with UUID: $UUID"
echo ""

kubectl apply -f - <<EOF
apiVersion: krkn.krkn-chaos.dev/v1alpha1
kind: KrknTargetRequest
metadata:
  name: request-${UUID}
spec:
  uuid: "${UUID}"
EOF

echo ""
echo "✅ Request created!"
echo ""
echo "Useful commands:"
echo "  # Watch all requests"
echo "  kubectl get ktr -w"
echo ""
echo "  # Get this specific request by UUID"
echo "  kubectl get ktr -l krkn.krkn-chaos.dev/uuid=${UUID}"
echo ""
echo "  # Get the request details"
echo "  kubectl get ktr request-${UUID} -o yaml"
echo ""
echo "  # Get the secret with cluster data"
echo "  kubectl get secret ${UUID} -o jsonpath='{.data.managed-clusters}' | base64 -d | jq ."
echo ""
