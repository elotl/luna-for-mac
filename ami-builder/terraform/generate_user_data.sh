#!/bin/sh

readonly name=$1; shift
readonly region=$1; shift

describe_cluster() {
    aws eks describe-cluster \
            --region="$region" \
            --name="$name" \
            --output=text \
            --query="cluster.{$*}"
}

readonly b64_cluster_ca="$(describe_cluster 'certificateAuthorityData: certificateAuthority.data' | base64 -d)"
readonly apiserver_endpoint="$(describe_cluster 'endpoint: endpoint')"

cat <<EOF
#!/bin/sh

mkdir -p /etc/kube/pki

cat <<_EOF > /etc/kube/pki/ca.crt
${b64_cluster_ca}
_EOF

cat <<_EOF > /etc/kube/kubeconfig.yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority: /etc/kube/pki/ca.crt
    server: ${apiserver_endpoint}
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
        - "${name}"
        - --region
        - "${region}"
_EOF
EOF
