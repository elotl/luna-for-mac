#!/usr/bin/env bash

# copied from https://github.com/awslabs/amazon-eks-ami/blob/master/files/bootstrap.sh
# TODO: change everything linux specific to mac alternatives
# Warning (02/23, Pawel): I haven't tested this script yet, don't use it.


set -o pipefail
set -o nounset
set -o errexit

err_report() {
    echo "Exited with error on line $1"
}
trap 'err_report $LINENO' ERR

IFS=$'\n\t'

function print_help {
    # TODO: remove not needed flags (we probably need only apiserver-ednpoint and b64-cluster-ca
    echo "usage: $0 [options] <cluster-name>"
    echo "Bootstraps an instance into an EKS cluster"
    echo ""
    echo "-h,--help print this help"
    echo "--b64-cluster-ca The base64 encoded cluster CA content. Only valid when used with --apiserver-endpoint. Bypasses calling \"aws eks describe-cluster\""
    echo "--apiserver-endpoint The EKS cluster API Server endpoint. Only valid when used with --b64-cluster-ca. Bypasses calling \"aws eks describe-cluster\""
    echo "--kubelet-extra-args Extra arguments to add to the kubelet. Useful for adding labels or taints."
    echo "--aws-api-retry-attempts Number of retry attempts for AWS API call (DescribeCluster) (default: 3)"
    echo "--dns-cluster-ip Overrides the IP address to use for DNS queries within the cluster. Defaults to 10.100.0.10 or 172.20.0.10 based on the IP address of the primary interface"
}

POSITIONAL=()

while [[ $# -gt 0 ]]; do
    key="$1"
    case $key in
        -h|--help)
            print_help
            exit 1
            ;;
        --b64-cluster-ca)
            B64_CLUSTER_CA=$2
            shift
            shift
            ;;
        --apiserver-endpoint)
            APISERVER_ENDPOINT=$2
            shift
            shift
            ;;
        --kubelet-extra-args)
            KUBELET_EXTRA_ARGS=$2
            shift
            shift
            ;;
        --aws-api-retry-attempts)
            API_RETRY_ATTEMPTS=$2
            shift
            shift
            ;;
        --dns-cluster-ip)
            DNS_CLUSTER_IP=$2
            shift
            shift
            ;;
        *)    # unknown option
            POSITIONAL+=("$1") # save it in an array for later
            shift # past argument
            ;;
    esac
done

set +u
set -- "${POSITIONAL[@]}" # restore positional parameters
CLUSTER_NAME="$1"
set -u

B64_CLUSTER_CA="${B64_CLUSTER_CA:-}"
APISERVER_ENDPOINT="${APISERVER_ENDPOINT:-}"
DNS_CLUSTER_IP="${DNS_CLUSTER_IP:-}"
KUBELET_EXTRA_ARGS="${KUBELET_EXTRA_ARGS:-}"
API_RETRY_ATTEMPTS="${API_RETRY_ATTEMPTS:-3}"

function _get_token() {
  local token_result=
  local http_result=

  token_result=$(curl -s -w "\n%{http_code}" -X PUT -H "X-aws-ec2-metadata-token-ttl-seconds: 600" "http://169.254.169.254/latest/api/token")
  http_result=$(echo "$token_result" | tail -n 1)
  if [[ "$http_result" != "200" ]]
  then
      echo -e "Failed to get token:\n$token_result"
      return 1
  else
      echo "$token_result" | head -n 1
      return 0
  fi
}

function get_token() {
  local token=
  local retries=20
  local result=1

  while [[ retries -gt 0 && $result -ne 0 ]]
  do
    retries=$[$retries-1]
    token=$(_get_token)
    result=$?
    [[ $result != 0 ]] && sleep 5
  done
  [[ $result == 0 ]] && echo "$token"
  return $result
}

function _get_meta_data() {
  local path=$1
  local metadata_result=

  metadata_result=$(curl -s -w "\n%{http_code}" -H "X-aws-ec2-metadata-token: $TOKEN" http://169.254.169.254/$path)
  http_result=$(echo "$metadata_result" | tail -n 1)
  if [[ "$http_result" != "200" ]]
  then
      echo -e "Failed to get metadata:\n$metadata_result\nhttp://169.254.169.254/$path\n$TOKEN"
      return 1
  else
      local lines=$(echo "$metadata_result" | wc -l)
      echo "$metadata_result" | head -n $(( lines - 1 ))
      return 0
  fi
}

function get_meta_data() {
  local metadata=
  local path=$1
  local retries=20
  local result=1

  while [[ retries -gt 0 && $result -ne 0 ]]
  do
    retries=$[$retries-1]
    metadata=$(_get_meta_data $path)
    result=$?
    [[ $result != 0 ]] && TOKEN=$(get_token)
  done
  [[ $result == 0 ]] && echo "$metadata"
  return $result
}


if [ -z "$CLUSTER_NAME" ]; then
    echo "CLUSTER_NAME is not defined"
    exit  1
fi


TOKEN=$(get_token)
AWS_DEFAULT_REGION=$(get_meta_data 'latest/dynamic/instance-identity/document' | jq .region -r)

MACHINE=$(uname -m)
if [[ "$MACHINE" != "x86_64" && "$MACHINE" != "aarch64" ]]; then
    echo "Unknown machine architecture '$MACHINE'" >&2
    exit 1
fi


### kubelet kubeconfig

CA_CERTIFICATE_DIRECTORY=/etc/kube/pki
CA_CERTIFICATE_FILE_PATH=$CA_CERTIFICATE_DIRECTORY/ca.crt
mkdir -p $CA_CERTIFICATE_DIRECTORY
if [[ -z "${B64_CLUSTER_CA}" ]] || [[ -z "${APISERVER_ENDPOINT}" ]]; then
    DESCRIBE_CLUSTER_RESULT="/tmp/describe_cluster_result.txt"

    # Retry the DescribeCluster API for API_RETRY_ATTEMPTS
    for attempt in `seq 0 $API_RETRY_ATTEMPTS`; do
        rc=0
        if [[ $attempt -gt 0 ]]; then
            echo "Attempt $attempt of $API_RETRY_ATTEMPTS"
        fi

        aws eks wait cluster-active \
            --region=${AWS_DEFAULT_REGION} \
            --name=${CLUSTER_NAME}

        aws eks describe-cluster \
            --region=${AWS_DEFAULT_REGION} \
            --name=${CLUSTER_NAME} \
            --output=text \
            --query 'cluster.{certificateAuthorityData: certificateAuthority.data, endpoint: endpoint, kubernetesNetworkConfig: kubernetesNetworkConfig.serviceIpv4Cidr}' > $DESCRIBE_CLUSTER_RESULT || rc=$?
        if [[ $rc -eq 0 ]]; then
            break
        fi
        if [[ $attempt -eq $API_RETRY_ATTEMPTS ]]; then
            exit $rc
        fi
        jitter=$((1 + RANDOM % 10))
        sleep_sec="$(( $(( 5 << $((1+$attempt)) )) + $jitter))"
        sleep $sleep_sec
    done
    B64_CLUSTER_CA=$(cat $DESCRIBE_CLUSTER_RESULT | awk '{print $1}')
    APISERVER_ENDPOINT=$(cat $DESCRIBE_CLUSTER_RESULT | awk '{print $2}')
    SERVICE_IPV4_CIDR=$(cat $DESCRIBE_CLUSTER_RESULT | awk '{print $3}')
fi

echo $B64_CLUSTER_CA | base64 -d > $CA_CERTIFICATE_FILE_PATH

sed -i s,CLUSTER_NAME,$CLUSTER_NAME,g /etc/kube/kubeconfig
sed -i s,MASTER_ENDPOINT,$APISERVER_ENDPOINT,g /etc/kube/kubeconfig
sed -i s,AWS_REGION,$AWS_DEFAULT_REGION,g /etc/kube/kubeconfig
### kubelet.service configuration

if [[ -z "${DNS_CLUSTER_IP}" ]]; then
  if [[ ! -z "${SERVICE_IPV4_CIDR}" ]] && [[ "${SERVICE_IPV4_CIDR}" != "None" ]] ; then
    #Sets the DNS Cluster IP address that would be chosen from the serviceIpv4Cidr. (x.y.z.10)
    DNS_CLUSTER_IP=${SERVICE_IPV4_CIDR%.*}.10
  else
    MAC=$(get_meta_data 'latest/meta-data/network/interfaces/macs/' | head -n 1 | sed 's/\/$//')
    TEN_RANGE=$(get_meta_data "latest/meta-data/network/interfaces/macs/$MAC/vpc-ipv4-cidr-blocks" | grep -c '^10\..*' || true )
    DNS_CLUSTER_IP=10.100.0.10
    if [[ "$TEN_RANGE" != "0" ]]; then
      DNS_CLUSTER_IP=172.20.0.10
    fi
  fi
else
  DNS_CLUSTER_IP="${DNS_CLUSTER_IP}"
fi

KUBELET_CONFIG=/etc/kubernetes/kubelet/kubelet-config.json
echo "$(jq ".clusterDNS=[\"$DNS_CLUSTER_IP\"]" $KUBELET_CONFIG)" > $KUBELET_CONFIG

INTERNAL_IP=$(get_meta_data 'latest/meta-data/local-ipv4')


mkdir -p /etc/systemd/system/kubelet.service.d

cat <<EOF > /etc/systemd/system/kubelet.service.d/10-kubelet-args.conf
[Service]
Environment='KUBELET_ARGS=--node-ip=$INTERNAL_IP --pod-infra-container-image=$PAUSE_CONTAINER'
EOF

if [[ -n "$KUBELET_EXTRA_ARGS" ]]; then
    cat <<EOF > /etc/systemd/system/kubelet.service.d/30-kubelet-extra-args.conf
[Service]
Environment='KUBELET_EXTRA_ARGS=$KUBELET_EXTRA_ARGS'
EOF
fi

# TODO: use launchctl
systemctl daemon-reload
systemctl enable kubelet
systemctl start kubelet
