#!/bin/bash

set -euxo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT_DIR=$SCRIPT_DIR/..
USERID="$(id -u)"
PROCRI_SOCK="/var/run/user/$USERID/procri.sock"
PROCRI_PID=""
if [ $(uname) == "Darwin" ]; then
    PROCRI_SOCK="/Users/$USER/procri.sock"
fi

function cleanup() {
    if [[ -n "$PROCRI_PID" ]]; then
        echo "pid: $PROCRI_PID"
        kill $PROCRI_PID
        echo "killed grpc server"
        echo "removing data dirs"
        rm -r /tmp/procri-data/
    fi
}

trap cleanup EXIT

critest -version

pushd $ROOT_DIR

make
./procri-darwin-amd64 --listen "$PROCRI_SOCK" --v=5 &
PROCRI_PID=$!
sleep 2

popd

FOCUS="[Conformance]"
#FOCUS="runtime should support exec with tty="
SKIP="runtime should support apparmor|image status get image fields should not have Uid|Username empty|runtime should support set hostname"

critest -runtime-endpoint "$PROCRI_SOCK" -ginkgo.focus="$FOCUS" -ginkgo.skip="$SKIP"
