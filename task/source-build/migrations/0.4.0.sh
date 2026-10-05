#!/usr/bin/env bash
set -euo pipefail

# generate-ta-tasks: sync-oci-ta-migration=true

declare -r pipeline_file=${1:?missing pipeline file}

declare -ra TASK_REF_NAMES=(
    source-build-oci-ta
)

tasks_selector="(.spec.tasks[]?, .spec.pipelineSpec.tasks[]?)"

find_tasks() {
    local task_refname=$1
    yq -r "${tasks_selector} |
        select(
            .taskRef.params != null and
            (.taskRef.params | any_c(.name == \"name\" and .value == \"${task_refname}\"))
        ) |
        .name" "$pipeline_file"
}

has_param() {
    local task_name=$1 param_name=$2
    yq -e "${tasks_selector} | select(.name == \"${task_name}\") |
        .params[]? | select(.name == \"${param_name}\")" "$pipeline_file" >/dev/null 2>&1
}

get_param_value() {
    local task_name=$1 param_name=$2
    yq -r "${tasks_selector} | select(.name == \"${task_name}\") |
        .params[] | select(.name == \"${param_name}\") | .value" "$pipeline_file"
}

task_names=()
for task_refname in "${TASK_REF_NAMES[@]}"; do
    while IFS= read -r name; do
        [[ -n "$name" ]] || continue
        task_names+=("$name")
    done < <(find_tasks "$task_refname")
done

if [[ ${#task_names[@]} -eq 0 ]]; then
    echo "Pipeline does not use source-build-oci-ta, skipping migration"
    exit 0
fi

mapfile -t task_names < <(printf '%s\n' "${task_names[@]}" | sort -u)

for task_name in "${task_names[@]}"; do
    if has_param "$task_name" PREFETCH_ARTIFACT; then
        if has_param "$task_name" CACHI2_ARTIFACT; then
            echo "Task $task_name already has PREFETCH_ARTIFACT; removing leftover CACHI2_ARTIFACT"
            pmt modify -f "$pipeline_file" task "$task_name" remove-param CACHI2_ARTIFACT
        else
            echo "Task $task_name already migrated to PREFETCH_ARTIFACT"
        fi
        continue
    fi

    if ! has_param "$task_name" CACHI2_ARTIFACT; then
        echo "Task $task_name has neither CACHI2_ARTIFACT nor PREFETCH_ARTIFACT, skipping"
        continue
    fi

    value=$(get_param_value "$task_name" CACHI2_ARTIFACT)
    echo "Renaming $task_name param CACHI2_ARTIFACT -> PREFETCH_ARTIFACT"
    pmt modify -f "$pipeline_file" task "$task_name" add-param PREFETCH_ARTIFACT "$value"
    pmt modify -f "$pipeline_file" task "$task_name" remove-param CACHI2_ARTIFACT
done
