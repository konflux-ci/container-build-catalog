#!/usr/bin/env bash
set -o errexit -o nounset -o pipefail

: "${FAIL_ON_CHANGES=false}"

# make sure we're running from the repo root
cd "$(dirname "${BASH_SOURCE[0]}")/.."

tmpdir=$(mktemp -d --tmpdir)
trap 'rm -r "$tmpdir"' EXIT

GOTOOLCHAIN=auto go build -C pipeline-generator/ -o "$tmpdir/pipeline-generator"

generate_args=()
if [[ "$FAIL_ON_CHANGES" == true ]]; then
    generate_args=(--backup-dir "$tmpdir/pipelines" --format-backup)
fi

"$tmpdir/pipeline-generator" "${generate_args[@]}" pipelines/

# Check if the newly generated pipelines are consistent with their previous state.
#
# Note: the generator can make cosmetic changes that do not affect the semantics in any way.
# We don't want this to cause failures in CI. That would be very annoying to deal with,
# because we get automatic migrations applied to the generated pipelines,
# and these migrations may not make exactly the same stylistic choices as the generator.
#
# We work around this by comparing the *formatted* backup created by the pipeline-generator
# against the newly generated files (which are formatted the same way).
compare_pipelines() {
    local any_changed=false

    local color=auto
    if [[ "${GITHUB_ACTIONS:-}" == true ]]; then
        color=always  # The GH actions UI can render colors
    fi

    local backup_path
    for backup_path in "$tmpdir/pipelines"/*.yaml; do
        local basename
        basename=$(basename "$backup_path")
        local pipeline_path=pipelines/${basename%.yaml}/$basename

        if ! diff -q "$backup_path" "$pipeline_path" >/dev/null; then
            any_changed=true
            echo "--------------------------------------------------------------------------------"
            echo "error: pipeline $(basename "$pipeline_path") changed after re-generation"
            echo "--------------------------------------------------------------------------------"
            diff -u --color="$color" "$backup_path" "$pipeline_path" || true
        fi
    done

    if [[ "$any_changed" == true ]]; then
        return 1
    fi

    echo "success: no semantically significant changes to any of the pipelines"
}

if [[ "$FAIL_ON_CHANGES" == true ]]; then
    compare_pipelines
fi
