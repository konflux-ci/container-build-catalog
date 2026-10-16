# Changelog

<!-- Format guidelines: https://keepachangelog.com/en/1.1.0/#how -->

## Unreleased

<!--
When you make changes without bumping the version right away, document them here.
If that's not something you ever plan to do, consider removing this section.
-->

*Nothing yet.*

## 0.3.2

### Changed

- Set `use-trusted-artifact` computeResources to project TA standard (4Gi memory
  req=lim, 50m CPU request, no limits.cpu) via recipe `additionalSteps` so
  values survive generate-ta / check-ta.

## 0.3.1

### Fixed
- Use Dockerfile as the file name in the uploaded artifact, regardless of the name of the actual file.

## 0.3

- Embedded Bash script is replaced with command `konflux-build-cli image push-containerfile`.

## 0.2

### Removed
- BREAKING: Support for Dockerfile downloading in Konflux Build Pipeline.

## 0.1

### Added

- The initial version of the `push-dockerfile-oci-ta` task!
