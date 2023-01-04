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

install_aws_iam_authenticator() {
  download_bin "https://github.com/kubernetes-sigs/aws-iam-authenticator/releases/download/v0.5.2/aws-iam-authenticator_0.5.2_darwin_amd64" aws-iam-authenticator
}

install_kubeconfig() {
    local DESCRIBE_CLUSTER_RESULT="/tmp/describe_cluster_result.txt"
    local CA_CERTIFICATE_FILE_PATH="/etc/kube/pki/ca.crt"
    mkdir -p $(dirname "$CA_CERTIFICATE_FILE_PATH")
    aws eks describe-cluster \
            --region=${cluster_region} \
            --name=${cluster_name} \
            --output=text \
            --query 'cluster.{certificateAuthorityData: certificateAuthority.data, endpoint: endpoint, kubernetesNetworkConfig: kubernetesNetworkConfig.serviceIpv4Cidr}' > $DESCRIBE_CLUSTER_RESULT
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
        - "${cluster_name}"
        - --region
        - "${cluster_region}"
EOF
}

install_procri() {
    local procri_url=${procri_url}
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

install_kubelet_config() {
    local kubelet_config='/etc/kube/kubelet-config.json'
    echo "Installing $kubelet_config"
    mkdir -p $(dirname "$kubelet_config")
    cat <<EOF > "$kubelet_config"
{
  "kind": "KubeletConfiguration",
  "apiVersion": "kubelet.config.k8s.io/v1beta1",
  "address": "0.0.0.0",
  "authentication": {
    "anonymous": {
      "enabled": false
    },
    "webhook": {
      "cacheTTL": "2m0s",
      "enabled": true
    },
    "x509": {
      "clientCAFile": "/etc/kube/pki/ca.crt"
    }
  },
  "authorization": {
    "mode": "Webhook",
    "webhook": {
      "cacheAuthorizedTTL": "5m0s",
      "cacheUnauthorizedTTL": "30s"
    }
  },
  "clusterDomain": "cluster.local",
  "hairpinMode": "hairpin-veth",
  "readOnlyPort": 0,
  "cgroupDriver": "cgroupfs",
  "cgroupsPerQOS": false,
  "enforceNodeAllocatable": [],
  "featureGates": {
    "RotateKubeletServerCertificate": true
  },
  "protectKernelDefaults": true,
  "serializeImagePulls": false,
  "serverTLSBootstrap": true,
  "tlsCipherSuites": ["TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305", "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384", "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384", "TLS_RSA_WITH_AES_256_GCM_SHA384", "TLS_RSA_WITH_AES_128_GCM_SHA256"]
}
EOF
}

install_kubelet() {
    readonly kubelet_url=${kubelet_url}
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
            <string>--max-pods=1</string>
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
    readonly kube_proxy_url=${kube_proxy_url}
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

if [ -z "${cluster_name}" ]; then
    echo "CLUSTER_NAME is not defined"
    exit  1
fi

if [ -z "${cluster_region}" ]; then
    echo "CLUSTER_REGION is not defined"
    exit  1
fi

install_aws_iam_authenticator
install_kubelet_config
install_kubeconfig
install_procri
install_kubelet
install_kube_proxy
