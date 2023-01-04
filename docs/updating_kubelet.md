# Updating kubelet

From time to time we should rebase our changes in kubelet on upstream kubernetes release and make sure everything still works correctly.
Here's a quick walkthrough what has to be done to created elotl kubelet release for new k8s version (e.g. `v1.21.3`):
1. Clone [elotl/kubernetes](https://github.com/elotl/kubernetes)
2. Checkout latest `releases/elotl-` branch
3. Create a new branch.
4. Run interactive rebase `$ git rebase -i HEAD~2` to squash all the commits with kubelet changes added on top of latest tagged commit (e.g. `v1.21.3`) on this branch.
5. Save SHA of squashed commit.
6. Fetch and checkout tag (e.g. `v1.21.3`) from kubernetes origin.
7. Create a new branch names `releases/elotl-<version>`, (e.g. `releases/elotl-v1.21.3`)
8. Run `git cherry-pick <squashed-commit-sha>`
9. Resolve all conflicts, and run `git cherry-pick --continue`.
10. Try to run `KUBE_BUILD_PLATFORMS=darwin/amd64 GOARCH=amd64 GOOS=darwin make WHAT=cmd/kubelet` and fix all issues.
11. Push a branch, a kubelet binary artifact will be built in CI.

## Adding smoke test for compatibility
In [gh actions](../.github/workflows) we have `e2e_kubelet_v*.yaml`  smoke tests defined for v1.18, v1.19, v1.20.

For adding new test, you need two things:
1. kubelet binary s3 url
2. Packed (`.tar.gz`) control-plane and node components binaries, all built for `amd64/darwin`. We need those packed in `bin/` folder:
    1. etcd
    2. kube-apiserver
    3. kube-controller-manager
    4. kube-scheduler
    5. kubectl

Once you have both uploaded to s3, you can copy [latest kubelet smoke test definition](../.github/workflows/e2e_kubelet_v1_20.yaml) and change two lines there:
```yaml
env:
          KUBELET_BIN_URL: "https://elotl-maccri.s3.amazonaws.com/kubelet-v1.20.3-rc.0-395-g41ba0426c0e"
          K8S_BIN_URL: "https://elotl-maccri.s3.amazonaws.com/kubernetes-v1.20.9etcd-v3.4.13-bin-darwin-amd64.tar.gz"
```
to the urls you got from 2.

