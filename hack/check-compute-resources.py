#!/bin/env python3
"""Enforce per-step computeResources rules on Tekton Task YAML files.

Rule (repo-aligned):
  - memory request and limit are required and must be equal
  - CPU request is required
  - CPU limit is optional; if set, must equal the CPU request

Scans task/${name}/**/${name}.yaml (skips archived-tasks/).
Honors .compute-resources-exceptions.yaml (one path per line).
"""

from __future__ import annotations

import argparse
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Iterator

try:
    import yaml
except ImportError:  # pragma: no cover
    print("PyYAML is required: pip install pyyaml", file=sys.stderr)
    sys.exit(2)

REPO_ROOT = Path(__file__).resolve().parent.parent
DEFAULT_EXCEPTIONS = REPO_ROOT / ".compute-resources-exceptions.yaml"
TASK_ROOT = REPO_ROOT / "task"


@dataclass(frozen=True)
class Finding:
    path: Path
    message: str


def is_task_file(path: Path) -> bool:
    """Return True when the path matches task/{name}/**/{name}.yaml."""

    rel = path.relative_to(REPO_ROOT)
    match rel.parts:
        case ["task", task_dir, *_, task_file] if task_file.endswith(".yaml"):
            return task_file.removesuffix(".yaml") == task_dir
        case _:
            return False


def iter_task_files() -> Iterator[Path]:
    if not TASK_ROOT.is_dir():
        return
    for path in sorted(TASK_ROOT.rglob("*.yaml")):
        if is_task_file(path):
            yield path


def load_exceptions(path: Path) -> set[str]:
    """Load exception paths: one path per line. Raise exception only for missing file."""
    with path.open() as fp:
        return {
            line.strip()
            for line in fp
            if line.strip() and not line.strip().startswith("#")
        }


def compute_resources(obj: dict[str, Any]) -> dict[str, Any] | None:
    """Return the resources block from a step, sidecar, or stepTemplate.

    Tekton v1 uses `computeResources`; Tekton v1beta1 uses `resources`.
    https://tekton.dev/docs/pipelines/tasks/#defining-steps
    """
    if "computeResources" in obj:
        return obj.get("computeResources") or {}
    if "resources" in obj:
        return obj.get("resources") or {}
    return None


def merge_compute_resources(
    base: dict[str, Any] | None,
    overlay: dict[str, Any] | None,
) -> dict[str, Any] | None:
    """Apply stepTemplate defaults, then per-step overrides.

    https://tekton.dev/docs/pipelines/tasks/#specifying-step-template
    """
    if overlay is None:
        return base
    if base is None:
        return overlay
    return {
        "requests": {**(base.get("requests") or {}), **(overlay.get("requests") or {})},
        "limits": {**(base.get("limits") or {}), **(overlay.get("limits") or {})},
    }


def check_resources(
    path: Path,
    kind: str,
    step: dict[str, Any],
    cr: dict[str, Any] | None,
) -> list[Finding]:
    name = step.get("name") or f"<{kind}>"
    findings: list[Finding] = []

    if cr is None:
        findings.append(
            Finding(path, f"{kind} '{name}': missing computeResources (or resources)")
        )
        return findings

    requests = cr.get("requests") or {}
    limits = cr.get("limits") or {}

    mem_req = requests.get("memory")
    mem_lim = limits.get("memory")
    if mem_req is None:
        findings.append(Finding(path, f"{kind} '{name}': missing memory request"))
    if mem_lim is None:
        findings.append(Finding(path, f"{kind} '{name}': missing memory limit"))
    if mem_req is not None and mem_lim is not None and mem_req != mem_lim:
        findings.append(
            Finding(
                path,
                f"{kind} '{name}': memory request ({mem_req}) must equal memory limit ({mem_lim})",
            )
        )

    cpu_req = requests.get("cpu")
    cpu_lim = limits.get("cpu")
    if cpu_req is None:
        findings.append(Finding(path, f"{kind} '{name}': missing cpu request"))
    if cpu_lim is not None and cpu_req is not None and cpu_lim != cpu_req:
        findings.append(
            Finding(
                path,
                f"{kind} '{name}': cpu limit ({cpu_lim}) must equal cpu request ({cpu_req})",
            )
        )

    return findings


def check_task_file(path: Path) -> list[Finding]:
    rel = path.relative_to(REPO_ROOT)
    data = yaml.safe_load(path.read_text())
    if not isinstance(data, dict):
        raise ValueError(f"{rel}: expected a mapping at document root")

    spec = data.get("spec") or {}
    template = spec.get("stepTemplate") or {}
    template_cr = compute_resources(template) if isinstance(template, dict) else None

    findings: list[Finding] = []
    for kind in ("steps", "sidecars"):
        for step in spec.get(kind) or []:
            if not isinstance(step, dict):
                continue
            # stepTemplate applies to steps only, not sidecars.
            cr = compute_resources(step)
            if kind == "steps":
                cr = merge_compute_resources(template_cr, cr)
            findings.extend(check_resources(rel, kind, step, cr))
    return findings


def collect_findings(exceptions: set[str]) -> list[Finding]:
    findings: list[Finding] = []
    for path in iter_task_files():
        rel = str(path.relative_to(REPO_ROOT))
        file_findings = check_task_file(path)
        if file_findings:
            if rel in exceptions:
                continue
            findings.extend(file_findings)
        elif rel in exceptions:
            findings.append(
                Finding(
                    path.relative_to(REPO_ROOT),
                    "listed in exceptions file but is now compliant; remove it",
                )
            )
    return findings


def emit(findings: list[Finding]) -> int:
    for finding in findings:
        print(f"ERROR: {finding.path} {finding.message}", file=sys.stderr)
    if findings:
        print(
            f"\n{len(findings)} computeResources violation(s) found.",
            file=sys.stderr,
        )
        return 1
    print("All scanned task files satisfy computeResources rules.")
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--exceptions",
        type=Path,
        default=DEFAULT_EXCEPTIONS,
        help="Path to exceptions file (default: .compute-resources-exceptions.yaml)",
    )
    args = parser.parse_args(argv)
    return emit(collect_findings(load_exceptions(args.exceptions)))


if __name__ == "__main__":
    sys.exit(main())
