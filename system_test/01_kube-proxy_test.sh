set +e

# XXX: We assume kube-proxy was copied from somewhere...
if [ ! -x ./bin/kube-proxy ]; then
        echo "ls ./bin/"
        ls ./bin/
	die "Kube-proxy not found"
fi

# XXX: Find a better way to determine default interface
export INTERFACE_TO_ADD_SERVICE_IP=en0

if ifconfig | egrep '^\tinet 10.0.0.1'; then
    log "Remove leftover VIP"
    sudo ifconfig "$INTERFACE_TO_ADD_SERVICE_IP" -alias 10.0.0.1
fi

# Create node in kube
./bin/kubectl apply -f - <<EOF
{
  "kind": "Node",
  "apiVersion": "v1",
  "metadata": {
    "name": "kubeproxynode"
  }
}
EOF

readonly kubeconfig=./kube-proxy_kubeconfig.yaml
./bin/kubectl config view > "$kubeconfig"

sudo ./bin/kube-proxy -v=5 --hostname-override kubeproxynode --kubeconfig "$kubeconfig" &
echo $! > ./kube-proxy.pid

wait_for_port 6443 10.0.0.1 || die "Can't reach apiserver"

# Verify the VIP was added
if ! ifconfig | egrep '^\tinet 10.0.0.1'; then
    die "VIP not found"
fi

# Verify we can connect to the data-plane’s HTTPS server
curl -vk https://10.0.0.1/ || die "Unable to connect to apiserver through kube-proxy"
kill -INT $(cat kube-proxy.pid)

# XXX: kube-proxy doesn’t cleanup its VIP when it terminates :-(
if ifconfig | egrep '^\tinet 10.0.0.1'; then
    log "WARNING: Removing leftover VIP 10.0.0.1"
    sudo ifconfig "$INTERFACE_TO_ADD_SERVICE_IP" -alias 10.0.0.1
fi

set -e
