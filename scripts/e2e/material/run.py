#!/usr/bin/env python3
"""End-to-end injection for the "material → assessment" rail (cases M1 to M5).

It runs against the LOCAL environment only (`make -C EduBack up-local`, with
`edugo-minio` up and, for the cases with model, `embeddinggemma` in Ollama): the
guard of `e2e_env.py` refuses anything else with exit code 3 before the first
request. It leaves one JSON per case in `<--out-dir>/<run>/`. See ../README.md.

    python3 scripts/e2e/material/run.py --list
    EDUGO_BASE_PASSWORD=<the password of the @edugo.test accounts> \
        python3 scripts/e2e/material/run.py --cases M1 --out-dir DIR
    EDUGO_BASE_PASSWORD=... python3 scripts/e2e/material/run.py --no-model --out-dir DIR

It asserts only what is deterministic (ADR 0048); what the model writes goes to `measured`.
Data goes in through the API; the SQL is read-only. It carries no secret: the
accounts' password comes from the environment and the service token is minted at
runtime with the SERVICE_JWT_SECRET from EduBack/edugo-api-learning/.env.local.
"""
import argparse
import base64
import datetime
import hashlib
import hmac
import http.client
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
PDF_DIR = e2e_env.PDF_DIR  # read only: the battery belongs to cmd/llm-harness

IDENTITY = e2e_env.IDENTITY
LEARNING = e2e_env.LEARNING
RABBIT_API = e2e_env.RABBIT_API
# RABBIT_USER / RABBIT_PASS, or the factory default of the local container.
RABBIT_USER = e2e_env.RABBIT_USER
RABBIT_PASS = e2e_env.RABBIT_PASS
RABBIT_VHOST = os.environ.get("RABBIT_VHOST", "/")
# Container, user and database are those of estado-pipeline.sh (no password: docker exec).
PG_CONTAINER = os.environ.get("EDUGO_PG_CONTAINER", "edugo-migrator-postgres")
PG_USER = os.environ.get("EDUGO_PG_USER", "edugo")
PG_DB = os.environ.get("EDUGO_PG_DB", "edugo")
WORKER_LOG = os.environ.get("EDUGO_WORKER_LOG", "/tmp/edugo-worker.log")
WORKER_PID = os.environ.get("EDUGO_WORKER_PID", "/tmp/edugo-worker.pid")

Q_MATERIAL = "edugo.material.assessment.requested"
Q_REVIEW = "edugo.attempt.review_requested"
Q_PREP = "edugo.question.prep_requested"
WORK_QUEUES = (Q_MATERIAL, Q_REVIEW, Q_PREP)

# The draft's quota lives in one place, the worker's config (target_questions_default).
DRAFT_MAX_QUESTIONS = 20

# What the worker leaves in the job's last_error when the school has the pipeline off.
PIPELINE_OFF_REASON = "La generación con IA está apagada en esta escuela."

# --- The case data ----------------------------------------------------------

SAN_IGNACIO = {
    "school_id": "b1000000-0000-0000-0000-000000000001",
    "teacher": "prof.gonzalez@edugo.test",  # author of the wave 0 material; 5to B
    "student": "est.diego@edugo.test",  # enrolled in Ciencias 5to B, active period
    "subject_id": "dd000000-0000-0000-0000-000000000002",  # Ciencias Naturales
    "offering_id": "c5000000-0000-0000-0000-000000000004",  # Ciencias, 5to B
    "context": None,  # the exchange's default context is already 5to B
}
GLOBAL_ENGLISH = {
    "school_id": "b3000000-0000-0000-0000-000000000003",
    "teacher": "prof.martinez@edugo.test",
    "subject_id": "dd000000-0000-0000-0000-000000000007",  # English Basics A2
    "context": {"school_id": "b3000000-0000-0000-0000-000000000003",
                "academic_unit_id": "ac000000-0000-0000-0000-000000000014"},
}
# `needs_model`: False only where the green does not depend on what a model writes
# (ADR 0048). Those are the only cases that can be a gate; the rest are measured.
CASES = {
    "M1": {"title": "Un PDF corto: job done con phase=2 y un borrador draft, no público",
           "school": SAN_IGNACIO, "pdf": "ciclo_del_agua.pdf", "timeout_min": 45,
           "needs_model": True},
    "M2": {"title": "Reanudación: matar el worker en la fase 1 y arrancarlo sin reinyectar",
           "school": SAN_IGNACIO, "pdf": "fotosintesis.pdf", "timeout_min": 60,
           "window_timeout_min": 25, "needs_model": True},
    "M3": {"title": "Escuela con el pipeline apagado: 202, job failed con su motivo y 202 al repetir",
           "school": GLOBAL_ENGLISH, "pdf": "ciclo_del_agua.pdf", "timeout_min": 3,
           "needs_model": False},
    "M4": {"title": "La cadena entera: publicar el borrador, rendirlo y que el worker corrija",
           "school": SAN_IGNACIO, "timeout_min": 20, "prep_wait_seconds": 300,
           "needs_model": True},
    "M5": {"title": "Un PDF largo (opcional): lo de M1 y ningún trozo tumba el job",
           "school": SAN_IGNACIO, "pdf": "fotosintesis.pdf", "timeout_min": 90,
           "needs_model": True},
}

# The draft's questions are written by the model: the student answers by type.
# Closed ones are answered right with the answer the teacher sees. Short and
# open ones come from these lists, in order and wrapping around; `--answers-file`
# overrides them by question index.
SHORT_ANSWERS = [
    {"intent": "sensata, en frase (no casa con la esperada: va a revisión)",
     "answer": "Creo que es cuando el agua se evapora por el calor del Sol y sube como vapor"},
    {"intent": "incompleta a propósito", "answer": "el agua"},
    {"intent": "sensata, una palabra", "answer": "la condensación"},
    {"intent": "no sabe", "answer": "no lo recuerdo"},
]
OPEN_ANSWERS = [
    {"intent": "sensata y completa",
     "answer": ("El agua de mares, ríos y lagos se evapora por el calor del Sol y sube como vapor. "
                "Arriba se enfría y se condensa en gotitas que forman las nubes. Cuando las gotas "
                "pesan demasiado caen como lluvia, nieve o granizo: es la precipitación. Esa agua "
                "corre por la superficie o se infiltra en el suelo y vuelve a los ríos y al mar, "
                "y el ciclo empieza de nuevo.")},
    {"intent": "incompleta a propósito", "answer": "El agua sube al cielo y después llueve."},
    {"intent": "parcial",
     "answer": "El Sol calienta el agua y la evapora; luego forma nubes. No recuerdo qué pasa después."},
]

UUID_RE = re.compile(r"^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$")


class ApiError(RuntimeError):
    pass


def now_iso():
    return datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def uid(value):
    """Lets only a UUID through: it is the only thing interpolated into the SQL."""
    value = str(value or "")
    if not UUID_RE.match(value):
        raise ApiError(f"no es un UUID: {value!r}")
    return value.lower()


# --- HTTP and tokens --------------------------------------------------------

def call(method, url, body=None, token=None, ok=(200, 201, 204)):
    """Returns (status, json). Raises ApiError if the status is not in `ok`."""
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            raw = resp.read()
            status, payload = resp.status, (json.loads(raw) if raw else None)
    except urllib.error.HTTPError as err:
        raw = err.read()
        try:
            payload = json.loads(raw)
        except ValueError:
            payload = raw.decode(errors="replace")[:300]
        status = err.code
    if ok is not None and status not in ok:
        raise ApiError(f"{method} {url.split('/api/')[-1]} → {status}: {payload}")
    return status, payload


def unwrap(payload):
    if isinstance(payload, dict) and isinstance(payload.get("data"), (dict, list)):
        return payload["data"]
    return payload


def as_list(payload, *keys):
    rows = unwrap(payload)
    if isinstance(rows, dict):
        for key in keys + ("items",):
            if isinstance(rows.get(key), list):
                return rows[key]
        return []
    return rows or []


def env_value(path, name):
    with open(path, encoding="utf-8") as fh:
        for line in fh:
            m = re.match(r"\s*" + re.escape(name) + r"\s*=\s*(.+)", line)
            if m:
                return m.group(1).strip().strip('"').strip("'")
    raise ApiError(f"{name} no está en {path}")


def token_for(email, desired_context=None):
    """Logs in to identity-core and exchanges in the local identity. Returns the access token."""
    password = os.environ.get("EDUGO_BASE_PASSWORD")
    if not password:
        raise ApiError("falta EDUGO_BASE_PASSWORD en el entorno")
    core = env_value(os.path.join(EDUBACK, "edugo-api-identity", ".env.local"),
                     "AUTH_EXCHANGE_IDENTITY_URL").rstrip("/")
    _, login = call("POST", core + "/api/v1/auth/login",
                    {"email": email, "password": password, "system": "edu2go.kmp"})
    body = {"identity_token": login["identity_token"]}
    if desired_context:
        body["desired_context"] = desired_context
    _, exchange = call("POST", IDENTITY + "/api/v1/auth/exchange", body)
    return exchange["access_token"]


def teacher_token(school):
    return token_for(school["teacher"], school.get("context"))


def b64url(raw):
    return base64.urlsafe_b64encode(raw).rstrip(b"=")


def service_token():
    """Hand-rolled HS256 with the worker's claims (contract §3). Lasts 15 minutes."""
    secret = env_value(os.path.join(EDUBACK, "edugo-api-learning", ".env.local"),
                       "SERVICE_JWT_SECRET")
    now = int(time.time())
    claims = {"token_use": "service", "client_id": "edugo-worker",
              "scopes": ["attempts.review", "questions.prep", "materials.pipeline"],
              "iss": "edugo-identity", "sub": "service:edugo-worker",
              "aud": ["edugo-api-learning"], "jti": str(uuid.uuid4()),
              "iat": now, "nbf": now, "exp": now + 900}
    head = b64url(json.dumps({"alg": "HS256", "typ": "JWT"}, separators=(",", ":")).encode())
    body = b64url(json.dumps(claims, separators=(",", ":")).encode())
    signature = hmac.new(secret.encode(), head + b"." + body, hashlib.sha256).digest()
    return (head + b"." + body + b"." + b64url(signature)).decode()


# --- Read-only SQL ----------------------------------------------------------

def sql(query):
    """Runs a SELECT on the local Postgres and returns its rows as dicts."""
    wrapped = f"SELECT coalesce(json_agg(t), '[]'::json) FROM ({query}) t"
    cmd = ["docker", "exec", "-e", "PGOPTIONS=-c default_transaction_read_only=on",
           PG_CONTAINER, "psql", "-U", PG_USER, "-d", PG_DB, "-tA", "-v", "ON_ERROR_STOP=1",
           "-c", wrapped]
    out = subprocess.run(cmd, capture_output=True, text=True, timeout=60)
    if out.returncode != 0:
        raise ApiError(f"psql rc={out.returncode}: {out.stderr.strip()[:300]}")
    return json.loads(out.stdout.strip() or "[]")


def job_row(job_id):
    rows = sql("SELECT j.id, j.material_id, j.school_id, j.status, j.phase, j.assessment_id, "
               "j.last_error, j.created_at, j.updated_at, j.completed_at, m.status AS material_status "
               "FROM content.material_pipeline_job j JOIN content.materials m ON m.id = j.material_id "
               f"WHERE j.id = '{uid(job_id)}'")
    return rows[0] if rows else None


def chunk_rows(job_id):
    return sql("SELECT id, seq, status, attempts, updated_at, length(chunk_text) AS chars, "
               "array_length(regexp_split_to_array(trim(chunk_text), '\\s+'), 1) AS words "
               f"FROM content.material_pipeline_chunk WHERE job_id = '{uid(job_id)}' ORDER BY seq")


def count_by(rows, key):
    out = {}
    for row in rows:
        out[str(row.get(key))] = out.get(str(row.get(key)), 0) + 1
    return out


def candidate_stats(job_id):
    rows = sql("SELECT status, payload->>'question_type' AS question_type, count(*) AS n "
               f"FROM content.material_pipeline_candidate WHERE job_id = '{uid(job_id)}' "
               "GROUP BY 1, 2")
    by_status, by_type, total = {}, {}, 0
    for row in rows:
        total += row["n"]
        by_status[row["status"]] = by_status.get(row["status"], 0) + row["n"]
        by_type.setdefault(str(row["question_type"]), {})[row["status"]] = row["n"]
    return {"total": total, "by_status": by_status, "by_type_and_status": by_type}


def question_rows(assessment_id):
    return sql("SELECT id, sort_order, question_type, points, llm_prep_status, "
               "(llm_prep_source_hash IS NOT NULL) AS has_source_hash, llm_prep_updated_at, "
               "(correct_answer IS NOT NULL) AS has_correct_answer "
               f"FROM assessment.question WHERE assessment_id = '{uid(assessment_id)}' "
               "ORDER BY sort_order")


def answer_rows(attempt_id):
    return sql("SELECT a.id, a.question_index, a.question_id, q.question_type, a.student_answer, "
               "a.review_status, a.is_correct, a.points_earned, a.max_points, "
               "r.review_source, r.points_awarded, r.feedback, r.updated_at AS review_updated_at "
               "FROM assessment.assessment_attempt_answer a "
               "LEFT JOIN assessment.question q ON q.id = a.question_id "
               "LEFT JOIN assessment.attempt_review r ON r.attempt_answer_id = a.id "
               f"WHERE a.attempt_id = '{uid(attempt_id)}' ORDER BY a.question_index")


def attempt_row(attempt_id):
    rows = sql("SELECT id, status, score, max_score, percentage, completed_at, ai_review_claimed_at "
               f"FROM assessment.assessment_attempt WHERE id = '{uid(attempt_id)}'")
    return rows[0] if rows else None


def school_settings(school_id):
    rows = sql(f"SELECT key, value FROM academic.school_settings WHERE school_id = '{uid(school_id)}'")
    return {row["key"]: row["value"] for row in rows}


# --- RabbitMQ, through the management API -----------------------------------

def rabbit(method, path, body=None):
    url = RABBIT_API.rstrip("/") + "/api/" + path
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    basic = base64.b64encode(f"{RABBIT_USER}:{RABBIT_PASS}".encode()).decode()
    req.add_header("Authorization", "Basic " + basic)
    req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req, timeout=15) as resp:
        raw = resp.read()
        return json.loads(raw) if raw else None


def queue_depth(name):
    vhost = urllib.parse.quote(RABBIT_VHOST, safe="")
    info = rabbit("GET", f"queues/{vhost}/{urllib.parse.quote(name, safe='')}")
    return {"ready": info.get("messages_ready", 0), "unacked": info.get("messages_unacknowledged", 0),
            "total": info.get("messages", 0)}


def depths():
    out = {}
    for name in WORK_QUEUES:
        for queue in (name, name + ".dlq"):
            try:
                out[queue] = queue_depth(queue)
            except (urllib.error.URLError, OSError, ValueError) as err:
                out[queue] = {"error": str(err)[:120]}
    return out


def peek_dlq(name, limit=5):
    """Peeks, requeueing them, at up to `limit` messages of a dead-letter queue."""
    vhost = urllib.parse.quote(RABBIT_VHOST, safe="")
    got = rabbit("POST", f"queues/{vhost}/{urllib.parse.quote(name, safe='')}/get",
                 {"count": limit, "ackmode": "ack_requeue_true", "encoding": "auto", "truncate": 2000})
    out = []
    for msg in got or []:
        out.append({"headers": (msg.get("properties") or {}).get("headers"),
                    "payload": str(msg.get("payload"))[:600]})
    return out


def wait_queue(name, predicate, timeout, interval=2.0):
    """Polls the queue until `predicate(depth)` holds. Returns (ok, last depth)."""
    deadline, last = time.time() + timeout, None
    while True:
        last = queue_depth(name)
        if predicate(last):
            return True, last
        if time.time() >= deadline:
            return False, last
        time.sleep(interval)


# --- The worker log ---------------------------------------------------------

KV_RE = re.compile(r'(\w+)=("(?:[^"\\]|\\.)*"|\S+)')


def parse_log_line(raw):
    fields = {}
    for key, value in KV_RE.findall(raw):
        if value.startswith('"'):
            value = value[1:-1].replace('\\"', '"')
        fields[key] = value
    return {"time": fields.get("time"), "msg": fields.get("msg", ""), "fields": fields,
            "raw": re.sub(r"\ssource=\S+", "", raw.strip())[:500]}


class LogTail:
    """Reads only what was written since it was created. Starting the worker TRUNCATES
    the log (`spawn` opens it with "wb"): if the file shrinks, it goes back to the start."""

    def __init__(self, path=WORKER_LOG):
        self.path = path
        self.offset = os.path.getsize(path) if os.path.exists(path) else 0
        self.start_offset = self.offset
        self.lines = []
        self.truncations = 0
        self._partial = b""
        self._head = self._read_head()

    def _read_head(self):
        """The first line carries the start time: if it changes, the log is another one."""
        try:
            with open(self.path, "rb") as fh:
                first = fh.readline(300)
        except OSError:
            return b""
        return first if first.endswith(b"\n") else b""  # only a complete line counts

    def restarted(self):
        """The worker was just started: the log is new and is read from the beginning.
        It has to be said: the first line is make's fixed banner, so the header does
        not change, and the new log may already have grown past the old one."""
        self.offset, self._partial = 0, b""
        self.truncations += 1

    def poll(self):
        if not os.path.exists(self.path):
            return self.lines
        size = os.path.getsize(self.path)
        head = self._read_head()
        # Truncated: the file shrank, or its header is no longer the one seen (the new
        # log may have grown past the old offset before this poll).
        replaced = bool(self._head) and bool(head) and head != self._head
        if head:
            self._head = head
        if size < self.offset or replaced:
            self.offset, self._partial = 0, b""
            self.truncations += 1
        if size > self.offset:
            with open(self.path, "rb") as fh:
                fh.seek(self.offset)
                data = self._partial + fh.read()
                self.offset = fh.tell()
            cut = data.rfind(b"\n")
            self._partial = data[cut + 1:] if cut >= 0 else data
            if cut >= 0:
                for raw in data[:cut].decode("utf-8", errors="replace").splitlines():
                    self.lines.append(parse_log_line(raw))
        return self.lines

    def find(self, msg_prefix, start=0, **fields):
        self.poll()
        out = []
        for line in self.lines[start:]:
            if not line["msg"].startswith(msg_prefix):
                continue
            if all(line["fields"].get(k) == str(v) for k, v in fields.items()):
                out.append(line)
        return out

    def wait(self, msg_prefix, timeout, start=0, **fields):
        deadline = time.time() + timeout
        while True:
            found = self.find(msg_prefix, start=start, **fields)
            if found or time.time() >= deadline:
                return found
            time.sleep(1.5)


def log_time(line):
    try:
        return datetime.datetime.fromisoformat(line["time"])
    except (TypeError, ValueError):
        return None


# --- The worker: kill and start ---------------------------------------------

def worker_pid():
    try:
        with open(WORKER_PID, encoding="utf-8") as fh:
            return int(fh.read().strip())
    except (OSError, ValueError):
        return None


def pid_alive(pid):
    if not pid:
        return False
    try:
        os.kill(pid, 0)
        return True
    except ProcessLookupError:
        return False
    except PermissionError:
        return True


def metrics_listener():
    out = subprocess.run(["lsof", "-nP", "-iTCP:9090", "-sTCP:LISTEN", "-t"],
                         capture_output=True, text=True, timeout=15)
    return [int(p) for p in out.stdout.split() if p.isdigit()]


def worker_up():
    return pid_alive(worker_pid()) or bool(metrics_listener())


def kill_worker():
    """SIGKILL to the worker's group (make → go run → binary): it dies mid-message,
    with no graceful shutdown. The Makefile's `_stop-worker` sends SIGTERM: no use here."""
    pid = worker_pid()
    killed = []
    if pid_alive(pid):
        try:
            os.killpg(pid, signal.SIGKILL)
        except (ProcessLookupError, PermissionError):
            os.kill(pid, signal.SIGKILL)
        killed.append(pid)
    time.sleep(1.0)
    for stray in metrics_listener():
        try:
            os.kill(stray, signal.SIGKILL)
            killed.append(stray)
        except ProcessLookupError:
            pass
    deadline = time.time() + 15
    while worker_up() and time.time() < deadline:
        time.sleep(0.5)
    return killed


def start_worker(timeout=240):
    """Starts the worker the way `make up-local` does and waits for `Worker listo`."""
    if worker_up():
        return True
    # The worker does not inherit the test accounts' password.
    env = {k: v for k, v in os.environ.items() if k not in ("EDUGO_BASE_PASSWORD", "RABBIT_PASS")}
    out = subprocess.run(["make", "-C", EDUBACK, "-s", "_start-worker-local"],
                         stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL,
                         stderr=subprocess.DEVNULL, timeout=120, env=env)
    if out.returncode != 0:
        raise ApiError(f"make _start-worker-local rc={out.returncode}")
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            with open(WORKER_LOG, encoding="utf-8", errors="replace") as fh:
                if "Worker listo" in fh.read():
                    return True
        except OSError:
            pass
        time.sleep(2)
    return False


# --- The result of a case ---------------------------------------------------

class Result:
    def __init__(self, case, title):
        self.data = {"case": case, "title": title, "started_at": now_iso(), "finished_at": None,
                     "asserted": [], "measured": {}, "lanes": [], "ids": {}, "notes": [],
                     "error": None}

    def check(self, name, expected, actual, passed, evidence=None):
        self.data["asserted"].append({"name": name, "expected": expected, "actual": actual,
                                      "pass": bool(passed), "evidence": evidence})
        print(f"  [{'ok' if passed else 'FALLA'}] {name}: {actual!r}", flush=True)
        return bool(passed)

    def measure(self, key, value):
        self.data["measured"][key] = value

    def note(self, text):
        self.data["notes"].append(text)
        print("  nota:", text, flush=True)

    def lane(self, answer_id, question_type, lane, log_line):
        self.data["lanes"].append({"answer_id": answer_id, "question_type": question_type,
                                   "lane": lane, "log_line": log_line})

    @property
    def ids(self):
        return self.data["ids"]

    def passed(self):
        return self.data["error"] is None and all(a["pass"] for a in self.data["asserted"])


def step(text):
    print(f"· {text}", flush=True)


# --- Shared pieces of the rail ----------------------------------------------

def resolve_pdf(name):
    path = name if os.path.isabs(name) or os.path.exists(name) else os.path.join(PDF_DIR, name)
    if not os.path.isfile(path):
        raise ApiError(f"no existe el PDF {name!r} (ni en {PDF_DIR})")
    return os.path.abspath(path)


def put_presigned(upload_url, data):
    """PUT to the presigned URL with the bare minimum (Host and Content-Length), as
    tested live: no Content-Type nor any other header (contract §9)."""
    parts = urllib.parse.urlsplit(upload_url)
    conn_cls = http.client.HTTPSConnection if parts.scheme == "https" else http.client.HTTPConnection
    conn = conn_cls(parts.netloc, timeout=120)
    try:
        target = parts.path + ("?" + parts.query if parts.query else "")
        conn.putrequest("PUT", target, skip_accept_encoding=True)
        conn.putheader("Content-Length", str(len(data)))
        conn.endheaders()
        conn.send(data)
        resp = conn.getresponse()
        body = resp.read()
        return resp.status, body.decode(errors="replace")[:300]
    finally:
        conn.close()


def upload_material(res, token, school, pdf_path, case):
    """The §9 sequence: material, file, presigned PUT and upload-complete."""
    with open(pdf_path, "rb") as fh:
        data = fh.read()
    name = os.path.basename(pdf_path)
    title = f"Inferencia e2e · {case} · {name} ({now_iso()})"
    _, created = call("POST", LEARNING + "/api/v1/materials", {
        "title": title, "description": f"Material del caso {case} del plan de inferencia e2e.",
        "subject_id": school["subject_id"], "is_public": False}, token)
    material_id = unwrap(created)["id"]
    _, file_resp = call("POST", f"{LEARNING}/api/v1/materials/{material_id}/files", {
        "file_name": name, "file_type": "application/pdf", "file_size_bytes": len(data)}, token)
    file_info = unwrap(file_resp)
    file_id = file_info.get("file_id") or file_info.get("id")
    status, body = put_presigned(file_info["upload_url"], data)
    if status not in (200, 204):
        raise ApiError(f"PUT prefirmado → {status}: {body}")
    call("POST", f"{LEARNING}/api/v1/materials/{material_id}/files/{file_id}/upload-complete",
         {"file_type": "application/pdf", "file_size_bytes": len(data)}, token)
    rows = sql("SELECT status, school_id, subject_id FROM content.materials "
               f"WHERE id = '{uid(material_id)}'")
    res.ids.update({"material_id": material_id, "file_id": file_id})
    res.measure("material", {"pdf": name, "bytes": len(data), "title": title,
                             "status_after_upload": rows[0]["status"] if rows else None})
    if not rows or rows[0]["school_id"] != school["school_id"]:
        raise ApiError(f"el material quedó en la escuela {rows[0]['school_id'] if rows else None}, "
                       f"no en {school['school_id']}: el contexto del token no es el esperado")
    return material_id


def request_generation(token, material_id):
    status, payload = call("POST", f"{LEARNING}/api/v1/materials/{material_id}/generate-assessment",
                           None, token, ok=None)
    return status, unwrap(payload)


def follow_job(res, job_id, tail, timeout, dlq_base):
    """Follows the job through SQL until `done` with `phase=2`. Leaves early if it drops
    to `failed` or if the message ended up in the dead-letter queue (the job stays
    `processing`: a transient error does not mark it). Only the delivery writes `done`,
    so a `done` below phase 2 must never be seen: if it is, the mark
    `done_below_phase2_seen_s` records when. Returns (job, reason, milestones)."""
    t0 = time.time()
    marks, reason, stuck = {}, "timeout", 0
    last_print, job = 0.0, None
    while time.time() - t0 < timeout:
        job = job_row(job_id)
        chunks = chunk_rows(job_id)
        by = count_by(chunks, "status")
        elapsed = time.time() - t0
        if chunks and "chunks_seen_s" not in marks:
            marks["chunks_seen_s"] = round(elapsed, 1)
        if job["status"] == "done" and "status_done_seen_s" not in marks:
            marks["status_done_seen_s"] = round(elapsed, 1)
        if job["status"] == "done" and job["phase"] < 2 and "done_below_phase2_seen_s" not in marks:
            marks["done_below_phase2_seen_s"] = round(elapsed, 1)
        if job["status"] == "done" and job["phase"] == 2:
            marks["phase2_seen_s"] = round(elapsed, 1)
            reason = "done_phase2"
            break
        if job["status"] == "failed":
            reason = "failed"
            break
        try:
            work, dead = queue_depth(Q_MATERIAL), queue_depth(Q_MATERIAL + ".dlq")
            stuck = stuck + 1 if (dead["total"] > dlq_base and work["total"] == 0) else 0
        except (urllib.error.URLError, OSError, ValueError):
            stuck = 0
        if stuck >= 4:
            reason = "dead_lettered"
            break
        if elapsed - last_print >= 30:
            last_print = elapsed
            print(f"    {elapsed:5.0f} s · job {job['status']}/phase={job['phase']} · trozos {by}",
                  flush=True)
        tail.poll()
        time.sleep(5)
    res.measure("job_wait", {"reason": reason, "waited_s": round(time.time() - t0, 1), **marks})
    return job_row(job_id), reason, marks


def measure_job(res, job_id, tail):
    """What the model writes: chunks, candidates, passes, warnings and timings."""
    chunks = chunk_rows(job_id)
    res.measure("chunks", {"total": len(chunks), "by_status": count_by(chunks, "status"),
                           "detail": [{"seq": c["seq"], "status": c["status"],
                                       "attempts": c["attempts"], "chars": c["chars"],
                                       "words": c["words"]} for c in chunks]})
    res.measure("candidates", candidate_stats(job_id))
    passes = {}
    for line in tail.find("fase 2 · pasada", job_id=job_id):
        m = re.match(r"fase 2 · pasada (\d)", line["msg"])
        keep = {k: v for k, v in line["fields"].items()
                if k not in ("time", "level", "msg", "severity", "service", "env", "version",
                             "source", "job_id")}
        passes[f"pasada_{m.group(1)}"] = {"msg": line["msg"], **keep}
    res.measure("reduce_passes", passes)
    warnings = {}
    for line in tail.find("", job_id=job_id):
        if line["fields"].get("level") in ("WARN", "ERROR"):
            warnings[line["msg"][:90]] = warnings.get(line["msg"][:90], 0) + 1
    res.measure("job_log_warnings", warnings)
    res.measure("chunks_isolated_in_log",
                len(tail.find("chunk envenenado aislado", job_id=job_id)))
    res.measure("delivery_events_in_log", len(tail.find("Error procesando evento")))
    times = {}
    for key, prefix in (("phase0_done", "fase 0 completa"), ("phase1_done", "fase 1 completa"),
                        ("phase2_done", "fase 2 completa")):
        found = tail.find(prefix, job_id=job_id)
        if found and log_time(found[-1]):
            times[key] = log_time(found[-1])
    job = job_row(job_id)
    timing = {"job_created_at": job["created_at"], "job_completed_at": job["completed_at"]}
    if "phase0_done" in times and "phase1_done" in times:
        timing["phase1_s"] = round((times["phase1_done"] - times["phase0_done"]).total_seconds(), 1)
    if "phase1_done" in times and "phase2_done" in times:
        timing["phase2_s"] = round((times["phase2_done"] - times["phase1_done"]).total_seconds(), 1)
    first = next((log_time(l) for l in tail.find("", job_id=job_id) if log_time(l)), None)
    if first and "phase0_done" in times:
        timing["phase0_s"] = round((times["phase0_done"] - first).total_seconds(), 1)
    if first and "phase2_done" in times:
        timing["total_s"] = round((times["phase2_done"] - first).total_seconds(), 1)
    if first:
        timing["first_job_line_log"] = first.isoformat()
    for key, moment in times.items():
        timing[key + "_log"] = moment.isoformat()
    res.measure("timing", timing)
    return chunks


def read_draft(res, token, job_id):
    """Asserts the draft: id via the internal route and via SQL, `draft`, not public."""
    job = job_row(job_id)
    status, internal = call("GET", f"{LEARNING}/api/v1/internal/pipeline/jobs/{job_id}", None,
                            service_token(), ok=None)
    internal = unwrap(internal) if isinstance(internal, dict) else {}
    res.check("la ruta interna del job responde 200 (token de servicio, materials.pipeline)",
              200, status, status == 200)
    res.check("el id del borrador sale igual por la ruta interna y por SQL",
              job["assessment_id"], internal.get("assessment_id"),
              bool(job["assessment_id"]) and internal.get("assessment_id") == job["assessment_id"],
              {"internal": {k: internal.get(k) for k in ("status", "phase", "chunk_counts")}})
    assessment_id = job["assessment_id"]
    if not assessment_id:
        return None
    res.ids["assessment_id"] = assessment_id
    _, payload = call("GET", f"{LEARNING}/api/v1/assessments/{assessment_id}", None, token)
    draft = unwrap(payload)
    row = sql("SELECT status, is_public, purpose, source_type, questions_count, title "
              f"FROM assessment.assessment WHERE id = '{uid(assessment_id)}'")[0]
    res.check("el borrador está en draft", "draft", draft.get("status"),
              draft.get("status") == "draft" and row["status"] == "draft", {"sql": row})
    res.check("el borrador no es público", False, draft.get("is_public"),
              draft.get("is_public") is False and row["is_public"] is False)
    questions = question_rows(assessment_id)
    selected = candidate_stats(job_id)["by_status"].get("selected", 0)
    res.check("el borrador tiene preguntas y son las candidatas `selected`",
              f"{selected} (≥1 y ≤{DRAFT_MAX_QUESTIONS})", len(questions),
              1 <= len(questions) <= DRAFT_MAX_QUESTIONS and len(questions) == selected,
              {"questions_count": row["questions_count"]})
    res.check("cada pregunta del borrador vale 1 punto", 1,
              sorted({q["points"] for q in questions}), all(q["points"] == 1 for q in questions))
    res.measure("draft", {"title": row["title"], "purpose": row["purpose"],
                          "source_type": row["source_type"], "questions": len(questions),
                          "question_types": count_by(questions, "question_type"),
                          "llm_prep_status_at_read": count_by(
                              [q for q in questions
                               if q["question_type"] in ("short_answer", "open_ended")],
                              "llm_prep_status")})
    return assessment_id


def note_dead_letters(res):
    for name in WORK_QUEUES:
        dlq = name + ".dlq"
        try:
            depth = queue_depth(dlq)
            if depth["total"]:
                res.note(f"cola muerta {dlq}: {depth['total']} mensaje(s): "
                         + json.dumps(peek_dlq(dlq), ensure_ascii=False)[:1200])
        except (urllib.error.URLError, OSError, ValueError) as err:
            res.note(f"no se pudo leer la cola muerta {dlq}: {err}")


def check_done_only_at_phase2(res, marks):
    """`done` is written only by the delivery: it is never seen below phase 2."""
    seen = marks.get("done_below_phase2_seen_s")
    res.check("nunca se ve el job en done con phase menor que 2", None, seen, seen is None)


def generation_case(res, args, cfg, school, material_id, token):
    """The common body of M1 and M5: request generation, follow the job, read the draft."""
    tail = LogTail()
    dlq_base = queue_depth(Q_MATERIAL + ".dlq")["total"]
    res.measure("queues_before", depths())
    step("generate-assessment")
    status, body = request_generation(token, material_id)
    job_id = (body or {}).get("id") if isinstance(body, dict) else None
    res.check("generate-assessment responde 202 (o 409 con el job activo, que se sigue)",
              "202 o 409", status, status in (202, 409) and bool(job_id), {"body": body})
    if status == 409:
        res.note("generate-assessment dio 409: ya había un job activo de ese material; se sigue ese "
                 "job, y parte de su recorrido puede ser anterior al inicio del caso.")
    if not job_id:
        raise ApiError(f"generate-assessment → {status}: {body}")
    res.ids["job_id"] = job_id
    timeout = (args.timeout_min or cfg["timeout_min"]) * 60
    step(f"siguiendo el job {job_id} (tope {timeout // 60} min)")
    job, reason, marks = follow_job(res, job_id, tail, timeout, dlq_base)
    chunks = measure_job(res, job_id, tail)

    check_done_only_at_phase2(res, marks)
    res.check("el job termina en done con phase=2", "done / 2", f"{job['status']} / {job['phase']}",
              job["status"] == "done" and job["phase"] == 2,
              {"reason": reason, "last_error": job["last_error"]})
    res.check("el material queda en ready", "ready", job["material_status"],
              job["material_status"] == "ready")
    open_chunks = [c["seq"] for c in chunks if c["status"] in ("pending", "processing")]
    res.check("ningún trozo queda pending o processing", [], open_chunks, not open_chunks)
    delivered = tail.find("fase 2 completa: draft entregado", job_id=job_id)
    res.check("el worker deja la línea «fase 2 completa: draft entregado»", "≥1 línea",
              len(delivered), bool(delivered), delivered[-1]["raw"] if delivered else None)
    passes = sorted(res.data["measured"]["reduce_passes"])
    res.check("el log trae las cuatro pasadas de la fase 2",
              ["pasada_1", "pasada_2", "pasada_3", "pasada_4"], passes, len(passes) == 4)
    if job["status"] == "failed" or reason in ("dead_lettered", "timeout"):
        res.note(f"el job no llegó a done/phase=2 ({reason}); last_error: {job['last_error']}")
    if job["status"] == "done" and job["phase"] == 2:
        _, view = call("GET", f"{LEARNING}/api/v1/materials/{material_id}/assessment-job", None,
                       teacher_token(school), ok=None)
        view = unwrap(view) if isinstance(view, dict) else {}
        res.check("assessment-job (la vista de la docente) dice done con phase=2", "done / 2",
                  f"{view.get('status')} / {view.get('phase')}",
                  view.get("status") == "done" and view.get("phase") == 2,
                  {k: view.get(k) for k in ("chunks_done", "chunks_total", "completed_at")})
        read_draft(res, teacher_token(school), job_id)
    ok, work = wait_queue(Q_MATERIAL, lambda d: d["total"] == 0, 60)
    res.check("la cola del material queda vacía", 0, work["total"], ok, work)
    dead = queue_depth(Q_MATERIAL + ".dlq")["total"]
    res.check("la cola muerta del material no crece", dlq_base, dead, dead == dlq_base)
    return job_id, chunks


# --- M1 ---------------------------------------------------------------------

def case_m1(res, args, cfg):
    school = cfg["school"]
    token = teacher_token(school)
    if args.material_id:
        material_id = uid(args.material_id)
        rows = sql("SELECT status, subject_id, school_id, title FROM content.materials "
                   f"WHERE id = '{material_id}' AND deleted_at IS NULL")
        if not rows:
            raise ApiError(f"el material {material_id} no existe en la base local")
        res.ids["material_id"] = material_id
        res.measure("material", rows[0])
        if not rows[0]["subject_id"]:
            res.note("el material no tiene materia: la entrega dará 422 y el job caerá a failed.")
    else:
        # Without --material-id the case uploads its own PDF, like M5: it does not depend
        # on a material that only exists in one database.
        pdf = resolve_pdf(cfg["pdf"])
        step(f"subiendo {os.path.basename(pdf)}")
        material_id = upload_material(res, token, school, pdf, "M1")
    generation_case(res, args, cfg, school, material_id, token)
    res.note("la corrida de julio (404 → 386 → 378 → 20) es referencia, no comparación: no consta "
             "con qué material se hizo.")


# --- M5 ---------------------------------------------------------------------

def case_m5(res, args, cfg):
    school = cfg["school"]
    pdf = resolve_pdf(args.pdf or cfg["pdf"])
    token = teacher_token(school)
    step(f"subiendo {os.path.basename(pdf)}")
    material_id = upload_material(res, token, school, pdf, "M5")
    job_id, chunks = generation_case(res, args, cfg, school, material_id, token)
    failed = [c["seq"] for c in chunks if c["status"] == "failed"]
    job = job_row(job_id)
    res.check("ningún trozo tumba el job (el job no cae a failed)", "status ≠ failed",
              job["status"], job["status"] != "failed", {"failed_chunks": failed})
    res.measure("chunks_failed_seq", failed)
    if not args.pdf:
        res.note("la batería del arnés no trae un PDF largo de verdad: fotosintesis.pdf es el más "
                 "largo en texto (dimensionado para 2 trozos). Para un PDF largo real, --pdf <ruta>.")


# --- M2 ---------------------------------------------------------------------

def chunk_index(chunks):
    return {c["id"]: c for c in chunks}


def case_m2(res, args, cfg):
    school = cfg["school"]
    pdf = resolve_pdf(args.pdf or cfg["pdf"])
    token = teacher_token(school)
    step(f"subiendo {os.path.basename(pdf)}")
    material_id = upload_material(res, token, school, pdf, "M2")
    tail = LogTail()
    dlq_base = queue_depth(Q_MATERIAL + ".dlq")["total"]
    res.measure("queues_before", depths())
    step("generate-assessment")
    status, body = request_generation(token, material_id)
    res.check("generate-assessment responde 202", 202, status, status == 202, {"body": body})
    job_id = body["id"]
    res.ids["job_id"] = job_id

    # The window: phase 1 with some chunk done but not all.
    step("esperando la ventana de la fase 1 (algún trozo done y no todos)")
    window_timeout = cfg["window_timeout_min"] * 60
    t0, window, why = time.time(), None, "timeout"
    while time.time() - t0 < window_timeout:
        job, chunks = job_row(job_id), chunk_rows(job_id)
        done = [c for c in chunks if c["status"] == "done"]
        open_ = [c for c in chunks if c["status"] in ("pending", "processing")]
        if chunks and len(chunks) < 2:
            why = "un_solo_trozo"
            break
        if job["status"] in ("done", "failed") or (chunks and not open_):
            why = "ventana_pasada"
            break
        if done and open_:
            window, why = chunks, "acertada"
            break
        time.sleep(1.5)
    res.measure("window", {"result": why, "waited_s": round(time.time() - t0, 1),
                           "chunks_total": len(chunk_rows(job_id))})
    timeout = (args.timeout_min or cfg["timeout_min"]) * 60
    if window is None:
        res.note(f"la ventana de la fase 1 no se acertó ({why}): el worker NO se mató y la "
                 "reanudación NO se afirma en esta corrida. Con un PDF de un solo trozo la ventana "
                 "no existe; prueba con --pdf y un PDF de dos trozos o más.")
        follow_job(res, job_id, tail, timeout, dlq_base)
        measure_job(res, job_id, tail)
        ok, work = wait_queue(Q_MATERIAL, lambda d: d["total"] == 0, 60)
        res.check("la cola del material queda vacía", 0, work["total"], ok, work)
        return

    # Kill first and snapshot afterwards: that way the worker does not close a chunk in between.
    step("matando el worker (SIGKILL al grupo)")
    killed = kill_worker()
    killed_at = now_iso()
    tail.poll()
    before_lines = len(tail.lines)
    before = chunk_rows(job_id)
    done_before = [c for c in before if c["status"] == "done"]
    res.measure("kill", {"pids": killed, "at": killed_at,
                         "chunks_before": [{k: c[k] for k in ("id", "seq", "status", "attempts",
                                                              "updated_at")} for c in before],
                         "phase0_lines_before_kill":
                             len(tail.find("material porcionado y persistido (fase 0)", job_id=job_id))})
    res.check("el worker queda abajo tras el SIGKILL", False, worker_up(), not worker_up())
    job = job_row(job_id)
    res.check("al morir, el job está en processing (fase 1 a medias)", "processing",
              job["status"], job["status"] == "processing", {"phase": job["phase"]})
    res.note(f"trozos done al matar: {[c['seq'] for c in done_before]} de {len(before)}.")

    step("esperando a que el broker devuelva el mensaje a la cola (sin reinyectar)")
    ok, depth = wait_queue(Q_MATERIAL, lambda d: d["ready"] >= 1 and d["unacked"] == 0, 90)
    res.check("el mensaje vuelve solo a la cola: 1 listo, 0 sin confirmar, nadie publicó",
              {"ready": 1, "unacked": 0}, depth, ok and depth["ready"] == 1)
    still = chunk_rows(job_id)
    res.check("con el worker muerto nadie toca los trozos", "sin cambios",
              "sin cambios" if still == before else "cambiaron", still == before)

    step("arrancando el worker (make _start-worker-local), sin reinyectar")
    was_down = not worker_up()
    ready = start_worker()
    res.check("el worker vuelve a arrancar («Worker listo»)", True, ready, ready)
    if was_down:
        tail.restarted()  # startup truncates the log: reread it whole, not trusting the size
    tail.poll()  # from here on the lines belong to the new start
    res.measure("log_truncations", tail.truncations)
    skipped = tail.wait("job ya porcionado, la fase 0 no se repite", 180, start=before_lines,
                        job_id=job_id)
    res.check("la fase 0 no se repite (línea «job ya porcionado…»)", "≥1 línea", len(skipped),
              bool(skipped), skipped[0]["raw"] if skipped else None)

    step(f"siguiendo el job hasta el final (tope {timeout // 60} min)")
    job, reason, marks = follow_job(res, job_id, tail, timeout, dlq_base)
    check_done_only_at_phase2(res, marks)
    after = chunk_rows(job_id)
    measure_job(res, job_id, tail)
    again = tail.find("material porcionado y persistido (fase 0)", start=before_lines, job_id=job_id)
    res.check("tras el arranque no hay otro porcionado («material porcionado y persistido»)", 0,
              len(again), not again)
    res.check("los trozos son las mismas filas (mismos id y seq): nadie reporcionó",
              sorted((c["seq"], c["id"]) for c in before),
              sorted((c["seq"], c["id"]) for c in after),
              sorted((c["seq"], c["id"]) for c in before) == sorted((c["seq"], c["id"]) for c in after))
    idx = chunk_index(after)
    redone = [c["seq"] for c in done_before
              if idx.get(c["id"], {}).get("updated_at") != c["updated_at"]
              or idx.get(c["id"], {}).get("attempts") != c["attempts"]
              or idx.get(c["id"], {}).get("status") != "done"]
    res.check("los trozos done no se rehacen (mismo updated_at, attempts y estado)", [], redone,
              not redone, {"done_before": [{k: c[k] for k in ("seq", "updated_at", "attempts")}
                                           for c in done_before]})
    done_seqs = {str(c["seq"]) for c in done_before}
    reprocessed = [l["fields"].get("seq") for l in
                   tail.find("chunk procesado (fase 1)", start=before_lines, job_id=job_id)
                   if l["fields"].get("seq") in done_seqs]
    res.check("el log del arranque nuevo no procesa ningún trozo que ya estaba done", [],
              reprocessed, not reprocessed)
    open_after = [c["seq"] for c in after if c["status"] in ("pending", "processing")]
    res.check("ningún trozo queda pending o processing (el que estaba en vuelo se retoma)", [],
              open_after, not open_after)
    res.measure("in_flight_at_kill", [{"seq": c["seq"], "status": c["status"]} for c in before
                                      if c["status"] != "done"])
    res.note("en M2 los tiempos por fase del log solo cubren el arranque nuevo: el log anterior "
             "se truncó al arrancar.")
    res.check("el job termina en done con phase=2", "done / 2", f"{job['status']} / {job['phase']}",
              job["status"] == "done" and job["phase"] == 2,
              {"reason": reason, "last_error": job["last_error"]})
    if job["assessment_id"]:
        res.ids["assessment_id"] = job["assessment_id"]
    ok, work = wait_queue(Q_MATERIAL, lambda d: d["total"] == 0, 60)
    res.check("la cola del material queda vacía", 0, work["total"], ok, work)
    dead = queue_depth(Q_MATERIAL + ".dlq")["total"]
    res.check("la cola muerta del material no crece (volvió por reentrega, no por reintentos)",
              dlq_base, dead, dead == dlq_base)


# --- M3 ---------------------------------------------------------------------

def case_m3(res, args, cfg):
    school = cfg["school"]
    settings = school_settings(school["school_id"])
    res.check("la escuela no tiene filas de política (resuelve el pipeline apagado)", {},
              settings, not settings)
    res.note("no se comprueba si el proceso vivo de academic define un LLM_*_DEFAULT: si lo hace, "
             "«sin filas» podría no ser «apagado» y el caso fallaría en la línea de log.")
    pdf = resolve_pdf(args.pdf or cfg["pdf"])
    token = teacher_token(school)
    step(f"subiendo {os.path.basename(pdf)} en la Academia Global English")
    material_id = upload_material(res, token, school, pdf, "M3")
    status_before = sql(f"SELECT status FROM content.materials WHERE id = '{uid(material_id)}'")[0]["status"]
    res.measure("material_status_before", status_before)
    tail = LogTail()
    dlq_base = queue_depth(Q_MATERIAL + ".dlq")["total"]

    step("generate-assessment (primera vez)")
    status, body = request_generation(token, material_id)
    res.check("generate-assessment responde 202", 202, status, status == 202, {"body": body})
    job_id = (body or {}).get("id")
    if not job_id:
        raise ApiError(f"generate-assessment → {status}: {body}")
    res.ids["job_id"] = job_id
    job = job_row(job_id)
    # The worker closes it in seconds: by the time of this read it may already be failed.
    res.check("learning crea el job (pending, o ya failed si el worker llegó antes)",
              "pending o failed", job and job["status"],
              bool(job) and job["status"] in ("pending", "failed"),
              {"school_id": job and job["school_id"], "phase": job and job["phase"]})

    step("esperando a que el worker cierre el job como failed")
    timeout = (args.timeout_min or cfg["timeout_min"]) * 60
    t0, seen = time.time(), []
    while time.time() - t0 < timeout:
        job = job_row(job_id)
        if job["status"] not in seen:
            seen.append(job["status"])
        if job["status"] != "pending":
            break
        tail.poll()
        time.sleep(2)
    job = job_row(job_id)
    res.measure("job_after", {"status": job["status"], "phase": job["phase"],
                              "last_error": job["last_error"], "statuses_seen": seen,
                              "waited_s": round(time.time() - t0, 1)})
    res.check("el job acaba en failed, fase 0, sin pasar por processing", "failed / phase=0",
              f"{job['status']} / phase={job['phase']}",
              job["status"] == "failed" and job["phase"] == 0 and "processing" not in seen,
              {"statuses_seen": seen})
    res.check("last_error empieza por el motivo del apagado", PIPELINE_OFF_REASON,
              job["last_error"], (job["last_error"] or "").startswith(PIPELINE_OFF_REASON))
    res.check("el material conserva el estado que tenía tras subirlo", status_before,
              job["material_status"], job["material_status"] == status_before)
    res.check("el job no tiene trozos", 0, len(chunk_rows(job_id)), not chunk_rows(job_id))
    ok, work = wait_queue(Q_MATERIAL, lambda d: d["total"] == 0, 30)
    res.check("el mensaje quedó confirmado: la cola del material está vacía", 0, work["total"], ok,
              work)
    dead = queue_depth(Q_MATERIAL + ".dlq")["total"]
    res.check("la cola muerta del material no crece", dlq_base, dead, dead == dlq_base)
    worked = tail.find("material porcionado y persistido (fase 0)", job_id=job_id)
    res.check("el worker no porciona el material", 0, len(worked), not worked)

    step("generate-assessment (segunda vez)")
    status2, body2 = request_generation(teacher_token(school), material_id)
    res.check("el segundo generate-assessment responde 202: un job failed no bloquea", 202, status2,
              status2 == 202, {"body": body2})
    job2_id = body2.get("id") if isinstance(body2, dict) else None
    res.check("el segundo disparo crea otro job", "un id distinto", job2_id,
              bool(job2_id) and job2_id != job_id)
    if job2_id:
        res.ids["second_job_id"] = job2_id
        t0 = time.time()
        while time.time() - t0 < timeout and job_row(job2_id)["status"] == "pending":
            time.sleep(2)
    jobs = sql("SELECT id, status, phase, last_error FROM content.material_pipeline_job "
               f"WHERE material_id = '{uid(material_id)}' ORDER BY created_at")
    res.check("hay dos jobs del material y los dos están en failed", ["failed", "failed"],
              [j["status"] for j in jobs],
              len(jobs) == 2 and all(j["status"] == "failed" for j in jobs), {"jobs": jobs})
    mat = sql(f"SELECT status FROM content.materials WHERE id = '{uid(material_id)}'")[0]
    res.check("el material sigue en el estado que tenía tras subirlo", status_before, mat["status"],
              mat["status"] == status_before)
    dead = queue_depth(Q_MATERIAL + ".dlq")["total"]
    res.check("la cola muerta del material sigue sin crecer", dlq_base, dead, dead == dlq_base)
    res.measure("material_status_after", mat["status"])


# --- M4 ---------------------------------------------------------------------

def load_m1_assessment(args, out_dir):
    if args.assessment_id:
        return uid(args.assessment_id), "--assessment-id"
    path = os.path.join(out_dir, "M1.json")
    if not os.path.isfile(path):
        raise ApiError(f"M4 encadena con M1: no existe {path} (M1 tiene que ir en la misma corrida, "
                       "--cases M1,M4) y no se dio --assessment-id")
    with open(path, encoding="utf-8") as fh:
        m1 = json.load(fh)
    aid = (m1.get("ids") or {}).get("assessment_id")
    if not aid:
        raise ApiError(f"{path} no trae ids.assessment_id: M1 no entregó un borrador")
    return uid(aid), path


def pick_answer(question, teacher_view, counters, overrides, index):
    """The student's answer to a question the model wrote, by type."""
    if str(index) in overrides:
        return overrides[str(index)], "fijada por --answers-file"
    qtype = question.get("question_type")
    correct = (teacher_view.get(question.get("id")) or {}).get("correct_answer")
    options = [o.get("option_text") for o in question.get("options") or []]
    if qtype == "multiple_choice":
        return (correct or (options[0] if options else "")), "la correcta (vista de la docente)"
    if qtype == "true_false":
        return (correct or "true"), "la correcta (vista de la docente)"
    if qtype == "multiple_select":
        # The answer is a STRING holding a JSON array of texts, like correct_answer.
        return (correct or json.dumps(options[:1], ensure_ascii=False)), "la correcta (vista de la docente)"
    if qtype == "short_answer":
        item = SHORT_ANSWERS[counters["short"] % len(SHORT_ANSWERS)]
        counters["short"] += 1
        return item["answer"], item["intent"]
    item = OPEN_ANSWERS[counters["open"] % len(OPEN_ANSWERS)]
    counters["open"] += 1
    return item["answer"], item["intent"]


def lane_for(tail, answer_id):
    reviewed = tail.find("answer revisada por LLM", answer_id=answer_id)
    listed = tail.find("short_answer con prep list: carril triturado", answer_id=answer_id)
    criteria = tail.find("open_ended con prep criteria: carril por criterios", answer_id=answer_id)
    if reviewed and reviewed[-1]["fields"].get("en_blanco_por_regla") == "true":
        return "blanco_por_regla", reviewed[-1]
    if listed:
        return "lista", listed[-1]
    if criteria:
        return "criterios", criteria[-1]
    if reviewed:
        return "global", reviewed[-1]  # no line of its own: inferred from the others' absence
    return "sin_worker", None


def case_m4(res, args, cfg):
    school = cfg["school"]
    assessment_id, source = load_m1_assessment(args, args.out_dir)
    res.ids["assessment_id"] = assessment_id
    res.note(f"borrador tomado de {source}.")
    teacher = teacher_token(school)
    settings = school_settings(school["school_id"])
    res.check("la escuela corrige con flow=teacher y la revisión encendida",
              {"llm.review.flow": "teacher", "llm.review.mode": "local"},
              {k: settings.get(k) for k in ("llm.review.flow", "llm.review.mode")},
              settings.get("llm.review.flow") == "teacher"
              and settings.get("llm.review.mode") in ("local", "api"))

    # 1 · The draft and the prep chained by the pipeline's delivery.
    _, payload = call("GET", f"{LEARNING}/api/v1/assessments/{assessment_id}", None, teacher)
    assessment = unwrap(payload)
    questions = question_rows(assessment_id)
    reviewable = [q for q in questions if q["question_type"] in ("short_answer", "open_ended")]
    res.measure("draft", {"status_at_start": assessment.get("status"),
                          "question_types": count_by(questions, "question_type"),
                          "reviewable": len(reviewable)})
    not_enqueued = [q["id"] for q in reviewable
                    if q["llm_prep_status"] in (None, "none") or not q["has_source_hash"]]
    res.check("cada corta y abierta del borrador salió de `none` y tiene source_hash "
              "(la entrega publicó question.prep_requested por cada una)", [], not_enqueued,
              not not_enqueued, {"reviewable": len(reviewable)})
    whole_log = LogTail()
    whole_log.offset, whole_log.start_offset = 0, 0
    prepared_in_log = [q["id"] for q in reviewable
                       if whole_log.find("pregunta preparada por LLM", question_id=q["id"])]
    res.measure("prep", {"status_at_case_start": count_by(reviewable, "llm_prep_status"),
                         "prepared_lines_in_current_log": len(prepared_in_log),
                         "note": "el log se trunca en cada arranque del worker: la ausencia de "
                                 "líneas no dice que no se prepararan"})
    if not reviewable:
        res.note("el borrador no trae cortas ni abiertas (lo decide el modelo): no hay nada que el "
                 "worker revise y lo de R6 NO es afirmable en esta corrida.")

    # 2 · Publish and assign.
    if assessment.get("status") == "draft":
        step("publicando el borrador")
        status, published = call("POST", f"{LEARNING}/api/v1/assessments/{assessment_id}/publish",
                                 None, teacher, ok=None)
        res.check("publicar el borrador responde 200", 200, status, status == 200,
                  None if status == 200 else published)
    else:
        res.note(f"el borrador ya estaba en {assessment.get('status')}: no se publica de nuevo.")
    row = sql(f"SELECT status FROM assessment.assessment WHERE id = '{assessment_id}'")[0]
    res.check("la evaluación queda published", "published", row["status"],
              row["status"] == "published")
    status, assigned = call("POST", f"{LEARNING}/api/v1/assessments/{assessment_id}/assignments",
                            {"subject_offering_id": school["offering_id"]}, teacher, ok=None)
    res.check("asignar a Ciencias 5to B responde 2xx (o 409 si ya estaba)", "200/201/204/409",
              status, status in (200, 201, 204, 409), None if status < 300 else assigned)

    # 3 · Wait (capped) for the prep to finish, and note whether it made it in time.
    wait_s = cfg["prep_wait_seconds"]
    t0 = time.time()
    while reviewable and time.time() - t0 < wait_s:
        current = [q for q in question_rows(assessment_id)
                   if q["question_type"] in ("short_answer", "open_ended")]
        if all(q["llm_prep_status"] == "ready" for q in current):
            break
        time.sleep(5)
    current = [q for q in question_rows(assessment_id)
               if q["question_type"] in ("short_answer", "open_ended")]
    res.data["measured"]["prep"].update({
        "status_before_student_submit": count_by(current, "llm_prep_status"),
        "waited_s": round(time.time() - t0, 1),
        "all_ready_before_submit": all(q["llm_prep_status"] == "ready" for q in current)})

    # 4 · The student takes it.
    overrides = {}
    if args.answers_file:
        with open(args.answers_file, encoding="utf-8") as fh:
            overrides = {str(k): v for k, v in json.load(fh).items()}
    teacher = teacher_token(school)  # the prep wait may have expired the previous one
    teacher_view = {q.get("id"): q for q in as_list(
        call("GET", f"{LEARNING}/api/v1/assessments/{assessment_id}/questions", None, teacher)[1],
        "questions")}
    student = token_for(school["student"], school.get("context"))
    step("el alumno abre el intento")
    status, started = call("POST", f"{LEARNING}/api/v1/assessments/{assessment_id}/start", None,
                           student, ok=None)
    res.check("el alumno abre el intento (200/201)", "200/201", status, status in (200, 201),
              None if status in (200, 201) else started)
    if status not in (200, 201):
        raise ApiError(f"start → {status}: {started}")
    started = unwrap(started)
    attempt_id = started["attempt"]["id"]
    res.ids["attempt_id"] = attempt_id
    counters, sent = {"short": 0, "open": 0}, []
    for index, question in enumerate(started.get("questions") or []):
        answer, intent = pick_answer(question, teacher_view, counters, overrides, index)
        call("PUT", f"{LEARNING}/api/v1/attempts/{attempt_id}/answers/{index}",
             {"answer": answer, "time_spent_seconds": 5}, student)
        sent.append({"question_index": index, "question_id": question.get("id"),
                     "question_type": question.get("question_type"), "answer": answer,
                     "intent": intent})
    res.measure("answers_sent", sent)

    tail = LogTail()  # only what was written since the submit
    review_dlq_base = queue_depth(Q_REVIEW + ".dlq")["total"]
    step("el alumno entrega")
    t_submit = time.time()
    status, submitted = call("POST", f"{LEARNING}/api/v1/attempts/{attempt_id}/submit", None,
                             student, ok=None)
    res.check("la entrega responde 200", 200, status, status == 200,
              None if status == 200 else submitted)
    if status != 200:
        raise ApiError(f"submit → {status}: {submitted}")
    answers = answer_rows(attempt_id)
    pending = [a for a in answers if a["review_status"] == "pending"]
    pending_ids = [a["id"] for a in pending]
    res.measure("after_submit", {"attempt_status": attempt_row(attempt_id)["status"],
                                 "review_status": count_by(answers, "review_status"),
                                 "pending_answer_ids": pending_ids})
    if not pending:
        res.note("ninguna respuesta quedó pendiente al entregar: learning cerró el intento sin "
                 "worker y lo de R6 NO es afirmable en esta corrida.")
        for a in answers:
            if a["question_type"] in ("short_answer", "open_ended"):
                res.lane(a["id"], a["question_type"], "sin_worker", None)
        return
    attempt = attempt_row(attempt_id)
    res.check("con respuestas pendientes, el intento queda en pending_review", "pending_review",
              attempt["status"], attempt["status"] == "pending_review")

    # 5 · The worker reviews.
    timeout = (args.timeout_min or cfg["timeout_min"]) * 60
    step(f"esperando la revisión del worker (tope {timeout // 60} min)")
    while time.time() - t_submit < timeout:
        answers = answer_rows(attempt_id)
        attempt = attempt_row(attempt_id)
        left = [a for a in answers if a["id"] in pending_ids and a["review_status"] == "pending"]
        if not left and not attempt["ai_review_claimed_at"]:
            break
        tail.poll()
        time.sleep(4)
    res.measure("review_wait_s", round(time.time() - t_submit, 1))
    answers = answer_rows(attempt_id)
    attempt = attempt_row(attempt_id)
    by_id = {a["id"]: a for a in answers}

    # 6 · The R6 checks, with flow=teacher.
    not_reviewed = [i for i in pending_ids if by_id[i]["review_status"] != "ai_reviewed"]
    res.check("todas las pendientes pasan a ai_reviewed", [], not_reviewed, not not_reviewed)
    not_llm = [i for i in pending_ids if by_id[i]["review_source"] != "llm"]
    res.check("cada una tiene su revisión con review_source=llm", [], not_llm, not not_llm)
    out_of_range = [i for i in pending_ids
                    if by_id[i]["points_awarded"] is None
                    or not 0 <= float(by_id[i]["points_awarded"]) <= float(by_id[i]["max_points"] or 0)]
    res.check("los puntos propuestos están entre 0 y el máximo de la pregunta", [], out_of_range,
              not out_of_range)
    res.check("el intento sigue en pending_review (el worker no finaliza)", "pending_review",
              attempt["status"], attempt["status"] == "pending_review")
    res.check("el candado queda liberado (ai_review_claimed_at nulo)", None,
              attempt["ai_review_claimed_at"], attempt["ai_review_claimed_at"] is None)
    released = tail.find("review completada, flujo teacher/short_answer: release-claim",
                         attempt_id=attempt_id)
    res.check("el log trae «review completada, flujo teacher/short_answer: release-claim»",
              "≥1 línea", len(released), bool(released), released[-1]["raw"] if released else None)
    finalized = tail.find("attempt finalizado (flow direct)", attempt_id=attempt_id)
    res.check("el worker no finaliza el intento (sin línea de flow direct)", 0, len(finalized),
              not finalized)
    no_line = [i for i in pending_ids if not tail.find("answer revisada por LLM", answer_id=i)]
    res.check("el log trae «answer revisada por LLM» por cada pendiente", [], no_line, not no_line)
    _, detail = call("GET", f"{LEARNING}/api/v1/attempts/{attempt_id}/review", None,
                     teacher_token(school))
    api_attempt = (unwrap(detail) or {}).get("attempt") or {}
    res.check("la vista de la docente no marca la IA en curso", False,
              bool(api_attempt.get("ai_review_in_progress")),
              not api_attempt.get("ai_review_in_progress"),
              {"status": api_attempt.get("status"), "review_stage": api_attempt.get("review_stage")})
    ok, work = wait_queue(Q_REVIEW, lambda d: d["total"] == 0, 60)
    res.check("la cola de revisión queda vacía", 0, work["total"], ok, work)
    dead = queue_depth(Q_REVIEW + ".dlq")["total"]
    res.check("la cola muerta de revisión no crece", review_dlq_base, dead, dead == review_dlq_base)

    # 7 · Each answer's lane and the proposals (measured).
    proposals = []
    for a in answers:
        if a["question_type"] not in ("short_answer", "open_ended"):
            continue
        lane, line = lane_for(tail, a["id"]) if a["id"] in pending_ids else ("sin_worker", None)
        res.lane(a["id"], a["question_type"], lane, line["raw"] if line else None)
        verdict = tail.find("answer revisada por LLM", answer_id=a["id"])
        proposals.append({"answer_id": a["id"], "question_index": a["question_index"],
                          "question_type": a["question_type"], "lane": lane,
                          "student_answer": a["student_answer"], "review_status": a["review_status"],
                          "points_awarded": a["points_awarded"], "max_points": a["max_points"],
                          "feedback": a["feedback"],
                          "verdict": verdict[-1]["fields"].get("verdict") if verdict else None,
                          "score": verdict[-1]["fields"].get("score") if verdict else None})
    res.measure("proposals", proposals)
    res.measure("lanes_count", count_by(res.data["lanes"], "lane"))
    enrolled = sql("SELECT e.offering_id, p.is_active FROM academic.subject_offering_enrollments e "
                   "JOIN academic.academic_periods p ON p.id = e.period_id "
                   f"WHERE e.offering_id = '{uid(school['offering_id'])}'")
    res.measure("offering_enrollments", enrolled)
    res.note("el caso no finaliza el intento: queda en pending_review con las propuestas de la IA, "
             "a la espera de la docente. Volver a correr M4 gasta otro intento del alumno.")


# --- The wrapper ------------------------------------------------------------

RUNNERS = {"M1": case_m1, "M2": case_m2, "M3": case_m3, "M4": case_m4, "M5": case_m5}


def run_case(name, args):
    cfg = CASES[name]
    res = Result(name, cfg["title"])
    print(f"\n== {name} · {cfg['title']}", flush=True)
    try:
        if not worker_up():
            res.note("el worker estaba abajo al empezar el caso: se arranca.")
            start_worker()
        RUNNERS[name](res, args, cfg)
    except Exception as err:  # the JSON is still written and the environment is restored
        res.data["error"] = f"{type(err).__name__}: {err}"
        res.measure("traceback", traceback.format_exc()[-1500:])
        print("  ERROR:", res.data["error"], flush=True)
    finally:
        try:
            if not worker_up():
                up = start_worker()
                res.note(f"el worker estaba abajo al terminar: se arrancó (listo={up}).")
            res.measure("worker_up_at_end", worker_up())
            end = depths()
            res.measure("queues_at_end", end)
            busy = {q: d for q, d in end.items() if not q.endswith(".dlq") and d.get("total")}
            if busy:
                res.note(f"quedan mensajes en colas de trabajo al terminar: {busy}")
            note_dead_letters(res)
        except Exception as err:  # the restore must not mask the result
            res.note(f"la restauración del entorno falló: {type(err).__name__}: {err}")
        res.data["finished_at"] = now_iso()
        res.data["needs_model"] = cfg["needs_model"]
        os.makedirs(args.out_dir, exist_ok=True)
        path = os.path.join(args.out_dir, f"{name}.json")
        with open(path, "w", encoding="utf-8") as fh:
            json.dump(res.data, fh, indent=2, ensure_ascii=False, default=str)
            fh.write("\n")
        failed = [a["name"] for a in res.data["asserted"] if not a["pass"]]
        print(f"== {name}: {'PASA' if res.passed() else 'NO PASA'} · "
              f"{len(res.data['asserted'])} afirmaciones, {len(failed)} en rojo · {path}", flush=True)
    return {"case": name, "needs_model": cfg["needs_model"], "error": res.data["error"],
            "seeded": bool(res.data["ids"]), "asserted": len(res.data["asserted"]),
            "failed": failed}


def main():
    parser = argparse.ArgumentParser(description="Casos M1 a M5 del riel material → evaluación.")
    parser.add_argument("--cases", help="lista separada por comas, p. ej. M1,M2")
    parser.add_argument("--list", action="store_true", help="lista los casos y sale")
    parser.add_argument("--material-id", help="M1: otro material ya subido (por defecto, sube ciclo_del_agua.pdf)")
    parser.add_argument("--pdf", help="M2, M3 y M5: otro PDF (nombre de la batería del arnés o ruta)")
    parser.add_argument("--assessment-id", help="M4: el borrador a rendir (por defecto, el del M1.json de esta misma corrida)")
    parser.add_argument("--answers-file", help="M4: JSON {\"<índice>\": \"respuesta\"} que pisa las respuestas")
    parser.add_argument("--timeout-min", type=int, help="tope de la espera larga del caso, en minutos")
    parser.add_argument("--out-dir",
                        help="obligatorio para correr: dentro se crea una carpeta por corrida")
    parser.add_argument("--no-model", action="store_true",
                        help="corre solo los casos que no necesitan modelo")
    args = parser.parse_args()
    if args.list:
        for name, cfg in CASES.items():
            mark = "modelo" if cfg["needs_model"] else "sin modelo"
            print(f"{name}  {mark:10}  tope {cfg['timeout_min']:>2} min  {cfg['title']}")
        return e2e_env.EXIT_OK
    if bool(args.cases) == args.no_model:
        print("hay que dar --cases o --no-model, uno de los dos (o --list)", file=sys.stderr)
        return e2e_env.EXIT_USAGE
    if args.no_model:
        names = [n for n, cfg in CASES.items() if not cfg["needs_model"]]
    else:
        names = [n.strip().upper() for n in args.cases.split(",") if n.strip()]
    unknown = [n for n in names if n not in CASES]
    if unknown:
        print(f"casos desconocidos: {unknown}; los que hay: {list(CASES)}", file=sys.stderr)
        return e2e_env.EXIT_USAGE
    try:
        e2e_env.check_out_dir(args.out_dir)
        e2e_env.enforce_guard()
        e2e_env.require_external_paths()
        # One folder per run: M4 reads the M1.json of its own run, or takes --assessment-id.
        args.out_dir = e2e_env.new_run_dir(args.out_dir)
    except e2e_env.UsageError as err:
        print(f"uso: {err}", file=sys.stderr)
        return e2e_env.EXIT_USAGE
    except e2e_env.GuardError as err:
        print(f"guarda: {err}", file=sys.stderr)
        return e2e_env.EXIT_GUARD
    except e2e_env.MissingPathError as err:
        print(f"entorno: {err}", file=sys.stderr)
        return e2e_env.EXIT_FAILED
    print(f"carpeta de la corrida: {args.out_dir}", flush=True)
    summaries = [run_case(name, args) for name in names]  # in series: a single environment
    print(e2e_env.unmet_line(summaries), flush=True)
    return e2e_env.exit_code(summaries)


if __name__ == "__main__":
    sys.exit(main())
