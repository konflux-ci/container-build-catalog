#!/bin/env python3
"""Enforce per-step computeResources rules on Tekton Task YAML files.

Rule (repo-aligned):
  - memory request and limit are required and must be equal
  - CPU request is required
  - CPU limit is optional; if set, must equal the CPU request

Scans task/${name}/**/${name}.yaml (skips archived-tasks/).
Honors .compute-resources-exceptions.yaml for grandfathered paths.
Emits GitHub Actions annotations when GITHUB_ACTIONS=true.

"""

# ---------------------------------------------------------------------------
# What this script does (high-level overview):
#
#   1. Walk every Tekton Task YAML file under the repo's `task/` directory.
#   2. For each file, open it and inspect each "step" and "sidecar" defined
#      inside the Task spec.
#   3. Check that every step/sidecar declares valid CPU and memory resource
#      requests/limits according to repo rules.
#   4. Report any violations as plain-text errors (or GitHub Actions
#      annotations when running in a CI pipeline).
#   5. Optionally write a list of "exception" paths so that already-broken
#      files can be skipped while new violations are still caught.
# ---------------------------------------------------------------------------

from __future__ import annotations  # Allow newer type-hint syntax on Python 3.9

# Standard-library imports — these come with Python, no installation needed.
import argparse   # Parse command-line arguments (--exceptions, --generate-exceptions, etc.)
import os         # Read environment variables (e.g. GITHUB_ACTIONS) and change directories
import sys        # Access argv, stderr, and exit codes
from dataclasses import dataclass   # Decorator that auto-generates __init__ / __repr__ for classes
from pathlib import Path            # Object-oriented file-system paths (safer than bare strings)
from typing import Any, Iterable, Iterator  # Type hints for better readability

# Third-party import — must be installed with: pip install pyyaml
try:
    import yaml   # Parse YAML files (Tekton Task definitions are written in YAML)
except ImportError:  # pragma: no cover
    # If PyYAML isn't installed, tell the user how to fix it and exit immediately.
    print("PyYAML is required: pip install pyyaml", file=sys.stderr)
    sys.exit(2)

# ---------------------------------------------------------------------------
# Top-level path constants
# ---------------------------------------------------------------------------

# __file__ is the path to *this* script.
# .resolve() turns it into an absolute path, .parent.parent goes up two levels
# (from hack/ to the repository root).
REPO_ROOT = Path(__file__).resolve().parent.parent

# Default location of the YAML file that lists "grandfathered" task paths
# that are allowed to skip the resource checks.
DEFAULT_EXCEPTIONS = REPO_ROOT / ".compute-resources-exceptions.yaml"

# The directory that holds all Tekton Task definitions in the repository.
TASK_ROOT = REPO_ROOT / "task"


# ---------------------------------------------------------------------------
# Data class: Finding
# ---------------------------------------------------------------------------
# A "Finding" is one policy violation discovered in a YAML file.
# frozen=True means instances are immutable (like a named tuple).
@dataclass(frozen=True)
class Finding:
    path: Path           # Which file the violation was found in
    message: str         # Human-readable description of the problem
    line: int | None = None  # Line number in the file (None if unknown)
    kind: str = "error"  # Severity — "error" or "warning"

    def format_gh(self) -> str:
        """Format the finding as a GitHub Actions workflow command.

        GitHub Actions reads lines printed to stderr that look like
        '::error file=...,line=...::message' and turns them into
        inline code annotations in pull requests.
        """
        attrs = [f"file={self.path.as_posix()}"]          # Convert Path to forward-slash string
        attrs.append(f"line={self.line if self.line is not None else 1}")  # Default to line 1
        return f"::{self.kind} {','.join(attrs)}::{self.message}"

    def format_plain(self, use_color: bool = True) -> str:
        """Format the finding as a coloured (or plain) terminal message.

        ANSI escape codes (\033[...m) add colour in terminals that support it.
        Red (1;31) is used for errors, yellow (1;33) for warnings.
        \033[0m resets the colour back to default.
        """
        if use_color:
            color = "\033[1;31m" if self.kind == "error" else "\033[1;33m"
            reset = "\033[0m"
            label = f"{color}{self.kind.upper()}{reset}"  # e.g.  ERROR  (in red)
        else:
            label = self.kind.upper()  # Plain text: "ERROR" or "WARNING"
        loc = f"{self.path}"
        if self.line is not None:
            loc = f"{loc}:{self.line}"  # Append ":42" style line reference
        return f"{label}: {loc} {self.message}"


# ---------------------------------------------------------------------------
# Helper: is_task_file
# ---------------------------------------------------------------------------
def is_task_file(path: Path) -> bool:
    """Return True only when the path matches the pattern task/{name}/**/{name}.yaml.

    The repository convention is:
        task/
          my-task/
            0.1/
              my-task.yaml    ← valid task file
            README.md         ← NOT a task file (different name)
    archived-tasks/ files are implicitly excluded because they live outside TASK_ROOT.
    """
    try:
        # Make the path relative to the repo root so we can inspect its parts.
        rel = path.relative_to(REPO_ROOT) if path.is_absolute() else path
    except ValueError:
        # relative_to() raises ValueError when the path is not inside REPO_ROOT.
        rel = path

    parts = rel.parts  # e.g. ("task", "my-task", "0.1", "my-task.yaml")

    # Structural pattern match (Python 3.10+):
    #   parts[0] must be "task"
    #   parts[1] is the task directory name
    #   parts[-1] must be a .yaml file whose stem matches the directory name
    match parts:
        case ["task", task_dir, *_, task_file] if task_file.endswith(".yaml"):
            # removesuffix strips ".yaml" so we compare just the base name
            return task_file.removesuffix(".yaml") == task_dir
        case _:
            return False  # Any other path shape is not a task file


# ---------------------------------------------------------------------------
# Generator: iter_task_files
# ---------------------------------------------------------------------------
def iter_task_files() -> Iterator[Path]:
    """Yield every valid task YAML file found under TASK_ROOT, in sorted order.

    Using a generator (yield) means files are processed one at a time instead
    of loading the entire list into memory at once.
    """
    if not TASK_ROOT.is_dir():
        return  # Nothing to scan — the task/ directory doesn't exist yet

    # rglob("*.yaml") recursively finds all .yaml files under TASK_ROOT.
    # sorted() gives deterministic ordering so output is reproducible.
    for path in sorted(TASK_ROOT.rglob("*.yaml")):
        if is_task_file(path):
            yield path  # Hand the path to the caller one at a time


# ---------------------------------------------------------------------------
# Helper: load_exceptions
# ---------------------------------------------------------------------------
def load_exceptions(path: Path) -> set[str]:
    """Read the exceptions YAML file and return a set of exempted path strings.

    The exceptions file can be a plain list:
        - task/some-task/0.1/some-task.yaml

    …or a mapping with an "exceptions" key:
        exceptions:
          - task/some-task/0.1/some-task.yaml

    Files listed here are allowed to fail resource checks without blocking CI.
    """
    if not path.is_file():
        return set()  # No exceptions file → nothing is exempted

    # Read and parse the YAML into a Python object (list or dict).
    data = yaml.safe_load(path.read_text()) or {}

    # Normalise both supported formats into a flat list of entries.
    if isinstance(data, list):
        entries = data
    elif isinstance(data, dict):
        entries = data.get("exceptions") or []
    else:
        # Unexpected format — abort with a clear error message.
        raise SystemExit(f"Unexpected exceptions file format: {path}")

    result: set[str] = set()  # A set gives O(1) membership tests later
    for entry in entries:
        if isinstance(entry, str):
            result.add(entry)           # Simple string path
        elif isinstance(entry, dict) and "path" in entry:
            result.add(str(entry["path"]))  # Dict with a "path" key
        else:
            raise SystemExit(f"Invalid exception entry in {path}: {entry!r}")
    return result


# ---------------------------------------------------------------------------
# Helper: find_line
# ---------------------------------------------------------------------------
def find_line(text: str, needle: str) -> int | None:
    """Search `text` line-by-line and return the 1-based line number of `needle`.

    Returns None if `needle` is not found anywhere in the text.
    This is used to attach a meaningful line number to each Finding so users
    can jump directly to the problem in their editor.
    """
    for i, line in enumerate(text.splitlines(), start=1):  # start=1 → 1-based index
        if needle in line:
            return i
    return None


# ---------------------------------------------------------------------------
# Helper: step_compute_resources
# ---------------------------------------------------------------------------
def step_compute_resources(step: dict[str, Any]) -> dict[str, Any] | None:
    """Extract the compute-resources block from a single step/sidecar dict.

    Tekton supports two spellings of the same field:
      - computeResources  (preferred, newer API)
      - resources         (older, still valid)

    Returns the resources dict, an empty dict {}, or None if neither key exists.
    None means the step has no resource configuration at all (a violation).
    """
    if "computeResources" in step:
        # `or {}` converts an explicit null/None YAML value to an empty dict
        return step.get("computeResources") or {}
    if "resources" in step:
        return step.get("resources") or {}
    return None  # Neither key present → step is missing resource declarations


# ---------------------------------------------------------------------------
# Core checker: check_resources
# ---------------------------------------------------------------------------
def check_resources(
    path: Path,
    kind: str,            # "step" or "sidecar" — used in error messages
    step: dict[str, Any], # The parsed YAML dict for one step/sidecar
    text: str,            # Raw file content — used to locate line numbers
) -> list[Finding]:
    """Validate the resource requests and limits for a single step or sidecar.

    Rules enforced:
      1. The computeResources (or resources) block must exist.
      2. A memory *request* is required.
      3. A memory *limit* is required.
      4. Memory request must equal memory limit (burstable QoS is disallowed).
      5. A CPU *request* is required.
      6. CPU *limit* is optional; but if set, it must match the CPU request.

    Returns a (possibly empty) list of Finding objects — one per violation.
    """
    # Determine the step name for use in messages (fall back to "<step>" if unnamed).
    name = step.get("name") or f"<{kind}>"

    # Try to find the line in the file where this step is declared.
    line = find_line(text, f"name: {name}") or find_line(text, f"- name: {name}")

    # Retrieve the computeResources block (or None if absent).
    cr = step_compute_resources(step)
    findings: list[Finding] = []

    # --- Rule 1: computeResources block must exist ---
    if cr is None:
        findings.append(
            Finding(
                path=path,
                line=line,
                message=f"{kind} '{name}': missing computeResources (or resources)",
            )
        )
        # No point checking individual fields if the whole block is missing.
        return findings

    # Extract the requests and limits sub-dicts (default to empty dict if absent).
    requests = cr.get("requests") or {}
    limits = cr.get("limits") or {}

    # Read individual resource values (None if the key is not set in the YAML).
    mem_req = requests.get("memory")
    mem_lim = limits.get("memory")

    # --- Rule 2: memory request must be present ---
    if mem_req is None:
        findings.append(
            Finding(
                path=path,
                line=line,
                message=f"{kind} '{name}': missing memory request",
            )
        )

    # --- Rule 3: memory limit must be present ---
    if mem_lim is None:
        findings.append(
            Finding(
                path=path,
                line=line,
                message=f"{kind} '{name}': missing memory limit",
            )
        )

    # --- Rule 4: memory request must equal memory limit ---
    # Only check equality when both values are available (avoids a second error
    # when one of them is already reported as missing).
    if mem_req is not None and mem_lim is not None and mem_req != mem_lim:
        findings.append(
            Finding(
                path=path,
                line=line,
                message=(
                    f"{kind} '{name}': memory request ({mem_req}) "
                    f"must equal memory limit ({mem_lim})"
                ),
            )
        )

    # Read CPU values.
    cpu_req = requests.get("cpu")
    cpu_lim = limits.get("cpu")

    # --- Rule 5: CPU request must be present ---
    if cpu_req is None:
        findings.append(
            Finding(
                path=path,
                line=line,
                message=f"{kind} '{name}': missing cpu request",
            )
        )

    # --- Rule 6: CPU limit is optional; if present it must match the request ---
    # CPU limit is optional; if present it must match the request.
    if cpu_lim is not None and cpu_req is not None and cpu_lim != cpu_req:
        findings.append(
            Finding(
                path=path,
                line=line,
                message=(
                    f"{kind} '{name}': cpu limit ({cpu_lim}) "
                    f"must equal cpu request ({cpu_req})"
                ),
            )
        )

    return findings  # Empty list means the step passed all checks


# ---------------------------------------------------------------------------
# File-level checker: check_task_file
# ---------------------------------------------------------------------------
def check_task_file(path: Path) -> list[Finding]:
    """Parse one Tekton Task YAML file and return all resource-policy violations.

    A Tekton Task looks like:
        apiVersion: tekton.dev/v1
        kind: Task
        spec:
          steps:
            - name: build
              computeResources: ...
          sidecars:
            - name: proxy
              computeResources: ...
    """
    text = path.read_text()  # Read the entire file as a string

    # Parse the YAML into a Python dict.
    try:
        data = yaml.safe_load(text)
    except yaml.YAMLError as exc:
        # The file contains invalid YAML — report it immediately.
        return [
            Finding(
                path=path,
                line=1,
                message=f"failed to parse YAML: {exc}",
            )
        ]

    # Sanity-check: the top-level YAML object must be a mapping (dict), not a
    # list or scalar, for this to be a valid Kubernetes/Tekton manifest.
    if not isinstance(data, dict):
        return [
            Finding(
                path=path,
                line=1,
                message="expected a mapping at document root",
            )
        ]

    # Navigate into spec: {steps: [...], sidecars: [...]}
    spec = data.get("spec") or {}
    findings: list[Finding] = []

    # Check both "steps" and "sidecars" using the same helper.
    # kind_key   — the YAML key inside spec (e.g. "steps")
    # kind_label — the human-readable label used in error messages (e.g. "step")
    for kind_key, kind_label in (("steps", "step"), ("sidecars", "sidecar")):
        for step in spec.get(kind_key) or []:
            if not isinstance(step, dict):
                continue  # Skip malformed entries (not a mapping)
            findings.extend(check_resources(path, kind_label, step, text))

    return findings


# ---------------------------------------------------------------------------
# Helper: rel_posix
# ---------------------------------------------------------------------------
def rel_posix(path: Path) -> str:
    """Return the path as a POSIX (forward-slash) string relative to REPO_ROOT.

    Used to produce consistent path strings for exception-file lookups and
    for human-readable output regardless of the OS path separator.
    Falls back to the absolute POSIX path if the path is outside the repo.
    """
    try:
        return path.resolve().relative_to(REPO_ROOT).as_posix()
    except ValueError:
        return path.as_posix()  # Path is outside the repo — use the full path


# ---------------------------------------------------------------------------
# Aggregator: collect_findings
# ---------------------------------------------------------------------------
def collect_findings(exceptions: set[str]) -> tuple[list[Finding], list[str]]:
    """Scan all task files and return (all_findings, failing_paths).

    - all_findings    : violations from non-exempted files (used to decide exit code)
    - failing_paths   : relative paths of *every* file that has violations,
                        including exempted ones (used when generating the
                        exceptions file so it stays up-to-date)
    """
    all_findings: list[Finding] = []
    failing_paths: list[str] = []   # Tracks files that fail, including excepted ones

    for path in iter_task_files():
        rel = rel_posix(path)               # Relative path string for comparison
        file_findings = check_task_file(path)  # Run policy checks on this file

        if not file_findings:
            continue  # File is clean — skip it

        failing_paths.append(rel)  # Record that this file has violations

        if rel in exceptions:
            continue  # File is grandfathered — don't add its errors to the report

        all_findings.extend(file_findings)  # Add violations to the main list

    return all_findings, failing_paths


# ---------------------------------------------------------------------------
# Helper: write_exceptions
# ---------------------------------------------------------------------------
def write_exceptions(path: Path, failing_paths: Iterable[str]) -> None:
    """Write (or overwrite) the exceptions YAML file with all currently failing paths.

    This is called when the user runs the script with --generate-exceptions.
    The resulting file can be committed so CI stops failing on existing issues
    while still catching newly introduced ones.
    """
    paths = sorted(set(failing_paths))  # Deduplicate and sort for a stable diff
    doc = {
        "exceptions": paths,
    }
    # A human-readable comment header explaining the purpose of the file.
    header = (
        "# Grandfathered task files that currently fail computeResources checks.\n"
        "# Remove an entry when the Task is fixed (prefer the same PR).\n"
        "# Generated for KONFLUX-12895 / KONFLUX-11510.\n"
    )
    # Concatenate the comment header with the YAML body and write to disk.
    path.write_text(header + yaml.safe_dump(doc, default_flow_style=False, sort_keys=False))


# ---------------------------------------------------------------------------
# Output emitter: emit
# ---------------------------------------------------------------------------
def emit(findings: list[Finding]) -> int:
    """Print all findings to stderr and return the appropriate exit code.

    Returns:
        0 — no violations found (success)
        1 — at least one violation found (failure)

    Output format is chosen automatically:
      - GitHub Actions CI  → GitHub workflow commands (::error ...)
      - Interactive TTY    → coloured plain text
      - Non-TTY (piped)   → plain text without ANSI colour codes
    """
    in_gh = os.getenv("GITHUB_ACTIONS") == "true"   # True when running in GitHub Actions
    use_color = (not in_gh) and sys.stderr.isatty()  # Only colour when a human is watching

    for finding in findings:
        if in_gh:
            print(finding.format_gh(), file=sys.stderr)    # GitHub annotation format
        else:
            print(finding.format_plain(use_color=use_color), file=sys.stderr)  # Human format

    if findings:
        # Print a summary count and point to documentation.
        print(
            f"\n{len(findings)} computeResources violation(s) found. "
            file=sys.stderr,
        )
        return 1  # Non-zero exit code signals failure to CI

    # All checked files are clean.
    print("All scanned task files satisfy computeResources rules.")
    return 0  # Zero exit code signals success


# ---------------------------------------------------------------------------
# Entry point: main
# ---------------------------------------------------------------------------
def main(argv: list[str] | None = None) -> int:
    """Parse CLI arguments, run checks, and return an exit code.

    Supported flags:
      --exceptions <file>      Path to the exceptions YAML (default: repo root)
      --generate-exceptions    Write a fresh exceptions file then exit 0
      --no-exceptions          Ignore the exceptions file; report every failure
    """
    parser = argparse.ArgumentParser(description=__doc__)

    # --exceptions lets the user point to a custom exceptions file.
    parser.add_argument(
        "--exceptions",
        type=Path,
        default=DEFAULT_EXCEPTIONS,
        help="Path to exceptions YAML (default: .compute-resources-exceptions.yaml)",
    )

    # --generate-exceptions regenerates the exceptions file from scratch and exits.
    parser.add_argument(
        "--generate-exceptions",
        action="store_true",
        help="Write current failing task paths to the exceptions file and exit 0",
    )

    # --no-exceptions runs a strict scan: every failure is reported, no skips.
    parser.add_argument(
        "--no-exceptions",
        action="store_true",
        help="Ignore the exceptions file (report all failures)",
    )

    args = parser.parse_args(argv)  # Parse sys.argv by default (or the provided list)

    # Change the working directory to the repo root so relative paths work correctly.
    os.chdir(REPO_ROOT)

    # --- Mode: generate (or regenerate) the exceptions file ---
    if args.generate_exceptions:
        # Collect violations with *no* exceptions so we capture every failing file.
        _, failing = collect_findings(exceptions=set())
        write_exceptions(args.exceptions, failing)
        print(
            f"Wrote {len(set(failing))} exception(s) to {args.exceptions.relative_to(REPO_ROOT)}"
        )
        return 0  # Always exit 0 after generating exceptions

    # --- Mode: normal check ---
    exceptions: set[str] = set()
    if not args.no_exceptions:
        # Load the grandfathered paths so they are skipped during the scan.
        exceptions = load_exceptions(args.exceptions)

    # Run the scan and emit results; the return value is the process exit code.
    findings, _ = collect_findings(exceptions=exceptions)
    return emit(findings)


# ---------------------------------------------------------------------------
# Script entry point
# ---------------------------------------------------------------------------
# When Python runs this file directly (python check-compute-resources.py),
# __name__ is "__main__".  When it is imported as a module (e.g. in tests),
# __name__ is the module name and this block is skipped — which is why tests
# can call main() without triggering sys.exit().
if __name__ == "__main__":
    sys.exit(main())
