# Changelog

<!-- Format guidelines: https://keepachangelog.com/en/1.1.0/#how -->

## Unreleased

<!--
When you make changes without bumping the version right away, document them here.
If that's not something you ever plan to do, consider removing this section.
-->

*Nothing yet.*

## 0.12.2

### Changed

- Parameter `CONTEXTUALIZE_SBOM` is now set to `false` by default. More
  information on this changed was provided in the changelog for 0.12.0.

## 0.12.1

### Changed

- Bump `prepare-sboms` step memory from 256Mi to 512Mi (requests = limits) to prevent OOM kills on large container images (GPU/ML, bootc, driver-toolkit).
- Remove `prepare-sboms` CPU limit (was 100m) to allow burst CPU and prevent throttling. CPU requests remain at 100m.

## 0.12.0

### Changed

- Parameter `CONTEXTUALIZE_SBOM` is now set to `false` by default. The SBOM
  contextualization received an overhaul, enabling the support for builder
  content contextualization in SBOMs. To get involved in UAT, set this value
  to `true` and report [issues](https://github.com/konflux-ci/mobster/issues)
  to Mobster maintainers.
- When `CONTEXTUALIZE_SBOM` is set to `true`, the built image will contain
  new labels, `io.buildah.stage.name` and `io.buildah.stage.base`.

## 0.11.2

### Fixed

- Per-arch RPM filtering for the prefetch SBOM.
  - With buildah task versions >= 0.10.0, < 0.11.2, the final SBOM would always
    include the `x86_64` RPMs (and no other arches) from the prefetch SBOM,
    even for images built on other arches.
  - Now, the SBOM will contain the RPMs for the corresponding arch.

## 0.11.1

### Changed

- Set a 5 minute I/O timeout for rsync transfers to and from the build VMs.
  The build will fail if the connection goes 5 minutes without transfering
  a single byte of data.

### Fixed

- Stopped rsyncing /var/workdir from build VMs back to the cluster.
  This directory contains the git repository and prefetched dependencies,
  which can be a lot of data. The rsync back was an unfortunate side effect
  of how tooling generates the remote-oci-ta task variant from the base task,
  and was completely unnecessary.

## 0.11.0

### Changed

- When scanning the built image with Syft, scans the image filesystem as
  a directory instead of scanning the the image as an OCI archive. This improves
  the scanning time, disk usage and may improve memory usage. More details in
  [konflux-build-cli/docs/design/syft-image-scanning.md].
- The remote variants of this task now push the built image to the registry directly
  from the build VM instead of rsyncing the image back to the cluster first.
  For large images, this significantly reduces the time spent on network transfers.

### Removed

- BREAKING: Removed the `sbom-syft-generate` step, SBOM generation now happens
  in the `build` step.
- BREAKING: Removed the `push` step, the push now happens in the `build` step.
- If you have step overrides configured for either of the two removed steps,
  the pipeline will fail with `invalid StepOverride`. See the migration guidance below.

### Migration guidance

Buildah v0.11.0 comes with a migration script that will attempt to automatically
fix the step overrides in your PipelineRuns. In most cases, no manual action will
be needed. But there are cases that the script cannot handle:

1. Your PipelineRun references an external Pipeline. In this case, the migration
   script will never get a chance to run on the PipelineRun.
2. Your PipelineRun is multi-platform, SBOM generation needs more resources
   than the build itself and the remote VMs do not have sufficient resources.

If the migration script doesn't solve the problem, please follow the procedure below.

#### Manual procedure

If you have `sbom-syft-generate` or `push` step overrides in the `.spec.taskRunSpecs`
section in your PipelineRun, please remove them. In most cases, this should be all.

However, if you were previously requesting more resources for SBOM generation
than for the build step itself, there is a chance that the build will fail.
In this case, move the relevant overrides to the build step. The same technically
applies for the push step, but it's highly unlikely that pushing would require
more resources than the build.

For example:

```diff
 spec:
   taskRunSpecs:
     - pipelineTaskName: build-container
       stepSpecs:
-        - name: sbom-syft-generate
+        - name: build
           computeResources:
             requests:
               memory: 16Gi
             limits:
               memory: 16Gi
```

This will work for build steps that run in-cluster - single-platform builds
and typically also the amd64 builds in a multi-platform build setup.

For build steps that run on remote VMs, the overrides have no effect. In case
the build fails, please switch to a larger VM flavor (consult the documentation
of your particular Konflux deployment to see what's available).

For example:

```diff
 spec:
   params:
     - name: build-platforms
       value:
         - localhost
-        - linux/arm64
+        - linux-mxlarge/arm64
```

[konflux-build-cli/docs/design/syft-image-scanning.md]: https://github.com/konflux-ci/konflux-build-cli/blob/3fe637dbb77f05e107682c186446bb027bf98f86/docs/design/syft-image-scanning.md

## 0.10.7

### Fixed

- Thanks to [konflux-build-cli#208], the task now supports per-containerfile
  ignore files, same as buildah itself.
  - Previously, the task supported the `.containerignore` and `.dockerignore` files
    in the root of the context directory, but not the `<containerfile>.containerignore`
    and `<containerfile>.dockerignore` files.

[konflux-build-cli#208]: https://github.com/konflux-ci/konflux-build-cli/pull/208

## 0.10.6

### Fixed

- Attaching SBOM attestations using keyless signing. This stopped working between
  versions 0.10.4 and 0.10.5, when the upload-sbom step upgraded cosign to v3.
  - Cosign v3 wants to use a "signing config" JSON file instead of accepting
    service URLs directly as CLI flags. The konflux-ci/konflux-ci deployment
    of Konflux doesn't provide the config file in the TUF mirror. Fixed
    by setting `--use-signing-config=false` to still allow direct URLs.

### Changed

- Consolidated all keyless signing code in the upload-sbom step.
  Previously, if keyless signing was enabled, the task would sign the image
  in the push step and then the SBOM in upload-sbom step. Now, it will sign both
  in the upload-sbom step. This has no practical impact, but enables a larger
  rework of the push step in the future.

## 0.10.5

### Added

- Added a new parameter RHSM_MOUNT_CA_CERTS to allow setting [konflux-build-cli]'s
  `--rhsm-mount-ca-certs` option.

## 0.10.4

### Fixed

- Restores the `/cachi2/cachi2.env` mount that version 0.10.3 removed.
  Despite being an undocumented implementation detail, some builds use
  the presence of this file as an indicator that the build is hermetic.
  Enable them to do so for the time being.

## 0.10.3

### Added

- Added new parameter ALLOW_CROSS_PLATFORM_IMAGES to allow usage of parent images
  which don't match build host architecture.

### Changed

- When used with prefetch-dependencies >= 0.3.1, no longer edits RUN instructions
  in order to set the prefetch environment variables. Instead, sets the variables
  using buildah `--mount` + `--secret` flags. Details in [konflux-build-cli#151].
  Fixes [#1200].
  - As a side effect, also no longer mounts the `/cachi2/cachi2.env` file,
    whose only purpose was to enable the automatic setting of prefetch variables.

[konflux-build-cli#151]: https://github.com/konflux-ci/konflux-build-cli/pull/151

## 0.10.2

### Fixed

- The injected `labels.json` file will now better match the actual image labels
  in cases when the containerfile includes quoted `LABEL` values. This is a result
  of [dockerfile-json#16].

[dockerfile-json#16]: https://github.com/konflux-ci/dockerfile-json/pull/16

## 0.10.1

### Changed

- Updated the image that runs the `build` and `push` steps.
  Notably, the new image comes with Buildah `v1.44.0`.

## 0.10

This version introduces [konflux-build-cli]. The `build` step replaces most of the Bash with
`konflux-build-cli image build`. Other steps still use Bash, this will change soon.

We expect version 0.10 to behave the same as version 0.9 for the vast majority
of use cases. All known (minor) differences documented below.

### Added

- The `vcs-url` label. Previously, the task would inject the following vcs-related labels:
  - `org.opencontainers.image.revision` and its [legacy counterpart][projectatomic-labels],
    `vcs-ref`
  - `org.opencontainers.image.source` and nothing else
    - Version 0.10 adds the missing legacy counterpart, `vcs-url`

### Changed

- The precedence of default annotations (those injected by the task automatically)
  - Before: `ANNOTATIONS_FILE` < `ANNOTATIONS` < default annotations
  - Now: default annotations < `ANNOTATIONS_FILE` < `ANNOTATIONS`
- When handling the `YUM_REPOS_D_SRC` and `YUM_REPOS_D_FETCHED` directories,
  injects only regular files into `/etc/yum.repos.d`. Previously, the task would
  inject the directories as a whole. `/etc/yum.repos.d` is a flat structure, so
  the task now injects only regular files to avoid injecting unexpected content.
- Prefetch integration:
  - Looks for both `prefetch.env` and `cachi2.env` in the prefetch dir (in this order).
    Version 0.3.1 of the prefetch task added `prefetch.env` and a future version
    will remove `cachi2.env`.
  - Doesn't rely specifically on `cachi2.repo` files to enable RPM integration,
    just needs any `*.repo` file at the expected path.
  - In case the `YUM_REPOS_D_SRC` or `YUM_REPOS_D_FETCHED` directories contain
    a repo file with the same name as the repo file from Hermeto, the Hermeto
    repo takes precedence. Previously, `YUM_REPOS_*` would take precedence.
  - Doesn't copy the prefetch files to `/tmp`, instead copies them to a directory
    on the same filesystem as the original files. This uses copy-on-write and avoids
    duplicating the underlying data.
- Red Hat subscription-manager integration:
  - Will mount the RHSM CA certificates into the build in two cases:
    - When using `ACTIVATION_KEY` and the containerfile doesn't include
      `subscription-manager register` (same as before)
    - When using `ENTITLEMENT_SECRET` (not done before and should have been)
  - When mounting RHSM CA certificates, mounts the whole `/etc/rhsm/ca` directory
    instead of mounting a specific file. This closes [#1621].

### Fixed

- Injecting metadata to `/usr/share/buildinfo` and `/root/buildinfo`:
  - Does not write any new files or modify any existing files in the source directory,
    injects the files using a separate build-context.
  - Will log a warning if the `TARGET` param is set and `SKIP_INJECTIONS=false`
    (using `TARGET` disables metadata injection anyway). Metadata injection never
    worked with a non-default target, version 0.10 just adds the warning.
  - Injecting `labels.json`:
    - Will skip LABEL instructions in stages that don't affect the labels of the final image.
    - Will correctly omit the `io.buildah.version` label when `SOURCE_DATE_EPOCH` is non-empty.
      Previously, `labels.json` would always include `io.buildah.version`.
- Pre-pulling base images for hermetic builds and base-arch verification (see [0.9.4](#094)):
  - Also pulls images referenced in `COPY --from=$image` and `RUN --mount=from=$image`.
    Previously, would only pull images referenced as `FROM $image`.
  - Does not pull images for unused stages (unless `SKIP_UNUSED_STAGES=false`).
  - Will skip image references with [transports][containers-transports] that don't
    represent pullable images. Specifically, will only pull transport-less references
    and `docker://` references. Previously, the task would skip `oci-archive:` references
    but fail on any other kind of non-standard reference.
- Modifying the containerfile to set prefetch environment variables in RUN instructions:
  - No longer mangles RUN instructions that use the exec form or a bare here-doc.
    Instead skips the instruction and logs a warning.

    ```dockerfile
    RUN ["echo", "skips exec-form commands"]

    RUN <<EOF
    echo "skips bare heredocs"
    EOF

    RUN bash -e <<EOF
    echo "supports heredocs if they start with something other than the <<marker"
    EOF
    ```

    - This partially fixes [#1200], in the sense that the containerfile at least
      doesn't become broken. The unsupported instructions don't automatically get
      the variables that may be required to make the hermetic build work though.
  - Fixes dozens of small bugs that most users never would have hit. For example,
    version 0.10:
    - Doesn't mangle heredoc lines that look line `RUN` instructions
    - Doesn't inject text into the middle of a string with quoted/escaped whitespace
    - Properly handles [backtick-escaped][dockerfile-escape] containerfiles

[konflux-build-cli]: https://github.com/konflux-ci/konflux-build-cli
[projectatomic-labels]: https://github.com/projectatomic/ContainerApplicationGenericLabels
[containers-transports]: https://www.mankier.com/5/containers-transports
[#1200]: https://github.com/konflux-ci/build-definitions/issues/1200
[dockerfile-escape]: https://docs.docker.com/reference/dockerfile/#escape
[#1621]: https://github.com/konflux-ci/build-definitions/issues/1621

## 0.9.4

### Fixed

- Validate base image architecture before build. The task now fails if a base image
  doesn't match the host architecture, preventing silent emulation builds.

## 0.9.3

### Fixed

- Added `--fail` flag and error handling to the `curl` call that retrieves the SSH key from the OTP
  server. Previously, HTTP errors (e.g. 400 when a one-time token was already consumed by a
  PipelineRun retry) were silently swallowed, writing the error body into `~/.ssh/id_rsa` and
  causing `Load key: invalid format` build failures.

## 0.9.2

### Changed

- The task now only stores one local OCI directory copy of the built image, not two.

## 0.9.1

### Changed

- The buildah image now uses version 1.4.1 of [konflux-ci/task-runner](https://github.com/konflux-ci/task-runner)
  - This version pulls in version 1.42.1 of syft that ensures 'redhat' is used as the namespace for hummingbird rpms

## 0.9

### Removed
- BREAKING: Support for Dockerfile downloading in Konflux Build Pipeline.

## 0.8.3

### Fixed

- Platform build arguments (BUILDPLATFORM, TARGETPLATFORM) now correctly include CPU variant
  for ARM architectures (e.g., `linux/arm/v7` or `linux/arm64/v8` instead of just `linux/arm`
  or `linux/arm64`).

## 0.8.2

### Changed

- The task now makes sure that only RPMs that match the architecture being built are
  passed to the `buildah bud` command. It also removes the same packages from the
  Hermeto SBOM to more accurately represent the build.

## 0.8.1

### Added

- The buildah task now supports injecting ENV variables into the dockerfile
  through the `ENV_VARS` array parameter.

## 0.8

### Changed

- The buildah image that runs the task now uses
  [konflux-ci/task-runner](https://github.com/konflux-ci/task-runner) as the base
  image and gets both the `buildah` binary and the relevant configuration from there.
  - This updates the `buildah` version from 1.41.5 to 1.42.2

## 0.7.1

### Added

- Started tracking changes in this file.
