# Changelog

<!-- Format guidelines: https://keepachangelog.com/en/1.1.0/#how -->

## Unreleased

<!--
When you make changes without bumping the version right away, document them here.
If that's not something you ever plan to do, consider removing this section.
-->

*Nothing yet.*

## 0.4.0

### Changed

- Renamed step env var `CACHI2_ARTIFACTS_DIR` to `PREFETCH_ARTIFACTS_DIR`.
- Prefetch artifacts directory `$(workspaces.workspace.path)/cachi2` is now `$(workspaces.workspace.path)/prefetch`.

Requires `prefetch-dependencies` >= 0.11.0

## 0.3.2

### Fixed

- Cargo prefetched dependencies are now included in the source image. They are
  vendored as unpacked source trees rather than archives, so previously they
  were missed by the archive-type filter and left out of the source image.

## 0.3.1

### Changed

- Nothing. Started using semver specification for version labels.

## 0.3

### Added

- BREAKING: new required parameter: `BINARY_IMAGE_DIGEST`.

  For pipelines that include the `build-image-index` task, pass its result to this param:

  ```yaml
  - name: BINARY_IMAGE_DIGEST
    value: "$(tasks.build-image-index.results.IMAGE_DIGEST)"
  ```

  For those that don't, pass the result from the `build-container` task instead:

  ```yaml
  - name: BINARY_IMAGE_DIGEST
    value: "$(tasks.build-container.results.IMAGE_DIGEST)"
  ```

  *Note: the MintMaker PR updating source-build to v0.3 should apply these changes automatically.*

- Started tracking changes in this file.
