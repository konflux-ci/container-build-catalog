# Changelog

<!-- Format guidelines: https://keepachangelog.com/en/1.1.0/#how -->

## Unreleased

<!--
When you make changes without bumping the version right away, document them here.
If that's not something you ever plan to do, consider removing this section.
-->

*Nothing yet.*

## 0.4.1

### Changed

- Set `use-trusted-artifact` computeResources to analyzer P95+5% (256Mi memory
  req=lim, 250m CPU request, no limits.cpu) via recipe `additionalSteps` so
  values survive generate-ta / check-ta. Left `get-base-images` and `build` at
  their existing defaults.

## 0.4.0

### Changed

- Renamed Trusted Artifact parameter `CACHI2_ARTIFACT` to `PREFETCH_ARTIFACT`.
- Renamed Trusted Artifact path `/var/workdir/cachi2` to `/var/workdir/prefetch`.

Recommended to use `prefetch-dependencies-oci-ta` >= 0.11.0 for naming consistency of the parameter.

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
