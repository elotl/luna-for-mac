# Luna For Mac

A software stack to attach Intel Mac nodes to Kubernetes. With Luna For Mac
you can attach Mac compute to your Kubernetes clusters and run Mac workload
on them.

# Kubernetes stack

1. [Node provisioning](deploy/eks-node/README.md)
2. [Supported features](docs/supported_features.md)
3. [Deploy EKS cluster with mac1.metal nodes](deploy/eks-cluster-flare/README.md)

# Known limitations

The current implementation can run only [1 pod per
node](deploy/eks-cluster-flare/install.sh#L158). Also because procri uses a
subprocess to run the workload, the procri service cannot be restarted without
interupting the pod.

# Build

The Luna For Mac suite is a set of three executables:

1. ProcCRI, a CRI implementation that runs on “bare metal” instead of a
   virtualized environment
2. Kubelet, the primary "node agent" that runs on each node.
3. Kube-proxy, the network proxy that runs on each node.

To build kube-proxy & kubelet you’ll need a copy of our [Kubernetes
repository](https://github.com/elotl/kubernetes). It contains 4 branches:

1. elotl/master which contains the current version of the code (currently 1.21)
2. releases/elotl-v1.21.3
3. releases/elotl-v1.20.9
4. releases/elotl-v1.19.13

To get kube-proxy & kubelet working with 1.22 and 1.23, we recommand you use
version 1.21 that’s forward compatible with these versions.

## Kube-proxy

To build kube-proxy:

    $ git clone https://github.com/elotl/kubernetes.git
    $ cd kubernetes
    $ git switch release/elotl-v1.21.3
    $ KUBE_BUILD_PLATFORMS=darwin/amd64 make WHAT=cmd/kube-proxy

The binary will be at `kubernetes/_output/local/bin/darwin/amd64/kube-proxy`.

## Kubelet

To build kubelet:

    $ git clone https://github.com/elotl/kubernetes.git
    $ cd kubernetes
    $ git switch release/elotl-v1.21.3
    $ KUBE_BUILD_PLATFORMS=darwin/amd64 make WHAT=cmd/kubelet

The binary will be at `kubernetes/_output/local/bin/darwin/amd64/kubelet`.

## ProCRI

ProCRI the CRI implementation we use to stub a fake container within the
macOS environment. You can build ProCRI for the legacy Intel architecture
and for the new Arm architecture:

    $ cd procri
    $ make all
    ...
    $ ls -lh procri-darwin-*
    -rwxrwxr-x 1 user user 39M Jan  8 16:20 procri-darwin-amd64
    -rwxrwxr-x 1 user user 38M Jan  8 16:20 procri-darwin-arm64

# Install executables on macOS

TODO

# Development

## Running system tests

System tests are placed in [system_test](system_test/README.md) folder and
are run if commit the message contains the string "[e2e-test]"

## EKS cluster with mac nodes

Check out the [README](deploy/README.md) about creating EKS-mac clusters
with terraform.
