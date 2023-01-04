set +e
./bin/kubectl --kubeconfig kubeconfig.kubelet get nodes -o wide
./bin/kubectl --kubeconfig kubeconfig.kubelet create sa default

run_test_create() {
  ./bin/kubectl --kubeconfig kubeconfig.kubelet apply -f long_running_pod.yaml
  local max_retries=0
  while true
  do
    max_retries=$(($max_retries+1))
    pod_phase=$(./bin/kubectl --kubeconfig kubeconfig.kubelet get pods -o jsonpath='{.items[0].status.phase}')

    if [ $pod_phase != "Running" ]; then
      echo "pod not running yet..."
      sleep 5
    else
      echo "pod is running."
      return
    fi
    if [ $max_retries -gt 5 ];then
          echo "max retries exceeded"
          break
    fi
  done
  echo "pod phase is $pod_phase should be Running"
  echo "\nkubelet logs:\n"
  cat kubelet.log
  echo "\n procri logs:\n"
  cat procri.log
  die "run_test_create failed."
}

run_test_delete() {
  local max_retries=0
  ./bin/kubectl --client-certificate=ca.crt --kubeconfig kubeconfig.kubelet delete -f long_running_pod.yaml
  while true
  do
    max_retries=$(($max_retries+1))
    zero_items=$(./bin/kubectl --kubeconfig kubeconfig.kubelet get pods -o jsonpath='{.items}')
    if [ $zero_items != "[]" ]; then
      echo "pod not deleted yet..."
      sleep 2
    else
      echo "pod deleted."
      return
    fi
    if [ $max_retries -gt 5 ];then
          echo "max retries exceeded"
          break
    fi
  done
  echo "pod not deleted"
  echo "\n kubelet logs:\n"
  cat kubelet.log
  echo "\n procri logs:\n"
  cat procri.log
  die "run_test_delete failed."
}

run_test_logs() {
  ./bin/kubectl --kubeconfig kubeconfig.kubelet apply -f log_test_pod.yaml
  max_retries=0
  while true
  do
    max_retries=$(($max_retries+1))
    pod_phase=$(./bin/kubectl --kubeconfig kubeconfig.kubelet get pods -o jsonpath='{.items[0].status.phase}')
    if [ $pod_phase != "Succeeded" ] && [ $pod_phase != "Running" ]; then
      echo "pod not running yet..."
      sleep 5
    else
      echo "pod is running."
      echo "printing logs:"
      ./bin/kubectl --kubeconfig kubeconfig.kubelet logs log-test-pod
      ./bin/kubectl --kubeconfig kubeconfig.kubelet delete -f log_test_pod.yaml
      set -e
      return
    fi
    if [ $max_retries -gt 5 ];then
          echo "max retries exceeded"
          break
    fi
  done
  echo "pod phase is $pod_phase should be Succeeded or Running"
  echo "\nkubelet logs:\n"
  cat kubelet.log
  echo "\n procri logs:\n"
  cat procri.log
  die "run_test_logs failed"

}

run_test_create
run_test_delete
run_test_logs
