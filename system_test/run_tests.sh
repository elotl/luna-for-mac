#!/bin/sh

. ./_common.sh

kctl() {
    if [ ! -x ./bin/kubectl ]; then
        die "ERROR: ./bin/kubectl is missing"
    fi
    ./bin/kubectl $@
}

# Used by cleanup. It doesn’t call kctl because we don’t want to exit when we
# execute this.
kubectl_delete_all() {
    if [ ! -x ./bin/kubectl ]; then
        log "WARNING: ./bin/kubectl is missing"
    else
        ./bin/kubectl delete --all $@
    fi
}

# If the environment variable PRINT_LOGS is set to yes, all the files with
# the extension .log will be printed on stdout.
cleanup() {
    # Cat log files to stdout if the job exited with an error
    local exitcode=$?
    if [ $exitcode != 0 -a "$PRINT_LOGS" = yes ]; then
        echo "Log files content:"
        for log_filename in ./*.log; do
            echo "--\n$log_filename\n--"
            cat "$log_filename"
        done
    fi

    set +e
    log "Cleaning up"
    for ns in $(kctl get namespaces | tail -n+2 | awk '{print $ 1}'); do
        kubectl_delete_all pods --namespace=$ns
        kubectl_delete_all deployments --namespace=$ns
        kubectl_delete_all services --namespace=$ns
        kubectl_delete_all daemonsets --namespace=$ns
    done
    if [ ! -z $CI ]; then
      echo "running in CI, ignoring further cleanups"
      exit $exitcode
    fi
    local pid
    log "Murdering all the daemons"
    for pid_file in ./*.pid ; do
        pid=$(cat "$pid_file")
        # XXX: why 2 kills?
        # prevent failure in cleanup failing GH action
        sudo pkill -KILL -P $pid
        sudo kill -9 $pid
        rm $pid_file
    done
    # Reap all the leftover processes before exiting
    wait
}
trap cleanup EXIT

# generates certs for server and client
generate_certs() {
    echo "Trying to generate certs..."
    openssl genrsa -out ca.key 2048
    openssl req -x509 -new -nodes -key ca.key -subj "/CN=127.0.0.1" -days 10000 -out ca.crt
    openssl genrsa -out server.key 2048
    openssl req -new -key server.key -out server.csr -config csr.conf
    openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key \
        -CAcreateserial -out server.crt -days 10000 \
        -extensions v3_ext -extfile csr.conf
    echo "certs: ca.crt, ca.key, server.key successfully generated."
}

readonly k8s_default_bin_url="https://elotl-maccri.s3.amazonaws.com/kubernetes-v1.18.16%2Betcd-v3.4.14-bin-darwin-amd64.tar.gz"
readonly k8s_bin_url="${K8S_BIN_URL:-$k8s_default_bin_url}"

# Download Kubernetes binaries and copy them to the ./bin/ directory
download_kubernetes_and_etcd_executables() {
    if [ ! -d bin ]; then
    	http_download "$k8s_bin_url"
    	tar zxf $(basename "$k8s_bin_url")
    else
        echo "Kubernetes already downloaded"
    fi
}

setup_kubernetes_control_plane() {
    if [ ! -r private.pem ]; then
        openssl genrsa -out private.pem
    fi

    kctl --kubeconfig kubeconfig.kube-proxy config set-cluster localhost --server https://localhost:6443
    kctl --kubeconfig kubeconfig.kube-proxy config set-context localhost --cluster localhost
    kctl --kubeconfig kubeconfig.kube-proxy config use-context localhost

    kctl config --kubeconfig kubeconfig.master set-cluster local --server=https://127.0.0.1:6443 --insecure-skip-tls-verify=true
    kctl config --kubeconfig kubeconfig.master set-credentials control-plane --client-certificate=ca.crt --client-key=ca.key
    kctl config --kubeconfig kubeconfig.master set-context default --cluster=local --user=control-plane
    kctl config --kubeconfig kubeconfig.master use-context default

	  local user=$(whoami)
    sudo mkdir -p /var/run/kubernetes
    sudo chown $user: /var/run/kubernetes
    sudo mkdir -p /var/lib/kubelet
    sudo chown $user: /var/lib/kubelet
}

start_kubernetes_control_plane() {
    rm -f ./etcd.log ./apiserver.log ./controller-manager.log ./scheduler.log
    ./bin/etcd \
    	--listen-client-urls=http://localhost:12379 \
    	--listen-peer-urls=http://localhost:12380 \
    	--advertise-client-urls=http://localhost:12379 \
    > ./etcd.log 2>&1 &
    echo "$!" > ./etcd.pid
    # service-account-issuer is fake
    ./bin/kube-apiserver \
        --etcd-servers http://localhost:12379 \
        --service-account-issuer https://localhost:9999 \
        --api-audiences localhost:9999 \
        --service-account-signing-key-file private.pem \
        --service-account-key-file private.pem \
        --client-ca-file=ca.crt \
        --tls-cert-file=server.crt \
        --tls-private-key-file=server.key \
        > ./apiserver.log 2>&1 &
    echo "$!" > ./kube-apiserver.pid
    ./bin/kube-controller-manager \
        --master https://localhost:6443 \
        --service-account-private-key-file=private.pem \
        --authentication-kubeconfig=kubeconfig.master \
        --kubeconfig=kubeconfig.master \
        --client-ca-file=ca.crt \
        --requestheader-client-ca-file=ca.crt \
        > ./controller-manager.log 2>&1 &
    echo "$!" > ./kube-controller-manager.pid
    ./bin/kube-scheduler \
        --master https://localhost:6443 \
        --authentication-kubeconfig=kubeconfig.master \
        --kubeconfig=kubeconfig.master \
        --client-ca-file=ca.crt \
        --requestheader-client-ca-file=ca.crt \
        > ./scheduler.log 2>&1 &
    echo "$!" > ./kube-scheduler.pid

    wait_for_port 6443
    wait_for_port 10259
}

readonly kubelet_default_bin_url="https://elotl-maccri.s3.amazonaws.com/kubelet-v1.18.15-34-g0c65647a6a3"
readonly kubelet_bin_url="${KUBELET_BIN_URL:-$kubelet_default_bin_url}"
readonly kubeproxy_default_bin_url="https://elotl-maccri.s3.amazonaws.com/kube-proxy-v1.21.0-alpha.1-1021-g1effa3faaa6"
readonly kubeproxy_bin_url="${KUBE_PROXY_BIN_URL:-$kubeproxy_default_bin_url}"
readonly script_dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

setup_kubernetes_node_components() {
  http_download $kubelet_bin_url
  http_download $kubeproxy_bin_url
  cp $(basename $kubelet_bin_url) $script_dir/bin/kubelet
  cp $(basename $kubeproxy_bin_url) $script_dir/bin/kube-proxy
  chmod +x $script_dir/bin/kubelet
  chmod +x $script_dir/bin/kube-proxy
}

start_kubernetes_node_components() {
    local procri_dir=$script_dir/../procri
    local procri_sock="/Users/$USER/procri.sock"
    make -C $procri_dir procri-darwin-amd64
    echo "copying procri to $script_dir/bin/procri"
    cp $procri_dir/procri-darwin-amd64 $script_dir/bin/procri
    echo "ls $script_dir/bin/"
    ls $script_dir/bin
    echo "starting procri"
    sudo ./bin/procri --listen "$procri_sock" --v=5 > procri.log 2>&1 &
    echo "$!" > ./procri.pid
    echo "started procri"
    kctl --kubeconfig kubeconfig.kubelet config set-cluster localhost --server https://localhost:6443 --insecure-skip-tls-verify=true
    kctl --kubeconfig kubeconfig.kubelet config set-credentials $(hostname) --client-certificate=ca.crt --client-key=ca.key
    kctl --kubeconfig kubeconfig.kubelet config set-context localhost --cluster localhost --user=$(hostname)
    kctl --kubeconfig kubeconfig.kubelet config use-context localhost

    # Start kubelet.
    echo "starting kubelet"
    sudo ./bin/kubelet --kubeconfig kubeconfig.kubelet \
        --cgroups-per-qos=false \
        --config=kubelet-config.json \
        --fail-swap-on=false \
         -v=5 \
        --container-runtime=remote \
        --feature-gates VolumeSubpath=false \
        --container-runtime-endpoint=unix://$procri_sock \
        --client-ca-file=ca.crt  > ./kubelet.log 2>&1 &
    echo "$!" > ./kubelet.pid
    echo "started kubelet"


    max_retries=0
    while true
    do
        echo "Time Now: `date +%H:%M:%S`"
        echo "Sleeping for 10 seconds"
        # Here 300 is 300 seconds i.e. 5 minutes * 60 = 300 sec
        max_retries=$(($max_retries+1))
        echo "checking node status..."
        nodeStatus=$(kctl --kubeconfig kubeconfig.kubelet get nodes -o jsonpath='{.items[].status.conditions[-1].status}')
        if [ $nodeStatus == "True" ];then
           echo "Node is ready"
           echo "untainting node"
           kctl --kubeconfig kubeconfig.kubelet taint nodes $(hostname) node.kubernetes.io/not-ready:NoSchedule-
           return
        else
           echo "Node not ready yet..."
           sleep 5
        fi
        if [ $max_retries -gt 10 ];then
          break
        fi
    done
    echo "Waiting timeout for node ready exceeded"
    echo "\n kubelet logs:\n"
    cat kubelet.log
    echo "\n procri.logs:\n"
    cat procri.log
    die "Node not ready"
}

download_kubernetes_and_etcd_executables
generate_certs
setup_kubernetes_control_plane
start_kubernetes_control_plane
setup_kubernetes_node_components
start_kubernetes_node_components


