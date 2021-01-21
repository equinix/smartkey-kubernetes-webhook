#!/usr/bin/env bash

set -euo pipefail

basedir="$(dirname "$0")/deployment"
keydir="$(mktemp -d)"

echo "Generating TLS keys in ${keydir} ..."
"${basedir}/generate-keys.sh" "$keydir"

echo "Creating Kubernetes objects ..."
kubectl apply -f "${basedir}/namespace.yaml"

kubectl -n smartkey-vault create secret tls webhook-server-tls \
    --cert "${keydir}/webhook-server-tls.crt" \
    --key "${keydir}/webhook-server-tls.key"

ca_pem_b64="$(openssl base64 -A <"${keydir}/ca.crt")"
sed -e 's@${CA_PEM_B64}@'"$ca_pem_b64"'@g' <"${basedir}/webhook-server.yaml" \
    | kubectl apply -f -

rm -rf "$keydir"

echo "The SmartKey webhook server has been deployed and configured!"