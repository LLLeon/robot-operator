#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

../vendor/k8s.io/code-generator/generate-groups.sh \
  "deepcopy,client,informer,lister" \
  robot-operator/pkg/generated \
  robot-operator/pkg/apis \
  robot:v1 \
  --output-base $(pwd)/../../
