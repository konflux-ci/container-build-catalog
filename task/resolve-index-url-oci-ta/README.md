# resolve-index-url-oci-ta task

Resolve the RHAI Python package index URL from an AIPCC base image label. Parses the Containerfile to find the FROM base image, resolves ARG values using build-args-file and build-args, then reads the `com.redhat.aiplatform.index_url` label via skopeo inspect. The resolved URL is emitted as a result for use as `pip-index-url` in the prefetch-dependencies task. This enables downstream teams building on AIPCC base images to automatically pick up the correct index URL without hardcoding it.

## Parameters
|name|description|default value|required|
|---|---|---|---|
|BUILD_ARGS|Array of build arguments in key=value format. These override values from BUILD_ARGS_FILE.|[]|false|
|BUILD_ARGS_FILE|Path to a file with build arguments, relative to the source root. Used to resolve ARG values in the Containerfile.|""|false|
|CA_TRUST_CONFIG_MAP_KEY|The name of the key in the ConfigMap that contains the CA bundle data.|ca-bundle.crt|false|
|CA_TRUST_CONFIG_MAP_NAME|The name of the ConfigMap to read CA bundle data from.|trusted-ca|false|
|CONTEXT|Path to the directory to use as context.|.|false|
|DOCKERFILE|Path to the Containerfile.|./Dockerfile|false|
|INDEX_URL_LABEL|Label name to read from the base container image.|com.redhat.aiplatform.index_url|false|
|SOURCE_ARTIFACT|The Trusted Artifact URI pointing to the artifact with the application source code.||true|

## Results
|name|description|
|---|---|
|pip-index-url|Resolved Python package index URL from the base image label. Empty if the label is not found or the base image cannot be resolved.|

## Additional info
