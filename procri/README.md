# Build procri locally

```bash
make procri
```

# Process wrapper CRI
## Running CRI validation tests
You need to ensure that you have [cri-tools](https://github.com/kubernetes-sigs/cri-tools/blob/462ddbe5c86eed10a00aab6cd36364286f1554fa/docs/validation.md#install) installed.
There's a helper script to spin up a server and execute validation tests (currently only basic scenarios, check out the `FOCUS` and `SKIP` variables inside the script)
```
✗ ./scripts/run_cri_validation_tests.sh       
+ trap cleanup EXIT
+ critest -version
critest version: v1.20.0
PASS
+ FOCUS='runtime should support basic operations on container'
+ SKIP='runtime should support apparmor|runtime should support reopening container log|runtime should support execSync with timeout|runtime should support execSync|runtime should support listing container stats'
+ make
fatal: No names found, cannot describe anything.
make: Nothing to be done for 'all'.
+ sleep 2
+ ./procri --listen /var/run/user/1000/procri.sock
I0212 15:48:17.928605 3864109 server.go:44] starting listener at /var/run/user/1000/procri.sock
I0212 15:48:17.928693 3864109 main.go:50] starting streaming server on 192.168.1.20:8099
+ critest -runtime-endpoint /var/run/user/1000/procri.sock '-ginkgo.focus=runtime should support basic operations on container' '-ginkgo.skip=runtime should support apparmor|runtime should support reopening container log|runtime should support execSync with timeout|runtime should support execSync|runtime should support listing container stats'
critest version: v1.20.0
Running Suite: CRI validation
=============================
Random Seed: 1613141299 - Will randomize all specs
Will run 6 of 83 specs

...(...)...
I0212 15:48:19.976257 3864109 podsandbox.go:122] RemovePodSandbox request &RemovePodSandboxRequest{PodSandboxId:create-PodSandbox-for-container-59145a26-6d41-11eb-b969-3c58c28989d3/cri-test-namespace59145a37-6d41-11eb-b969-3c58c28989d3,}
I0212 15:48:19.976283 3864109 podsandbox.go:131] RemovePodSandbox for create-PodSandbox-for-container-59145a26-6d41-11eb-b969-3c58c28989d3/cri-test-namespace59145a37-6d41-11eb-b969-3c58c28989d3 succeeded
[AfterEach] [k8s.io] Container
  /go/src/github.com/kubernetes-sigs/cri-tools/pkg/framework/framework.go:51
•SSSSSSSSSSSSSSSSSSSSSS
Ran 6 of 83 Specs in 0.017 seconds
SUCCESS! -- 6 Passed | 0 Failed | 0 Pending | 77 Skipped
PASS
+ cleanup
+ echo 'pid: 3864109'
pid: 3864109
+ kill 3864109
+ echo 'killed grpc server'
killed grpc server

```
