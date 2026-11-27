"""Building blocks shared by the cases: seed an exam through the API, fix or wait for the
preparation, take the attempt, read the queue, wait for the worker and read the result."""
import datetime
import json
import time

import common as c
from cases_data import OPEN, SHORT, SKIP

REVIEWABLE = (SHORT, OPEN)


def context(actors):
    return {"school_id": actors["school"], "academic_unit_id": actors["unit"]}


def teacher(actors):
    return c.token_for(actors["teacher"], context(actors))


def student(actors):
    return c.token_for(actors["student"], context(actors))


def parse_ts(value):
    try:
        return datetime.datetime.fromisoformat(str(value).replace("Z", "+00:00"))
    except ValueError:
        return None


# ---------------------------------------------------------------- event and artifact contracts

def _envelope_problems(event, event_type):
    problems = []
    if not isinstance(event, dict):
        return ["body is not a JSON object"]
    for key in ("event_id", "event_type", "event_version", "timestamp", "payload"):
        if key not in event:
            problems.append("missing " + key)
    extra = set(event) - {"event_id", "event_type", "event_version", "timestamp", "payload"}
    if extra:
        problems.append("unexpected keys: " + ",".join(sorted(extra)))
    if event.get("event_type") != event_type:
        problems.append(f"event_type={event.get('event_type')}")
    if event.get("event_version") != "1.0":
        problems.append(f"event_version={event.get('event_version')}")
    return problems


def review_event_problems(event):
    """Checks against schemas/events/attempt-review_requested-v1.schema.json."""
    problems = _envelope_problems(event, "attempt.review_requested")
    payload = event.get("payload") if isinstance(event, dict) else None
    if not isinstance(payload, dict):
        return problems + ["payload is not an object"]
    for key in ("attempt_id", "assessment_id", "school_id"):
        if not c.UUID_RE.match(str(payload.get(key, ""))):
            problems.append(key + " is not a uuid")
    answers = payload.get("answers")
    if not isinstance(answers, list) or not answers:
        problems.append("answers is empty")
    for ref in answers or []:
        if not c.UUID_RE.match(str(ref.get("answer_id", ""))) or \
                ref.get("question_type") not in REVIEWABLE:
            problems.append(f"bad answer ref: {ref}")
    return problems


def prep_event_problems(event):
    """Checks against schemas/events/question-prep_requested-v1.schema.json."""
    problems = _envelope_problems(event, "question.prep_requested")
    payload = event.get("payload") if isinstance(event, dict) else None
    if not isinstance(payload, dict):
        return problems + ["payload is not an object"]
    for key in ("question_id", "assessment_id"):
        if not c.UUID_RE.match(str(payload.get(key, ""))):
            problems.append(key + " is not a uuid")
    if payload.get("reason") not in ("created", "updated", "feedback", "backfill"):
        problems.append(f"reason={payload.get('reason')}")
    return problems


def _strings(value, minimum):
    return isinstance(value, list) and len(value) >= minimum and \
        all(isinstance(v, str) and v.strip() for v in value)


def prep_contract_problems(artifact, question_type):
    """The v1 contract of `llm_prep` (contract of injection, section 2)."""
    if not isinstance(artifact, dict):
        return ["artifact is not an object"]
    problems = []
    if artifact.get("version") != 1:
        problems.append(f"version={artifact.get('version')}")
    if artifact.get("question_type") != question_type:
        problems.append(f"question_type={artifact.get('question_type')}")
    if question_type == SHORT:
        kind = artifact.get("content_kind")
        if kind not in ("list", "number", "date", "term", "free"):
            problems.append(f"content_kind={kind}")
        items, verbatim = artifact.get("items"), artifact.get("items_verbatim")
        if not _strings(items, 1) or (kind != "list" and len(items) != 1):
            problems.append("items does not fit content_kind")
        if not _strings(verbatim, 1) or len(verbatim) != len(items or []):
            problems.append("items_verbatim does not match items")
    else:
        if not isinstance(artifact.get("question_intent"), str) or \
                not artifact["question_intent"].strip():
            problems.append("question_intent is empty")
        if not _strings(artifact.get("main_ideas"), 1):
            problems.append("main_ideas is empty")
        if artifact.get("criteria") and not _strings(artifact["criteria"], 1):
            problems.append("criteria has an empty element")
    return problems


def message_json(message):
    try:
        return json.loads(message["payload"])
    except (TypeError, ValueError):
        return None


# ---------------------------------------------------------------- SQL readers

def question_row(question_id):
    rows = c.sql_rows(
        "SELECT id, question_type, llm_prep_status, llm_prep_source_hash, llm_prep_updated_at, "
        "llm_prep FROM assessment.question WHERE id='" + c.uid(question_id) + "'")
    return rows[0] if rows else None


def attempt_row(attempt_id):
    rows = c.sql_rows(
        "SELECT id, status, score, max_score, percentage, completed_at, ai_review_claimed_at "
        "FROM assessment.assessment_attempt WHERE id='" + c.uid(attempt_id) + "'")
    return rows[0] if rows else None


def answer_rows(attempt_id):
    """One row per answer, with its review (if any), ordered by question_index."""
    return c.sql_rows(
        "SELECT a.id AS answer_id, a.question_index, a.question_id, q.question_type, "
        "a.student_answer, a.review_status, a.is_correct, a.points_earned, a.max_points, "
        "r.points_awarded, r.feedback, r.review_source, r.updated_at AS review_updated_at "
        "FROM assessment.assessment_attempt_answer a "
        "LEFT JOIN assessment.question q ON q.id = a.question_id "
        "LEFT JOIN assessment.attempt_review r ON r.attempt_answer_id = a.id "
        "WHERE a.attempt_id='" + c.uid(attempt_id) + "' ORDER BY a.question_index")


def grade_items(attempt_id):
    return c.sql_rows(
        "SELECT id, source, value, subject_id, period_id, membership_id FROM academic.grade_item "
        "WHERE source_attempt_id='" + c.uid(attempt_id) + "'")


def active_enrollment(membership_id, subject_id):
    """Same predicate as learning's ResolveActivePeriod: decides if grade_item is written."""
    rows = c.sql_rows(
        "SELECT soe.period_id FROM academic.subject_offering_enrollments soe "
        "JOIN academic.academic_periods p ON p.id = soe.period_id "
        "WHERE soe.student_membership_id='" + c.uid(membership_id) + "' AND soe.subject_id='"
        + c.uid(subject_id) + "' AND p.is_active = true LIMIT 1")
    return rows[0]["period_id"] if rows else None


def by_key(exam, rows):
    """{question key: answer row}."""
    keys = {q["id"]: q["key"] for q in exam["questions"]}
    return {keys.get(r["question_id"], r["question_id"]): r for r in rows}


# ---------------------------------------------------------------- seeding through the API

def create_draft(case, actors, title, questions):
    """Creates a draft assessment and its questions. Every short/open one enqueues a prep."""
    token = teacher(actors)
    stamp = c.now_iso()
    _, created = c.call("POST", c.LEARNING + "/api/v1/assessments", {
        "subject_id": actors["subject"], "title": f"E2E {case.name} · {title} ({stamp})",
        "description": "Sembrada por el guion de inyección (inferencia-e2e/revision).",
        "purpose": "exam", "shuffle_questions": False, "passing_score": 60,
        "pass_threshold": 60, "max_attempts": 3}, token)
    assessment_id = c.unwrap(created)["id"]
    exam = {"assessment_id": assessment_id, "questions": []}
    for spec in questions:
        exam["questions"].append(add_question(actors, assessment_id, spec))
    case.ids["assessment_id"] = assessment_id
    case.ids["questions"] = {q["key"]: q["id"] for q in exam["questions"]}
    return exam


def add_question(actors, assessment_id, spec):
    _, created = c.call("POST", f"{c.LEARNING}/api/v1/assessments/{assessment_id}/questions",
                        spec["body"], teacher(actors))
    data = c.unwrap(created) or {}
    return {"key": spec["key"], "id": data["id"], "type": spec["body"]["question_type"],
            "points": spec["body"].get("points", 1), "answer": spec.get("answer"),
            "prep": spec.get("prep"), "status_on_create": data.get("llm_prep_status")}


def pin_prep(question_id, artifact):
    """Fixes the artifact through the internal PUT, with the row's current source_hash."""
    _, source = c.internal("GET", f"/questions/{question_id}/prep-source")
    source_hash = (c.unwrap(source) or {}).get("source_hash")
    _, saved = c.internal("PUT", f"/questions/{question_id}/llm-prep",
                          {"llm_prep": artifact, "source_hash": source_hash,
                           "consumed_feedback": False})
    return (c.unwrap(saved) or {}).get("status")


def wait_ready(case, exam, timeout_each=None):
    """Waits for the worker to prepare every short/open question. Returns the seconds it took."""
    pending = [q for q in exam["questions"] if q["type"] in REVIEWABLE]
    t0 = time.time()

    def all_ready():
        return all(question_row(q["id"])["llm_prep_status"] == "ready" for q in pending)
    done = c.poll_soft(all_ready, (timeout_each or c.PER_ANSWER_TIMEOUT) * max(1, len(pending)), 3)
    if not done:
        case.note("no todas las preguntas llegaron a llm_prep_status=ready dentro del tope")
    return round(time.time() - t0, 1), bool(done)


def prepare_exam(case, actors, cfg):
    """Seeds, resolves the preparation race and publishes + assigns the exam.

    offline: worker stopped, prep queue purged, artifacts fixed by the internal PUT; a
             question without a fixed artifact stays `pending` (global judgement).
             The worker is left STOPPED: the caller starts it after the submit.
    model:   worker up, the model prepares every question; artifacts are fixed afterwards.
    """
    mode = cfg["prep_mode"]
    if mode == "offline":
        c.stop_worker(case.tail)
        foreign = c.peek(c.PREP_QUEUE)
        if foreign:
            case.note(f"la cola de preparación tenía {len(foreign)} mensaje(s) antes de sembrar; la "
                      "purga de este caso también los quita")
    else:
        c.start_worker(case.tail)
    exam = create_draft(case, actors, cfg["title"], cfg["questions"])
    if mode == "offline":
        case.measure("prep_messages_purged", len(c.peek(c.PREP_QUEUE)))
        c.purge(c.PREP_QUEUE)
    else:
        seconds, ready = wait_ready(case, exam)
        case.measure("prep_seconds", seconds)
        case.measure("prep_all_ready", ready)
        case.measure("prep_artifacts", {q["key"]: question_row(q["id"])["llm_prep"]
                                        for q in exam["questions"]})
    for q in exam["questions"]:
        if q["prep"]:
            status = pin_prep(q["id"], q["prep"])
            case.check(f"artefacto fijado por el PUT interno ({q['key']})", "ready", status)
    token = teacher(actors)
    c.call("POST", f"{c.LEARNING}/api/v1/assessments/{exam['assessment_id']}/publish", None, token)
    c.call("POST", f"{c.LEARNING}/api/v1/assessments/{exam['assessment_id']}/assignments",
           {"subject_offering_id": actors["offering"]}, token, ok=(200, 201, 204, 409))
    case.measure("prep_status_at_publish", {q["key"]: question_row(q["id"])["llm_prep_status"]
                                            for q in exam["questions"]})
    return exam


def start_attempt(case, actors, exam):
    """Starts the attempt and saves the answers; does NOT submit."""
    token = student(actors)
    _, started = c.call("POST", f"{c.LEARNING}/api/v1/assessments/{exam['assessment_id']}/start",
                        None, token)
    data = c.unwrap(started)
    attempt_id = data["attempt"]["id"]
    order = {q["id"]: i for i, q in enumerate(data.get("questions") or [])}
    for position, q in enumerate(exam["questions"]):
        if q["answer"] is SKIP:
            continue
        index = order.get(q["id"], position)
        c.call("PUT", f"{c.LEARNING}/api/v1/attempts/{attempt_id}/answers/{index}",
               {"answer": q["answer"], "time_spent_seconds": 5}, token)
    case.ids["attempt_id"] = attempt_id
    return attempt_id


def submit_attempt(case, actors, attempt_id):
    _, submitted = c.call("POST", f"{c.LEARNING}/api/v1/attempts/{attempt_id}/submit", None,
                          student(actors))
    result = c.unwrap(submitted) or {}
    summary = result.get("attempt") or result
    case.measure("submit", {k: summary.get(k) for k in ("status", "score", "max_score",
                                                        "percentage", "passed")})
    return summary


def take_attempt(case, actors, exam):
    attempt_id = start_attempt(case, actors, exam)
    submit_attempt(case, actors, attempt_id)
    return attempt_id


def captured_review_message(case, attempt_id, expected_answer_ids=None):
    """With the worker stopped: the message learning published, read without consuming it."""
    found = c.find_messages(c.REVIEW_QUEUE, attempt_id)
    case.check("learning publicó attempt.review_requested del intento", 1, len(found))
    if not found:
        return None
    message = found[0]
    event = message_json(message)
    case.check("el mensaje cumple su esquema", [], review_event_problems(event),
               evidence=message["payload"])
    if expected_answer_ids is not None and event:
        actual = sorted(r["answer_id"] for r in event["payload"]["answers"])
        case.check("el evento lleva solo las respuestas pendientes",
                   sorted(expected_answer_ids), actual)
    case.ids["review_event_id"] = (event or {}).get("event_id")
    return message


def wait_reviewed(case, attempt_id, answers, extra=60):
    """Polls until the worker left no answer `pending` and released (or finalized)."""
    t0 = time.time()

    def settled():
        attempt = attempt_row(attempt_id)
        if attempt["status"] == "completed":
            return True
        pending = [r for r in answer_rows(attempt_id)
                   if r["question_type"] in REVIEWABLE and r["review_status"] == "pending"]
        return not pending and attempt["ai_review_claimed_at"] is None
    done = c.poll_soft(settled, c.PER_ANSWER_TIMEOUT * max(1, answers) + extra, 3)
    case.measure("review_seconds", round(time.time() - t0, 1))
    if not done:
        case.note("el worker no dejó resuelto el intento dentro del tope")
    return bool(done)


def record_lanes(case, rows):
    """Writes the lane of every short/open answer, read from the worker log."""
    for row in rows:
        if row["question_type"] in REVIEWABLE:
            case.lanes.append(c.lane_of(case.tail, row["answer_id"], row["question_type"]))
    return {entry["answer_id"]: entry for entry in case.lanes}


def measure_proposals(case, exam, rows):
    """What the model wrote: measured, never asserted."""
    case.measure("proposals", {key: {"student_answer": r["student_answer"],
                                     "points_awarded": r["points_awarded"],
                                     "max_points": r["max_points"], "feedback": r["feedback"],
                                     "review_status": r["review_status"]}
                               for key, r in by_key(exam, rows).items()})


def assert_ai_reviewed(case, key, row):
    case.check(f"{key}: review_status", "ai_reviewed", row["review_status"])
    case.check(f"{key}: review_source", "llm", row["review_source"])


def assert_released(case, attempt_id):
    attempt = attempt_row(attempt_id)
    case.check("candado liberado (ai_review_claimed_at nulo)", None,
               attempt["ai_review_claimed_at"])
    lines = case.tail.wait_for(("candado liberado", "attempt_id=" + attempt_id), 15)
    case.check("el log dice «candado liberado» para el intento", True, bool(lines),
               evidence=lines[-1] if lines else None)
    return attempt


def assert_grade_item(case, actors, attempt_id):
    """grade_item is written only with an active enrollment: checked here, by SQL."""
    period = active_enrollment(actors["student_membership"], actors["subject"])
    case.note(f"inscripción con periodo activo del alumno en la materia: {period or 'no hay'}")
    items = grade_items(attempt_id)
    attempt = attempt_row(attempt_id)
    if period:
        case.check("academic.grade_item: una fila auto_scored del intento", ["auto_scored"],
                   [i["source"] for i in items], evidence=items)
        if items and attempt["percentage"] is not None:
            case.check("grade_item.value es el porcentaje del intento, a 2 decimales",
                       round(float(attempt["percentage"]), 2), round(float(items[0]["value"]), 2))
    else:
        case.check("sin inscripción activa no hay fila en academic.grade_item", 0, len(items))
