# Changelog

## Unreleased

<!--
When you make changes without bumping the version right away, document them here.
If that's not something you ever plan to do, consider removing this section.
-->

*Nothing yet.*

## 0.1

### Added

- Initial implementation of the resolve-index-url task.
  Resolves the RHAI Python package index URL from an AIPCC base image
  by parsing the Containerfile, resolving ARG values from build-args-file
  and build-args, and reading the `com.redhat.aiplatform.index_url` label
  via skopeo inspect. Emits the resolved URL as `pip-index-url` result
  for use with the prefetch-dependencies task.
