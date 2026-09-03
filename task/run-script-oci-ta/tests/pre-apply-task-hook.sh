#!/bin/bash

# This script is called before applying the task to set up required resources
TASK_COPY="$1"
TEST_NS="$2"

echo "Using test namespace: $TEST_NS"

# Kind registry TLS is not reliably trusted via the mounted trusted-ca bundle
# under deploy-local CI. Opt the task into insecure registry mode for tests only.
yq -i '
  .spec.stepTemplate.env = (.spec.stepTemplate.env // []) + [
    {"name": "ORAS_OPTIONS", "value": "--insecure"}
  ]
' "$TASK_COPY"

# Production steps request ~12Gi; a kind node cannot schedule that (Pending /
# ExceededNodeResources). Drop requests/limits on the temp Task copy only.
yq -i 'del(.spec.steps[].computeResources)' "$TASK_COPY"

echo "Pre-requirements setup complete"
