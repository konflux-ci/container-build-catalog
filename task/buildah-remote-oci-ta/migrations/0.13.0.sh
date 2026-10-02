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
	;;
PipelineRun)
	tasks_selector=".spec.pipelineSpec.tasks[]"
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

if [ ${#all_build_tasks[@]} -eq 0 ]; then
	echo "No inlined buildah tasks found, skipping task-param migration"
	exit 0
fi

################################################################################
# Migrate BUILD_ARGS_FILE -> BUILD_ARGS_FILES on buildah tasks.
#
# When the old value referenced the string pipeline param build-args-file,
# migrate it to a one-element array so the task param type matches the new
# BUILD_ARGS_FILES array API without renaming the pipeline param:
#
#   - name: BUILD_ARGS_FILES
#     value: ["$(params.build-args-file)"]
#
# That way pipelineRef-only PipelineRuns that still pass build-args-file
# (and cannot be auto-migrated) keep working.
################################################################################

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
	elif [[ -z "$local_val" || "$local_val" == "null" ||
		("$local_val" == *'params.build-args-file'* && "$local_val" != *'params.build-args-files'*) ]]; then
		# Keep the singular pipeline param so pipelineRef-only PipelineRuns that
		# still pass build-args-file continue to work.
		# shellcheck disable=SC2016
		pmt modify -f "$pipeline_file" task "$task_name" add-param -t array BUILD_ARGS_FILES \
			'$(params.build-args-file)'
	else
		pmt modify -f "$pipeline_file" task "$task_name" add-param -t array BUILD_ARGS_FILES "$local_val"
	fi
done
