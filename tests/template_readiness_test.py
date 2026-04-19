"""Template-readiness checks for bootstrap documentation and entrypoints."""

from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[1]


def _read(relative_path: str) -> str:
    return (REPO_ROOT / relative_path).read_text(encoding="utf-8")


def _assert_contains_all(contents: str, expected_snippets: tuple[str, ...], *, label: str) -> None:
    missing = [snippet for snippet in expected_snippets if snippet not in contents]
    assert not missing, f"{label} is missing expected snippets: {missing}"


def _read_env_keys(relative_path: str) -> set[str]:
    keys: set[str] = set()
    for raw_line in _read(relative_path).splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, _ = line.split("=", 1)
        keys.add(key)
    return keys


def test_readme_documents_template_bootstrap_flow() -> None:
    readme = _read("README.md")

    _assert_contains_all(
        readme,
        (
            "## Before Using This As A Template",
            "`src/backend/controller`",
            "`src/backend/gateway`",
            "`src/backend/repository`",
            "`src/backend/handlers`",
            "`src/frontend`",
            "`env.example`",
            "`docker-compose.yaml`",
            "`uv sync`",
            "`just lint`",
            "`just test`",
            "`bun install`",
            "`bun run dev`",
            "`bun run build`",
            "`docker compose up --build`",
            "starter examples",
            "only mounts `/` and `/health`",
        ),
        label="README.md",
    )


def test_template_readiness_workflow_runs_documented_entrypoints() -> None:
    workflow = _read(".github/workflows/template-readiness.yml")

    _assert_contains_all(
        workflow,
        (
            "name: Template Readiness",
            "pull_request:",
            'branches: ["main"]',
            "workflow_dispatch:",
            'python-version: "3.12"',
            "uv sync",
            "just lint",
            "just test",
        ),
        label=".github/workflows/template-readiness.yml",
    )


def test_root_entrypoints_match_documented_commands() -> None:
    justfile = _read("Justfile")
    pyproject = _read("pyproject.toml")
    frontend_package = _read("src/frontend/package.json")

    _assert_contains_all(
        justfile,
        (
            "lint:",
            "uv run ruff check .",
            "uv run ty check",
            "test:",
            "uv run pytest",
        ),
        label="Justfile",
    )
    _assert_contains_all(
        pyproject,
        (
            'requires-python = ">=3.12"',
            'testpaths = ["src", "tests"]',
        ),
        label="pyproject.toml",
    )
    _assert_contains_all(
        frontend_package,
        (
            '"dev": "vite"',
            '"build": "tsc --noEmit && vite build"',
            '"preview": "vite preview"',
        ),
        label="src/frontend/package.json",
    )


def test_env_example_and_compose_cover_bootstrap_configuration() -> None:
    env_keys = _read_env_keys("env.example")
    docker_compose = _read("docker-compose.yaml")

    assert {
        "OPENAI_API_KEY",
        "OPENAI_MODEL",
        "POSTGRES_HOST",
        "POSTGRES_PORT",
        "POSTGRES_DB",
        "POSTGRES_USER",
        "POSTGRES_PASSWORD",
        "DATABASE_URL",
        "BACKEND_PORT",
        "FRONTEND_PORT",
        "VITE_BACKEND_URL",
    }.issubset(env_keys)

    _assert_contains_all(
        docker_compose,
        (
            "${POSTGRES_HOST}",
            "${POSTGRES_PORT}",
            "${POSTGRES_DB}",
            "${POSTGRES_USER}",
            "${POSTGRES_PASSWORD}",
            "${BACKEND_PORT}",
            "${FRONTEND_PORT}",
            "${VITE_BACKEND_URL}",
            "./src/backend/db/init-db.sql:/docker-entrypoint-initdb.d/init-db.sql:ro",
        ),
        label="docker-compose.yaml",
    )


def test_backend_layer_readmes_identify_starter_surfaces() -> None:
    layer_docs = {
        "src/backend/controller/README.md": (
            "starter examples",
            "document_controller.py",
            "appointment_controller.py",
        ),
        "src/backend/gateway/README.md": (
            "optional",
            "llm_gateway.py",
            "OPENAI_MODEL",
        ),
        "src/backend/handlers/README.md": (
            "starter examples",
            "document_handlers.py",
            "appointment_handlers.py",
            "not mounted by default",
        ),
        "src/backend/repository/README.md": (
            "starter examples",
            "document_repository.py",
            "appointment_repository.py",
            "src/backend/db/init-db.sql",
        ),
    }

    for relative_path, expected_snippets in layer_docs.items():
        contents = _read(relative_path)
        assert contents.strip(), f"{relative_path} should not be empty"
        _assert_contains_all(contents, expected_snippets, label=relative_path)
