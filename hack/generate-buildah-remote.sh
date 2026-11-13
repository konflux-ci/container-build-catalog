#!/bin/bash

set -euo pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." >/dev/null 2>&1 && pwd )"

cd "${ROOT}/buildah-remote-generator"
GOTOOLCHAIN=auto GOSUMDB=sum.golang.org go build -o /tmp/buildah-remote-generator main.go

/tmp/buildah-remote-generator \
    --buildah-task="${ROOT}/task/buildah/buildah.yaml" \
    --remote-task="${ROOT}/task/buildah-remote/buildah-remote.yaml"

/tmp/buildah-remote-generator \
    --buildah-task="${ROOT}/task/buildah-oci-ta/buildah-oci-ta.yaml" \
    --remote-task="${ROOT}/task/buildah-remote-oci-ta/buildah-remote-oci-ta.yaml"
