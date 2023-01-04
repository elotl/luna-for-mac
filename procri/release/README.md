# Install script for the Mac kubelet stack

To install the kubelet on a Mac node copy the file install.sh to the node. Then
run the script as root:

    $ sudo bash -ex ./install.sh
    ...
    To finish setting-up the services copy the configuration files to /etc/kube
    $

This will download kubelet, kueb-proxy, and procri on the node and set-up the
appropriate plist files to automatically start the services on boot.

Both kube-proxy and kubelet use the kubeconfig at: /etc/kube/kubeconfig.yaml

# Create a new release of ProCRI

To upload the current version of procri as `procri-<commit hash>` to S3:

    $ cd mac-cri/procri
    $ sh -ex ./release/build_and_upload.sh

The binary will be available at:
`https://procri-dev.s3.amazonaws.com/procri-<commit has>`

# TODO

How are we going to manage non-kubeconfig configuration? We should probably have
a single kubeconfig here’s what I’m thinking about:

    /etc/kube <- directory that contains the kubelet daemons configuration
    /etc/kube/kubeconfig.yaml <- the common kubeconfig for kubelet & kube-proxy
    /etc/kube/kubelet.yaml
    /etc/kube/kube-proxy.yaml
    /etc/kube/procri.yaml
