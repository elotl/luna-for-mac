# Integration test

These shell scripts are meant to be run on a Mac.

The file `_common.sh` contains a bunch of utility function.

The file `run_tests.sh` is the script running the tests. It sets-up a Kubernetes
data plane server, and clean-up all the left-over resources when it exits. It
then runs `01_kube-proxy_test.sh` that verifies that kube-proxy works correctly
but connecting to the apiserver through the virtual IP 10.0.0.1.
Kubelet <-> procri integration is tested in `02_kubelet_procri_test.sh`

To execute the tests:

    $ cd ~/mac-cri/system_test
    $ dash ./run_tests.sh
    or
    $ bash ./run_tests.sh
    
 

## ATTENTION REQUIRED: kube-proxy

Currently kube-proxy must be compiled for Darwin and copied into the system
tests’ directory:

    $ go get github.com/elotl/kubernetes
    $ cd $HOME/go/src/github.com/elotl/kubernetes
    $ git checkout -b elotl/master origin/elotl/master
    $ env GOOS=darwin GOARCH=amd64 make WHAT=cmd/kube-proxy
