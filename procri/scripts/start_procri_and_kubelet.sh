#!/bin/bash

set -euxo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT_DIR=$SCRIPT_DIR/..
PROCRI_SOCK="/Users/$USER/procri.sock"

function cleanup() {
	echo "killing procri & kubelet"
	local pid
	for pid_file in ./*.pid ; do
		pid=$(cat "$pid_file")
		sudo pkill -KILL -P $pid
		sudo kill -9 $pid
		rm $pid_file
	done
	wait	
}

pushd $ROOT_DIR
make

trap cleanup EXIT

./procri --listen "$PROCRI_SOCK" --v=5 > procri.log 2>&1 &
echo "$!" > ./procri.pid
sleep 2
# wait_for_file $PROCRI_SOCK
popd

kubectl --kubeconfig kubeconfig.kubelet config set-cluster localhost --server http://localhost:8080
kubectl --kubeconfig kubeconfig.kubelet config set-context localhost --cluster localhost
kubectl --kubeconfig kubeconfig.kubelet config use-context localhost

# Start kubelet.
sudo ./kubelet --kubeconfig $SCRIPT_DIR/kubeconfig.kubelet --fail-swap-on=false -v=5 --container-runtime=remote --feature-gates VolumeSubpath=false --container-runtime-endpoint=unix://$PROCRI_SOCK > $ROOT_DIR/kubelet.log 2>&1 &
echo "$!" > ./kubelet.pid
echo "Started all services"
read _
