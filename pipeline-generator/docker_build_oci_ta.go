package main

import (
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

const dockerBuildOciTaDescription = `This pipeline is ideal for building container images from a Containerfile while maintaining trust after pipeline customization.

_Uses ` + "`buildah`" + ` to create a container image leveraging [trusted artifacts](https://konflux-ci.dev/architecture/ADR/0036-trusted-artifacts.html). It also optionally creates a source image and runs some build-time tests. Information is shared between tasks using OCI artifacts instead of PVCs. EC will pass the [` + "`trusted_task.trusted`" + `](https://conforma.dev/docs/policy/packages/release_trusted_task.html#trusted_task__trusted) policy as long as all data used to build the artifact is generated from trusted tasks.
This pipeline is pushed as a Tekton bundle to [quay.io](https://quay.io/repository/konflux-ci/tekton-catalog/pipeline-docker-build-oci-ta?tab=tags)_
`

func GenerateDockerBuildOciTa(dockerBuild tektonapi.Pipeline, existing *tektonapi.Pipeline) (tektonapi.Pipeline, error) {
	p := NewPipelineEditor(dockerBuild, existing)

	p.Pipeline.Spec.Description = dockerBuildOciTaDescription
	p.Pipeline.Name = "docker-build-oci-ta"
	p.Pipeline.Labels = map[string]string{
		"pipelines.openshift.io/used-by":  "build-cloud",
		"pipelines.openshift.io/runtime":  "generic",
		"pipelines.openshift.io/strategy": "docker",
	}

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
		"quay.io/konflux-ci/tekton-catalog/task-prefetch-dependencies-oci-ta:0.3.2@sha256:389aea03a065e8118d36b7acb85b05cd13f6750e7e10ff8a85f270ee65b0167b",
	)
	p.AddTaskParam("prefetch-dependencies", "SOURCE_ARTIFACT", StringValue("$(tasks.clone-repository.results.SOURCE_ARTIFACT)"))
	p.AddTaskParam("prefetch-dependencies", "ociStorage", StringValue("$(params.output-image).prefetch"))
	p.AddTaskParam("prefetch-dependencies", "ociArtifactExpiresAfter", StringValue("$(params.image-expires-after)"))

	p.SetTaskRef(
		"build-container",
		"buildah-oci-ta",
		"quay.io/konflux-ci/tekton-catalog/task-buildah-oci-ta:0.10.5@sha256:5fcf620537b0dece56a9c9a8090ce7592a874da92cc2a5d398abdc2382eb8a3c",
	)
	p.AddTaskParam("build-container", "SOURCE_ARTIFACT", StringValue("$(tasks.prefetch-dependencies.results.SOURCE_ARTIFACT)"))
	p.AddTaskParam("build-container", "PREFETCH_ARTIFACT", StringValue("$(tasks.prefetch-dependencies.results.PREFETCH_ARTIFACT)"))

	p.SetTaskRef(
		"build-source-image",
		"source-build-oci-ta",
		"quay.io/konflux-ci/tekton-catalog/task-source-build-oci-ta:0.3@sha256:7c5575ac8e292f27f57716c021ab0324460dc958e73946724c588c5228e5f372",
	)
	p.AddTaskParam("build-source-image", "SOURCE_ARTIFACT", StringValue("$(tasks.prefetch-dependencies.results.SOURCE_ARTIFACT)"))
	p.AddTaskParam("build-source-image", "PREFETCH_ARTIFACT", StringValue("$(tasks.prefetch-dependencies.results.PREFETCH_ARTIFACT)"))

	p.SetTaskRef(
		"sast-snyk-check",
		"sast-snyk-check-oci-ta",
		"quay.io/konflux-ci/tekton-catalog/task-sast-snyk-check-oci-ta:0.5@sha256:eba24f5d9f4b18aa71e523b9b3dbcf22982aa4b018824260a090b19dfc9abf6f",
	)
	p.AddTaskParam("sast-snyk-check", "SOURCE_ARTIFACT", StringValue("$(tasks.prefetch-dependencies.results.SOURCE_ARTIFACT)"))

	p.SetTaskRef(
		"sast-shell-check",
		"sast-shell-check-oci-ta",
		"quay.io/konflux-ci/tekton-catalog/task-sast-shell-check-oci-ta:0.1@sha256:61b27e6ad5daba761d41bb37efb790ed98380603fd4fe2f86d156def5bd72ecc",
	)
	p.AddTaskParam("sast-shell-check", "SOURCE_ARTIFACT", StringValue("$(tasks.prefetch-dependencies.results.SOURCE_ARTIFACT)"))

	p.SetTaskRef(
		"sast-unicode-check",
		"sast-unicode-check-oci-ta",
		"quay.io/konflux-ci/tekton-catalog/task-sast-unicode-check-oci-ta:0.4@sha256:eb9d5392f215cb8b52b16382098cac4885b1e6cd989f88ebd83fdb234d283eb9",
	)
	p.AddTaskParam("sast-unicode-check", "SOURCE_ARTIFACT", StringValue("$(tasks.prefetch-dependencies.results.SOURCE_ARTIFACT)"))

	p.SetTaskRef(
		"push-dockerfile",
		"push-dockerfile-oci-ta",
		"quay.io/konflux-ci/tekton-catalog/task-push-dockerfile-oci-ta:0.3.1@sha256:5a6cbebd89e5bc163b38231859767f7f6a0dd66cf1333699574379f062731183",
	)
	p.AddTaskParam("push-dockerfile", "SOURCE_ARTIFACT", StringValue("$(tasks.prefetch-dependencies.results.SOURCE_ARTIFACT)"))

	p.ReorderArrays()
	return p.Pipeline, nil
}
