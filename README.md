# Luna For Mac

A software stack to attach Intel Mac nodes to Kubernetes. With Luna For Mac
you can attach Mac compute to your Kubernetes clusters and run Mac workload
on them. It supports OSX 10.9 (Catalina) and up.

The easiest way to test Luna For Mac is to check out [Luna For Mac
Demo](https://github.com/elotl/luna-for-mac-demo). This repository contains
all the instructions and tools to set-up a Mac nodes with AWS EC2 and EKS.

It is a generic Kubernetes node stack that can be used on bare metal or on a
local virtual machine or even in the Cloud via AWS mac*.metal instances.

# Kubernetes stack

1. [Node provisioning](docs/node_provisioning.md)
2. [Supported features](docs/supported_features.md)
3. [Deploy EKS cluster with mac1.metal nodes](deploy/eks-cluster-flare/)

# Known limitations

The current implementation can run only [1 pod per
node](deploy/eks-cluster-flare/install.sh#L158). Also because ProCRI uses
a sub-process to run the workload, the ProCRI service cannot be restarted
without interrupting the pod.

# Build

The Luna For Mac suite is a set of three executable:

1. ProcCRI, a CRI implementation that runs on “bare metal” instead of a virtualized environment
2. Kubelet, the primary "node agent" that runs on each node.
3. Kube-proxy, the network proxy that runs on each node.

To build kube-proxy & kubelet you’ll need a copy of our [Kubernetes
repository](https://github.com/elotl/kubernetes). It contains 4 branches:

1. elotl/master which contains the current version of the code (currently 1.21)
2. releases/elotl-v1.21.3 3. releases/elotl-v1.20.9 4. releases/elotl-v1.19.13

To get kube-proxy & kubelet working with 1.22 and 1.23, we recommend you
use version 1.21 that’s forward compatible with these versions.

## Kube-proxy

To build kube-proxy for the legacy x86 platform:

    $ git clone https://github.com/elotl/kubernetes.git
    $ cd kubernetes
    $ git switch release/elotl-v1.21.3
    $ KUBE_BUILD_PLATFORMS=darwin/amd64 make WHAT=cmd/kube-proxy

Or to build for the new Arm platform:

    $ KUBE_BUILD_PLATFORMS=darwin/amd64 make WHAT=cmd/kube-proxy

The binary will be at `kubernetes/_output/local/bin/darwin/*/kube-proxy`.

## Kubelet

To build kubelet for the legacy x86 platform:

    $ git clone https://github.com/elotl/kubernetes.git $
    $ cd kubernetes
    $ git switch release/elotl-v1.21.3
    $ KUBE_BUILD_PLATFORMS=darwin/amd64 make WHAT=cmd/kubelet

Or to build for the new Arm platform:

    $ KUBE_BUILD_PLATFORMS=darwin/arm64 make WHAT=cmd/kubelet

The binary will be at `kubernetes/_output/local/bin/darwin/*/kubelet`.

## ProCRI

ProCRI the CRI implementation we use to stub a fake container within the
macOS environment. You can build ProCRI for the legacy Intel architecture
and for the new Arm architecture:

    $ cd procri
    $ make all ...
    $ ls -lh procri-darwin-*
    -rwxrwxr-x 1 user user 39M Jan  8 16:20 procri-darwin-amd64
    -rwxrwxr-x 1 user user 38M Jan  8 16:20 procri-darwin-arm64

# Packer Image builder

We initially targeted mac1.metal instances on EKS as our initial platform. We
created an image builder for this platform in [ami-builder](ami-builder/). It
requires [Packer](https://www.packer.io/). To build it:

    $ cd ami-builder/base-image
    $ packer build .

# Install executable on macOS

To install the binaries on macOS.

First install the system services kubelet and kube-proxy. Both of these
services will run a system daemons. The plist files are located at
[ami-builder/base-img/files/](ami-builder/base-img/files/):

    # install kube-proxy /usr/local/bin/kube-proxy
    # cp com.kubernetes.kube-proxy.plist /Library/LaunchDaemon
    # install kubelet /usr/local/bin/kubelet
    # cp com.kubernetes.kubelet.plist /Library/LaunchDaemon

Second create an admin account that will be used to run the CRI daemon
that will execute the workload. Here we’ll call this account `kube-user`,
but feel free to use something else. First copy the binary and plist file
to the right location:

    # install procri-darwin-amd64 /usr/local/bin/procri
    # sudo -u kube-user mkdir -p /Users/kube-user/Library/LaunchAgents
    # cp com.elotl.procri.plist /Users/kube-user/Library/LaunchAgents

Finally you must enable ProCRI’s plist. This step is a bit tricky: you
will have to execute the following command logged as a real admin user. This
means you may have to VNC into the node to get access to the user interface
in order to execute this command:

    $ whoami
    kube-user
    $ launchctl enable gui/$(id -u)/com.elotl.procri

Then you’ll have to get the kube-user to login automatically when the
computer boots in order for the ProCRI launch daemon to start up:

    $ sudo /usr/bin/dscl . -passwd /Users/kube-user kube-user
    $ sudo /usr/bin/defaults write /Library/Preferences/com.apple.loginwindow autoLoginUser kube-user

# Development

## Running system tests

System tests are placed in [system_test](system_test/README.md) folder and
are run if commit the message contains the string "[e2e-test]"

## EKS cluster with mac nodes

Check out the [README](deploy/README.md) about creating EKS-mac clusters
with terraform.
