"""Shared helpers for the injection script (preparation and review rails).

Everything here talks to the LOCAL environment only: the APIs, RabbitMQ's
management API, the local Postgres container and the worker / Ollama processes
started by `EduBack/Makefile`. `e2e_env.enforce_guard` (called by run.py before the
first request) is what makes that true. No secret is written in this file: they are
read at run time from the environment or from the pieces' own local files.
"""
import base64
import datetime
import hashlib
import hmac
import json
import os
import re
import signal
import subprocess
import sys
import time
import traceback
import urllib.error
import urllib.parse
import urllib.request
import uuid

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.dirname(HERE))  # e2e_env.py lives one level up

import e2e_env  # noqa: E402 - needs the path above

EDUBACK = e2e_env.EDUBACK

IDENTITY = e2e_env.IDENTITY
LEARNING = e2e_env.LEARNING
ACADEMIC = e2e_env.ACADEMIC
OLLAMA = e2e_env.OLLAMA
RABBIT_API = e2e_env.RABBIT_API

WORKER_LOG = "/tmp/edugo-worker.log"
WORKER_PID = "/tmp/edugo-worker.pid"
WORKER_READY_LINE = "Worker listo - esperando mensajes..."

REVIEW_QUEUE = "edugo.attempt.review_requested"
PREP_QUEUE = "edugo.question.prep_requested"
MATERIAL_QUEUE = "edugo.material.assessment.requested"
WORK_QUEUES = (REVIEW_QUEUE, PREP_QUEUE, MATERIAL_QUEUE)


def dlq(queue):
    return queue + ".dlq"


PG_CONTAINER = os.environ.get("EDUGO_PG_CONTAINER", "edugo-migrator-postgres")
# Versioned in EduBack/edugo-dev-environment/migrator/docker-compose.local.yml.
# `docker exec ... psql` uses the container's local socket: no password needed.
PG_USER = os.environ.get("EDUGO_PG_USER", "edugo")
PG_DB = os.environ.get("EDUGO_PG_DB", "edugo")

PER_ANSWER_TIMEOUT = int(os.environ.get("EDUGO_ANSWER_TIMEOUT", "300"))
UUID_RE = re.compile(r"^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$")


class ApiError(RuntimeError):
    pass


class EnvError(RuntimeError):
    pass


def now_iso():
    return datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def uid(value):
    """Guard for every identifier interpolated into SQL."""
    if not isinstance(value, str) or not UUID_RE.match(value):
        raise EnvError(f"not a uuid: {value!r}")
    return value


def poll(fn, timeout, interval=2.0, what="condition"):
    """Calls fn() until it returns something truthy; raises on timeout."""
    deadline = time.time() + timeout
    while True:
        value = fn()
        if value:
            return value
        if time.time() >= deadline:
            raise EnvError(f"timeout ({timeout} s) waiting for: {what}")
        time.sleep(interval)


def poll_soft(fn, timeout, interval=2.0):
    """Like poll, but returns None on timeout instead of raising."""
    try:
        return poll(fn, timeout, interval)
    except EnvError:
        return None


# ---------------------------------------------------------------- HTTP

def call(method, url, body=None, token=None, ok=(200, 201, 204), headers=None, timeout=30,
         basic=None, raw_body=None):
    """Returns (status, json). Raises ApiError when status is not in `ok` (None = accept all)."""
    data = raw_body if raw_body is not None else (
        json.dumps(body).encode() if body is not None else None)
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    if basic:
        req.add_header("Authorization", "Basic " + base64.b64encode(basic.encode()).decode())
    for key, value in (headers or {}).items():
        req.add_header(key, value)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read()
            status = resp.status
    except urllib.error.HTTPError as err:
        raw = err.read()
        status = err.code
    try:
        payload = json.loads(raw) if raw else None
    except ValueError:
        payload = raw.decode(errors="replace")[:300]
    if ok is not None and status not in ok:
        raise ApiError(f"{method} {url.split('/api/')[-1]} -> {status}: {payload}")
    return status, payload


def unwrap(payload):
    if isinstance(payload, dict) and isinstance(payload.get("data"), (dict, list)):
        return payload["data"]
    return payload


def error_code(payload):
    return payload.get("code") if isinstance(payload, dict) else None


def env_file_value(path, key):
    with open(path, encoding="utf-8") as fh:
        for line in fh:
            m = re.match(r"\s*" + re.escape(key) + r"\s*=\s*(.*)", line)
            if m:
                return m.group(1).strip().strip('"').strip("'")
    raise EnvError(f"{key} is not in {path}")


# ---------------------------------------------------------------- user tokens

_TOKENS = {}


def core_url():
    path = os.path.join(EDUBACK, "edugo-api-identity", ".env.local")
    return env_file_value(path, "AUTH_EXCHANGE_IDENTITY_URL").rstrip("/")


def token_for(email, desired_context):
    """Login in identity-core and exchange in the local identity. Cached for 8 minutes."""
    key = (email, json.dumps(desired_context, sort_keys=True))
    cached = _TOKENS.get(key)
    if cached and time.time() - cached[1] < 480:
        return cached[0]
    password = os.environ.get("EDUGO_BASE_PASSWORD")
    if not password:
        raise EnvError("EDUGO_BASE_PASSWORD is missing from the environment")
    _, login = call("POST", core_url() + "/api/v1/auth/login",
                    {"email": email, "password": password, "system": "edu2go.kmp"})
    body = {"identity_token": login["identity_token"]}
    if desired_context:
        body["desired_context"] = desired_context
    _, exchange = call("POST", IDENTITY + "/api/v1/auth/exchange", body)
    _TOKENS[key] = (exchange["access_token"], time.time())
    return exchange["access_token"]


# ---------------------------------------------------------------- service token

def _b64url(raw):
    return base64.urlsafe_b64encode(raw).rstrip(b"=")


def service_token(audience="edugo-api-learning",
                  scopes=("attempts.review", "questions.prep", "materials.pipeline")):
    """HS256 service token minted per call. Never printed, never stored."""
    secret = env_file_value(os.path.join(EDUBACK, "edugo-api-learning", ".env.local"),
                            "SERVICE_JWT_SECRET")
    now = int(time.time())
    claims = {"token_use": "service", "client_id": "edugo-worker", "scopes": list(scopes),
              "iss": "edugo-identity", "sub": "service:edugo-worker", "aud": [audience],
              "jti": str(uuid.uuid4()), "iat": now, "nbf": now, "exp": now + 900}
    head = _b64url(json.dumps({"alg": "HS256", "typ": "JWT"}, separators=(",", ":")).encode())
    body = _b64url(json.dumps(claims, separators=(",", ":")).encode())
    signing = head + b"." + body
    sig = _b64url(hmac.new(secret.encode(), signing, hashlib.sha256).digest())
    return (signing + b"." + sig).decode()


def internal(method, path, body=None, ok=(200, 201, 204)):
    """Call to learning's /api/v1/internal with a fresh service token."""
    return call(method, LEARNING + "/api/v1/internal" + path, body, service_token(), ok=ok)


def school_settings(school_id):
    """Resolved settings of a school, as academic gives them to the worker. Returns {key: value}."""
    token = service_token("edugo-api-academic", ("schools.settings.read",))
    _, payload = call("GET", f"{ACADEMIC}/api/v1/internal/schools/{uid(school_id)}/settings",
                      None, token)
    data = unwrap(payload) or {}
    return {s["key"]: s["value"] for s in data.get("settings", [])}


# ---------------------------------------------------------------- RabbitMQ (management API)

def _rabbit_auth():
    # RABBIT_USER / RABBIT_PASS, or the factory default of the local container.
    return f"{e2e_env.RABBIT_USER}:{e2e_env.RABBIT_PASS}"


def _rabbit(method, path, body=None, ok=(200, 201, 204)):
    return call(method, RABBIT_API + "/api" + path, body, basic=_rabbit_auth(), ok=ok)


def _q(queue):
    return "/queues/%2F/" + urllib.parse.quote(queue, safe="")


def queue_depth(queue):
    """{'ready','unacked','total'}. Management stats lag a few seconds: poll, do not read once."""
    _, info = _rabbit("GET", _q(queue))
    return {"ready": info.get("messages_ready", 0),
            "unacked": info.get("messages_unacknowledged", 0),
            "total": info.get("messages", 0)}


def peek(queue, count=50):
    """Reads messages WITHOUT consuming them (they are requeued). Exact, unlike the stats."""
    _, rows = _rabbit("POST", _q(queue) + "/get",
                      {"count": count, "ackmode": "ack_requeue_true", "encoding": "auto"})
    out = []
    for row in rows or []:
        props = row.get("properties") or {}
        out.append({"payload": row.get("payload"), "payload_encoding": row.get("payload_encoding"),
                    "headers": props.get("headers") or {},
                    "delivery_mode": props.get("delivery_mode"),
                    "exchange": row.get("exchange"), "routing_key": row.get("routing_key"),
                    "redelivered": row.get("redelivered")})
    return out


def find_messages(queue, needle, count=200):
    """Messages of a queue whose body contains `needle` (an id or a per-run suffix)."""
    return [m for m in peek(queue, count) if needle in (m.get("payload") or "")]


def publish(queue, payload, headers=None):
    """Publishes through the default exchange with the queue name as routing key. Persistent."""
    text = payload if isinstance(payload, str) else json.dumps(payload)
    _, res = _rabbit("POST", "/exchanges/%2F/amq.default/publish", {
        "properties": {"delivery_mode": 2, "content_type": "application/json",
                       "headers": headers or {}},
        "routing_key": queue, "payload": text, "payload_encoding": "string"})
    if not (res or {}).get("routed"):
        raise EnvError(f"message to {queue} was not routed")
    return True


def purge(queue):
    _rabbit("DELETE", _q(queue) + "/contents")


def wait_queue_empty(queue, timeout=120):
    """Polls until the queue has neither ready nor unacked messages."""
    def empty():
        depth = queue_depth(queue)
        return depth["total"] == 0 and depth["ready"] == 0 and depth["unacked"] == 0
    return bool(poll_soft(empty, timeout, 2.0))


def depths():
    out = {}
    for queue in WORK_QUEUES:
        out[queue] = queue_depth(queue)["total"]
        out[dlq(queue)] = queue_depth(dlq(queue))["total"]
    return out


# ---------------------------------------------------------------- SQL (local container)

def _psql(sql):
    proc = subprocess.run(
        ["docker", "exec", "-i", PG_CONTAINER, "psql", "-U", PG_USER, "-d", PG_DB,
         "-v", "ON_ERROR_STOP=1", "-At", "-c", sql],
        capture_output=True, text=True, timeout=60)
    if proc.returncode != 0:
        raise EnvError(f"psql rc={proc.returncode}: {proc.stderr.strip()[:300]}")
    return proc.stdout.strip()


def sql_rows(select):
    """Read-only query; returns a list of dicts."""
    if not select.lstrip().lower().startswith(("select", "with")):
        raise EnvError("sql_rows only reads")
    out = _psql(f"SELECT coalesce(json_agg(t), '[]'::json) FROM ({select}) t")
    return json.loads(out) if out else []


def sql_update_review_flow(school_id, value):
    """The ONLY write by SQL of this script (case R7)."""
    if value not in ("direct", "teacher"):
        raise EnvError("llm.review.flow only takes direct or teacher")
    return _psql("UPDATE academic.school_settings SET value='" + value + "' WHERE school_id='"
                 + uid(school_id) + "' AND key='llm.review.flow'")


# ---------------------------------------------------------------- worker and Ollama

def _make(target):
    # The worker and Ollama must not inherit the test accounts' password.
    env = {k: v for k, v in os.environ.items() if k not in ("EDUGO_BASE_PASSWORD", "RABBIT_PASS")}
    proc = subprocess.run(["make", "-C", EDUBACK, "-s", target], stdin=subprocess.DEVNULL,
                          capture_output=True, text=True, timeout=300, env=env)
    return proc.returncode, (proc.stdout + proc.stderr).strip()[-400:]


def _worker_listening():
    """The worker's metrics port: still bound while the old process shuts down."""
    proc = subprocess.run(["lsof", "-nP", "-iTCP:9090", "-sTCP:LISTEN", "-t"],
                          capture_output=True, text=True, timeout=15)
    return bool(proc.stdout.split())


def worker_alive():
    try:
        with open(WORKER_PID, encoding="utf-8") as fh:
            pid = int(fh.read().strip())
        os.kill(pid, 0)
        return True
    except (OSError, ValueError):
        return False


def _log_has_ready():
    try:
        with open(WORKER_LOG, encoding="utf-8", errors="replace") as fh:
            return WORKER_READY_LINE in fh.read()
    except OSError:
        return False


def stop_worker(tail=None):
    """Stops the worker. Drains the log first: starting it again truncates the file."""
    if tail is not None:
        tail.drain()
    rc, out = _make("_stop-worker")
    if rc != 0:
        raise EnvError(f"make _stop-worker rc={rc}: {out}")
    # The pid file is gone at once; the Go process may take a moment to let go of the
    # queues and of :9090. Starting (or seeding) before that races with it.
    poll(lambda: not worker_alive(), 30, 1.0, "worker stopped")
    poll_soft(lambda: not _worker_listening(), 20, 1.0)


def start_worker(tail=None):
    """Starts the worker (local mode) and waits for its ready line in the NEW log."""
    if worker_alive():
        return
    if tail is not None:
        tail.drain()
    rc, out = _make("_start-worker-local")
    if rc != 0:
        raise EnvError(f"make _start-worker-local rc={rc}: {out}")
    if tail is not None:
        tail.reset()
    poll(lambda: worker_alive() and _log_has_ready(), 240, 2.0, "worker ready line")


def ollama_up():
    try:
        status, _ = call("GET", OLLAMA + "/api/tags", ok=None, timeout=3)
        return status == 200
    except (urllib.error.URLError, OSError):
        return False


def stop_ollama():
    """`make _stop-ollama` only kills what make started; the fallback kills the listener."""
    _make("_stop-ollama")
    if poll_soft(lambda: not ollama_up(), 10, 1.0):
        return "make"
    proc = subprocess.run(["lsof", "-nP", "-iTCP:11434", "-sTCP:LISTEN", "-t"],
                          capture_output=True, text=True, timeout=15)
    for pid in proc.stdout.split():
        try:
            os.kill(int(pid), signal.SIGTERM)
        except (OSError, ValueError):
            pass
    if poll_soft(lambda: not ollama_up(), 20, 1.0):
        return "lsof"
    raise EnvError("Ollama still answers on :11434 after stopping it (is the desktop app "
                   "relaunching it?): the case cannot run")


def start_ollama():
    if ollama_up():
        return
    rc, out = _make("_ensure-ollama")
    if rc != 0:
        raise EnvError(f"make _ensure-ollama rc={rc}: {out}")
    poll(ollama_up, 60, 1.0, "Ollama on :11434")


# ---------------------------------------------------------------- worker log

FIELD_RE = re.compile(r'(\w+)=("(?:[^"\\]|\\.)*"|\S+)')


def parse_line(line):
    fields = {}
    for key, value in FIELD_RE.findall(line):
        if value.startswith('"'):
            value = value[1:-1].replace('\\"', '"')
        fields[key] = value
    return fields


def line_time(line):
    stamp = parse_line(line).get("time")
    try:
        return datetime.datetime.fromisoformat(stamp).timestamp() if stamp else None
    except ValueError:
        return None


class LogTail:
    """Reads only what the worker logs after the case started, across worker restarts."""

    def __init__(self, path=WORKER_LOG):
        self.path = path
        self.lines = []
        try:
            self.offset = os.path.getsize(path)
        except OSError:
            self.offset = 0

    def reset(self):
        self.offset = 0

    def drain(self):
        try:
            size = os.path.getsize(self.path)
        except OSError:
            return self.lines
        if size < self.offset:  # truncated by a restart
            self.offset = 0
        with open(self.path, "rb") as fh:
            fh.seek(self.offset)
            chunk = fh.read()
        end = chunk.rfind(b"\n")
        if end >= 0:
            self.lines.extend(chunk[:end].decode(errors="replace").split("\n"))
            self.offset += end + 1
        return self.lines

    def grep(self, *needles):
        return [ln for ln in self.drain() if all(n in ln for n in needles)]

    def wait_for(self, needles, timeout, at_least=1):
        found = poll_soft(lambda: len(self.grep(*needles)) >= at_least, timeout, 1.0)
        return self.grep(*needles) if found else []


LANE_LIST_MSG = "short_answer con prep list: carril triturado"
LANE_CRITERIA_MSG = "open_ended con prep criteria: carril por criterios"
REVIEWED_MSG = "answer revisada por LLM"


def lane_of(tail, answer_id, question_type):
    """Lane of one answer, read from the worker log (contract section 4)."""
    lines = tail.grep("answer_id=" + answer_id)
    reviewed = [i for i, ln in enumerate(lines) if REVIEWED_MSG in ln]
    entry = {"answer_id": answer_id, "question_type": question_type, "lane": "sin_worker",
             "log_line": None}
    if not reviewed:
        lane_lines = [ln for ln in lines if LANE_LIST_MSG in ln or LANE_CRITERIA_MSG in ln]
        if lane_lines:
            entry["lane"] = "lista" if LANE_LIST_MSG in lane_lines[-1] else "criterios"
            entry["log_line"] = lane_lines[-1]
            entry["incomplete"] = True  # lane chosen, no verdict line
        return entry
    last = reviewed[-1]
    before = lines[(reviewed[-2] + 1) if len(reviewed) > 1 else 0:last]
    entry["log_line"] = lines[last]
    if parse_line(lines[last]).get("en_blanco_por_regla") == "true":
        entry["lane"] = "blanco_por_regla"
    elif any(LANE_LIST_MSG in ln for ln in before):
        entry["lane"] = "lista"
        entry["lane_line"] = [ln for ln in before if LANE_LIST_MSG in ln][-1]
    elif any(LANE_CRITERIA_MSG in ln for ln in before):
        entry["lane"] = "criterios"
        entry["lane_line"] = [ln for ln in before if LANE_CRITERIA_MSG in ln][-1]
    else:
        entry["lane"] = "global"
    return entry


# ---------------------------------------------------------------- case context and result file

class Case:
    def __init__(self, name, title, needs_model=True):
        self.name, self.title, self.needs_model = name, title, needs_model
        self.started_at = now_iso()
        self.asserted, self.lanes, self.notes = [], [], []
        self.measured, self.ids = {}, {}
        self.error = None
        self.cleanups = []
        self.tail = LogTail()

    def check(self, name, expected, actual, passed=None, evidence=None):
        """Deterministic assertion (ADR 0048). Never use it for what the model writes."""
        if passed is None:
            passed = expected == actual
        self.asserted.append({"name": name, "expected": expected, "actual": actual,
                              "pass": bool(passed), "evidence": evidence})
        return bool(passed)

    def check_in(self, name, allowed, actual, evidence=None, tol=0.011):
        ok = actual is not None and any(abs(float(actual) - float(a)) <= tol for a in allowed)
        return self.check(name, {"one_of": sorted(set(round(float(a), 2) for a in allowed))},
                          actual, ok, evidence)

    def measure(self, key, value):
        self.measured[key] = value

    def note(self, text):
        self.notes.append(text)

    def defer(self, label, fn):
        self.cleanups.append((label, fn))

    def as_dict(self):
        return {"case": self.name, "title": self.title, "needs_model": self.needs_model,
                "started_at": self.started_at,
                "finished_at": now_iso(), "asserted": self.asserted, "measured": self.measured,
                "lanes": self.lanes, "ids": self.ids, "notes": self.notes, "error": self.error}


def restore_environment(case):
    """Leaves the worker up (and Ollama, for a case with model) and reports the queues.

    A case without model never starts Ollama here: it only undoes what it did itself,
    through its own cleanups.
    """
    for label, fn in reversed(case.cleanups):
        try:
            fn()
        except Exception as err:  # noqa: BLE001 - a failed cleanup must not hide the others
            case.note(f"la limpieza «{label}» falló: {err}")
    restores = [("worker arriba", lambda: start_worker(case.tail))]
    if case.needs_model:
        restores.insert(0, ("Ollama arriba", start_ollama))
    for label, fn in restores:
        try:
            fn()
        except Exception as err:  # noqa: BLE001
            case.note(f"restaurar «{label}» falló: {err}")
    try:
        for queue in (REVIEW_QUEUE, PREP_QUEUE):
            if not wait_queue_empty(queue, 60):
                case.note(f"la cola de trabajo {queue} no queda vacía al terminar el caso")
        left = depths()
        case.measured["queues_at_end"] = left
        dead = {q: n for q, n in left.items() if q.endswith(".dlq") and n}
        case.note("colas muertas al terminar: " + (json.dumps(dead) if dead else "vacías"))
    except Exception as err:  # noqa: BLE001
        case.note(f"no se pudieron leer las colas al terminar: {err}")


def run_case(name, title, fn, out_dir, needs_model=True):
    """Runs one case; always restores the environment and always writes its JSON in out_dir."""
    case = Case(name, title, needs_model)
    try:
        case.measured["queues_at_start"] = depths()
        fn(case)
    except Exception as err:  # noqa: BLE001 - the result file records it
        case.error = f"{type(err).__name__}: {err}"
        case.note(traceback.format_exc(limit=4))
    finally:
        restore_environment(case)
        os.makedirs(out_dir, exist_ok=True)
        path = os.path.join(out_dir, name + ".json")
        with open(path, "w", encoding="utf-8") as fh:
            json.dump(case.as_dict(), fh, indent=2, ensure_ascii=False, default=str)
    failed = [a["name"] for a in case.asserted if not a["pass"]]
    return {"case": name, "needs_model": needs_model, "error": case.error,
            "seeded": bool(case.ids), "asserted": len(case.asserted), "failed": failed}
