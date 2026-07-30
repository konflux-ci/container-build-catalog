package build

import (
	"github.com/devfile/library/v2/pkg/util"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
)

const (
	SCENARIO_LIST_ENV  string = "SCENARIO_LIST"
	SCENARIO_GROUP_ENV string = "SCENARIO_GROUP"

	githubUrlFormat  = "https://github.com/%s/%s"
	gitlabbUrlFormat = "https://gitlab.com/%s/%s"
)

var (
	additionalTags            = []string{"test-tag1", "test-tag2"}
	githubOrg                 = utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe")
	gitlabOrg                 = utils.GetEnv(constants.GITLAB_QE_ORG_ENV, "konflux-qe")
	gitlabBasicAuthSecretName = "gitlab-basic-auth-secret-" + util.GenerateRandomString(4)
)
