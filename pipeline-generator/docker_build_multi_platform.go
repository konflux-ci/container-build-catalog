package main

import (
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

const dockerBuildMultiPlatformDescription = `This pipeline is ideal for building multi-arch container images from a Containerfile while maintaining trust after pipeline customization.

_Uses ` + "`buildah`" + ` to create a multi-platform container image leveraging [trusted artifacts](https://konflux-ci.dev/architecture/ADR/0036-trusted-artifacts.html). It also optionally creates a source image and runs some build-time tests. This pipeline requires that the [multi platform controller](https://github.com/konflux-ci/multi-platform-controller) is deployed and configured on your Konflux instance. Information is shared between tasks using OCI artifacts instead of PVCs. EC will pass the [` + "`trusted_task.trusted`" + `](https://conforma.dev/docs/policy/packages/release_trusted_task.html#trusted_task__trusted) policy as long as all data used to build the artifact is generated from trusted tasks.
This pipeline is pushed as a Tekton bundle to [quay.io](https://quay.io/repository/konflux-ci/tekton-catalog/pipeline-docker-build-multi-platform-oci-ta?tab=tags)_
`

func GenerateDockerBuildMultiPlatform(dockerBuildOciTa tektonapi.Pipeline, existing *tektonapi.Pipeline) (tektonapi.Pipeline, error) {
	p := NewPipelineEditor(dockerBuildOciTa, existing)

	p.Pipeline.Spec.Description = dockerBuildMultiPlatformDescription
	p.Pipeline.Name = "docker-build-multi-platform-oci-ta"

	// Fan the build task out over the requested platforms using the remote buildah task.
	p.RenameTask("build-container", "build-images")
	p.SetTaskRef(
		"build-images",
		"buildah-remote-oci-ta",
		"quay.io/konflux-ci/tekton-catalog/task-buildah-remote-oci-ta:0.10.5@sha256:eb277ec7b44443f0506a60ac940a2e52178d60f17cb0f51a6966daed5b3755de",
	)
	p.AddTaskMatrixParam("build-images", "PLATFORM", ArrayValue("$(params.build-platforms)"))
	p.AddTaskParam("build-images", "IMAGE_APPEND_PLATFORM", StringValue("true"))

	// Build the image index from the per-platform image refs produced by the matrixed build task.
	p.SetTaskParam("build-image-index", "IMAGES", ArrayValue("$(tasks.build-images.results.IMAGE_REF[*])"))
	p.SetTaskRunAfter("build-image-index", []string{"build-images"})

	// Fan the per-platform scans out over the same platforms.
	p.AddTaskMatrixParam("roxctl-scan", "image-platform", ArrayValue("$(params.build-platforms)"))
	p.AddTaskMatrixParam("ecosystem-cert-preflight-checks", "platform", ArrayValue("$(params.build-platforms)"))
	p.AddTaskMatrixParam("clamav-scan", "image-arch", ArrayValue("$(params.build-platforms)"))

	// Always build the image index by default.
	p.SetParamDefault("build-image-index", "true")

	p.Pipeline.Spec.Params = append(p.Pipeline.Spec.Params, tektonapi.ParamSpec{
		Name:        "build-platforms",
		Description: "List of platforms to build the container images on. The available set of values is determined by the configuration of the multi-platform-controller.",
		Type:        tektonapi.ParamTypeArray,
		Default:     ArrayValue("linux/x86_64"),
	})

	p.ReorderArrays()
	return p.Pipeline, nil
}
