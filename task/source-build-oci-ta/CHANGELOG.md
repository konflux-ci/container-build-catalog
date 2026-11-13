# Changelog

<!-- Format guidelines: https://keepachangelog.com/en/1.1.0/#how -->

## Unreleased

### Changed

- Raise `use-trusted-artifact` to fleet MAX+5% `3Gi`/`700m` (was under-sized at `256Mi`/`250m`).

## 0.4.1

### Changed

- Override `use-trusted-artifact` to analyzer P95+5% `256Mi`/`250m` via recipe `additionalSteps` (below `ta-generator@v1.2.0` default `3Gi`) (KONFLUX-16160).

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
