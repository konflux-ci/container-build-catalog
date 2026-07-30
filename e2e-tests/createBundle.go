package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	remoteimg "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	tektonpipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

type Snapshot struct {
	Application string              `json:"application"`
	Components  []SnapshotComponent `json:"components"`
}

type SnapshotComponent struct {
	Name           string `json:"name"`
	ContainerImage string `json:"containerImage"`
}

func ParseSnapshotImages(snapshotJSON string) (map[string]string, error) {
	var snapshot Snapshot
	if err := json.Unmarshal([]byte(snapshotJSON), &snapshot); err != nil {
		return nil, fmt.Errorf("failed to parse snapshot JSON: %w", err)
	}

	componentImages := make(map[string]string, len(snapshot.Components))
	for _, c := range snapshot.Components {
		taskName := strings.TrimPrefix(c.Name, "task-")
		componentImages[taskName] = c.ContainerImage
	}
	return componentImages, nil
}

var updatedTaskBundleMap map[string]string

// List of tasks for every build pipeline, optimize it later to read from the pipeline dynamically, rather than defining static list
var dockerBuildPipelineTasks = []string{"init", "git-clone", "prefetch-dependencies", "buildah", "build-image-index", "source-build", "deprecated-image-check", "clair-scan", "ecosystem-cert-preflight-checks", "sast-snyk-check", "clamav-scan", "sast-shell-check", "sast-unicode-check", "apply-tags", "push-dockerfile", "rpms-signature-scan"}
var dockerBuildOciTAPipelineTasks = []string{"init", "git-clone-oci-ta", "prefetch-dependencies-oci-ta", "buildah-oci-ta", "build-image-index", "source-build-oci-ta", "deprecated-image-check", "clair-scan", "ecosystem-cert-preflight-checks", "sast-snyk-check-oci-ta", "clamav-scan", "sast-shell-check-oci-ta", "sast-unicode-check-oci-ta", "apply-tags", "push-dockerfile-oci-ta", "rpms-signature-scan"}
var dockerBuildMultiPlatformOciTAPipelineTasks = []string{"init", "git-clone-oci-ta", "prefetch-dependencies-oci-ta", "buildah-remote-oci-ta", "build-image-index", "source-build-oci-ta", "deprecated-image-check", "clair-scan", "ecosystem-cert-preflight-checks", "sast-snyk-check-oci-ta", "clamav-scan", "sast-shell-check-oci-ta", "sast-unicode-check-oci-ta", "apply-tags", "push-dockerfile-oci-ta", "rpms-signature-scan"}
var dockerBuildOciTAMinPipelineTasks = []string{"init", "git-clone-oci-ta-min", "prefetch-dependencies-oci-ta-min", "buildah-oci-ta-min", "build-image-index-min", "deprecated-image-check", "clamav-scan-min", "sast-shell-check-oci-ta-min", "sast-unicode-check-oci-ta-min", "rpms-signature-scan", "tpa-scan"}
var fbcBuilderPipelineTasks = []string{"init", "git-clone-oci-ta", "fbc-inject-lifecycle-oci-ta", "run-opm-command-oci-ta", "prefetch-dependencies-oci-ta", "buildah-remote-oci-ta", "build-image-index", "deprecated-image-check", "apply-tags", "validate-fbc", "fbc-fips-check-oci-ta"}

// Result Paths for different pipelines
var baseResultPath = "/tekton/results/"
var resultPathForDockerBuild = baseResultPath + "docker-build-pipeline-bundle"
var resultPathForDockerBuildOciTA = baseResultPath + "docker-build-oci-ta-pipeline-bundle"
var resultPathForDockerBuildOciTAMin = baseResultPath + "docker-build-oci-ta-min-pipeline-bundle"
var resultPathForDockerBuildMultiPlatformOciTA = baseResultPath + "docker-build-multi-platform-oci-ta-pipeline-bundle"
var resultPathForFbcBuilder = baseResultPath + "fbc-builder-pipeline-bundle"

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Please provide at least one argument.")
		os.Exit(1)
	}
	snapshotString := os.Args[1]

	var err error
	updatedTaskBundleMap, err = ParseSnapshotImages(snapshotString)
	if err != nil {
		fmt.Printf("failed to parse snapshot with error: %v\n", err)
		os.Exit(1)
	}

	if isDockerBuildPipelineChanged() {
		newDockerBuildPipeline, err := createBuildPipelineBundle("docker-build")
		if err != nil {
			fmt.Printf("failed to create docker-build pipeline bundle with error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[DEBUG] UPDATED DOCKER-BUILD BUNDLE: %v\n", newDockerBuildPipeline)
		writeResult(resultPathForDockerBuild, newDockerBuildPipeline)
	}
	if isDockerBuildOciTAPipelineChanged() {
		newDockerBuildOciTaPipeline, err := createBuildPipelineBundle("docker-build-oci-ta")
		if err != nil {
			fmt.Printf("failed to create docker-build-oci-ta pipeline bundle with error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[DEBUG] UPDATED DOCKER-BUILD-OCI-TA BUNDLE: %v\n", newDockerBuildOciTaPipeline)
		writeResult(resultPathForDockerBuildOciTA, newDockerBuildOciTaPipeline)
	}
	if isDockerBuildOciTAMinPipelineChanged() {
		newDockerBuildOciTAMinPipeline, err := createBuildPipelineBundle("docker-build-oci-ta-min")
		if err != nil {
			fmt.Printf("failed to create docker-build-oci-ta-min pipeline bundle with error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[DEBUG] UPDATED DOCKER-BUILD-OCI-TA-MIN BUNDLE: %v\n", newDockerBuildOciTAMinPipeline)
		writeResult(resultPathForDockerBuildOciTAMin, newDockerBuildOciTAMinPipeline)
	}
	if isDockerBuildMultiPlatformOciTAPipelineChanged() {
		newDockerBuildMultiPlatformOciTAPipeline, err := createBuildPipelineBundle("docker-build-multi-platform-oci-ta")
		if err != nil {
			fmt.Printf("failed to create docker-build-multi-platform-oci-ta pipeline bundle with error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[DEBUG] UPDATED DOCKER-BUILD-MULTI-PLATFORM-OCI-TA BUNDLE: %v\n", newDockerBuildMultiPlatformOciTAPipeline)
		writeResult(resultPathForDockerBuildMultiPlatformOciTA, newDockerBuildMultiPlatformOciTAPipeline)
	}
	if isFbcBuilderPipelineChanged() {
		newFbcBuilderPipeline, err := createBuildPipelineBundle("fbc-builder")
		if err != nil {
			fmt.Printf("failed to create fbc-builder pipeline bundle with error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[DEBUG] UPDATED FBC-BUILDER BUNDLE: %v\n", newFbcBuilderPipeline)
		writeResult(resultPathForFbcBuilder, newFbcBuilderPipeline)
	}
}

func writeResult(resultPath, outputData string) {
	err := os.WriteFile(resultPath, []byte(outputData), 0644)
	if err != nil {
		fmt.Printf("failed to write result in resultPath %s with error: %v\n", resultPath, err)
		os.Exit(1)
	}
}

func isDockerBuildPipelineChanged() bool {
	for taskName, _ := range updatedTaskBundleMap {
		if slices.Contains(dockerBuildPipelineTasks, taskName) {
			return true
		}
	}
	return false
}

func isDockerBuildOciTAPipelineChanged() bool {
	for taskName, _ := range updatedTaskBundleMap {
		if slices.Contains(dockerBuildOciTAPipelineTasks, taskName) {
			return true
		}
	}
	return false
}

func isDockerBuildOciTAMinPipelineChanged() bool {
	for taskName, _ := range updatedTaskBundleMap {
		if slices.Contains(dockerBuildOciTAMinPipelineTasks, taskName) {
			return true
		}
	}
	return false
}

func isDockerBuildMultiPlatformOciTAPipelineChanged() bool {
	for taskName, _ := range updatedTaskBundleMap {
		if slices.Contains(dockerBuildMultiPlatformOciTAPipelineTasks, taskName) {
			return true
		}
	}
	return false
}

func isFbcBuilderPipelineChanged() bool {
	for taskName, _ := range updatedTaskBundleMap {
		if slices.Contains(fbcBuilderPipelineTasks, taskName) {
			return true
		}
	}
	return false
}

func getDefaultPipelineBundleRef(pipelineName string) (string, error) {
	defaultPipelineBundleMap := map[string]string{
		"docker-build":                       "quay.io/konflux-ci/tekton-catalog/pipeline-docker-build:devel",
		"docker-build-oci-ta":                "quay.io/konflux-ci/tekton-catalog/pipeline-docker-build-oci-ta:devel",
		"docker-build-oci-ta-min":            "quay.io/konflux-ci/tekton-catalog/pipeline-docker-build-oci-ta-min:devel",
		"docker-build-multi-platform-oci-ta": "quay.io/konflux-ci/tekton-catalog/pipeline-docker-build-multi-platform-oci-ta:devel",
		"fbc-builder":                        "quay.io/konflux-ci/tekton-catalog/pipeline-fbc-builder:devel",
	}
	if bundleRef, ok := defaultPipelineBundleMap[pipelineName]; ok {
		return bundleRef, nil
	} else {
		return "", fmt.Errorf("did not find default pipeline bundle associated with the pipeline name")
	}
}

func getNewBundle(taskBundle string) string {
	parts := strings.Split(taskBundle, "/")
	last := parts[len(parts)-1]
	namePart := strings.SplitN(last, ":", 2)[0]
	taskName := strings.TrimPrefix(namePart, "task-")
	if bundle, ok := updatedTaskBundleMap[taskName]; ok {
		return bundle
	}
	return ""
}

// CreateBuildPipelineBundle creates a new pipeline bundle after replacing the existing
// task "taskName" with new task bundle "customTaskBundle" and returns the newly built pipeline bundle reference
func createBuildPipelineBundle(pipelineName constants.BuildPipelineType) (string, error) {
	var pipelineBundle string
	var tektonObj runtime.Object
	var newBuildPipelineRef name.Reference
	var err error
	var newPipelineYaml []byte
	var authenticator authn.Authenticator

	if pipelineBundle, err = getDefaultPipelineBundleRef(string(pipelineName)); err != nil {
		return "", fmt.Errorf("failed to get the pipeline bundle ref: %+v", err)
	}
	// Extract the pipeline as tekton object from the bundle
	if tektonObj, err = tekton.ExtractTektonObjectFromBundle(pipelineBundle, "pipeline", pipelineName); err != nil {
		return "", fmt.Errorf("failed to extract the Tekton Pipeline from bundle: %+v", err)
	}

	pipelineObject := tektonObj.(*tektonpipeline.Pipeline)

	for i := range pipelineObject.PipelineSpec().Tasks {
		t := &pipelineObject.PipelineSpec().Tasks[i]
		for k, param := range t.TaskRef.Params {
			if param.Name == "bundle" {
				newBundle := getNewBundle(param.Value.StringVal)
				if newBundle != "" {
					t.TaskRef.Params[k].Value = *tektonpipeline.NewStructuredValues(newBundle)
				}
			}
		}
	}

	if newPipelineYaml, err = yaml.Marshal(pipelineObject); err != nil {
		return "", fmt.Errorf("error when marshalling a new pipeline to YAML: %v", err)
	}

	if err = utils.CreateDockerConfigFile(os.Getenv("QUAY_TOKEN")); err != nil {
		return "", fmt.Errorf("failed to create docker config file: %+v", err)
	}
	tag := fmt.Sprintf("%d-%s", time.Now().Unix(), util.GenerateRandomString(4))
	quayOrg := utils.GetEnv("QUAY_ORG", constants.DefaultQuayOrg)
	newBuildPipelineImg := strings.ReplaceAll(constants.DefaultImagePushRepo, constants.DefaultQuayOrg, quayOrg)
	newBuildPipelineRef, err = name.ParseReference(fmt.Sprintf("%s:pipeline-bundle-%s", newBuildPipelineImg, tag))
	if err != nil {
		return "", fmt.Errorf("invalid reference %q got error: %v", newBuildPipelineRef, err)
	}
	if authenticator, err = utils.GetAuthenticatorForImageRef(newBuildPipelineRef, os.Getenv("QUAY_TOKEN")); err != nil {
		return "", fmt.Errorf("error when getting authenticator: %v", err)
	}
	authOption := remoteimg.WithAuth(authenticator)

	// Build and Push the tekton bundle
	if err = tekton.BuildAndPushTektonBundle(newPipelineYaml, newBuildPipelineRef, authOption); err != nil {
		return "", fmt.Errorf("error when building/pushing a tekton pipeline bundle: %v", err)
	}

	return newBuildPipelineRef.String(), nil
}
