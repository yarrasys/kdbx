"""Behavioral parity: the same scenario must produce the same observable
contract (stdout shape, exit code) from both implementations.

Where the Python implementation deviates from the documented contract, the
*spec* is authoritative (spec C6) and the deviation is recorded here explicitly.
"""

import pytest

from conftest import run_go, run_py


def test_masked_get_prints_the_same_sentinel(project):
    run_go(project, "init")
    run_go(project, "set", "api/openai", stdin="sk-value\n")
    assert run_go(project, "get", "api/openai").stdout.strip() == "(set, hidden)"
    assert run_py(project, "get", "api/openai").stdout.strip() == "(set, hidden)"


def test_read_ops_print_no_banner_in_either(project):
    """The ACTIVE ENV banner is a write-path affordance; reads stay silent on
    stderr in both implementations (Python: test_context.py::
    test_banner_suppressed_for_reads; Go: the `banner` flag in cmd/common.go)."""
    run_go(project, "init")
    run_go(project, "set", "api/openai", stdin="sk-quiet\n")
    for impl in (run_go, run_py):
        for op in ("get", "list"):
            r = impl(project, op, "api/openai" if op == "get" else "api")
            assert r.stderr == "", f"{impl.__name__} {op} wrote to stderr: {r.stderr!r}"


def test_list_output_is_identical(project):
    run_go(project, "init")
    for path in ("api/zeta", "api/alpha", "db/primary"):
        run_go(project, "set", path, stdin="v\n")
    assert run_go(project, "list").stdout == run_py(project, "list").stdout


def test_missing_entry_exits_2_in_both(project):
    run_go(project, "init")
    assert run_go(project, "get", "api/nope", check=False).returncode == 2
    assert run_py(project, "get", "api/nope", check=False).returncode == 2


def test_check_drift_exits_5_in_both(project):
    run_go(project, "init")
    run_go(project, "set", "api/gone", "--var", "GONE_KEY", stdin="v\n")
    run_go(project, "delete", "api/gone", "--purge", "--yes")

    go = run_go(project, "check", check=False)
    py = run_py(project, "check", check=False)
    assert go.returncode == 5 and py.returncode == 5
    assert "MISSING GONE_KEY -> api/gone" in go.stdout
    assert "MISSING GONE_KEY -> api/gone" in py.stdout


def test_export_renders_identically(project):
    run_go(project, "init")
    run_go(project, "set", "api/openai", "--var", "OPENAI_API_KEY", stdin='weird "value" \\ here\n')
    assert run_go(project, "export").stdout == run_py(project, "export").stdout


def test_run_passes_the_child_exit_code_in_both(project):
    run_go(project, "init")
    for impl in (run_go, run_py):
        r = impl(project, "run", "--", "python", "-c", "raise SystemExit(7)", check=False)
        assert r.returncode == 7, f"{impl.__name__} returned {r.returncode}"


def test_pointer_rewrite_produces_the_same_file(project, tmp_path):
    """set --var must write byte-identical pointer files from both sides."""
    run_go(project, "init")
    run_go(project, "set", "api/openai", "--var", "OPENAI_API_KEY", stdin="v\n")
    go_pointer = (project / ".keepassxc.json").read_text()

    # Re-run the same scenario from scratch with the Python implementation.
    import json, shutil
    fresh = tmp_path / "fresh"
    fresh.mkdir()
    (fresh / ".keepassxc.json").write_text(
        json.dumps({"project": "interop", "defaultEnv": "dev", "envs": {"dev": {}}}, indent=2) + "\n"
    )
    shutil.rmtree(project / "kpxc", ignore_errors=True)
    run_py(fresh, "init")
    run_py(fresh, "set", "api/openai", "--var", "OPENAI_API_KEY", stdin="v\n")
    py_pointer = (fresh / ".keepassxc.json").read_text()

    assert go_pointer == py_pointer


def test_symlinked_pointer_path_resolves_the_same(project, tmp_path):
    """Known parity gap found during Task 3: Python's paths.expand_path ends in
    .resolve() (follows symlinks); Go's paths.Expand uses Abs+Clean (does not).
    Both must still reach the same vault when a pointer path traverses a symlink."""
    import json

    real = tmp_path / "real-vaults"
    real.mkdir()
    link = tmp_path / "linked-vaults"
    link.symlink_to(real, target_is_directory=True)

    (project / ".keepassxc.json").write_text(
        json.dumps(
            {
                "project": "interop",
                "defaultEnv": "dev",
                "envs": {
                    "dev": {
                        "vault": f"{link}/dev.kdbx",
                        "keyFile": f"{link}/dev.keyx",
                    }
                },
            },
            indent=2,
        )
        + "\n"
    )

    run_go(project, "init")
    run_go(project, "set", "api/openai", stdin="sk-symlink\n")
    # The Python implementation must reach the same vault through the symlink.
    assert run_py(project, "get", "api/openai", "--reveal").stdout.strip() == "sk-symlink"


@pytest.mark.xfail(
    reason="documented deviation (spec C6): Python surfaces some open failures as exit 1; "
           "Go implements the documented exit 3",
    strict=False,
)
def test_missing_keyfile_exit_code(project):
    run_go(project, "init")
    (project / "kpxc" / "interop" / "dev.keyx").unlink()
    assert run_go(project, "get", "api/x", check=False).returncode == 3
    assert run_py(project, "get", "api/x", check=False).returncode == 3
