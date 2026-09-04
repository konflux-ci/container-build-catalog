# Managing Pipeline bundles via Konflux

## Context

The Konflux Build team publishes pipeline bundles to `quay.io/konflux-ci/tekton-catalog/pipeline-*:devel`.
These bundles provide the default pipeline definition that a user gets when they onboard to Konflux.

Today, the [build-definitions] CI is what creates these bundles.
We want to move most of the pipeline definitions to this repo and release them using Konflux.

The build-definitions pipeline works like this:

1. For tasks defined in the same repo:
    1. Push new task bundles for all tasks that changed in the PR.
    2. Determine the latest matching task bundle for the tasks that did not change in the PR.
2. For tasks defined in an external repo:
    1. Take the task bundle value from `external-task/{name}/{version}/{name}.yaml`
3. Replace taskRef placeholders in the pipeline yaml with the resolved task bundle references.
4. Push the pipeline bundle.

> [!NOTE]
> The taskRef placeholders in the pipeline yaml look something like this:
>
> ```yaml
> kind: Pipeline
> spec:
>   tasks:
>     - name: build-container
>       taskRef:
>         name: buildah-oci-ta
>         version: current
>     - name: sast-unicode-check
>       taskRef:
>         name: sast-unicode-check-oci-ta
>         version: "0.4"
> ```
>
> The `version` attributes make this an invalid Pipeline object. A build script in the pipeline
> replaces them with valid bundle resolver parameters.

In the context of the build-definitions repo, this setup has some advantages:

1. Different teams can self-service their task updates without needing to be owners of the pipelines.
   They update the task yaml, which they own, and the build script automatically injects
   the reference of the new task bundle into the pipeline yaml.
    - This will not be relevant after moving the pipelines out of build-definitions.
      We will not be giving other teams merge permissions.
2. It's possible to update a task and a pipeline that references this task in the same PR.
   If the pipeline yaml referenced the resolved task bundle, we would first have to merge
   a PR that updates the task, wait for the task to get released, and then update the pipeline.

This setup also has disadvantages:

1. The pipeline files are not valid Pipeline objects.
   We cannot easily run schema validation on them or use them to run a build, for example.
2. The actual Pipeline gets assembled dynamically at push time and we can't see the final result
   before it gets released.
3. The workflow depends on a highly customized CI pipeline that has full control of building
   *and releasing* the task bundles.
   It would be hard (but possible) to replicate using a standard-ish Konflux approach.

## Decision

For external tasks (tasks defined in repos other than this one), we will use their resolved bundle
references directly in the pipeline yaml. Considering we have no intention of letting other teams
self-service task updates, this a strictly simpler and better solution:

- No need for the `external-task/` tree and the logic that resolves placeholders for these.
- Pipeline source files are closer to what actually gets released.
- **When a new task version comes with a migration script, we get the changes applied for free.**
  - The files will be valid-ish Pipeline files. If we enable MintMaker for them,
    it will apply migration scripts the same way it does for `.tekton/` Pipelines and PipelineRuns.

For local tasks (tasks defined in this repo), we will go with the following option:

### A) Handle local tasks the same way as external tasks

Use the resolved bundle references directly in the pipeline yaml.

Pros:

1. Same simple approach for all tasks.
2. No custom logic, the default tkn-bundle task just works.
3. The source files are 1:1 identical to the released files.
4. The source files are fully valid Pipelines.

Cons:

1. Not possible to update a task and a pipeline that references it in the same PR.
    - The flow would be to merge a task update first and wait for MintMaker to update the pipeline.
      A nice side effect is that MintMaker would also auto-apply the migration script, if any.
2. Pipeline updates lag behind task updates even for local tasks, not just external tasks.
3. If a PR makes a breaking task change that would require pipeline changes, the PR breaks e2e tests.
    - Due to point 1, it may not be possible to make the pipeline changes in the same PR.
    - Working around this would likely require additional complexity in the test pipeline
      (e.g. auto-apply the relevant migration scripts to pipelines before running the tests).
4. We run e2e-tests more often (because we should test the separate pipeline update PR as well).

### E2E testing

Our current test setup works like this:

- When a task gets updated:
  - Fetch the latest released pipelines.
  - Inject the new task bundle into them.
  - Run the pipelines.
- This ensures the updated task will work as expected in existing pipelines.

What we need for pipelines:

- When a *pipeline* gets updated:
  - Take the newly built pipeline bundle.
  - Run the pipeline.
- This ensures the updated pipeline will work for components that get it during Konflux onboarding.

Now consider what should happen if a PR updates both a task and a pipeline.
As described in the cons section above, the PR cannot update the pipeline with the new task reference,
so the pipeline change would be something unrelated (or related, but not a task version bump).
Should we take the new pipeline bundle, inject the new task bundle into it, and test that?

No, that would mean we're testing a different pipeline than the one that we will actually release.
We should run two independent test suites:

- One that takes released pipelines and injects the updated task bundle.
- One that takes the new pipeline bundle and doesn't inject anything.

To better support this, pipeline Components should be in a separate Application from task Components.

## Alternatives considered

### B) Try to preserve the auto-injection flow

Keep using placeholders for local tasks.
Inject their bundle references into the source files using a custom script with the [run-script] task.

The pros and cons are essentially the inverse of option A.

Additional pros:

1. May simplify the e2e test setup.
    - Instead of determining what tasks have changed and injecting their bundle references
      into pipelines, just use the pipeline bundle built for the same PR.

Additional cons:

1. The build pipeline becomes complex and fragile.
2. We might want to run e2e-tests post-merge anyway, to ensure the dynamic injection went as expected.
    - That would mean option B doesn't actually solve disadvantage 4 of option A.

We cannot use the same approach as build-definitions, where one pipeline first builds task bundles
and then also the pipeline bundles. In the current state of Konflux, one pipeline must correspond
to one built artifact. Our approach would work like this:

1. PipelineRuns that build pipeline bundles execute on changes to any of the tasks
   included in the pipeline (and on changes to the pipeline yaml itself, of course).
2. A custom script waits for builds or releases to complete and injects task bundle references
   into the pipeline yaml.
3. The tkn-bundle task receives the already modified pipeline yaml and pushes it.

The custom script resolves task bundles differently for PR pipelines and for push pipelines.

#### Logic for PR pipelines

1. For local tasks that changed in the PR, the script waits for the `on-pr-{commit_sha}` tag
   to appear in the build repo.
2. For local tasks that did not change, the script gets the task's current version
   and looks up the `{version}` tag in the *release* repo.

#### Logic for push pipelines

1. For all local tasks, the script gets the task's current version
   and waits for the `{version}` tag to appear in the *release* repo.
    - Note: for the tasks whose version didn't change in the merged PR, the tag will already be present.

> [!WARNING]
> If we merge a task change without bumping the version of that task, the proposed process
> would find the `{version}` tag right away and resolve to the previous release of the same version.
> The `{task}:{version}@{digest}` reference in the released pipeline bundle wouldn't match the
> `{task}:{version}@{digest}` reference that MintMaker will propose to users.
>
> Whenever this happens, the solution would be to wait for the task release to complete and then
> merge another PR that triggers a pipeline build. The new build would pick up the latest release.
>
> Once we figure out how to release tasks conditionally (only when their version changes),
> this should stop being a problem, since the merge wouldn't result in a new task release.

[build-definitions]: https://github.com/konflux-ci/build-definitions
[run-script]: https://konflux-ci.dev/docs/patterns/running-user-scripts-on-the-build-pipeline/
