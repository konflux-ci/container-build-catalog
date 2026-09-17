# resolve-index-url task

Resolve the RHAI Python package index URL from an AIPCC base image label. Parses the Containerfile to find the FROM base image, resolves ARG values using build-args-file and build-args, then reads the `com.redhat.aiplatform.index_url` label via skopeo inspect. The resolved URL is emitted as a result for use as `pip-index-url` in the prefetch-dependencies task. This enables downstream teams building on AIPCC base images to automatically pick up the correct index URL without hardcoding it.

## Parameters
|name|description|default value|required|
|---|---|---|---|
|DOCKERFILE|Path to the Containerfile.|./Dockerfile|false|
|CONTEXT|Path to the directory to use as context.|.|false|
|BUILD_ARGS_FILE|Path to a file with build arguments, relative to the source root. Used to resolve ARG values in the Containerfile.|""|false|
|BUILD_ARGS|Array of build arguments in key=value format. These override values from BUILD_ARGS_FILE.|[]|false|
|INDEX_URL_LABEL|Label name to read from the base container image.|com.redhat.aiplatform.index_url|false|
|CA_TRUST_CONFIG_MAP_NAME|The name of the ConfigMap to read CA bundle data from.|trusted-ca|false|
|CA_TRUST_CONFIG_MAP_KEY|The name of the key in the ConfigMap that contains the CA bundle data.|ca-bundle.crt|false|

## Results
|name|description|
|---|---|
|pip-index-url|Resolved Python package index URL from the base image label. Empty if the label is not found or the base image cannot be resolved.|

## How it works

1. Locates the Containerfile at `CONTEXT/DOCKERFILE` (or `DOCKERFILE`) in the source checkout
2. Reads build arguments from `BUILD_ARGS_FILE` and `BUILD_ARGS`
3. Extracts the first `FROM` instruction and resolves any `${ARG}` references
4. Runs `skopeo inspect` on the resolved base image to read the label specified by `INDEX_URL_LABEL`
5. Writes the label value to the `pip-index-url` result

If any step fails (Containerfile not found, image not resolvable, label not present), the task emits an empty string and exits successfully, allowing the pipeline to continue with PyPI defaults.

## Pipeline wiring example

```yaml
- name: resolve-index-url
  params:
    - name: DOCKERFILE
      value: $(params.dockerfile)
    - name: CONTEXT
      value: $(params.path-context)
    - name: BUILD_ARGS_FILE
      value: $(params.build-args-file)
    - name: BUILD_ARGS
      value: $(params.build-args)
  runAfter:
    - clone-repository
  taskRef:
    name: resolve-index-url
  workspaces:
    - name: source
      workspace: workspace

- name: prefetch-dependencies
  params:
    - name: pip-index-url
      value: $(tasks.resolve-index-url.results.pip-index-url)
    # ...other params
  runAfter:
    - resolve-index-url
```

## Additional info
