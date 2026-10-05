#!/usr/bin/env bash
set -euo pipefail

# generate-ta-tasks: sync-oci-ta-migration=true

declare -r pipeline_file=${1:?missing pipeline file}

if ! grep -qF 'results.CACHI2_ARTIFACT' "$pipeline_file"; then
    echo "No CACHI2_ARTIFACT result references found, skipping"
    exit 0
fi

echo "Rewriting results.CACHI2_ARTIFACT → results.PREFETCH_ARTIFACT"
sed -i 's/results\.CACHI2_ARTIFACT/results.PREFETCH_ARTIFACT/g' "$pipeline_file"
