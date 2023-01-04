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

# XXX henry: if there’s a Makefile to build procri for Darwin we should use it
pushd $ROOT_DIR

./release/build.sh

readonly procri_dev_bucket="procri-dev"

echo "Uploading procri build $current_tag"
aws s3 cp --acl public-read procri s3://$procri_dev_bucket/procri-$current_tag

popd
