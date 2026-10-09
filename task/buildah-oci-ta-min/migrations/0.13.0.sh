#!/usr/bin/env bash

set -euo pipefail

# generate-ta-tasks: sync-oci-ta-migration=true

declare -r pipeline_file=${1:?missing pipeline file}

kind=$(yq '.kind' "$pipeline_file")
case "$kind" in
Pipeline)
	tasks_selector=".spec.tasks[]"
	;;
PipelineRun)
	tasks_selector=".spec.pipelineSpec.tasks[]"
	;;
*)
	echo "Not a Pipeline or PipelineRun, skipping migration"
	exit 0
	;;
esac

buildah_task_refs=("buildah" "buildah-oci-ta" "buildah-oci-ta-min" "buildah-remote" "buildah-remote-oci-ta")
all_build_tasks=()
for task_refname in "${buildah_task_refs[@]}"; do
	task_filter="${tasks_selector} | select(.taskRef.params[] | (.name == \"name\" and .value == \"${task_refname}\"))"
	if yq -e "$task_filter" "$pipeline_file" >/dev/null 2>&1; then
		readarray -t -O ${#all_build_tasks[@]} all_build_tasks < <(yq -e "${task_filter} | .name" "$pipeline_file")
	fi
done

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

	local_val=$(
		taskname=$task_name yq -r \
			"${tasks_selector} | select(.name == env(taskname)) | .params[] | select(.name == \"BUILD_ARGS_FILE\") | .value" \
			"$pipeline_file"
	)
	param_path=$(
		taskname=$task_name yq -o json --indent 0 \
			"${tasks_selector} | select(.name == env(taskname)) | .params[] | select(.name == \"BUILD_ARGS_FILE\") | path" \
			"$pipeline_file"
	)

	if [[ -z "$local_val" || "$local_val" == "null" ]]; then
		# Empty / unset: rename to BUILD_ARGS_FILES with an empty array.
		pmt modify -f "$pipeline_file" generic replace "$param_path" '{"name":"BUILD_ARGS_FILES","value":[]}'

	elif [[ "$local_val" == *'params.build-args-file'* && "$local_val" != *'params.build-args-files'* ]]; then
		# Keep referencing the string pipeline param as a one-element array.
		pmt modify -f "$pipeline_file" task "$task_name" remove-param BUILD_ARGS_FILE
		# shellcheck disable=SC2016
		pmt modify -f "$pipeline_file" task "$task_name" add-param -t array BUILD_ARGS_FILES \
			'$(params.build-args-file)'

	else
		# Literal / other value: wrap as a one-element BUILD_ARGS_FILES array.
		pmt modify -f "$pipeline_file" task "$task_name" remove-param BUILD_ARGS_FILE
		pmt modify -f "$pipeline_file" task "$task_name" add-param -t array BUILD_ARGS_FILES "$local_val"
	fi
done
