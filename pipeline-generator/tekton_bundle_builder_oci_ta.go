package main

import (
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func GenerateTektonBundleBuilderOciTa(tektonBundleBuilder tektonapi.Pipeline, existing *tektonapi.Pipeline) (tektonapi.Pipeline, error) {
	p := NewPipelineEditor(tektonBundleBuilder, existing)

	p.Pipeline.Name = "tekton-bundle-builder-oci-ta"

	// This pipeline shares data using trusted artifacts instead of the workspace,
	// so drop it along with every task binding to it.
	p.RemovePipelineWorkspace("workspace")

	p.SetTaskRef(
		"clone-repository",
		"git-clone-oci-ta",
		"quay.io/konflux-ci/tekton-catalog/task-git-clone-oci-ta:0.2.4@sha256:df3c42d78223f07b40a84dd29e5c8860d14777ffdf150ea08c738770f51216dc",
	)
	p.AddTaskParam("clone-repository", "ociStorage", StringValue("$(params.output-image).git"))
	p.AddTaskParam("clone-repository", "ociArtifactExpiresAfter", StringValue("$(params.image-expires-after)"))

	p.SetTaskRef(
		"prefetch-dependencies",
		"prefetch-dependencies-oci-ta",
		"quay.io/konflux-ci/tekton-catalog/task-prefetch-dependencies-oci-ta:0.10.1@sha256:c09c1c3ced67fb8740b2925a7de09c0810d4e8c8f148d41c21ffe37810d85c62",
	)
	p.AddTaskParam("prefetch-dependencies", "SOURCE_ARTIFACT", StringValue("$(tasks.clone-repository.results.SOURCE_ARTIFACT)"))
	p.AddTaskParam("prefetch-dependencies", "ociStorage", StringValue("$(params.output-image).prefetch"))
	p.AddTaskParam("prefetch-dependencies", "ociArtifactExpiresAfter", StringValue("$(params.image-expires-after)"))

	p.SetTaskRef(
		"build-container",
		"tkn-bundle-oci-ta",
		"quay.io/konflux-ci/tekton-catalog/task-tkn-bundle-oci-ta:0.2@sha256:176713599d937ae3b0e20bc9aa097235a88b34c39c12c698ba82c57727eb9dd5",
	)
	p.AddTaskParam("build-container", "SOURCE_ARTIFACT", StringValue("$(tasks.prefetch-dependencies.results.SOURCE_ARTIFACT)"))

	p.SetTaskRef(
		"sast-shell-check",
		"sast-shell-check-oci-ta",
		"quay.io/konflux-ci/tekton-catalog/task-sast-shell-check-oci-ta:0.1@sha256:61b27e6ad5daba761d41bb37efb790ed98380603fd4fe2f86d156def5bd72ecc",
	)
	p.AddTaskParam("sast-shell-check", "SOURCE_ARTIFACT", StringValue("$(tasks.prefetch-dependencies.results.SOURCE_ARTIFACT)"))
	p.AddTaskParam("sast-shell-check", "CACHI2_ARTIFACT", StringValue("$(tasks.prefetch-dependencies.results.CACHI2_ARTIFACT)"))

	p.SetTaskRef(
		"sast-unicode-check",
		"sast-unicode-check-oci-ta",
		"quay.io/konflux-ci/tekton-catalog/task-sast-unicode-check-oci-ta:0.4@sha256:eb9d5392f215cb8b52b16382098cac4885b1e6cd989f88ebd83fdb234d283eb9",
	)
	p.AddTaskParam("sast-unicode-check", "SOURCE_ARTIFACT", StringValue("$(tasks.prefetch-dependencies.results.SOURCE_ARTIFACT)"))
	p.AddTaskParam("sast-unicode-check", "CACHI2_ARTIFACT", StringValue("$(tasks.prefetch-dependencies.results.CACHI2_ARTIFACT)"))

	p.ReorderArrays()
	return p.Pipeline, nil
}
