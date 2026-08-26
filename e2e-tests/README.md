# E2E Tests - Build Pipeline Scenarios

This document explains the test scenarios defined in `tests/build/scenarios.go` and how scenarios are grouped and their selection logic.

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
