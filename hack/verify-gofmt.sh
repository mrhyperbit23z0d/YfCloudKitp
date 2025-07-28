#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

GO_VERSION=($(go version))

if [[ -z $(echo "${GO_VERSION[2]}" | grep -E 'go1.4|go1.5') ]]; then
  echo "Unknown go version '${GO_VERSION[2]}', skipping gofmt."
  exit 0
fi

S2I_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${S2I_ROOT}/hack/common.sh"

cd "${S2I_ROOT}"

find_files() {
  find . -not \( \
      \( \
        -wholename './output' \
        -o -wholename './_output' \
        -o -wholename './release' \
        -o -wholename './target' \
        -o -wholename '*/Godeps/*' \
      \) -prune \
    \) -name '*.go'
}

bad_files=$(find_files | xargs gofmt -s -l)
if [[ -n "${bad_files}" ]]; then
  echo "!!! gofmt needs to be run on the following files: "
  echo "${bad_files}"
  echo "Try running 'gofmt -s -d [path]'"
  exit 1
fi
