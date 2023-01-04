#!/bin/bash
# Copyright 2021 Elotl Inc
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

readonly current_tag=$(git describe --dirty)
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT_DIR=$SCRIPT_DIR/..

pushd $ROOT_DIR
make test
env GOOS=darwin GOARCH=amd64 make procri-darwin-amd64
env GOOS=darwin GOARCH=arm64 make procri-darwin-arm64

echo "Current tag is $current_tag"
