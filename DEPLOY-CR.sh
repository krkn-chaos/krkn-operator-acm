export KUBECONFIG=/Users/tsebasti/Scrap/acm-hub-krkn/auth/kubeconfig
UUID="$(date +%s)"
kubectl apply -f - <<EOF
  apiVersion: krkn.krkn-chaos.dev/v1alpha1
  kind: KrknTargetRequest
  metadata:
    name: my-request
  spec:
    uuid: "$UUID"
EOF

