#!/bin/sh

if [ $(whoami) != root ]; then
    echo "This install script require root permissions"
    exit 1
fi

set -e

readonly bin_install_dir=/usr/local/bin
readonly plist_dir=/Library/LaunchDaemons

# Install binaries in /usr/local/bin
# Usage: install URL [EXECUTABLE FILENAME]
download_bin() {
    local url=$1; shift
    local name=$1
    : ${name:=$(basename "$url")}

    if [ ! -x "$bin_install_dir/$name" ]; then
        curl -L --progress-bar "$url" -o "$bin_install_dir/$name"
        chmod +x "$bin_install_dir/$name"
    fi
}

service_plist_filename() {
    echo "$plist_dir/$@.plist"
}

# Stop & Unload the old service to avoid leaking system artifacts when we
# change the plist.
stop_service() {
    local service_name=$1
    local plist_filename=$(service_plist_filename "$service_name")
    if [ -r "$plist_filename" ]; then
        launchctl unload "$plist_filename" || true
    fi
}

start_service() {
    local service_name=$1
    local plist_filename=$(service_plist_filename "$service_name")
    launchctl load "$plist_filename"
}

install_kubeconfig() {
    local DESCRIBE_CLUSTER_RESULT="/tmp/describe_cluster_result.txt"
    local CA_CERTIFICATE_FILE_PATH="/etc/kube/pki/ca.crt"
    aws eks describe-cluster \
            --region=${CLUSTER_REGION} \
            --name=${CLUSTER_NAME} \
            --output=text \
            --query 'cluster.{certificateAuthorityData: certificateAuthority.data, endpoint: endpoint, kubernetesNetworkConfig: kubernetesNetworkConfig.serviceIpv4Cidr}' > $DESCRIBE_CLUSTER_RESULT || rc=$
    local B64_CLUSTER_CA=$(cat $DESCRIBE_CLUSTER_RESULT | awk '{print $1}')
    local APISERVER_ENDPOINT=$(cat $DESCRIBE_CLUSTER_RESULT | awk '{print $2}')
    echo $B64_CLUSTER_CA | base64 -d > $CA_CERTIFICATE_FILE_PATH
    cat <<EOF > /etc/kube/kubeconfig.yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
  certificate-authority: $CA_CERTIFICATE_FILE_PATH
  server: $APISERVER_ENDPOINT
name: kubernetes
contexts:
- context:
  cluster: kubernetes
  user: kubelet
name: kubelet
current-context: kubelet
users:
- name: kubelet
user:
  exec:
    apiVersion: client.authentication.k8s.io/v1alpha1
    command: /usr/local/bin/aws-iam-authenticator
    args:
      - "token"
      - "-i"
      - "$CLUSTER_NAME"
      - --region
      - "$CLUSTER_REGION"
EOF
}

install_procri() {
    local procri_url=https://procri-dev.s3.amazonaws.com/procri-0d3f7ef
    local procri_service_name=com.elotl.procri

    download_bin "$procri_url" procri

    stop_service "$procri_service_name"

    # Write plist file
    cat <<EOF > "$(service_plist_filename $procri_service_name)"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
        <key>Label</key>
        <string>com.elotl.procri</string>
        <key>UserName</key>
        <string>root</string>
        <key>ProgramArguments</key>
        <array>
            <string>sudo</string>
            <string>/usr/local/bin/procri</string>
            <string>--v</string>
            <string>3</string>
        </array>
        <key>RunAtLoad</key>
        <true/>
        <key>KeepAlive</key>
        <dict>
            <key>SuccessfulExit</key>
            <false/>
        </dict>
        <key>StandardErrorPath</key>
        <string>/var/log/procri.log</string>
        <key>StandardOutPath</key>
        <string>/var/log/procri.log</string>
</dict>
</plist>
EOF

    start_service "$procri_service_name" || {
        echo "Couldn't start procri"
        exit 1
    }
}

install_kubelet() {
    readonly kubelet_url=https://procri-dev.s3.amazonaws.com/kubelet
    readonly kubelet_service_name="com.kubernetes.kubelet"

    download_bin "$kubelet_url" kubelet

    stop_service "$kubelet_service_name"

    cat <<EOF > "$(service_plist_filename $kubelet_service_name)"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
        <key>Label</key>
        <string>com.kubernetes.kubelet</string>
        <key>UserName</key>
        <string>root</string>
        <key>ProgramArguments</key>
        <array>
            <string>sudo</string>
            <string>/usr/local/bin/kubelet</string>
            <string>--cloud-provider</string>
            <string>aws</string>
            <string>--kubeconfig</string>
            <string>/etc/kube/kubeconfig.yaml</string>
            <string>--fail-swap-on=false</string>
            <string>-v=5</string>
            <string>--container-runtime</string>
            <string>remote</string>
            <string>--feature-gates</string>
            <string>VolumeSubpath=false</string>
            <string>--cgroups-per-qos=false</string>
            <string>--container-runtime-endpoint</string>
            <string>unix:///var/run/procri.sock</string>
            <string>--config</string>
            <string>/etc/kube/kubelet-config.json</string>
        </array>
        <key>RunAtLoad</key>
        <true/>
        <key>KeepAlive</key>
        <dict>
            <key>SuccessfulExit</key>
            <false/>
        </dict>
        <key>EnvironmentVariables</key>
        <dict>
            <key>PATH</key>
            <string>/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin</string>
        </dict>
        <key>StandardErrorPath</key>
        <string>/var/log/kubelet.log</string>
        <key>StandardOutPath</key>
        <string>/var/log/kubelet.log</string>
</dict>
</plist>
EOF

    start_service "$kubelet_service_name" || {
        echo "Couldn't start kubelet"
        exit 1
    }
}

install_kube_proxy() {
    readonly kube_proxy_url=https://procri-dev.s3.amazonaws.com/kube-proxy
    readonly kube_proxy_service_name="com.kubernetes.kube-proxy"

    download_bin "$kube_proxy_url" kube-proxy

    stop_service "$kube_proxy_service_name"

    cat <<EOF > "$(service_plist_filename $kube_proxy_service_name)"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
        <key>Label</key>
        <string>com.kubernetes.kube-proxy</string>
        <key>UserName</key>
        <string>root</string>
        <key>ProgramArguments</key>
        <array>
            <string>/usr/local/bin/kube-proxy</string>
            <string>--kubeconfig</string>
            <string>/etc/kube/kubeconfig.yaml</string>
            <string>-v=5</string>
        </array>
        <key>RunAtLoad</key>
        <true/>
        <key>KeepAlive</key>
        <dict>
            <key>SuccessfulExit</key>
            <false/>
        </dict>
        <key>StandardErrorPath</key>
        <string>/var/log/kube-proxy.log</string>
        <key>StandardOutPath</key>
        <string>/var/log/kube-proxy.log</string>
</dict>
</plist>
EOF

    start_service "$kube_proxy_service_name" || {
        echo "Couldn't start kube-proxy"
        exit 1
    }
}

if [ -z "$CLUSTER_NAME" ]; then
    echo "CLUSTER_NAME is not defined"
    exit  1
fi

if [ -z "$CLUSTER_REGION" ]; then
    echo "CLUSTER_REGION is not defined"
    exit  1
fi

install_kubeconfig
install_procri
install_kubelet
install_kube_proxy

cat <<EOF
To finish setting-up the services copy the configuration files to /etc/kube.
EOF
