#!/bin/bash

set -euo pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." >/dev/null 2>&1 && pwd )"

cd "${ROOT}/task-generator/remote"
GOTOOLCHAIN=auto GOSUMDB=sum.golang.org go build -o /tmp/remote-generator main.go

/tmp/remote-generator \
    --buildah-task="${ROOT}/task/buildah/buildah.yaml" \
    --remote-task="${ROOT}/task/buildah-remote/buildah-remote.yaml"

/tmp/remote-generator \
    --buildah-task="${ROOT}/task/buildah-oci-ta/buildah-oci-ta.yaml" \
    --remote-task="${ROOT}/task/buildah-remote-oci-ta/buildah-remote-oci-ta.yaml"

# Remote builds unpack larger trusted artifacts than local buildah-oci-ta.
# Fleet MAX+5% (15d / 5% margin, 2026-11-12): use-TA ~2213Mi → 3Gi / ~850m.
# Apply after generate so values are not lost when buildah-oci-ta use-TA is leaner.
REMOTE_OCI_TA="${ROOT}/task/buildah-remote-oci-ta/buildah-remote-oci-ta.yaml"
yq -i '
  (.spec.steps[] | select(.name == "use-trusted-artifact") | .computeResources.limits.memory) = "3Gi" |
  (.spec.steps[] | select(.name == "use-trusted-artifact") | .computeResources.requests.memory) = "3Gi" |
  (.spec.steps[] | select(.name == "use-trusted-artifact") | .computeResources.requests.cpu) = "850m"
' "${REMOTE_OCI_TA}"
