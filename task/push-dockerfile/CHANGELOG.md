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

- Version bump in lockstep with `push-dockerfile-oci-ta` 0.3.2 (oci-ta
  `use-trusted-artifact` computeResources). No functional change in this base
  task; ta-generator copies `app.kubernetes.io/version` from the base recipe.

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

- The initial version of the `push-dockerfile` task!
