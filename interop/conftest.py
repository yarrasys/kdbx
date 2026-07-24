"""Fixtures for the kdbx Go/Python interop suite.

Run with:
  uv run --with pytest --with pykeepass python -m pytest interop -v
"""

import json
import os
import pathlib
import shutil
import subprocess

import pytest

GO_BIN = os.environ.get("KDBX_BIN", str(pathlib.Path(__file__).resolve().parents[1] / "kdbx"))
PY_REPO = pathlib.Path(
    os.environ.get("KDBX_PY_REPO", "/Users/nabsha/work/yarrasys/extensions")
)
PY_ENTRY = PY_REPO / "skills" / "kdbx" / "kdbx.py"

POINTER = {
    "project": "interop",
    "defaultEnv": "dev",
    "envs": {"dev": {}},
}


@pytest.fixture
def project(tmp_path, monkeypatch):
    """An isolated project dir with a pointer file and a private KEEPASSXC_DIR."""
    monkeypatch.setenv("KEEPASSXC_DIR", str(tmp_path / "kpxc"))
    monkeypatch.delenv("KDBX_ENV", raising=False)
    (tmp_path / ".keepassxc.json").write_text(json.dumps(POINTER, indent=2) + "\n")
    return tmp_path


def run_go(project, *args, stdin=None, check=True):
    """Invoke the Go binary inside project."""
    return subprocess.run(
        [GO_BIN, *args],
        cwd=project,
        input=stdin,
        capture_output=True,
        text=True,
        check=check,
    )


def run_py(project, *args, stdin=None, check=True):
    """Invoke the Python reference implementation inside project."""
    if not PY_ENTRY.exists():
        pytest.skip(f"Python reference implementation not found at {PY_ENTRY}")
    return subprocess.run(
        ["uv", "run", "--locked", str(PY_ENTRY), *args],
        cwd=project,
        input=stdin,
        capture_output=True,
        text=True,
        check=check,
    )


@pytest.fixture
def has_keepassxc_cli():
    return shutil.which("keepassxc-cli") is not None
