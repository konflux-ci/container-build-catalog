#!/usr/bin/env python3
# /// script
# requires-python = ">=3.11"
# dependencies = [
#   "pyyaml",
# ]
# ///
"""Check Task steps/sidecars for computeResources (v1) or resources (v1beta1)."""

import argparse
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Iterator

try:
    import yaml
except ImportError:
    print("PyYAML is required. Run: uv run hack/check-compute-resources.py", file=sys.stderr)
    sys.exit(2)

EXCEPTIONS_NAME = ".compute-resources-exceptions.yaml"


@dataclass(frozen=True)
class Finding:
    path: Path
    message: str


def catalog_root(explicit: Path | None) -> Path:
    if explicit is not None:
        return explicit.expanduser().resolve()
    try:
        out = subprocess.check_output(
            ["git", "rev-parse", "--show-toplevel"],
            text=True,
            stderr=subprocess.DEVNULL,
        )
        return Path(out.strip())
    except (OSError, subprocess.CalledProcessError):
        return Path(__file__).resolve().parent.parent


def is_task_file(path: Path, root: Path) -> bool:
    rel = path.relative_to(root)
    match rel.parts:
        case ["task", task_dir, *_, task_file] if task_file.endswith(".yaml"):
            return task_file.removesuffix(".yaml") == task_dir
        case _:
            return False


def iter_task_files(root: Path) -> Iterator[Path]:
    task_root = root / "task"
    if not task_root.is_dir():
        return
    for path in sorted(task_root.rglob("*.yaml")):
        if is_task_file(path, root):
            yield path


def load_exceptions(path: Path, *, required: bool) -> dict[str, dict[str, set[str]]]:
    if not path.is_file():
        if required:
            raise FileNotFoundError(path)
        return {}

    data = yaml.safe_load(path.read_text())
    if data is None:
        return {}
    if not isinstance(data, dict) or not isinstance(data.get("exceptions"), list):
        raise ValueError(f"{path}: expected a mapping with an 'exceptions' list")

    result: dict[str, dict[str, set[str]]] = {}
    for entry in data["exceptions"]:
        if not isinstance(entry, dict) or "path" not in entry:
            raise ValueError(f"{path}: invalid exception entry: {entry!r}")
        rel = str(entry["path"])
        steps = entry.get("steps") or []
        sidecars = entry.get("sidecars") or []
        if not isinstance(steps, list) or not isinstance(sidecars, list):
            raise ValueError(f"{path}: steps/sidecars must be lists in {rel}")
        if not steps and not sidecars:
            raise ValueError(f"{path}: {rel} must list steps and/or sidecars")
        result[rel] = {
            "steps": {str(name) for name in steps},
            "sidecars": {str(name) for name in sidecars},
        }
    return result


def resource_field(api_version: Any) -> str:
    return "resources" if str(api_version).endswith("/v1beta1") else "computeResources"


def compute_resources(obj: dict[str, Any]) -> dict[str, Any] | None:
    if "computeResources" in obj:
        return obj.get("computeResources") or {}
    if "resources" in obj:
        return obj.get("resources") or {}
    return None


def merge_compute_resources(
    base: dict[str, Any] | None,
    overlay: dict[str, Any] | None,
) -> dict[str, Any] | None:
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
    field: str,
    api_version: str,
) -> list[Finding]:
    name = step.get("name") or f"<{kind}>"
    findings: list[Finding] = []

    if cr is None:
        other = "resources" if field == "computeResources" else "computeResources"
        findings.append(
            Finding(
                path,
                f"{kind} '{name}': missing {field} "
                f"({api_version} requires {field}, not {other})",
            )
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


def check_task_file(path: Path, rel: Path) -> dict[tuple[str, str], list[Finding]]:
    data = yaml.safe_load(path.read_text())
    if not isinstance(data, dict):
        raise ValueError(f"{rel}: expected a mapping at document root")

    api_version = str(data.get("apiVersion") or "tekton.dev/v1")
    field = resource_field(api_version)
    spec = data.get("spec") or {}
    template = spec.get("stepTemplate") or {}
    template_cr = compute_resources(template) if isinstance(template, dict) else None

    items: dict[tuple[str, str], list[Finding]] = {}
    for kind in ("steps", "sidecars"):
        for step in spec.get(kind) or []:
            if not isinstance(step, dict):
                continue
            name = str(step.get("name") or f"<{kind}>")
            cr = compute_resources(step)
            if kind == "steps":
                cr = merge_compute_resources(template_cr, cr)
            items.setdefault((kind, name), []).extend(
                check_resources(rel, kind, step, cr, field, api_version)
            )
    return items


def collect_findings(
    root: Path, exceptions: dict[str, dict[str, set[str]]]
) -> list[Finding]:
    findings: list[Finding] = []
    seen: set[str] = set()
    for path in iter_task_files(root):
        rel = path.relative_to(root)
        rel_s = rel.as_posix()
        seen.add(rel_s)
        items = check_task_file(path, rel)
        allow = exceptions.get(rel_s, {"steps": set(), "sidecars": set()})
        violated = {key for key, item_findings in items.items() if item_findings}
        for (kind, name), item_findings in items.items():
            if name in allow.get(kind, set()):
                continue
            findings.extend(item_findings)
        for kind in ("steps", "sidecars"):
            for name in sorted(allow.get(kind, set())):
                if (kind, name) not in items:
                    findings.append(
                        Finding(
                            rel,
                            f"{kind} '{name}': listed in exceptions file but does not exist; remove it",
                        )
                    )
                elif (kind, name) not in violated:
                    findings.append(
                        Finding(
                            rel,
                            f"{kind} '{name}': listed in exceptions file but is now compliant; remove it",
                        )
                    )
    for rel_s in exceptions:
        if rel_s not in seen:
            findings.append(
                Finding(
                    Path(rel_s),
                    "listed in exceptions file but is not a scanned task file; remove it",
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
        "root",
        nargs="?",
        type=Path,
        help="Catalog root (default: git toplevel, else this script's repository)",
    )
    parser.add_argument(
        "--exceptions",
        type=Path,
        default=None,
        help=f"Exceptions YAML (default: {EXCEPTIONS_NAME} under the catalog root, if present)",
    )
    args = parser.parse_args(argv)
    root = catalog_root(args.root)
    required = args.exceptions is not None
    exceptions_path = args.exceptions if required else root / EXCEPTIONS_NAME
    return emit(collect_findings(root, load_exceptions(exceptions_path, required=required)))


if __name__ == "__main__":
    sys.exit(main())
