# Changelog

<!-- Format guidelines: https://keepachangelog.com/en/1.1.0/#how -->

## Unreleased

### Changed

- Right-size `use-trusted-artifact` to fleet MAX+5% `768Mi`/`300m` (was generator `3Gi`/`50m`) via recipe `additionalSteps`.

## 0.3.2

### Changed

- Add generator default `use-trusted-artifact` computeResources (`3Gi`/`50m`) via `ta-generator@v1.2.0`.

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
