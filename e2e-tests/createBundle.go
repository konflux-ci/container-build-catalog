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

func ParseSnapshotImages(snapshotJSON, snapshotType, componentName string) (map[string]string, error) {
	var snapshot Snapshot
	if err := json.Unmarshal([]byte(snapshotJSON), &snapshot); err != nil {
		return nil, fmt.Errorf("failed to parse snapshot JSON: %w", err)
	}

	componentImages := make(map[string]string, len(snapshot.Components))
	for _, c := range snapshot.Components {
		// In case of group snapshot, get all the components and for component snapshot, get the containerImage only component matching the componentName
		if (snapshotType == "group") || (snapshotType == "component" && componentName == c.Name) {
			taskName := strings.TrimPrefix(c.Name, "task-")
			componentImages[taskName] = c.ContainerImage
		}
	}
	return componentImages, nil
}

// updatedTaskBundleMap to contain updated task bundle references after extracting it from the snapshot
var updatedTaskBundleMap map[string]string

// pipelinesNameList contains names of the current build pipelines
var pipelinesNameList = []string{"docker-build", "docker-build-oci-ta", "docker-build-oci-ta-min", "docker-build-multi-platform-oci-ta", "fbc-builder"}

// pipelineNameVsTaskListMap map to contain pipeline name as key and its task list as value
var pipelineNameVsTaskListMap = make(map[string][]string)

// Result Paths for different pipelines
var baseResultPath = "/tekton/results/"
var resultPathForDockerBuild = baseResultPath + "docker-build-pipeline-bundle"
var resultPathForDockerBuildOciTA = baseResultPath + "docker-build-oci-ta-pipeline-bundle"
var resultPathForDockerBuildOciTAMin = baseResultPath + "docker-build-oci-ta-min-pipeline-bundle"
var resultPathForDockerBuildMultiPlatformOciTA = baseResultPath + "docker-build-multi-platform-oci-ta-pipeline-bundle"
var resultPathForFbcBuilder = baseResultPath + "fbc-builder-pipeline-bundle"

// pipelineVsResultsMap contains pipeline name as key and related result path as value
var pipelineVsResultsMap = map[string]string{
	"docker-build":                       resultPathForDockerBuild,
	"docker-build-oci-ta":                resultPathForDockerBuildOciTA,
	"docker-build-oci-ta-min":            resultPathForDockerBuildOciTAMin,
	"docker-build-multi-platform-oci-ta": resultPathForDockerBuildMultiPlatformOciTA,
	"fbc-builder":                        resultPathForFbcBuilder,
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Please provide all the arguments. First value should be snapshot, second value should be snapshot_type and third value should be component_name")
		os.Exit(1)
	}
	snapshotString := os.Args[1]
	snapshotType := os.Args[2]
	componentName := os.Args[3]

	var err error
	updatedTaskBundleMap, err = ParseSnapshotImages(snapshotString, snapshotType, componentName)
	if err != nil {
		fmt.Printf("[ERROR] failed to parse snapshot with error: %v\n", err)
		os.Exit(1)
	}

	err = fetchListOfTasksInEachPipeline()
	if err != nil {
		fmt.Printf("[ERROR] failed to fetch list of tasks with error: %v\n", err)
		os.Exit(1)
	}

	for _, pipelineName := range pipelinesNameList {
		if isPipelineChanged(pipelineName) {
			newBuildPipeline, err := createBuildPipelineBundle(constants.BuildPipelineType(pipelineName))
			if err != nil {
				fmt.Printf("[ERROR] failed to create docker-build pipeline bundle with error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("[DEBUG] CREATED NEW PIPELINE BUNDLE %s FOR PIPELINE NAME %s\n", newBuildPipeline, pipelineName)
			resultPath := getResultPath(pipelineName)
			if resultPath != "" {
				writeResult(resultPath, newBuildPipeline)
			} else {
				fmt.Printf("[ERROR] could not find result path for the pipeline %s, exiting...", pipelineName)
				os.Exit(1)
			}
		} else {
			// If pipeline is not changed, then fetch and write default values of pipeline bundle references to result
			bundleRef, err := getDefaultPipelineBundleRef(pipelineName)
			if err != nil {
				fmt.Printf("[ERROR] could not get default pipeline bundle for the pipeline %s, exiting...", pipelineName)
				os.Exit(1)
			}
			resultPath := getResultPath(pipelineName)
			if resultPath != "" {
				writeResult(resultPath, bundleRef)
			} else {
				fmt.Printf("[ERROR] could not find result path for the pipeline %s, exiting...", pipelineName)
				os.Exit(1)
			}
		}
	}

}

func writeResult(resultPath, outputData string) {
	err := os.WriteFile(resultPath, []byte(outputData), 0644)
	if err != nil {
		fmt.Printf("[ERROR] failed to write result in resultPath %s with error: %v\n", resultPath, err)
		os.Exit(1)
	}
}

func isPipelineChanged(pipelineName string) bool {
	for taskName, _ := range updatedTaskBundleMap {
		if slices.Contains(pipelineNameVsTaskListMap[pipelineName], taskName) {
			return true
		}
	}
	return false
}

func getResultPath(pipelineName string) string {
	if resultPath, ok := pipelineVsResultsMap[pipelineName]; ok {
		return resultPath
	} else {
		return ""
	}
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

func fetchListOfTasksInEachPipeline() error {
	var pipelineBundle string
	var err error
	var tektonObj runtime.Object

	for _, pipelineName := range pipelinesNameList {
		if pipelineBundle, err = getDefaultPipelineBundleRef(pipelineName); err != nil {
			return fmt.Errorf("failed to fetch the pipeline bundle ref: %+v", err)
		}
		// Extract the pipeline as tekton object from the bundle
		if tektonObj, err = tekton.ExtractTektonObjectFromBundle(pipelineBundle, "pipeline", constants.BuildPipelineType(pipelineName)); err != nil {
			return fmt.Errorf("failed to extract the pipeline %s from bundle with error: %+v", pipelineName, err)
		}
		pipelineObject := tektonObj.(*tektonpipeline.Pipeline)
		for i := range pipelineObject.PipelineSpec().Tasks {
			t := &pipelineObject.PipelineSpec().Tasks[i]
			for _, param := range t.TaskRef.Params {
				if param.Name == "name" {
					pipelineNameVsTaskListMap[pipelineName] = append(pipelineNameVsTaskListMap[pipelineName], param.Value.StringVal)
				}
			}
		}
	}

	return nil
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
