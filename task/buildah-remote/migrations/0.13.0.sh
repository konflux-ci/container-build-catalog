#!/usr/bin/env bash

set -euo pipefail

# generate-ta-tasks: sync-oci-ta-migration=true

declare -r pipeline_file=${1:?missing pipeline file}

################################################################################
# Check if the Tekton file is a Pipeline or PipelineRun.
################################################################################

kind=$(yq '.kind' "$pipeline_file")
case "$kind" in
Pipeline)
	tasks_selector=".spec.tasks[]"
	params_selector=".spec.params"
	;;
PipelineRun)
	tasks_selector=".spec.pipelineSpec.tasks[]"
	params_selector=".spec.pipelineSpec.params"
	;;
*)
	echo "Not a Pipeline or PipelineRun, skipping migration"
	exit 0
	;;
esac

################################################################################
# Find all buildah tasks in the Tekton file.
################################################################################

buildah_task_refs=("buildah" "buildah-oci-ta" "buildah-oci-ta-min" "buildah-remote" "buildah-remote-oci-ta")

all_build_tasks=()
for task_refname in "${buildah_task_refs[@]}"; do
	task_filter="${tasks_selector} | select(.taskRef.params[] | (.name == \"name\" and .value == \"${task_refname}\"))"
	if yq -e "$task_filter" "$pipeline_file" >/dev/null 2>&1; then
		readarray -t -O ${#all_build_tasks[@]} all_build_tasks < <(yq -e "${task_filter} | .name" "$pipeline_file")
	fi
done

################################################################################
# Function to wrap a string as a JSON array.
################################################################################

scalar_to_array_json() {
	local val=$1
	if [[ -z "$val" || "$val" == "null" ]]; then
		echo '[]'
		return
	fi
	v=$val yq -o json -n '[strenv(v)]'
}

################################################################################
# Function to migrate a string parameter to an array parameter.
################################################################################

migrate_string_param_to_array() {
	local selector=$1
	local field=$2
	local set_type=$3

	local param_filter="${selector}[] | select(.name == \"build-args-file\" or .name == \"build-args-files\")"
	if ! yq -e "$param_filter" "$pipeline_file" >/dev/null 2>&1; then
		return 0
	fi

	local param_path current_name
	param_path=$(
		yq -o json --indent 0 \
			"${param_filter} | path" \
			"$pipeline_file"
	)
	current_name=$(yq -r "${param_filter} | .name" "$pipeline_file" | head -n1)

	if [[ "$current_name" == "build-args-file" ]]; then
		echo "Renaming build-args-file to build-args-files (${field})"
		local name_path
		name_path=$(yq -o json --indent 0 '. + ["name"]' <<<"$param_path")
		pmt modify -f "$pipeline_file" generic replace "$name_path" "build-args-files"
	fi

	if [[ "$set_type" == "true" ]]; then
		local type_path
		type_path=$(yq -o json --indent 0 '. + ["type"]' <<<"$param_path")
		pmt modify -f "$pipeline_file" generic replace "$type_path" "array"
	fi

	if ! yq -e "${selector}[] | select(.name == \"build-args-files\") | has(\"${field}\")" "$pipeline_file" >/dev/null 2>&1; then
		return 0
	fi

	local tag
	tag=$(yq "${selector}[] | select(.name == \"build-args-files\") | .${field} | tag" "$pipeline_file")
	if [[ "$tag" == "!!seq" ]]; then
		return 0
	fi

	echo "Converting build-args-files ${field} from string to array"
	local old_val new_val updated
	old_val=$(yq -r "${selector}[] | select(.name == \"build-args-files\") | .${field}" "$pipeline_file")
	new_val=$(scalar_to_array_json "$old_val")
	# Replace the whole param object. Replacing only the scalar field makes
	# pmt rewrite the parent mapping and emits a blank line plus a nested
	# sequence with no extra indent.
	updated=$(
		field=$field arr=$new_val yq -o json --indent 0 \
			"${selector}[] | select(.name == \"build-args-files\") | .[strenv(field)] = env(arr)" \
			"$pipeline_file"
	)
	pmt modify -f "$pipeline_file" generic replace "$param_path" "$updated"
}

################################################################################
# Migrate all buildah tasks.
################################################################################

migrate_string_param_to_array "$params_selector" "default" "true"

if [[ "$kind" == "PipelineRun" ]]; then
	# Includes PipelineRuns that use pipelineRef (no inlined pipelineSpec).
	migrate_string_param_to_array ".spec.params" "value" "false"
fi

if [ ${#all_build_tasks[@]} -eq 0 ]; then
	echo "No inlined buildah tasks found, skipping task-param migration"
	exit 0
fi

for task_name in "${all_build_tasks[@]}"; do
	[[ -z "$task_name" ]] && continue
	if ! taskname=$task_name yq -e \
		"${tasks_selector} | select(.name == env(taskname)) | .params[] | select(.name == \"BUILD_ARGS_FILE\")" \
		"$pipeline_file" >/dev/null 2>&1; then
		continue
	fi

	echo "Replacing BUILD_ARGS_FILE with BUILD_ARGS_FILES on task ${task_name}"
	local_tag=$(
		taskname=$task_name yq \
			"${tasks_selector} | select(.name == env(taskname)) | .params[] | select(.name == \"BUILD_ARGS_FILE\") | .value | tag" \
			"$pipeline_file"
	)
	local_val=$(
		taskname=$task_name yq -r \
			"${tasks_selector} | select(.name == env(taskname)) | .params[] | select(.name == \"BUILD_ARGS_FILE\") | .value" \
			"$pipeline_file"
	)
	local_items=()
	if [[ "$local_tag" == "!!seq" ]]; then
		readarray -t local_items < <(
			taskname=$task_name yq -r \
				"${tasks_selector} | select(.name == env(taskname)) | .params[] | select(.name == \"BUILD_ARGS_FILE\") | .value[]" \
				"$pipeline_file"
		)
	fi

	pmt modify -f "$pipeline_file" task "$task_name" remove-param BUILD_ARGS_FILE
	if [[ "$local_tag" == "!!seq" && ${#local_items[@]} -gt 0 ]]; then
		pmt modify -f "$pipeline_file" task "$task_name" add-param -t array BUILD_ARGS_FILES "${local_items[@]}"
	elif [[ "$local_val" == *'params.build-args-file'* || -z "$local_val" || "$local_val" == "null" ]]; then
		# shellcheck disable=SC2016
		pmt modify -f "$pipeline_file" task "$task_name" add-param -t array BUILD_ARGS_FILES \
			'$(params.build-args-files[*])'
	else
		pmt modify -f "$pipeline_file" task "$task_name" add-param -t array BUILD_ARGS_FILES "$local_val"
	fi
done
