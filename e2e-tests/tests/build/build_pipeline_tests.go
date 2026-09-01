package build

import (
	"fmt"
	"strings"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/clients/has"
	"github.com/konflux-ci/e2e-tests/pkg/clients/ociregistry"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils/build"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/image/reference"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"knative.dev/pkg/apis"
)

var _ = framework.BuildSuiteDescribe("Build pipeline E2E test", Label("build-pipeline-e2e"), func() {
	defer GinkgoRecover()

	scenarioEntries := GetScenarioEntries()
	DescribeTableSubtree("Build pipeline", func(scenarioName string) {
		// separate Describe is needed to avoid skipping tests for other scenarios when one fails
		Describe(fmt.Sprintf("scenario %q", scenarioName), Ordered, func() {
			var f *framework.Framework
			var err error
			var application *appservice.Application
			var testNamespace, applicationName string
			var componentName string
			var component *appservice.Component
			var scenario TestScenarioSpec
			var plr *pipeline.PipelineRun

			BeforeAll(func() {
				f, err = framework.NewFramework(fmt.Sprintf("build-e2e-%s", scenarioName))
				Expect(err).ShouldNot(HaveOccurred(), "failed to create test namespace")
				testNamespace = f.UserNamespace

				applicationName = fmt.Sprintf("application-%s", util.GenerateRandomString(4))
				application, err = f.AsKubeAdmin.HasController.CreateApplication(applicationName, testNamespace)
				Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("failed to create application %s", applicationName))

				componentName = fmt.Sprintf("component-%s-%s", scenarioName, util.GenerateRandomString(4))
				component, err = CreateComponent(f, scenarioName, testNamespace, componentName, application)
				Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("failed to create component for scenario: %s", scenarioName))

				scenario = GetScenario(scenarioName)
				GinkgoWriter.Printf("RUNNING SCENARIO: %s\n", scenario.Name)
			})

			AfterAll(func() {
				if !CurrentSpecReport().Failed() {
					Eventually(func() error {
						return f.AsKubeAdmin.HasController.DeleteAllComponentsInASpecificNamespace(testNamespace, time.Minute*2)
					}, 2*time.Minute, 10*time.Second).Should(Succeed())
					Eventually(func() error {
						return f.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(testNamespace, time.Minute*2)
					}, 2*time.Minute, 10*time.Second).Should(Succeed())
				}

				// Cleanup github scenario branches
				for _, branches := range GithubPacAndBaseBranches {
					err = f.AsKubeAdmin.CommonController.Github.DeleteRef(branches.RepoName, branches.PacBranchName)
					if err != nil {
						Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
					}
					err = f.AsKubeAdmin.CommonController.Github.DeleteRef(branches.RepoName, branches.BaseBranchName)
					if err != nil {
						Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
					}
				}

				// Cleanup gitlab scenario branches
				for _, branches := range GitlabPacAndBaseBranches {
					err = f.AsKubeAdmin.CommonController.Gitlab.DeleteBranch(branches.RepoName, branches.PacBranchName)
					if err != nil {
						Expect(err.Error()).To(ContainSubstring("404 Not Found"))
					}
					err = f.AsKubeAdmin.CommonController.Gitlab.DeleteBranch(branches.RepoName, branches.BaseBranchName)
					if err != nil {
						Expect(err.Error()).To(ContainSubstring("404 Not Found"))
					}
				}
			})
			It("triggers a PipelineRun", func() {
				timeout := 5 * time.Minute
				Eventually(func() error {
					plr, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("PipelineRun has not been created yet for the component %s/%s\n", testNamespace, componentName)
						return err
					}
					if !plr.HasStarted() {
						return fmt.Errorf("pipelinerun %s/%s hasn't started yet", plr.GetNamespace(), plr.GetName())
					}
					return nil
				}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", componentName, testNamespace))
			})
			It("the PipelineRun should eventually finish", func() {
				if scenario.DefaultBranch == "symlink" {
					var podLogs string
					Eventually(func() bool {
						plr, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
						if err != nil {
							GinkgoWriter.Printf("failed to get the pipelinerun for the component %s/%s\n", testNamespace, componentName)
							return false
						}
						prReason := plr.GetStatusCondition().GetCondition(apis.ConditionSucceeded).GetReason()
						if prReason != "Failed" {
							GinkgoWriter.Printf("current pipelinerun reason: %v\n", prReason)
							return false
						}
						podLogs, err = GetPipelineRunPodLogs(f.AsKubeAdmin.CommonController, plr.Name, testNamespace)
						return prReason == "Failed" && err == nil && strings.Contains(podLogs, "symlink check: found 1 symlink(s) pointing outside the directory")
					}, 5*time.Minute, constants.PipelineRunPollingInterval).Should(BeTrue(), "symlink pipelinerun is not failed with correct error message as expected")
				} else {
					Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "", "", "",
						f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, plr)).To(Succeed())
				}
			})
			It("should push Dockerfile to registry", func() {
				if scenario.DefaultBranch == "symlink" || scenario.PipelineBundleName == constants.DockerBuildOciTAMin || scenario.PipelineBundleName == constants.FbcBuilder {
					Skip("Skipping push Dockerfile to registry validation, it is not applicable here")
					return
				}
				EnsureOriginalDockerfileIsPushed(f.AsKubeAdmin, plr)
			})
			It("should validate tekton taskrun test results", func() {
				if scenario.DefaultBranch == "symlink" {
					Skip("Skipping tekton task results validation, not applicable")
					return
				}
				Expect(build.ValidateBuildPipelineTestResults(plr, f.AsKubeAdmin.CommonController.KubeRest(), scenario.PipelineBundleName == constants.FbcBuilder, scenario.PipelineBundleName == constants.DockerBuildOciTAMin)).To(Succeed())
			})
			It("floating tags are created successfully", func() {
				if !scenario.CheckAdditionalTags {
					Skip(fmt.Sprintf("floating tag validation is not needed for: %s", scenarioName))
				}
				builtImage := build.GetBinaryImage(plr)
				Expect(builtImage).ToNot(BeEmpty(), "built image url is empty")
				builtImageRef, err := reference.Parse(builtImage)
				Expect(err).ShouldNot(HaveOccurred(),
					fmt.Sprintf("cannot parse image pullspec: %s", builtImage))
				for _, tagName := range additionalTags {
					_, err := build.GetImageTag(builtImageRef.Namespace, builtImageRef.Name, tagName)
					Expect(err).ShouldNot(HaveOccurred(),
						fmt.Sprintf("failed to get tag %s from image repo", tagName),
					)
				}
			})
			It("image manifest mediaType is correct", func() {
				if scenario.DefaultBranch == "symlink" {
					Skip(fmt.Sprintf("mediaType validation is not required for scenario: %s", scenarioName))
				} else {
					builtImage := build.GetBinaryImage(plr)
					switch scenario.ManifestMediaType {
					case "docker":
						if scenario.PipelineBundleName == constants.FbcBuilder || scenario.PipelineBundleName == constants.DockerBuildMultiPlatformOciTa {
							// Check for docker.manifest.list mediaType
							Expect(build.GetBuiltImageManifestMediaType(builtImage)).Should(Equal(build.MediaTypeDockerManifestList), "mediaType of the image manifest is not of type docker.manifest.list")
						} else {
							// Check for docker.manifest mediaType
							Expect(build.GetBuiltImageManifestMediaType(builtImage)).Should(Equal(build.MediaTypeDockerManifest), "mediaType of the image manifest is not of type docker.manifest")
						}
					case "oci":
						if scenario.PipelineBundleName == constants.FbcBuilder || scenario.PipelineBundleName == constants.DockerBuildMultiPlatformOciTa {
							// Check for oci image index mediaType
							Expect(build.GetBuiltImageManifestMediaType(builtImage)).Should(Equal(build.MediaTypeOciImageIndex), "mediaType of the image manifest is not of type oci.image.index")
						} else {
							// Check for oci image manifest mediaType
							Expect(build.GetBuiltImageManifestMediaType(builtImage)).Should(Equal(build.MediaTypeOciManifest), "mediaType of the image is not of type oci.image.manifest")
						}
					default:
						Fail(fmt.Sprintf("Unknown ManifestMediaType value %s in scenario \n", scenario.ManifestMediaType))
					}
				}
			})
			It("should ensure pruning labels are set", func() {
				// Check pruning labels in only one scenario
				if scenario.Name == "sample-python-basic-oci-docker-build" {
					var image *v1.ConfigFile
					Eventually(func() error {
						image, err = build.ImageFromPipelineRun(plr)
						return err
					}, time.Minute*2, time.Second*10).Should(Succeed(), "timed out while trying fetch image config")

					labels := image.Config.Labels
					Expect(labels).ToNot(BeEmpty())

					expiration, ok := labels["quay.expires-after"]
					Expect(ok).To(BeTrue())
					Expect(expiration).To(Equal(constants.DefaultImageTagExpiration))
				} else {
					Skip(fmt.Sprintf("pruning label validation is not required for scenario: %s", scenarioName))
				}
			})
			It("should have Hermeto content in the SBOM in case the build was hermetic", func() {
				if !scenario.EnableHermetic {
					Skip("Hermetic build is not enabled, skipping the test")
				}

				taskRun, err := f.AsKubeAdmin.TektonController.GetTaskRunFromPipelineRun(f.AsKubeAdmin.CommonController.KubeRest(), plr, "build-container")
				Expect(err).NotTo(HaveOccurred())

				var sbomBlobUrl string

				for _, r := range taskRun.Status.Results {
					if r.Name == "SBOM_BLOB_URL" {
						sbomBlobUrl = r.Value.StringVal
					}
				}
				Expect(sbomBlobUrl).NotTo(BeEmpty())

				imageRef, err := reference.Parse(sbomBlobUrl)
				Expect(err).NotTo(HaveOccurred())

				c := ociregistry.NewOciRegistryV2Client(imageRef.Registry)

				sbom, err := build.FetchSbomFromRegistry(c, imageRef.Namespace, imageRef.Name, imageRef.ID)
				Expect(err).NotTo(HaveOccurred())

				hasHermetoPackages := false
				for _, pkg := range sbom.GetPackages() {
					if pkg.GetCreatedBy() == build.SbomPackageCreatedByHermeto {
						hasHermetoPackages = true
						break
					}
				}
				Expect(hasHermetoPackages).To(BeTrue(), "no hermeto packages found")
			})
			It("check for source images if enabled in pipeline", func() {
				if scenario.PipelineBundleName == constants.FbcBuilder || scenario.PipelineBundleName == constants.DockerBuildOciTAMin {
					Skip(fmt.Sprintf("Not applicable, skiping source image validation for the scenario %s", scenarioName))
					return
				}

				isSourceBuildEnabled := build.IsSourceBuildEnabled(plr)
				GinkgoWriter.Printf("Source build is enabled: %v\n", isSourceBuildEnabled)
				if !isSourceBuildEnabled {
					Skip("Skipping source image check since it is not enabled in the pipeline")
				}

				binaryImage := build.GetBinaryImage(plr)
				if binaryImage == "" {
					Fail("Failed to get the binary image url from pipelinerun")
				}

				binaryImageRef, err := reference.Parse(binaryImage)
				Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("cannot parse binary image pullspec %s", binaryImage))

				tagInfo, err := build.GetImageTag(binaryImageRef.Namespace, binaryImageRef.Name, binaryImageRef.Tag)
				Expect(err).ShouldNot(HaveOccurred(),
					fmt.Sprintf("failed to get tag %s info for constructing source container image", binaryImageRef.Tag),
				)

				srcImageRef := reference.DockerImageReference{
					Registry:  binaryImageRef.Registry,
					Namespace: binaryImageRef.Namespace,
					Name:      binaryImageRef.Name,
					Tag:       fmt.Sprintf("%s.src", strings.Replace(tagInfo.ManifestDigest, ":", "-", 1)),
				}
				srcImage := srcImageRef.String()
				tagExists, err := build.DoesTagExistsInQuay(srcImage)
				Expect(err).ShouldNot(HaveOccurred(),
					fmt.Sprintf("failed to check existence of source container image %s", srcImage))
				Expect(tagExists).To(BeTrue(),
					fmt.Sprintf("cannot find source container image %s", srcImage))

				CheckSourceImage(srcImage, scenario.GitURL, scenario.PipelineBundleName, f.AsKubeAdmin, plr)
			})
		})

	}, scenarioEntries)
})
