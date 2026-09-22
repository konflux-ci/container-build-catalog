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
		Name:               "prefetch-cargo",
		Group:              "hermetic",
		RepoName:           "rust-cargo-sample-app",
		Host:               "github.com",
		Revision:           "60c3ec5b2bf1f94b07fec9f8b5926c5a0c285e7f",
		ContextDir:         ".",
		DockerFilePath:     "Dockerfile",
		PipelineBundleName: constants.DockerBuild,
		EnableHermetic:     true,
		PrefetchInput:      "cargo",
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
	scenario_group := "all"
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
