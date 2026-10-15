# E2E Tests - Build Pipeline Scenarios

This document explains the test scenarios defined in `tests/build/scenarios.go` and how scenarios are grouped and their selection logic.

## Task and Pipeline Bundle Modes

- **Task mode** tests changed task bundles by injecting them into released pipeline bundles.
- **Pipeline mode** tests changed pipeline bundles from the Snapshot as-is; unchanged pipeline flavors use their default `:devel` bundles.
- If a change includes both tasks and pipelines, run two independent integration tests (ITs): task mode with the changed task injected into released pipelines, and pipeline mode with the new pipeline bundle as-is.

See the [pipeline bundle management design](../docs/design/pipeline-bundle-management.md) for the rationale and details.

When running `konflux-e2e-tests` from a fork or feature branch, set IntegrationTestScenario pipeline parameters `catalog-git-url` and `catalog-git-revision` to the same values as the scenario git resolver (Tekton cannot use `prepare-e2e` results in git task resolvers).

For **group** integration tests, set pipeline parameter `e2e-bundle-mode` to `task` or `pipeline` on the scenario (task application group vs pipeline application group). Component tests infer mode from the component name. If a group PR changes both task and pipeline catalog directories, `e2e-bundle-mode` is required on the scenario.

## createBundle unit tests

PRs that change `createBundle.go`, `createBundle_test.go`, or `.github/workflows/e2e-createbundle-unit-tests.yaml` run `go test` in this directory via the **createBundle unit tests** workflow.

## Scenario Groups

Scenarios are organized into the following groups:

### 1. `basic` group
Basic test scenarios covering common build patterns and configurations.

**Purpose**: These scenarios acts as sanity/smoke tests and cover the most common use cases.

### 2. `hermetic` group

Hermetic build related scenarios that test dependency prefetching for various package managers.

**Purpose**: Test hermetic builds where dependencies are prefetched and builds are isolated from external networks.

### 3. `source-build` group
Source container build scenarios that test different source build related functionality.

**Purpose**: Test source container generation with various parent image scenarios.

### 4. `excluded` group
Scenarios that are not included currently in the regular test runs. Blocked due to multi-platform-controller is not available in upstream konflux environment yet.

- `multiarch-oci` - Multi-architecture build with OCI media type
- `multiarch-docker` - Multi-architecture build with Docker media type
- `fbc` - File-Based Catalog (FBC) builder pipeline


## Scenario Selection Logic

The `GetScenarioEntries()` function determines which scenarios to run based on environment variables:

### Environment Variables

1. **`SCENARIO_GROUP`**: Selects scenarios by group (takes precedence over scenario list)
2. **`SCENARIO_LIST`**: Comma-separated list of specific scenario names to run

If both values are not set in the running environment, scenario named `sample-python-basic-oci-docker-build-oci-ta` will run.

### Examples

**Default behavior** (no environment variables):
```bash
# Runs only: sample-python-basic-oci-docker-build-oci-ta
ginkgo --v --label-filter="build-pipeline-e2e" ./tests/build/
```

**Run specific scenarios**:
```bash
export SCENARIO_LIST="sample-python-basic-docker,from-scratch,oci-archive"
ginkgo --v --label-filter="build-pipeline-e2e" ./tests/build/
```

**Run basic scenarios**:
```bash
export SCENARIO_GROUP="basic"
ginkgo --v --label-filter="build-pipeline-e2e" ./tests/build/
```

**Run all the basic and hermetic scenarios**:
```bash
export SCENARIO_GROUP="all"
ginkgo --v --label-filter="build-pipeline-e2e" ./tests/build/
```

**Note**
We are not including `source-build` related scenarios when setting `all`, since we already run them as part of `build-tasks-dockerfiles` repository [CI](https://github.com/konflux-ci/build-tasks-dockerfiles/blob/8892d0bbfd5caa01a91b69aac7ee8f457ae78271/integration-tests/tasks/e2e-test.yaml#L24) and to avoid rerunning them again here.
Currently `source-build` scenarios are kept in `scenarios.go` to enable them when we move `source-build` task functionality fully to the container-build-catalog repository.

## Adding New Scenarios

To add a new test scenario:

1. Add a new `TestScenarioSpec` entry to the `TestScenarios` slice
2. Assign an appropriate `Group` value
3. Ensure all mandatory fields are populated
4. Add optional fields as needed for specific test requirements
5. Document the scenario purpose in comments if it's non-obvious

Example:
```go
{
    Name:               "my-new-scenario",
    Group:              "basic",  // or "hermetic", "source-build", etc.
    RepoName:           "my-test-repo",
    Host:               "github.com",
    Revision:           "abc123...",
    ContextDir:         ".",
    DockerFilePath:     "Dockerfile",
    PipelineBundleName: constants.DockerBuild,
    ManifestMediaType:  "docker",
},
```
