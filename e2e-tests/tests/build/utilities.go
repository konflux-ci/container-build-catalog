package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	remoteimg "github.com/google/go-containerregistry/pkg/v1/remote"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/clients/oras"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/build"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/image/reference"
	tektonpipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

type TestBranches struct {
	RepoName       string
	PacBranchName  string
	BaseBranchName string
}

var GithubPacAndBaseBranches []TestBranches
var GitlabPacAndBaseBranches []TestBranches

func CreateComponent(f *framework.Framework, scenarioName, testNamespace, componentName string, application *appservice.Application) (*appservice.Component, error) {
	var err error
	var gitUrl string
	scenario := GetScenario(scenarioName)
	pipelineBundleName := scenario.PipelineBundleName
	Expect(pipelineBundleName).ShouldNot(BeEmpty())
	customBuildBundle := getDefaultPipeline(pipelineBundleName)
	if scenario.EnableHermetic {
		//Update the build pipeline bundle with param hermetic=true
		customBuildBundle, err = enableHermeticBuildInPipelineBundle(customBuildBundle, pipelineBundleName, scenario.PrefetchInput)
		if err != nil {
			return nil, fmt.Errorf("failed to enable hermetic build in the pipeline bundle with: %v", err)
		}
	}
	if scenario.OverrideMediaType != "" {
		// Update the pipeline bundle with updating BUILDAH_FORMAT value
		customBuildBundle, err = enableDockerMediaTypeInPipelineBundle(customBuildBundle, pipelineBundleName, scenario.OverrideMediaType)
		if err != nil {
			return nil, fmt.Errorf("failed to update BUILDAH_FORMAT in the pipeline bundle with: %v", err)
		}
	}
	if scenario.CheckAdditionalTags {
		//Update the pipeline bundle to apply additional tags
		customBuildBundle, err = applyAdditionalTagsInPipelineBundle(customBuildBundle, pipelineBundleName, additionalTags)
		if err != nil {
			return nil, fmt.Errorf("failed to apply additinal tags in the pipeline bundle with: %v", err)
		}
	}
	if scenario.WorkingDirMount != "" {
		//Update the pipeline bundle to apply WORKINGDIR_MOUNT
		customBuildBundle, err = addWorkingDirMountInPipelineBundle(customBuildBundle, pipelineBundleName, scenario.WorkingDirMount)
		if err != nil {
			return nil, fmt.Errorf("failed to apply WORKINGDIR_MOUNT in the pipeline bundle with: %v", err)

		}
	}
	buildPipelineAnnotation := map[string]string{
		"build.appstudio.openshift.io/pipeline": fmt.Sprintf(`{"name":"%s", "bundle": "%s"}`, pipelineBundleName, customBuildBundle),
	}
	baseBranchName := fmt.Sprintf("base-%s", util.GenerateRandomString(6))
	pacBranchName := constants.PaCPullRequestBranchPrefix + componentName

	if scenario.DefaultBranch == "" {
		scenario.DefaultBranch = "main"
	}

	if scenario.Host == "github.com" {
		err = f.AsKubeAdmin.CommonController.Github.CreateRef(scenario.RepoName, scenario.DefaultBranch, scenario.Revision, baseBranchName)
		Expect(err).ShouldNot(HaveOccurred())
		gitUrl = fmt.Sprintf(githubUrlFormat, githubOrg, scenario.RepoName)
		scenario.GitURL = gitUrl
		// construct to cleanup the branches later
		GithubPacAndBaseBranches = append(GithubPacAndBaseBranches, TestBranches{
			RepoName:       scenario.RepoName,
			PacBranchName:  pacBranchName,
			BaseBranchName: baseBranchName,
		})

	} else if scenario.Host == "gitlab.com" && scenario.AuthMode == "basic-auth" {
		gitlabToken := utils.GetEnv(constants.GITLAB_BOT_TOKEN_ENV, "")
		if gitlabToken == "" {
			return nil, fmt.Errorf("gitlab token env is empty, must be set for running the gitlab scenario")
		}
		projectID := fmt.Sprintf("%s/%s", gitlabOrg, scenario.RepoName)
		gitUrl = fmt.Sprintf(gitlabbUrlFormat, gitlabOrg, scenario.RepoName)
		scenario.GitURL = gitUrl
		err = f.AsKubeAdmin.CommonController.Gitlab.CreateGitlabNewBranch(projectID, baseBranchName, scenario.Revision, scenario.DefaultBranch)
		Expect(err).ShouldNot(HaveOccurred(), "failed to create new branch in gitlab")
		// create basic-auth build secret
		secretAnnotations := map[string]string{}
		err = CreateGitlabBuildSecret(f, gitlabBasicAuthSecretName, secretAnnotations, gitlabToken, application)
		Expect(err).ShouldNot(HaveOccurred(), "failed to create basic auth secret")

		GitlabPacAndBaseBranches = append(GitlabPacAndBaseBranches, TestBranches{
			RepoName:       projectID,
			PacBranchName:  pacBranchName,
			BaseBranchName: baseBranchName,
		})
	} else {
		return nil, fmt.Errorf("Unknown host %s in the scenario, yet to be implemented", scenario.Host)
	}

	componentObj := appservice.ComponentSpec{
		ComponentName: componentName,
		Source: appservice.ComponentSource{
			ComponentSourceUnion: appservice.ComponentSourceUnion{
				GitSource: &appservice.GitSource{
					URL:           gitUrl,
					Revision:      baseBranchName,
					Context:       scenario.ContextDir,
					DockerfileURL: scenario.DockerFilePath,
				},
			},
		},
	}

	c, err := f.AsKubeAdmin.HasController.CreateComponentCheckImageRepository(componentObj, testNamespace, "", "", application.Name, false, utils.MergeMaps(utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationRequestPublicRepo), buildPipelineAnnotation))
	Expect(err).ShouldNot(HaveOccurred())
	Expect(c.Name).Should(Equal(componentName))

	GinkgoWriter.Printf("Created component for scenario: %s\n", scenarioName)

	return c, nil
}

func getDefaultPipeline(pipelineBundleName constants.BuildPipelineType) string {
	switch pipelineBundleName {
	case "docker-build":
		return utils.GetEnv(constants.CUSTOM_DOCKER_BUILD_PIPELINE_BUNDLE_ENV, "quay.io/konflux-ci/tekton-catalog/pipeline-docker-build:devel")
	case "docker-build-oci-ta":
		return utils.GetEnv(constants.CUSTOM_DOCKER_BUILD_OCI_TA_PIPELINE_BUNDLE_ENV, "quay.io/konflux-ci/tekton-catalog/pipeline-docker-build-oci-ta:devel")
	case "docker-build-oci-ta-min":
		return utils.GetEnv(constants.CUSTOM_DOCKER_BUILD_OCI_TA_MIN_PIPELINE_BUNDLE_ENV, "quay.io/konflux-ci/tekton-catalog/pipeline-docker-build-oci-ta-min:devel")
	case "docker-build-multi-platform-oci-ta":
		return utils.GetEnv(constants.CUSTOM_DOCKER_BUILD_OCI_MULTI_PLATFORM_TA_PIPELINE_BUNDLE_ENV, "quay.io/konflux-ci/tekton-catalog/pipeline-docker-build-multi-platform-oci-ta:devel")
	case "fbc-builder":
		return utils.GetEnv(constants.CUSTOM_FBC_BUILDER_PIPELINE_BUNDLE_ENV, "quay.io/konflux-ci/tekton-catalog/pipeline-fbc-builder:devel")
	default:
		return ""
	}
}

// CreateGitlabBuildSecret creates a Kubernetes secret for GitLab build credentials
func CreateGitlabBuildSecret(f *framework.Framework, secretName string, annotations map[string]string, token string, application *appservice.Application) error {
	ownerRef := metav1.OwnerReference{
		APIVersion: "appstudio.redhat.com/v1alpha1",
		Kind:       "Application",
		Name:       application.Name,
		UID:        application.UID,
	}
	buildSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            secretName,
			Namespace:       f.UserNamespace,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
			Labels: map[string]string{
				"appstudio.redhat.com/credentials": "scm",
				"appstudio.redhat.com/scm.host":    "gitlab.com",
			},
		},
		Type: "kubernetes.io/basic-auth",
		StringData: map[string]string{
			"password": token,
		},
	}
	if annotations != nil {
		buildSecret.Annotations = annotations
	}
	_, err := f.AsKubeAdmin.CommonController.CreateSecret(f.UserNamespace, &buildSecret)
	if err != nil {
		return fmt.Errorf("error creating build secret: %v", err)
	}
	return nil
}

// this function takes a bundle and prefetchInput value as inputs and creates a bundle with param hermetic=true
// and then push the bundle to quay using format: quay.io/<QUAY_E2E_ORGANIZATION>/test-images:<generated_tag>
func enableHermeticBuildInPipelineBundle(customDockerBuildBundle string, pipelineBundleName constants.BuildPipelineType, prefetchInput string) (string, error) {
	var tektonObj runtime.Object
	var err error
	var newPipelineYaml []byte
	var authenticator authn.Authenticator
	// Extract docker-build pipeline as tekton object from the bundle
	if tektonObj, err = tekton.ExtractTektonObjectFromBundle(customDockerBuildBundle, "pipeline", pipelineBundleName); err != nil {
		return "", fmt.Errorf("failed to extract the Tekton Pipeline from bundle: %+v", err)
	}
	dockerPipelineObject := tektonObj.(*tektonpipeline.Pipeline)
	// Update hermetic params value to true and also update prefetch-input param value
	for i := range dockerPipelineObject.PipelineSpec().Params {
		if dockerPipelineObject.PipelineSpec().Params[i].Name == "hermetic" {
			dockerPipelineObject.PipelineSpec().Params[i].Default.StringVal = "true"
		}
		if dockerPipelineObject.PipelineSpec().Params[i].Name == "prefetch-input" {
			dockerPipelineObject.PipelineSpec().Params[i].Default.StringVal = prefetchInput
		}
	}
	if newPipelineYaml, err = yaml.Marshal(dockerPipelineObject); err != nil {
		return "", fmt.Errorf("error when marshalling a new pipeline to YAML: %v", err)
	}

	tag := fmt.Sprintf("%d-%s", time.Now().Unix(), util.GenerateRandomString(4))
	quayOrg := utils.GetEnv(constants.QUAY_E2E_ORGANIZATION_ENV, constants.DefaultQuayOrg)
	newDockerBuildPipelineImg := strings.ReplaceAll(constants.DefaultImagePushRepo, constants.DefaultQuayOrg, quayOrg)
	var newDockerBuildPipeline, _ = name.ParseReference(fmt.Sprintf("%s:pipeline-bundle-%s", newDockerBuildPipelineImg, tag))
	// Build and Push the tekton bundle
	if authenticator, err = utils.GetAuthenticatorForImageRef(newDockerBuildPipeline, os.Getenv("QUAY_TOKEN")); err != nil {
		return "", fmt.Errorf("error when getting authenticator: %v", err)
	}
	authOption := remoteimg.WithAuth(authenticator)
	if err = tekton.BuildAndPushTektonBundle(newPipelineYaml, newDockerBuildPipeline, authOption); err != nil {
		return "", fmt.Errorf("error when building/pushing a tekton pipeline bundle: %v", err)
	}
	return newDockerBuildPipeline.String(), nil
}

// this function takes a bundle and mediaType value as inputs and creates a bundle with param BUILDAH_FORMAT=<mediaType>
// and then push the bundle to quay using format: quay.io/<QUAY_E2E_ORGANIZATION>/test-images:<generated_tag>
func enableDockerMediaTypeInPipelineBundle(customDockerBuildBundle string, pipelineBundleName constants.BuildPipelineType, mediaType string) (string, error) {
	var tektonObj runtime.Object
	var err error
	var newPipelineYaml []byte
	var authenticator authn.Authenticator
	// Extract docker-build pipeline as tekton object from the bundle
	if tektonObj, err = tekton.ExtractTektonObjectFromBundle(customDockerBuildBundle, "pipeline", pipelineBundleName); err != nil {
		return "", fmt.Errorf("failed to extract the Tekton Pipeline from bundle: %+v", err)
	}
	dockerPipelineObject := tektonObj.(*tektonpipeline.Pipeline)
	// Update BUILDAH_FORMAT params value to <mediaType> (received as a function input) only for the required tasks
	for i := range dockerPipelineObject.PipelineSpec().Tasks {
		t := &dockerPipelineObject.PipelineSpec().Tasks[i]
		if t.Name == "build-container" || t.Name == "build-image-index" || t.Name == "sast-coverity-check" || t.Name == "build-images" {
			exist := false
			for param_idx := range t.Params {
				param := &t.Params[param_idx]
				if param.Name == "BUILDAH_FORMAT" {
					param.Value = *tektonpipeline.NewStructuredValues(mediaType)
					exist = true
					break
				}
			}
			if !exist {
				// param wasn't updated, add it as new param
				t.Params = append(t.Params, tektonpipeline.Param{Name: "BUILDAH_FORMAT", Value: *tektonpipeline.NewStructuredValues(mediaType)})
			}
		}
	}
	if newPipelineYaml, err = yaml.Marshal(dockerPipelineObject); err != nil {
		return "", fmt.Errorf("error when marshalling a new pipeline to YAML: %v", err)
	}

	tag := fmt.Sprintf("%d-%s", time.Now().Unix(), util.GenerateRandomString(4))
	quayOrg := utils.GetEnv(constants.QUAY_E2E_ORGANIZATION_ENV, constants.DefaultQuayOrg)
	newDockerBuildPipelineImg := strings.ReplaceAll(constants.DefaultImagePushRepo, constants.DefaultQuayOrg, quayOrg)
	var newDockerBuildPipeline, _ = name.ParseReference(fmt.Sprintf("%s:pipeline-bundle-%s", newDockerBuildPipelineImg, tag))
	// Build and Push the tekton bundle
	if authenticator, err = utils.GetAuthenticatorForImageRef(newDockerBuildPipeline, os.Getenv("QUAY_TOKEN")); err != nil {
		return "", fmt.Errorf("error when getting authenticator: %v", err)
	}
	authOption := remoteimg.WithAuth(authenticator)
	if err = tekton.BuildAndPushTektonBundle(newPipelineYaml, newDockerBuildPipeline, authOption); err != nil {
		return "", fmt.Errorf("error when building/pushing a tekton pipeline bundle: %v", err)
	}
	return newDockerBuildPipeline.String(), nil

}

// this function takes a bundle and additonalTags string slice as inputs
// and creates a bundle with adding ADDITIONAL_TAGS params in the apply-tags task
// and then push the bundle to quay using format: quay.io/<QUAY_E2E_ORGANIZATION>/test-images:<generated_tag>
func applyAdditionalTagsInPipelineBundle(customDockerBuildBundle string, pipelineBundleName constants.BuildPipelineType, additionalTags []string) (string, error) {
	var tektonObj runtime.Object
	var err error
	var newPipelineYaml []byte
	var authenticator authn.Authenticator
	// Extract docker-build pipeline as tekton object from the bundle
	if tektonObj, err = tekton.ExtractTektonObjectFromBundle(customDockerBuildBundle, "pipeline", pipelineBundleName); err != nil {
		return "", fmt.Errorf("failed to extract the Tekton Pipeline from bundle: %+v", err)
	}
	dockerPipelineObject := tektonObj.(*tektonpipeline.Pipeline)
	// Update ADDITIONAL_TAGS params arrays with additionalTags in apply-tags task
	for i := range dockerPipelineObject.PipelineSpec().Tasks {
		t := &dockerPipelineObject.PipelineSpec().Tasks[i]
		if t.Name == "apply-tags" {
			t.Params = append(t.Params, tektonpipeline.Param{Name: "ADDITIONAL_TAGS", Value: *tektonpipeline.NewStructuredValues(additionalTags[0], additionalTags[1:]...)})
		}
	}

	if newPipelineYaml, err = yaml.Marshal(dockerPipelineObject); err != nil {
		return "", fmt.Errorf("error when marshalling a new pipeline to YAML: %v", err)
	}

	tag := fmt.Sprintf("%d-%s", time.Now().Unix(), util.GenerateRandomString(4))
	quayOrg := utils.GetEnv(constants.QUAY_E2E_ORGANIZATION_ENV, constants.DefaultQuayOrg)
	newDockerBuildPipelineImg := strings.ReplaceAll(constants.DefaultImagePushRepo, constants.DefaultQuayOrg, quayOrg)
	var newDockerBuildPipeline, _ = name.ParseReference(fmt.Sprintf("%s:pipeline-bundle-%s", newDockerBuildPipelineImg, tag))
	// Build and Push the tekton bundle
	if authenticator, err = utils.GetAuthenticatorForImageRef(newDockerBuildPipeline, os.Getenv("QUAY_TOKEN")); err != nil {
		return "", fmt.Errorf("error when getting authenticator: %v", err)
	}
	authOption := remoteimg.WithAuth(authenticator)
	if err = tekton.BuildAndPushTektonBundle(newPipelineYaml, newDockerBuildPipeline, authOption); err != nil {
		return "", fmt.Errorf("error when building/pushing a tekton pipeline bundle: %v", err)
	}
	return newDockerBuildPipeline.String(), nil
}

// this function takes a bundle and workindDirMount string as inputs
// and creates a bundle with added WORKINDDIR_MOUNT param in the buildah task
// and then pushes the bundle to quay using format: quay.io/<QUAY_E2E_ORGANIZATION>/test-images:<generated_tag>
func addWorkingDirMountInPipelineBundle(customDockerBuildBundle string, pipelineBundleName constants.BuildPipelineType, workingDirMount string) (string, error) {
	var tektonObj runtime.Object
	var err error
	var newPipelineYaml []byte
	var authenticator authn.Authenticator
	// Extract docker-build pipeline as tekton object from the bundle
	if tektonObj, err = tekton.ExtractTektonObjectFromBundle(customDockerBuildBundle, "pipeline", pipelineBundleName); err != nil {
		return "", fmt.Errorf("failed to extract the Tekton Pipeline from bundle: %+v", err)
	}
	dockerPipelineObject := tektonObj.(*tektonpipeline.Pipeline)
	// Update WORKINGDIR_MOUNT param value for build-container task
	for i := range dockerPipelineObject.PipelineSpec().Tasks {
		t := &dockerPipelineObject.PipelineSpec().Tasks[i]
		if t.Name == "build-container" {
			t.Params = append(t.Params, tektonpipeline.Param{Name: "WORKINGDIR_MOUNT", Value: tektonpipeline.ParamValue{
				Type:      tektonpipeline.ParamTypeString,
				StringVal: workingDirMount,
			}})
		}
	}
	if newPipelineYaml, err = yaml.Marshal(dockerPipelineObject); err != nil {
		return "", fmt.Errorf("error when marshalling a new pipeline to YAML: %v", err)
	}

	tag := fmt.Sprintf("%d-%s", time.Now().Unix(), util.GenerateRandomString(4))
	quayOrg := utils.GetEnv(constants.QUAY_E2E_ORGANIZATION_ENV, constants.DefaultQuayOrg)
	newDockerBuildPipelineImg := strings.ReplaceAll(constants.DefaultImagePushRepo, constants.DefaultQuayOrg, quayOrg)
	var newDockerBuildPipeline, _ = name.ParseReference(fmt.Sprintf("%s:pipeline-bundle-%s", newDockerBuildPipelineImg, tag))
	// Build and Push the tekton bundle
	if authenticator, err = utils.GetAuthenticatorForImageRef(newDockerBuildPipeline, os.Getenv("QUAY_TOKEN")); err != nil {
		return "", fmt.Errorf("error when getting authenticator: %v", err)
	}
	authOption := remoteimg.WithAuth(authenticator)
	if err = tekton.BuildAndPushTektonBundle(newPipelineYaml, newDockerBuildPipeline, authOption); err != nil {
		return "", fmt.Errorf("error when building/pushing a tekton pipeline bundle: %v", err)
	}
	return newDockerBuildPipeline.String(), nil

}

func EnsureOriginalDockerfileIsPushed(hub *framework.ControllerHub, pr *tektonpipeline.PipelineRun) {
	binaryImage := build.GetBinaryImage(pr)
	Expect(binaryImage).ShouldNot(BeEmpty())

	binaryImageRef, err := reference.Parse(binaryImage)
	Expect(err).Should(Succeed())

	tagInfo, err := build.GetImageTag(binaryImageRef.Namespace, binaryImageRef.Name, binaryImageRef.Tag)
	Expect(err).Should(Succeed())

	dockerfileImageTag := fmt.Sprintf("%s.dockerfile", strings.Replace(tagInfo.ManifestDigest, ":", "-", 1))

	dockerfileImage := reference.DockerImageReference{
		Registry:  binaryImageRef.Registry,
		Namespace: binaryImageRef.Namespace,
		Name:      binaryImageRef.Name,
		Tag:       dockerfileImageTag,
	}.String()
	exists, err := build.DoesTagExistsInQuay(dockerfileImage)
	Expect(err).Should(Succeed())
	Expect(exists).Should(BeTrue(), fmt.Sprintf("image doesn't exist: %s", dockerfileImage))

	// Ensure the original Dockerfile used for build was pushed
	c := hub.CommonController.KubeRest()
	originDockerfileContent, err := build.ReadDockerfileUsedForBuild(c, hub.TektonController, pr)
	Expect(err).Should(Succeed())

	storePath, err := oras.PullArtifacts(dockerfileImage)
	Expect(err).Should(Succeed())
	entries, err := os.ReadDir(storePath)
	Expect(err).Should(Succeed())
	for _, entry := range entries {
		if entry.Type().IsRegular() && entry.Name() == "Dockerfile" {
			content, err := os.ReadFile(filepath.Join(storePath, entry.Name()))
			Expect(err).Should(Succeed())
			Expect(string(content)).Should(Equal(string(originDockerfileContent)))
			return
		}
	}
	Fail(fmt.Sprintf("Dockerfile is not found from the pulled artifacts for %s", dockerfileImage))
}
