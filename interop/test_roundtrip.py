"""Artifact compatibility: a vault written by one implementation must be fully
usable by the other, and by KeePassXC itself."""

import subprocess

import pytest

from conftest import run_go, run_py


def _vault_paths(project):
    base = project / "kpxc" / "interop"
    return base / "dev.kdbx", base / "dev.keyx"


def test_go_created_vault_is_readable_by_python(project):
    run_go(project, "init")
    run_go(project, "set", "api/openai", "--var", "OPENAI_API_KEY", stdin="sk-from-go\n")

    out = run_py(project, "get", "api/openai", "--reveal")
    assert out.stdout.strip() == "sk-from-go"


def test_python_created_vault_is_readable_by_go(project):
    run_py(project, "init")
    run_py(project, "set", "api/openai", "--var", "OPENAI_API_KEY", stdin="sk-from-python\n")

    out = run_go(project, "get", "api/openai", "--reveal")
    assert out.stdout.strip() == "sk-from-python"


def test_protected_custom_property_survives_both_directions(project):
    run_py(project, "init")
    run_py(project, "set", "api/openai:ORG_ID", stdin="org-from-python\n")

    assert run_go(project, "get", "api/openai:ORG_ID", "--reveal").stdout.strip() == "org-from-python"

    run_go(project, "set", "api/openai:PROJECT_ID", stdin="proj-from-go\n")
    assert run_py(project, "get", "api/openai:PROJECT_ID", "--reveal").stdout.strip() == "proj-from-go"


def test_recycle_bin_semantics_agree(project):
    run_go(project, "init")
    run_go(project, "set", "api/temp", stdin="value\n")
    run_go(project, "delete", "api/temp")

    # Neither implementation lists a soft-deleted entry.
    assert "api/temp" not in run_go(project, "list").stdout
    assert "api/temp" not in run_py(project, "list").stdout


def test_pykeepass_reads_a_go_written_vault_directly(project):
    run_go(project, "init")
    run_go(project, "set", "api/openai", stdin="sk-direct\n")
    vault, keyfile = _vault_paths(project)

    script = (
        "import sys;from pykeepass import PyKeePass;"
        "kp=PyKeePass(sys.argv[1],keyfile=sys.argv[2]);"
        "e=kp.find_entries(title='openai',first=True);"
        "print(e.password)"
    )
    out = subprocess.run(
        ["uv", "run", "--with", "pykeepass", "python", "-c", script, str(vault), str(keyfile)],
        capture_output=True, text=True, check=True,
    )
    assert out.stdout.strip() == "sk-direct"


def test_keepassxc_cli_reads_a_go_written_vault(project, has_keepassxc_cli):
    if not has_keepassxc_cli:
        pytest.skip("keepassxc-cli not installed")
    run_go(project, "init")
    run_go(project, "set", "api/openai", stdin="sk-cli\n")
    vault, keyfile = _vault_paths(project)

    out = subprocess.run(
        ["keepassxc-cli", "ls", "--no-password", "-k", str(keyfile), str(vault), "-R"],
        capture_output=True, text=True, check=True,
    )
    assert "openai" in out.stdout


def test_rekey_by_go_keeps_the_vault_python_readable(project):
    run_py(project, "init")
    run_py(project, "set", "api/openai", stdin="sk-rotate\n")
    run_go(project, "rekey", "--yes")

    assert run_py(project, "get", "api/openai", "--reveal").stdout.strip() == "sk-rotate"
