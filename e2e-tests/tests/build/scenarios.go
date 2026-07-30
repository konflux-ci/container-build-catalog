package build

import (
	"fmt"
	"strings"

	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	. "github.com/onsi/ginkgo/v2"
)

type TestScenarioSpec struct {
	Name                string                      // Mandatory: Unique name in string format
	Group               string                      // Mandatory: Group Name to group different sceanrios together
	RepoName            string                      // Mandatory: Git repo to be used during the test
	Host                string                      // Mandatory: git host where it is hosted, ex. github.com or gitlab.com
	GitURL              string                      // Optional: Constructed during the test execution
	Revision            string                      // Mandatory: Git Revision to be used during the test
	DefaultBranch       string                      // Optional: Default branch to be used during the test, ff not defined, "main" will be used
	AuthMode            string                      // Optional: Only for the gitlab related sceanrios
	ContextDir          string                      // Mandatory: Context directory in the test repository
	DockerFilePath      string                      // Mandatory: Docker file path in the test repository
	PipelineBundleName  constants.BuildPipelineType // Mandatory: Pipeline Bundle Type to be used during the test
	EnableHermetic      bool                        // Optional: Only applicable for the hermetic scenarios
	PrefetchInput       string                      // Optional: Only applicable for the hermetic scenarios
	CheckAdditionalTags bool                        // Optional: Only applicable for the additional tags test scenario
	ManifestMediaType   string                      // Mandatory: to verify the resulting image's media type
	OverrideMediaType   string                      // Optional: Only for the media type override verification scenarios
	WorkingDirMount     string                      // Optional: Only for the oci-archive test scenario
}

var TestScenarios = []TestScenarioSpec{
	{
		Name:               "sample-python-basic-oci-docker-build",
		Group:              "basic",
		RepoName:           "devfile-sample-python-basic",
		Host:               "github.com",
		Revision:           "47fc22092005aabebce233a9b6eab994a8152bbd",
		ContextDir:         ".",
		DockerFilePath:     constants.DockerFilePath,
		PipelineBundleName: constants.DockerBuild,
		EnableHermetic:     false,
		PrefetchInput:      "",
		ManifestMediaType:  "oci",
		OverrideMediaType:  "oci",
	},
	{
		Name:               "sample-python-basic-oci-docker-build-oci-ta",
		Group:              "basic",
		RepoName:           "devfile-sample-python-basic-oci-ta-clone",
		Host:               "github.com",
		Revision:           "47fc22092005aabebce233a9b6eab994a8152bbd",
		ContextDir:         ".",
		DockerFilePath:     constants.DockerFilePath,
		PipelineBundleName: constants.DockerBuildOciTA,
		EnableHermetic:     false,
		PrefetchInput:      "",
		ManifestMediaType:  "oci",
		OverrideMediaType:  "oci",
	},
	{
		Name:               "sample-python-basic-symlink",
		Group:              "basic",
		RepoName:           "devfile-sample-python-basic-symlink-clone",
		Host:               "github.com",
		Revision:           "27ecfca9c9dad35e4f07ebbcd706f31cb7ce849f",
		DefaultBranch:      "symlink",
		ContextDir:         ".",
		DockerFilePath:     constants.DockerFilePath,
		PipelineBundleName: constants.DockerBuild,
		EnableHermetic:     false,
		PrefetchInput:      "",
		ManifestMediaType:  "docker",
	},
	{
		Name:               "sample-python-basic-docker",
		Group:              "basic",
		RepoName:           "devfile-sample-python-basic-clone",
		Host:               "github.com",
		Revision:           "47fc22092005aabebce233a9b6eab994a8152bbd",
		ContextDir:         ".",
		DockerFilePath:     constants.DockerFilePath,
		PipelineBundleName: constants.DockerBuild,
		EnableHermetic:     false,
		PrefetchInput:      "",
		ManifestMediaType:  "docker",
	},
	{
		Name:               "multiarch-oci",
		Group:              "excluded",
		RepoName:           "multiarch-sample-repo",
		Host:               "github.com",
		Revision:           "bc0452861279eb59da685ba86918938c6c9d8310",
		ContextDir:         ".",
		DockerFilePath:     "Dockerfile",
		PipelineBundleName: constants.DockerBuildMultiPlatformOciTa,
		EnableHermetic:     false,
		PrefetchInput:      "",
		ManifestMediaType:  "oci",
		OverrideMediaType:  "oci",
	},
	{
		Name:               "multiarch-docker",
		Group:              "excluded",
		RepoName:           "multiarch-sample-repo-clone",
		Host:               "github.com",
		Revision:           "bc0452861279eb59da685ba86918938c6c9d8310",
		ContextDir:         ".",
		DockerFilePath:     "Dockerfile",
		PipelineBundleName: constants.DockerBuildMultiPlatformOciTa,
		EnableHermetic:     false,
		PrefetchInput:      "",
		ManifestMediaType:  "docker",
	},
	{
		Name:               "sample-gitlab-basic-auth",
		Group:              "basic",
		RepoName:           "sample-python-basic",
		Host:               "gitlab.com",
		Revision:           "47fc22092005aabebce233a9b6eab994a8152bbd",
		DefaultBranch:      "main",
		AuthMode:           "basic-auth",
		ContextDir:         ".",
		DockerFilePath:     constants.DockerFilePath,
		PipelineBundleName: constants.DockerBuildOciTAMin,
		EnableHermetic:     false,
		ManifestMediaType:  "docker",
	},
	{
		Name:               "fbc",
		Group:              "excluded",
		RepoName:           "fbc-sample-repo",
		Host:               "github.com",
		Revision:           "8e374e107fecf03f3c64c528bb53798039661414",
		ContextDir:         "4.13",
		DockerFilePath:     "catalog.Dockerfile",
		PipelineBundleName: constants.FbcBuilder,
		EnableHermetic:     false,
		PrefetchInput:      "",
		ManifestMediaType:  "oci",
	},
	{
		Name:               "from-scratch",
		Group:              "basic",
		RepoName:           "docker-file-from-scratch",
		Host:               "github.com",
		Revision:           "a3ea25fc3a1523db84ff96ee9958f637aea3abcd",
		ContextDir:         ".",
		DockerFilePath:     "Containerfile",
		PipelineBundleName: constants.DockerBuild,
		EnableHermetic:     false,
		PrefetchInput:      "",
		ManifestMediaType:  "docker",
	},
	{
		Name:               "oci-archive",
		Group:              "basic",
		RepoName:           "oci-archive-test",
		Host:               "github.com",
		Revision:           "a63b71ce92cee3a8d4624ef15a232d43f93b42b9",
		ContextDir:         ".",
		DockerFilePath:     "Dockerfile",
		PipelineBundleName: constants.DockerBuild,
		EnableHermetic:     false,
		PrefetchInput:      "",
		WorkingDirMount:    "/buildcontext",
		ManifestMediaType:  "oci",
		OverrideMediaType:  "oci",
	},
	{
		Name:               "prefetch-gomod",
		Group:              "hermetic",
		RepoName:           "retrodep",
		Host:               "github.com",
		Revision:           "d8e3195d1ab9dbee1f621e3b0625a589114ac80f",
		ContextDir:         ".",
		DockerFilePath:     "Dockerfile",
		PipelineBundleName: constants.DockerBuild,
		EnableHermetic:     true,
		PrefetchInput:      "gomod",
		ManifestMediaType:  "docker",
	},
	{
		Name:                "prefetch-pip",
		Group:               "hermetic",
		RepoName:            "pip-sample-repo",
		Host:                "github.com",
		Revision:            "1ecda839ba9ca55070d75c86c26a1bb07d777bba",
		ContextDir:          ".",
		DockerFilePath:      "Dockerfile",
		PipelineBundleName:  constants.DockerBuild,
		EnableHermetic:      true,
		PrefetchInput:       "pip",
		CheckAdditionalTags: true,
		ManifestMediaType:   "docker",
	},
	{
		Name:               "prefetch-bundler",
		Group:              "hermetic",
		RepoName:           "ruby-bundler-sample-app",
		Host:               "github.com",
		Revision:           "a38f17f2aceefcde5c8f9792b608fffdd204e3d6",
		ContextDir:         ".",
		DockerFilePath:     "Dockerfile",
		PipelineBundleName: constants.DockerBuild,
		EnableHermetic:     true,
		PrefetchInput:      "bundler",
		ManifestMediaType:  "docker",
	},
	{
		Name:               "prefetch-cargo",
		Group:              "hermetic",
		RepoName:           "rust-cargo-sample-app",
		Host:               "github.com",
		Revision:           "7aed0c607c1cb6a33239135a3bab9bd6e7a66049",
		ContextDir:         ".",
		DockerFilePath:     "Dockerfile",
		PipelineBundleName: constants.DockerBuild,
		EnableHermetic:     true,
		PrefetchInput:      "cargo",
		ManifestMediaType:  "docker",
	},
	{
		Name:               "prefetch-npm",
		Group:              "hermetic",
		RepoName:           "nodejs-npm-sample-repo",
		Host:               "github.com",
		Revision:           "23da12cd11784c3a25cb65445cb7ecad68e7ba25",
		ContextDir:         ".",
		DockerFilePath:     "Dockerfile",
		PipelineBundleName: constants.DockerBuild,
		EnableHermetic:     true,
		PrefetchInput:      "npm",
		ManifestMediaType:  "docker",
	},
	{
		Name:               "prefetch-yarn-classic",
		Group:              "hermetic",
		RepoName:           "nodejs-yarn-sample-app",
		Host:               "github.com",
		Revision:           "20e4aad4d5ddc79f87137a4c285b4067e21aa982",
		ContextDir:         ".",
		DockerFilePath:     "Dockerfile",
		PipelineBundleName: constants.DockerBuild,
		EnableHermetic:     true,
		PrefetchInput:      "yarn",
		ManifestMediaType:  "docker",
	},
	{
		Name:               "prefetch-yarn-modern",
		Group:              "hermetic",
		RepoName:           "nodejs-yarn-modern-sample-app",
		Host:               "github.com",
		Revision:           "0060a06e9b84e5b3c24a896cb23ac865a5205ab1",
		ContextDir:         ".",
		DockerFilePath:     "Dockerfile",
		PipelineBundleName: constants.DockerBuild,
		EnableHermetic:     true,
		PrefetchInput:      "yarn",
		ManifestMediaType:  "docker",
	},
	{
		Name:               "prefetch-rpm",
		Group:              "hermetic",
		RepoName:           "rpm-sample-app",
		Host:               "github.com",
		Revision:           "3a3fb169e0c8998b51d7403ba934de5c1f194b1d",
		ContextDir:         ".",
		DockerFilePath:     "Containerfile",
		PipelineBundleName: constants.DockerBuild,
		EnableHermetic:     true,
		PrefetchInput:      "rpm",
		ManifestMediaType:  "docker",
	},
	{
		Name:               "prefetch-generic",
		Group:              "hermetic",
		RepoName:           "generic-fetcher-sample-app",
		Host:               "github.com",
		Revision:           "d08d8d4e79d2a2f1f1c28c55cd8fbdc6c344ca14",
		ContextDir:         ".",
		DockerFilePath:     "Dockerfile",
		PipelineBundleName: constants.DockerBuild,
		EnableHermetic:     true,
		PrefetchInput:      "generic",
		ManifestMediaType:  "docker",
	},
	{
		Name:               "source-build-parent-image-with-digest-only",
		Group:              "source-build",
		RepoName:           "source-build-parent-image-with-digest-only",
		Host:               "github.com",
		Revision:           "a4f744581c0768eb84a4345f11d04090bb14bdff",
		ContextDir:         ".",
		DockerFilePath:     "Dockerfile",
		PipelineBundleName: constants.DockerBuild,
		EnableHermetic:     false,
		PrefetchInput:      "",
		ManifestMediaType:  "docker",
	},
	{
		Name:               "source-build-use-latest-parent-image",
		Group:              "source-build",
		RepoName:           "source-build-use-latest-parent-image",
		Host:               "github.com",
		Revision:           "b4584ac47e1df84114a10debf262b6d40f6a95f8",
		ContextDir:         ".",
		DockerFilePath:     "Dockerfile",
		PipelineBundleName: constants.DockerBuild,
		EnableHermetic:     false,
		PrefetchInput:      "",
		ManifestMediaType:  "docker",
	},
	{
		Name:               "source-build-parent-image-from-registry-rh-io",
		Group:              "source-build",
		RepoName:           "source-build-parent-image-from-registry-rh-io",
		Host:               "github.com",
		Revision:           "3f5dcac703a35dcb7b29312be72f86221d0f10ee",
		ContextDir:         ".",
		DockerFilePath:     "Dockerfile",
		PipelineBundleName: constants.DockerBuild,
		EnableHermetic:     false,
		PrefetchInput:      "",
		ManifestMediaType:  "docker",
	},
	{
		Name:               "source-build-base-on-konflux-image",
		Group:              "source-build",
		RepoName:           "source-build-base-on-konflux-image",
		Host:               "github.com",
		Revision:           "b6960c7602f21c531e3ead4df1dd1827e6f208f6",
		ContextDir:         ".",
		DockerFilePath:     "Dockerfile",
		PipelineBundleName: constants.DockerBuild,
		EnableHermetic:     false,
		PrefetchInput:      "",
		ManifestMediaType:  "docker",
	},
}

// GetScenario returns the TestScenarioSpec with the matching scenario name
func GetScenario(scenarioName string) TestScenarioSpec {
	for _, testScenario := range TestScenarios {
		if testScenario.Name == scenarioName {
			return testScenario
		}
	}
	return TestScenarioSpec{}
}

// GetScenarioEntries returns the entries for scenarios to run
func GetScenarioEntries() []TableEntry {
	var entries []TableEntry
	// Scenario group takes precendence than scenario list if both set
	scenario_group := utils.GetEnv(SCENARIO_GROUP_ENV, "")
	if scenario_group == "" {
		fmt.Println("scenario group is empty")
		scenario_list := utils.GetEnv(SCENARIO_LIST_ENV, "sample-python-basic-oci-docker-build-oci-ta")
		scenarioNames := strings.Split(scenario_list, ",")
		for _, scenarioName := range scenarioNames {
			entries = append(entries, Entry("", scenarioName))
		}
		return entries
	} else if scenario_group == "all" {
		fmt.Println("Scenario group is all, so include both basic and hermetic scenario")
		for _, testScenario := range TestScenarios {
			if testScenario.Group == "basic" || testScenario.Group == "hermetic" {
				entries = append(entries, Entry("", testScenario.Name))
			}
		}
		return entries
	} else {
		fmt.Println("For all other cases, run basic scenarios")
		for _, testScenario := range TestScenarios {
			if testScenario.Group == "basic" {
				entries = append(entries, Entry("", testScenario.Name))
			}
		}
		return entries
	}
}
