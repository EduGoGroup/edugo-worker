"""What the two injection scripts share: where things live outside this repo, the
local-environment guard, the `--out-dir` rule and the exit-code rule.

Exit codes of both scripts:
    0  everything asserted by the cases without model passed (see `exit_code`)
    1  a case without model has a failed assertion, or a case could not run
    2  invalid usage
    3  the guard refused: this is not the local environment; nothing was called

Standard library only. The guard's decision (`guard_problems`) takes the facts as
parameters so it can be tested without network; `collect_guard_facts` reads them.
"""
import datetime
import os
import subprocess
import urllib.error
import urllib.parse
import urllib.request

EXIT_OK, EXIT_FAILED, EXIT_USAGE, EXIT_GUARD = 0, 1, 2, 3

HERE = os.path.dirname(os.path.abspath(__file__))
WORKER_ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))

# `EduBack/` holds what lives outside this repo: its Makefile (worker and Ollama
# targets) and the `.env.local` of the APIs. By default it is the worker's parent.
EDUBACK_VAR = "EDUGO_EDUBACK_DIR"
EDUBACK = os.path.abspath(os.environ.get(EDUBACK_VAR) or os.path.join(WORKER_ROOT, ".."))
PDF_DIR = os.path.join(WORKER_ROOT, "cmd", "llm-harness", "testdata", "material")

IDENTITY = os.environ.get("EDUGO_IDENTITY_URL", "http://localhost:8070")
LEARNING = os.environ.get("EDUGO_LEARNING_URL", "http://localhost:8065")
ACADEMIC = os.environ.get("EDUGO_ACADEMIC_URL", "http://localhost:8060")
OLLAMA = os.environ.get("EDUGO_OLLAMA_URL", "http://localhost:11434")
RABBIT_API = os.environ.get("RABBIT_API", "http://localhost:15672")
# Factory default of the local RabbitMQ container; override with the environment.
RABBIT_USER = os.environ.get("RABBIT_USER", "guest")
RABBIT_PASS = os.environ.get("RABBIT_PASS", "guest")  # edugo:not-a-secret

MODE_FILE = os.environ.get("EDUGO_MODE_FILE", "/tmp/edugo-mode")
LOCAL_PG_PORT = os.environ.get("EDUGO_LOCAL_PG_PORT", "5433")
LOOPBACK_HOSTS = ("localhost", "127.0.0.1", "::1")

# The run of 2026-11-24 is evidence (R-058): no result is ever written under it.
EVIDENCE_PARTS = ("analisis", "inferencia-e2e")


class UsageError(Exception):
    """Invalid invocation: exit code 2."""


class GuardError(Exception):
    """Not the local environment: exit code 3."""


class MissingPathError(RuntimeError):
    """Something that lives outside the repo is not where it is expected: exit code 1."""


def all_urls():
    """Every URL a script talks to with a credential or a write."""
    return {"EDUGO_IDENTITY_URL": IDENTITY, "EDUGO_LEARNING_URL": LEARNING,
            "EDUGO_ACADEMIC_URL": ACADEMIC, "EDUGO_OLLAMA_URL": OLLAMA,
            "RABBIT_API": RABBIT_API}


def api_urls():
    """The three APIs whose live process is inspected by the guard."""
    return {"EDUGO_IDENTITY_URL": IDENTITY, "EDUGO_LEARNING_URL": LEARNING,
            "EDUGO_ACADEMIC_URL": ACADEMIC}


# ---------------------------------------------------------------- external paths

def require_path(path, variable=EDUBACK_VAR):
    """Returns `path` if it exists; otherwise the message names the variable that fixes it."""
    if not os.path.exists(path):
        raise MissingPathError(f"{path} does not exist: set {variable} to the EduBack/ folder "
                               "that holds the Makefile and the APIs' .env.local")
    return path


def external_paths(eduback=None):
    eduback = eduback or EDUBACK
    return [os.path.join(eduback, "Makefile"),
            os.path.join(eduback, "edugo-api-learning", ".env.local"),
            os.path.join(eduback, "edugo-api-identity", ".env.local")]


def require_external_paths(eduback=None):
    for path in external_paths(eduback):
        require_path(path)


# ---------------------------------------------------------------- the guard

def is_loopback(url):
    try:
        host = urllib.parse.urlsplit(url).hostname
    except ValueError:
        return False
    return host in LOOPBACK_HOSTS


def url_problems(urls):
    """Check (a): every URL is loopback. Needs no request."""
    return [f"{name}={url} is not a loopback URL (localhost, 127.0.0.1 or ::1)"
            for name, url in sorted(urls.items()) if not is_loopback(url)]


def mode_problems(mode):
    """Check (b): `make up-local` wrote `local` in the mode file."""
    if mode is None:
        return [f"{MODE_FILE} does not exist: the stack is down or was not started with "
                "`make -C EduBack up-local`"]
    if mode.strip() != "local":
        return [f"{MODE_FILE} says {mode.strip()!r}, not 'local': the APIs do not point to the "
                "local database"]
    return []


def connection_problems(api_db_connections):
    """Check (c): the live process of each API holds a connection to the local Postgres.

    `api_db_connections` maps the API's variable name to True (connected), False (a process
    listens but has no connection to the local Postgres) or None (nothing listens / no answer).
    """
    out = []
    for name, connected in sorted(api_db_connections.items()):
        if connected is None:
            out.append(f"{name}: no process answers /health on its port")
        elif not connected:
            out.append(f"{name}: its process has no open connection to the local Postgres "
                       f"(:{LOCAL_PG_PORT})")
    return out


def guard_problems(urls, mode, api_db_connections):
    """The whole decision, from facts. Empty list = this is the local environment.

    Fails closed: a fact that could not be read counts as a problem.
    """
    return url_problems(urls) + mode_problems(mode) + connection_problems(api_db_connections)


def read_mode(path=None):
    try:
        with open(path or MODE_FILE, encoding="utf-8") as fh:
            return fh.read()
    except OSError:
        return None


def _lsof(*args):
    try:
        proc = subprocess.run(("lsof", "-nP") + args, capture_output=True, text=True, timeout=15)
    except (OSError, subprocess.SubprocessError):
        return ""
    return proc.stdout


def _health(url):
    """Unauthenticated read; makes the API touch its database before lsof looks."""
    try:
        with urllib.request.urlopen(url.rstrip("/") + "/health", timeout=5) as resp:
            return resp.status == 200
    except (urllib.error.URLError, OSError, ValueError):
        return False


def process_has_local_db_connection(url):
    """True / False / None for one API (see `connection_problems`)."""
    port = urllib.parse.urlsplit(url).port
    if not port or not _health(url):
        return None
    pids = _lsof(f"-iTCP:{port}", "-sTCP:LISTEN", "-t").split()
    if not pids:
        return None
    suffix = f":{LOCAL_PG_PORT}"
    for pid in pids:
        for line in _lsof("-a", "-p", pid, "-iTCP").splitlines():
            target = line.split("->")[-1].split(" ")[0] if "->" in line else ""
            if target.endswith(suffix) and target[:-len(suffix)].strip("[]") in LOOPBACK_HOSTS:
                return True
    return False


def enforce_guard(urls=None, apis=None):
    """Raises GuardError unless this is the local environment.

    Checks (a) and (b) run first and need no request: a non-loopback URL is never called.
    """
    urls = all_urls() if urls is None else urls
    apis = api_urls() if apis is None else apis
    problems = url_problems(urls) + mode_problems(read_mode())
    if not problems:
        facts = {name: process_has_local_db_connection(url) for name, url in apis.items()}
        problems = connection_problems(facts)
    if problems:
        raise GuardError("not the local environment; nothing was called:\n  - "
                         + "\n  - ".join(problems))


# ---------------------------------------------------------------- --out-dir

def inside_evidence(path):
    parts = os.path.realpath(path).split(os.sep)
    width = len(EVIDENCE_PARTS)
    return any(tuple(parts[i:i + width]) == EVIDENCE_PARTS for i in range(len(parts)))


def check_out_dir(out_dir):
    """Validates `--out-dir` without creating anything. Returns its absolute path."""
    if not out_dir:
        raise UsageError("--out-dir is required to run cases")
    path = os.path.abspath(out_dir)
    if inside_evidence(path):
        raise UsageError(f"--out-dir {path} is inside the evidence of the 2026-11-24 run "
                         "(documentacion/analisis/inferencia-e2e/): choose another folder")
    return path


def new_run_dir(out_dir, now=None):
    """Creates and returns one folder per run inside `--out-dir`, named by the UTC time."""
    base = check_out_dir(out_dir)
    now = now or datetime.datetime.now(datetime.timezone.utc)
    stamp = now.strftime("%Y%m%dT%H%M%SZ")
    os.makedirs(base, exist_ok=True)
    attempt = 1
    while True:
        path = os.path.join(base, stamp if attempt == 1 else f"{stamp}-{attempt}")
        try:
            os.mkdir(path)
            return path
        except FileExistsError:
            attempt += 1


# ---------------------------------------------------------------- exit code (ADR 0048)

def could_not_run(summary):
    """A case that broke before seeding anything: the environment, not the model."""
    return bool(summary.get("error")) and not summary.get("seeded")


def is_unmet(summary):
    """A case with model that did not reach its result: counted, never a gate."""
    return bool(summary.get("needs_model")) and not could_not_run(summary) and \
        bool(summary.get("failed") or summary.get("error"))


def exit_code(summaries):
    """0 or 1 from the cases' summaries.

    Each summary: {"needs_model": bool, "error": str or None, "failed": [names],
    "seeded": bool}. A case without model must pass: a failed assertion or any error is 1.
    A case with model is measured: failed assertions and errors after seeding (a timeout
    waiting for the model) are counted and do not raise the code. A case that could not
    run at all is 1 in both kinds.
    """
    for summary in summaries:
        if could_not_run(summary):
            return EXIT_FAILED
        if not summary.get("needs_model") and (summary.get("failed") or summary.get("error")):
            return EXIT_FAILED
    return EXIT_OK


def unmet_line(summaries):
    """The line both scripts print for the cases with model."""
    unmet = [s["case"] for s in summaries if is_unmet(s)]
    return f"casos con modelo no cumplidos: {len(unmet)}" + (
        " (" + ", ".join(unmet) + ")" if unmet else "")
