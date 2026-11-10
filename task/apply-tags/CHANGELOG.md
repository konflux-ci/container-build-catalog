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

- BREAKING: set empty string as default value for `ADDITIONAL_TAGS_FROM_LABEL` task parameter.
  Now `konflux.additional-tags` (or another label to look for additional tags) needs to be provided explicitly.
  This is done for optimization purposes to not to check the default label when it's not needed.

## 0.3.2

### Added

- `ADDITIONAL_TAGS_FROM_LABEL` parameter to specify image label to read additional tags from.
  For now set previously hardcoded value `konflux.additional-tags` by default to avoid changing the task behavior.

## 0.3.1

### Changed

- Nothing. Started using semver specification for version labels.

## 0.3

- Switched from bash implementation to Konflux Build CLI.
- Deprecated older `0.1` and `0.2` versions.
