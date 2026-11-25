#!/usr/bin/env python3
"""Injection script for the preparation (P1-P3) and review (R1-R13) cases.

    python3 scripts/e2e/revision/run.py --list
    EDUGO_BASE_PASSWORD=... python3 scripts/e2e/revision/run.py --cases P1,R1 --out-dir DIR
    EDUGO_BASE_PASSWORD=... python3 scripts/e2e/revision/run.py --no-model --out-dir DIR

One function per case; each one seeds its own exam through the API, restores the
environment and writes <DIR>/<run>/<CASE>.json. It refuses to run outside the local
environment (exit code 3). See ../README.md.
"""
import argparse
import json
import sys
import time
import uuid

import common as c
import flow as f
from cases_data import ACTORS, CASES, OPEN, ORDER, SAN_IGNACIO
from common import e2e_env

PREPARED_MSG = "pregunta preparada por LLM"
PREP_OFF_MSG = "preparación apagada para la escuela"
REVIEW_OFF_MSG = "review apagado para la escuela"
NOT_CLAIMABLE_MSG = "intento no reclamable, se abstiene (ACK)"
NO_PENDING_MSG = "review sin pendientes"
TEACHER_FLOW_MSG = "review completada, flujo teacher/short_answer: release-claim (sin finalize)"
DIRECT_MSG = "attempt finalizado (flow direct)"
HANDLER_ERROR_MSG = "Error procesando evento"


def offline_review(case, cfg, expected_pending_keys=None):
    """Seed with the worker stopped, submit, read the message; the worker stays stopped."""
    actors = ACTORS[cfg["actors"]]
    exam = f.prepare_exam(case, actors, cfg)
    attempt_id = f.take_attempt(case, actors, exam)
    rows = f.by_key(exam, f.answer_rows(attempt_id))
    expected = None
    if expected_pending_keys is not None:
        expected = [rows[k]["answer_id"] for k in expected_pending_keys if k in rows]
    message = f.captured_review_message(case, attempt_id, expected)
    return actors, exam, attempt_id, message


def finish_review(case, exam, attempt_id, answers):
    c.start_worker(case.tail)
    f.wait_reviewed(case, attempt_id, answers)
    rows = f.answer_rows(attempt_id)
    f.record_lanes(case, rows)
    f.measure_proposals(case, exam, rows)
    return f.by_key(exam, rows)


def num(value):
    """None stays None: «no review» must not pass as «0 points»."""
    return None if value is None else float(value)


def lane(case, row):
    for entry in case.lanes:
        if entry["answer_id"] == row["answer_id"]:
            return entry
    return {"lane": None, "log_line": None}


# ================================================================ preparation

def case_p1(case):
    cfg = CASES["P1"]
    actors = ACTORS[cfg["actors"]]
    c.stop_worker(case.tail)
    exam = f.create_draft(case, actors, cfg["title"], cfg["questions"])
    before = {}
    for q in exam["questions"]:
        found = c.find_messages(c.PREP_QUEUE, q["id"])
        event = f.message_json(found[0]) if found else None
        case.check(f"{q['key']}: learning publicó question.prep_requested", 1, len(found))
        case.check(f"{q['key']}: el mensaje cumple su esquema", [],
                   f.prep_event_problems(event) if event else ["no message"],
                   evidence=found[0]["payload"] if found else None)
        case.check(f"{q['key']}: reason", "created", (event or {}).get("payload", {}).get("reason"))
        before[q["key"]] = f.question_row(q["id"])
        case.check(f"{q['key']}: nace en pending", "pending", before[q["key"]]["llm_prep_status"])
    t0 = time.time()
    c.start_worker(case.tail)
    seconds, _ = f.wait_ready(case, exam)
    case.measure("prep_seconds_from_worker_start", round(time.time() - t0, 1))
    case.measure("prep_seconds_polling", seconds)
    artifacts = {}
    for q in exam["questions"]:
        row = f.question_row(q["id"])
        artifacts[q["key"]] = row["llm_prep"]
        case.check(f"{q['key']}: llm_prep_status", "ready", row["llm_prep_status"])
        case.check(f"{q['key']}: hay source_hash", True, bool(row["llm_prep_source_hash"]))
        case.check(f"{q['key']}: el source_hash no cambió al preparar",
                   before[q["key"]]["llm_prep_source_hash"], row["llm_prep_source_hash"])
        case.check(f"{q['key']}: el artefacto cumple el contrato v1", [],
                   f.prep_contract_problems(row["llm_prep"], q["type"]))
        lines = case.tail.grep(PREPARED_MSG, "question_id=" + q["id"])
        case.check(f"{q['key']}: el log dice «{PREPARED_MSG}»", True, bool(lines),
                   evidence=lines[-1] if lines else None)
    case.measure("artifacts", artifacts)
    case.check("la cola de preparación queda vacía", True, c.wait_queue_empty(c.PREP_QUEUE, 60))


BACKFILL_WHERE = ("q.question_type IN ('short_answer','open_ended') AND "
                  "(q.llm_prep_status = 'none' OR q.llm_prep_status IS NULL)")


def backfill_candidates_by_school():
    rows = c.sql_rows("SELECT a.school_id, count(*) AS n FROM assessment.question q "
                      "JOIN assessment.assessment a ON a.id = q.assessment_id WHERE "
                      + BACKFILL_WHERE + " GROUP BY a.school_id")
    return {r["school_id"]: r["n"] for r in rows}


def case_p2(case):
    """Global act: it does not filter by school and it changes statuses for good."""
    limit = CASES["P2"]["limit"]
    c.stop_worker(case.tail)
    before = backfill_candidates_by_school()
    total = sum(before.values())
    case.measure("candidates_before_by_school", before)
    if total == 0:
        case.note("no hay candidatas en none/NULL: toda pregunta creada por la API nace pending")
    queue_before = len(c.peek(c.PREP_QUEUE))
    _, payload = c.internal("POST", "/questions/prep-backfill", {"limit": limit})
    result = c.unwrap(payload) or {}
    case.check("enqueued = min(limit, candidatas)", min(limit, total), result.get("enqueued"))
    case.check("remaining = candidatas - enqueued", total - min(limit, total),
               result.get("remaining"))
    messages = [m for m in c.peek(c.PREP_QUEUE, 500)
                if ((f.message_json(m) or {}).get("payload") or {}).get("reason") == "backfill"]
    case.check("mensajes de backfill en la cola = enqueued", result.get("enqueued"), len(messages))
    case.measure("prep_queue_before_backfill", queue_before)
    enqueued = []
    for message in messages:
        event = f.message_json(message)
        case.check("el mensaje de backfill cumple su esquema", [], f.prep_event_problems(event),
                   evidence=message["payload"])
        qid = event["payload"]["question_id"]
        school = c.sql_rows("SELECT a.school_id FROM assessment.question q JOIN "
                            "assessment.assessment a ON a.id = q.assessment_id WHERE q.id='"
                            + c.uid(qid) + "'")[0]["school_id"]
        enqueued.append((qid, school))
        case.check(f"encolada {qid}: pasa a pending", "pending",
                   f.question_row(qid)["llm_prep_status"])
    case.ids["enqueued_questions"] = [qid for qid, _ in enqueued]
    after = backfill_candidates_by_school()
    case.measure("candidates_after_by_school", after)
    case.check("las candidatas que quedan por SQL = remaining", result.get("remaining"),
               sum(after.values()))
    modes = {school: c.school_settings(school).get("llm.review.mode")
             for school in {s for _, s in enqueued}}
    case.measure("llm_review_mode_by_school", modes)
    dead_before = len(c.peek(c.dlq(c.PREP_QUEUE), 500))
    c.start_worker(case.tail)
    emptied = c.wait_queue_empty(c.PREP_QUEUE, c.PER_ANSWER_TIMEOUT * max(1, len(enqueued)) + 60)
    case.check("la cola de preparación se vacía", True, emptied)
    for qid, school in enqueued:
        expected = "pending" if modes.get(school) in (None, "off") else "ready"
        if expected == "ready":
            c.poll_soft(lambda q=qid: f.question_row(q)["llm_prep_status"] == "ready", 30, 2)
        case.check(f"encolada {qid} (escuela con llm.review.mode={modes.get(school)})", expected,
                   f.question_row(qid)["llm_prep_status"])
    case.measure("prep_dlq_delta", len(c.peek(c.dlq(c.PREP_QUEUE), 500)) - dead_before)
    case.note("P2 no se restaura: las preguntas encoladas quedan pending o ready para siempre")


def case_p3(case):
    cfg = CASES["P3"]
    actors = ACTORS[cfg["actors"]]
    c.stop_worker(case.tail)
    exam = f.create_draft(case, actors, cfg["title"], cfg["questions"])
    qid = exam["questions"][0]["id"]
    found = c.find_messages(c.PREP_QUEUE, qid)
    case.check("mensaje real de la creación, leído de la cola", 1, len(found))
    body = found[0]["payload"]
    c.start_worker(case.tail)
    f.wait_ready(case, exam)
    first = f.question_row(qid)
    case.check("precondición: la pregunta llegó a ready", "ready", first["llm_prep_status"])
    dead_before = len(c.find_messages(c.dlq(c.PREP_QUEUE), qid))
    t0 = time.time()
    c.publish(c.PREP_QUEUE, body)
    lines = case.tail.wait_for((PREPARED_MSG, "question_id=" + qid), c.PER_ANSWER_TIMEOUT, 2)
    case.measure("redelivery_seconds", round(time.time() - t0, 1))
    case.check(f"el log dice «{PREPARED_MSG}» otra vez (2 líneas)", True, len(lines) >= 2,
               evidence=lines[-1] if lines else None)
    c.wait_queue_empty(c.PREP_QUEUE, 60)
    second = f.question_row(qid)
    t1, t2 = (f.parse_ts(first["llm_prep_updated_at"]), f.parse_ts(second["llm_prep_updated_at"]))
    case.check("llm_prep_updated_at avanza (se reescribe)", True, bool(t1 and t2 and t2 > t1),
               evidence=[first["llm_prep_updated_at"], second["llm_prep_updated_at"]])
    case.check("el estado sigue en ready", "ready", second["llm_prep_status"])
    case.check("el mensaje no vuelve: la cola queda vacía", 0,
               len(c.find_messages(c.PREP_QUEUE, qid)))
    case.check("el mensaje no va a la cola muerta", dead_before,
               len(c.find_messages(c.dlq(c.PREP_QUEUE), qid)))
    case.measure("artifact_changed", first["llm_prep"] != second["llm_prep"])
    case.measure("artifacts", {"first": first["llm_prep"], "second": second["llm_prep"]})
    # The 409: a PUT with an old hash does not write.
    status, payload = c.internal("PUT", f"/questions/{qid}/llm-prep",
                                 {"llm_prep": second["llm_prep"], "source_hash": cfg["stale_hash"],
                                  "consumed_feedback": False}, ok=None)
    case.check("PUT con hash viejo: status", 409, status)
    case.check("PUT con hash viejo: código", "PREP_HASH_MISMATCH", c.error_code(payload))
    third = f.question_row(qid)
    case.check("el PUT rechazado no escribe (llm_prep_updated_at igual)",
               second["llm_prep_updated_at"], third["llm_prep_updated_at"])


# ================================================================ review

def case_r1(case):
    cfg = CASES["R1"]
    _, exam, attempt_id, _ = offline_review(case, cfg, ["a_falta_un_item", "b_otra_cosa"])
    at_submit = f.by_key(exam, f.answer_rows(attempt_id))
    exact = at_submit["c_conjunto_exacto"]
    case.check("(c) learning la deja auto_graded al entregar, sin worker", "auto_graded",
               exact["review_status"])
    case.check("(c) con el máximo", num(exact["max_points"]), num(exact["points_earned"]))
    rows = finish_review(case, exam, attempt_id, 2)
    for key in ("a_falta_un_item", "b_otra_cosa"):
        f.assert_ai_reviewed(case, key, rows[key])
        entry = lane(case, rows[key])
        case.check(f"{key}: carril de lista en el log", "lista", entry["lane"],
                   evidence=entry.get("lane_line"))
    case.check("(a) 0 puntos por regla (ítem que falta, sin token sobrante)", 0.0,
               num(rows["a_falta_un_item"]["points_awarded"]))
    max_b = float(rows["b_otra_cosa"]["max_points"])
    case.check_in("(b) 0 o el máximo", [0, max_b], rows["b_otra_cosa"]["points_awarded"])
    case.check("(c) no llega al worker", "sin_worker", lane(case, rows["c_conjunto_exacto"])["lane"])
    case.check("(c) sigue auto_graded", "auto_graded", rows["c_conjunto_exacto"]["review_status"])
    f.assert_released(case, attempt_id)
    case.measure("b_equivalence_verdict", {"points": rows["b_otra_cosa"]["points_awarded"],
                                           "feedback": rows["b_otra_cosa"]["feedback"]})


def case_r2(case):
    """The field's case, as control: the model prepares; the lane is noted, not asserted."""
    cfg = CASES["R2"]
    actors = ACTORS[cfg["actors"]]
    exam = f.prepare_exam(case, actors, cfg)
    attempt_id = f.take_attempt(case, actors, exam)
    f.wait_reviewed(case, attempt_id, len(exam["questions"]))
    rows = f.answer_rows(attempt_id)
    f.record_lanes(case, rows)
    f.measure_proposals(case, exam, rows)
    for key, row in f.by_key(exam, rows).items():
        if row["review_status"] == "auto_graded":
            case.note(f"{key}: learning la cerró al entregar (auto_graded); no fue al worker")
            continue
        f.assert_ai_reviewed(case, key, row)
        case.check_in(f"{key}: el número sale del veredicto (0 o el máximo)",
                      [0, float(row["max_points"])], row["points_awarded"])
    f.assert_released(case, attempt_id)


def case_r3(case):
    cfg = CASES["R3"]
    keys = [q["key"] for q in cfg["questions"]]
    _, exam, attempt_id, _ = offline_review(case, cfg, keys)
    rows = finish_review(case, exam, attempt_id, len(keys))
    total = len(cfg["questions"][0]["prep"]["criteria"])
    for key in keys:
        row = rows[key]
        f.assert_ai_reviewed(case, key, row)
        entry = lane(case, row)
        case.check(f"{key}: carril de criterios en el log", "criterios", entry["lane"])
        counted = c.parse_line(entry.get("lane_line") or "").get("criteria")
        case.check(f"{key}: la línea del carril cuenta los criterios", str(total), counted,
                   evidence=entry.get("lane_line"))
        top = float(row["max_points"])
        allowed = [0, top] + [top * (0.3 + 0.4 * k / total) for k in range(total + 1)]
        case.check_in(f"{key}: puntos = 0, el máximo o 0,3 + 0,4·(cumplidos/total) del máximo",
                      allowed, row["points_awarded"])
    f.assert_released(case, attempt_id)


def case_r4(case):
    cfg = CASES["R4"]
    _, exam, attempt_id, _ = offline_review(case, cfg, ["sin_prep"])
    qid = exam["questions"][0]["id"]
    case.check("la pregunta no tiene preparación al entregar", "pending",
               f.question_row(qid)["llm_prep_status"])
    rows = finish_review(case, exam, attempt_id, 1)
    row = rows["sin_prep"]
    f.assert_ai_reviewed(case, "sin_prep", row)
    case.check("va por el juicio global (ninguna línea de carril)", "global",
               lane(case, row)["lane"])
    top = float(row["max_points"])
    case.check_in("puntos = 1, 0,5 o 0 del máximo", [top, top / 2, 0], row["points_awarded"])
    f.assert_released(case, attempt_id)


def case_r5(case):
    cfg = CASES["R5"]
    keys = [q["key"] for q in cfg["questions"]]
    _, exam, attempt_id, _ = offline_review(case, cfg, keys)
    rows = finish_review(case, exam, attempt_id, len(keys))
    for key in keys:
        row = rows.get(key)
        if row is None:
            case.check(f"{key}: la respuesta existe tras entregar", True, False)
            continue
        f.assert_ai_reviewed(case, key, row)
        case.check(f"{key}: 0 puntos", 0.0, num(row["points_awarded"]))
        case.check(f"{key}: comentario", "Respuesta en blanco.", row["feedback"])
        entry = lane(case, row)
        case.check(f"{key}: en_blanco_por_regla=true en «answer revisada por LLM»",
                   "blanco_por_regla", entry["lane"], evidence=entry["log_line"])
    f.assert_released(case, attempt_id)
    case.note("el log no registra las llamadas al modelo: su ausencia no se afirma por él")


def case_r6(case):
    """Mixed attempt (T5a of plan 040). The model prepares; ends with the teacher's finalize."""
    cfg = CASES["R6"]
    actors = ACTORS[cfg["actors"]]
    exam = f.prepare_exam(case, actors, cfg)
    attempt_id = f.take_attempt(case, actors, exam)
    f.wait_reviewed(case, attempt_id, len(exam["questions"]))
    rows = f.answer_rows(attempt_id)
    f.record_lanes(case, rows)
    f.measure_proposals(case, exam, rows)
    keyed = f.by_key(exam, rows)
    for key, row in keyed.items():
        if row["review_status"] == "auto_graded":
            case.note(f"{key}: learning la cerró al entregar; el intento dejó de ser mixto")
            continue
        f.assert_ai_reviewed(case, key, row)
    attempt = f.assert_released(case, attempt_id)
    case.check("con flow=teacher el intento sigue pending_review", "pending_review",
               attempt["status"])
    lines = case.tail.grep(TEACHER_FLOW_MSG, "attempt_id=" + attempt_id)
    case.check("el log dice release-claim sin finalize", True, bool(lines),
               evidence=lines[-1] if lines else None)
    case.check("sin finalizar no hay grade_item", 0, len(f.grade_items(attempt_id)))
    c.call("POST", f"{c.LEARNING}/api/v1/attempts/{attempt_id}/finalize", {}, f.teacher(actors))
    case.check("tras el finalize de la docente: completed", "completed",
               f.attempt_row(attempt_id)["status"])
    f.assert_grade_item(case, actors, attempt_id)


def flow_value(school):
    rows = c.sql_rows("SELECT value FROM academic.school_settings WHERE school_id='"
                      + c.uid(school) + "' AND key='llm.review.flow'")
    return rows[0]["value"] if rows else None


def direct_flow_review(case, cfg):
    """Body of R7 and R7n. Writes by SQL: llm.review.flow -> direct, restored in the cleanup."""
    actors = ACTORS[cfg["actors"]]
    original = flow_value(SAN_IGNACIO)
    case.check("precondición: llm.review.flow de San Ignacio es teacher", "teacher", original)
    if original is None:
        raise c.EnvError("San Ignacio has no llm.review.flow row: the UPDATE would do nothing")
    c.stop_worker(case.tail)

    def restore():
        c.sql_update_review_flow(SAN_IGNACIO, "teacher")
        c.stop_worker(case.tail)  # the worker caches the policy for 60 s: restart drops it
        case.check("política restaurada a teacher (SQL)", "teacher", flow_value(SAN_IGNACIO))
        case.check("política restaurada a teacher (GET interno de academic)", "teacher",
                   c.school_settings(SAN_IGNACIO).get("llm.review.flow"))
    case.defer("restaurar llm.review.flow=teacher y reiniciar el worker", restore)

    c.sql_update_review_flow(SAN_IGNACIO, "direct")
    case.check("la política quedó en direct (SQL)", "direct", flow_value(SAN_IGNACIO))
    case.check("la política resuelta por academic es direct", "direct",
               c.school_settings(SAN_IGNACIO).get("llm.review.flow"))
    exam = f.prepare_exam(case, actors, cfg)
    attempt_id = f.take_attempt(case, actors, exam)
    message = f.captured_review_message(case, attempt_id)
    event = (f.message_json(message) if message else None) or {}
    types = sorted({r["question_type"] for r in (event.get("payload") or {}).get("answers", [])})
    case.check("el evento solo lleva abiertas", [OPEN], types)
    c.start_worker(case.tail)
    done = c.poll_soft(lambda: f.attempt_row(attempt_id)["status"] == "completed",
                       c.PER_ANSWER_TIMEOUT * len(exam["questions"]) + 60, 3)
    rows = f.answer_rows(attempt_id)
    f.record_lanes(case, rows)
    f.measure_proposals(case, exam, rows)
    attempt = f.attempt_row(attempt_id)
    case.check("el worker finaliza sin docente: completed", "completed", attempt["status"],
               evidence=None if done else "timeout")
    lines = case.tail.grep(DIRECT_MSG, "attempt_id=" + attempt_id)
    case.check(f"el log dice «{DIRECT_MSG}»", True, bool(lines),
               evidence=lines[-1] if lines else None)
    case.check("el candado NO queda liberado (ai_review_claimed_at sigue puesto)", True,
               attempt["ai_review_claimed_at"] is not None,
               evidence=attempt["ai_review_claimed_at"])
    for key, row in f.by_key(exam, rows).items():
        f.assert_ai_reviewed(case, key, row)
    f.assert_grade_item(case, actors, attempt_id)
    case.measure("attempt", {k: attempt[k] for k in ("score", "max_score", "percentage")})


def case_r7(case):
    direct_flow_review(case, CASES["R7"])


def case_r7n(case):
    """R7 with blank answers: the worker settles them by rule and never calls the model."""
    direct_flow_review(case, CASES["R7n"])


def case_r8(case):
    cfg = CASES["R8"]
    actors = ACTORS[cfg["actors"]]
    settings = c.school_settings(actors["school"])
    case.check("precondición: la Academia resuelve llm.review.mode=off", "off",
               settings.get("llm.review.mode"), evidence=settings)
    dead_before = len(c.peek(c.dlq(c.REVIEW_QUEUE), 500))
    _, exam, attempt_id, _ = offline_review(case, cfg, ["abierta"])
    c.start_worker(case.tail)
    lines = case.tail.wait_for((REVIEW_OFF_MSG, "attempt_id=" + attempt_id), 90)
    case.check(f"el worker confirma con «{REVIEW_OFF_MSG}»", True, bool(lines),
               evidence=lines[-1] if lines else None)
    case.check("la cola de revisión queda vacía", True, c.wait_queue_empty(c.REVIEW_QUEUE, 60))
    attempt = f.attempt_row(attempt_id)
    rows = f.by_key(exam, f.answer_rows(attempt_id))
    f.record_lanes(case, list(rows.values()))
    case.check("el intento queda pending_review", "pending_review", attempt["status"])
    case.check("la respuesta queda pending", "pending", rows["abierta"]["review_status"])
    case.check("sin fila en attempt_review", None, rows["abierta"]["review_source"])
    case.check("sin candado", None, attempt["ai_review_claimed_at"])
    case.check("nada a la cola muerta", dead_before, len(c.peek(c.dlq(c.REVIEW_QUEUE), 500)))
    case.note("el intento de la Academia queda pending_review, sin corregir: es lo esperado")


def case_r9(case):
    cfg = CASES["R9"]
    keys = [q["key"] for q in cfg["questions"]]
    actors, exam, attempt_id, _ = offline_review(case, cfg, keys)
    rows = f.by_key(exam, f.answer_rows(attempt_id))
    target = rows["la_puntua_la_docente"]
    c.call("POST", f"{c.LEARNING}/api/v1/attempts/{attempt_id}/answers/{target['answer_id']}/review",
           {"points_awarded": cfg["teacher_points"], "feedback": cfg["teacher_feedback"]},
           f.teacher(actors))
    scored = f.by_key(exam, f.answer_rows(attempt_id))["la_puntua_la_docente"]
    case.check("la docente deja la respuesta reviewed", "reviewed", scored["review_status"])
    case.check("review_source de la docente", "teacher", scored["review_source"])
    status, payload = c.internal(
        "POST", f"/attempts/{attempt_id}/answers/{target['answer_id']}/review",
        {"points_awarded": 0, "feedback": "Inyectado por el guion: no debe escribirse."}, ok=None)
    case.check("review interno sobre una reviewed: status", 409, status)
    case.check("review interno sobre una reviewed: código", "ATTEMPT_ANSWER_NOT_REVIEWABLE",
               c.error_code(payload))
    status, payload = c.internal("GET", f"/attempts/{attempt_id}/answers", ok=None)
    case.measure("internal_pending_list_status", status)
    if status == 200:
        listed = [a["answer_id"] for a in (c.unwrap(payload) or {}).get("answers", [])]
        case.check("la reviewed no sale entre las pendientes del worker", False,
                   target["answer_id"] in listed, evidence=listed)
    final = finish_review(case, exam, attempt_id, 1)
    kept = final["la_puntua_la_docente"]
    case.check("el worker no la toca: sigue reviewed", "reviewed", kept["review_status"])
    case.check("review_source sigue en teacher", "teacher", kept["review_source"])
    case.check("los puntos de la docente no cambian", float(cfg["teacher_points"]),
               float(kept["points_awarded"]))
    case.check("attempt_review.updated_at no cambia", scored["review_updated_at"],
               kept["review_updated_at"])
    case.check("el worker no deja línea de veredicto para ella", "sin_worker",
               lane(case, kept)["lane"])
    f.assert_ai_reviewed(case, "la_revisa_la_ia", final["la_revisa_la_ia"])
    f.assert_released(case, attempt_id)


def case_r10(case):
    cfg = CASES["R10"]
    actors, exam, attempt_id, message = offline_review(case, cfg, ["abierta"])
    if not message:
        raise c.EnvError("no review message to copy")
    body = message["payload"]
    rows = finish_review(case, exam, attempt_id, 1)
    f.assert_ai_reviewed(case, "abierta", rows["abierta"])
    first = rows["abierta"]["review_updated_at"]
    dead_before = len(c.find_messages(c.dlq(c.REVIEW_QUEUE), attempt_id))

    # 1 · repeated event while the attempt is still pending_review
    c.publish(c.REVIEW_QUEUE, body)
    lines = case.tail.wait_for((NO_PENDING_MSG, "attempt_id=" + attempt_id), 90)
    case.check(f"pending_review: el log dice «{NO_PENDING_MSG}»", True, bool(lines),
               evidence=lines[-1] if lines else None)
    case.check("pending_review: la cola queda vacía", True, c.wait_queue_empty(c.REVIEW_QUEUE, 60))
    again = f.by_key(exam, f.answer_rows(attempt_id))["abierta"]
    case.check("pending_review: attempt_review.updated_at no cambia", first,
               again["review_updated_at"])
    attempt = f.assert_released(case, attempt_id)
    case.check("pending_review: el intento sigue pending_review", "pending_review",
               attempt["status"])

    # 2 · repeated event once the teacher finalized
    c.call("POST", f"{c.LEARNING}/api/v1/attempts/{attempt_id}/finalize", {}, f.teacher(actors))
    case.check("la docente finaliza: completed", "completed", f.attempt_row(attempt_id)["status"])
    second = f.by_key(exam, f.answer_rows(attempt_id))["abierta"]["review_updated_at"]
    c.publish(c.REVIEW_QUEUE, body)
    lines = case.tail.wait_for((NOT_CLAIMABLE_MSG, "attempt_id=" + attempt_id), 90)
    case.check(f"completed: el log dice «{NOT_CLAIMABLE_MSG}»", True, bool(lines),
               evidence=lines[-1] if lines else None)
    case.check("completed: la cola queda vacía", True, c.wait_queue_empty(c.REVIEW_QUEUE, 60))
    last = f.by_key(exam, f.answer_rows(attempt_id))["abierta"]
    case.check("completed: attempt_review.updated_at no cambia", second, last["review_updated_at"])
    case.check("completed: el intento sigue completed", "completed",
               f.attempt_row(attempt_id)["status"])
    case.check("ningún mensaje a la cola muerta", dead_before,
               len(c.find_messages(c.dlq(c.REVIEW_QUEUE), attempt_id)))


def case_r11(case):
    cfg = CASES["R11"]
    _, exam, attempt_id, message = offline_review(case, cfg, ["abierta"])
    if not message:
        raise c.EnvError("no review message to copy")
    body = message["payload"]
    if len(c.peek(c.REVIEW_QUEUE)) == 1:
        c.purge(c.REVIEW_QUEUE)  # only ours: the event goes in by the queue, after the claim
    else:
        case.note("la cola de revisión tenía mensajes ajenos: no se purga; el original se queda")
    dead_before = len(c.find_messages(c.dlq(c.REVIEW_QUEUE), attempt_id))
    _, claim = c.internal("POST", f"/attempts/{attempt_id}/claim")
    case.defer("liberar el candado tomado por el guion",
               lambda: c.internal("POST", f"/attempts/{attempt_id}/release-claim", ok=None))
    claimed_at = f.attempt_row(attempt_id)["ai_review_claimed_at"]
    case.check("el guion toma el candado", True, claimed_at is not None,
               evidence=(c.unwrap(claim) or {}).get("claimed_at"))
    status, payload = c.internal("POST", f"/attempts/{attempt_id}/claim", ok=None)
    case.check("segundo claim: status", 409, status)
    case.check("segundo claim: código", "AI_REVIEW_CLAIM_ACTIVE", c.error_code(payload))
    c.start_worker(case.tail)
    c.publish(c.REVIEW_QUEUE, body)
    lines = case.tail.wait_for((NOT_CLAIMABLE_MSG, "attempt_id=" + attempt_id), 90)
    case.check(f"el worker se abstiene: «{NOT_CLAIMABLE_MSG}»", True, bool(lines),
               evidence=lines[-1] if lines else None)
    case.check("confirma el mensaje: la cola queda vacía", True,
               c.wait_queue_empty(c.REVIEW_QUEUE, 60))
    row = f.by_key(exam, f.answer_rows(attempt_id))["abierta"]
    f.record_lanes(case, [row])
    case.check("no escribe: la respuesta sigue pending", "pending", row["review_status"])
    case.check("no escribe: sin fila en attempt_review", None, row["review_source"])
    case.check("el candado del guion sigue intacto", claimed_at,
               f.attempt_row(attempt_id)["ai_review_claimed_at"])
    case.check("nada a la cola muerta", dead_before,
               len(c.find_messages(c.dlq(c.REVIEW_QUEUE), attempt_id)))
    status, _ = c.internal("POST", f"/attempts/{attempt_id}/release-claim", ok=None)
    case.check("release-claim del guion: status", 204, status)
    case.check("candado liberado al terminar", None,
               f.attempt_row(attempt_id)["ai_review_claimed_at"])
    case.note("el intento queda pending_review con la respuesta pending: nadie lo reinyecta")


def error_times(case, needle):
    lines = case.tail.grep(HANDLER_ERROR_MSG, needle)
    stamps = [t for t in (c.line_time(ln) for ln in lines) if t is not None]
    gaps = [round(b - a, 1) for a, b in zip(stamps, stamps[1:])]
    return lines, gaps


def wait_dead(queue, needle, timeout):
    return c.poll_soft(lambda: c.find_messages(c.dlq(queue), needle, 500), timeout, 3) or []


def review_until_dead_letter(case, cfg):
    """Body of R12 and R12n: Ollama down before the submit, up to the dead-letter queue.

    Ollama is only stopped (and brought back in the cleanup) if it was up: a run without
    model, with Ollama already down, kills nothing.
    """
    actors = ACTORS[cfg["actors"]]
    exam = f.prepare_exam(case, actors, cfg)  # worker stopped, prep queue purged
    attempt_id = f.start_attempt(case, actors, exam)
    c.start_worker(case.tail)  # the queues are empty
    if c.ollama_up():
        case.defer("levantar Ollama", c.start_ollama)
        case.measure("ollama_stopped_by", c.stop_ollama())
    else:
        case.measure("ollama_stopped_by", "already_down")
    case.check("Ollama está parado antes de entregar", False, c.ollama_up())
    t0 = time.time()
    f.submit_attempt(case, actors, attempt_id)
    dead = wait_dead(c.REVIEW_QUEUE, attempt_id, 240)
    case.measure("seconds_to_dead_letter", round(time.time() - t0, 1))
    case.check("el mensaje llega a la cola muerta", 1, len(dead))
    lines, gaps = error_times(case, attempt_id)
    if not lines:
        lines, gaps = error_times(case, "")
        case.note("«Error procesando evento» no nombra el intento: se cuentan todas las líneas")
    case.check("cuatro ejecuciones fallidas (la original y tres reintentos)", 4, len(lines),
               evidence=lines[-1] if lines else None)
    case.measure("seconds_between_failures", gaps)
    case.note("lo esperado entre fallas: 5, 10 y 20 s; x-retry-count de 1 a 3 no se ve en la "
              "cola (el reintento se consume al instante): se deduce de las 4 fallas y del 3 final")
    released = case.tail.grep("candado liberado", "attempt_id=" + attempt_id)
    case.check("candado liberado en cada salida (4 líneas)", 4, len(released))
    attempt = f.attempt_row(attempt_id)
    case.check("candado liberado (ai_review_claimed_at nulo)", None,
               attempt["ai_review_claimed_at"])
    row = f.by_key(exam, f.answer_rows(attempt_id))["abierta"]
    case.check("la respuesta sigue pending", "pending", row["review_status"])
    case.check("la cola de revisión queda vacía", True, c.wait_queue_empty(c.REVIEW_QUEUE, 60))
    if dead:
        headers = dead[0]["headers"]
        case.check("cola muerta: x-retry-count", 3, headers.get("x-retry-count"), evidence=headers)
        case.check("cola muerta: x-original-routing-key", c.REVIEW_QUEUE,
                   headers.get("x-original-routing-key"))
        case.check("cola muerta: hay x-failed-at", True, "x-failed-at" in headers)
        case.check("cola muerta: el mensaje no es persistente (delivery_mode distinto de 2)",
                   True, dead[0]["delivery_mode"] != 2, evidence=dead[0]["delivery_mode"])
    return exam, attempt_id, dead


def case_r12n(case):
    """R12 up to the dead-letter queue: no reinjection, so no call to the model."""
    review_until_dead_letter(case, CASES["R12n"])
    case.note("el mensaje queda en la cola muerta de revisión y el intento en pending_review: "
              "no se reinyecta")


def case_r12(case):
    exam, attempt_id, dead = review_until_dead_letter(case, CASES["R12"])
    # Back up, and reinjected WITHOUT x-retry-count.
    c.start_ollama()
    case.check("Ollama vuelve", True, c.ollama_up())
    if dead:
        c.publish(c.REVIEW_QUEUE, dead[0]["payload"], headers={})
        f.wait_reviewed(case, attempt_id, 1)
        rows = f.answer_rows(attempt_id)
        f.record_lanes(case, rows)
        f.measure_proposals(case, exam, rows)
        f.assert_ai_reviewed(case, "abierta (reinyectada sin x-retry-count)", rows[0])
        case.check("candado liberado tras corregir", None,
                   f.attempt_row(attempt_id)["ai_review_claimed_at"])
        case.note("la copia original sigue en la cola muerta de revisión: no se retira")


def case_r12b(case):
    """Optional. Everything here is measured: the window cannot be hit at will."""
    cfg = CASES["R12b"]
    actors = ACTORS[cfg["actors"]]
    exam = f.prepare_exam(case, actors, cfg)
    attempt_id = f.start_attempt(case, actors, exam)
    c.start_worker(case.tail)
    case.defer("levantar Ollama", c.start_ollama)
    f.submit_attempt(case, actors, attempt_id)
    first = case.tail.wait_for((c.REVIEWED_MSG, "attempt_id=" + attempt_id), c.PER_ANSWER_TIMEOUT)
    case.measure("reviewed_before_stopping", len(first))
    case.measure("ollama_stopped_by", c.stop_ollama())
    failed = case.tail.wait_for(("candado liberado", "attempt_id=" + attempt_id), 300)
    case.measure("released_after_failure", len(failed))
    c.start_ollama()
    # With Ollama back, the retries in flight either correct the attempt or end in the
    # dead-letter queue within about 35 s: look there first, not after the long wait.
    early = c.poll_soft(lambda: c.find_messages(c.dlq(c.REVIEW_QUEUE), attempt_id, 500), 60, 3)
    if early:
        case.note("el mensaje llegó a la cola muerta antes de que Ollama volviera: se reinyecta")
        c.publish(c.REVIEW_QUEUE, early[0]["payload"], headers={})
    settled = f.wait_reviewed(case, attempt_id, len(exam["questions"]))
    dead = c.find_messages(c.dlq(c.REVIEW_QUEUE), attempt_id, 500)
    if not settled and dead and not early:
        case.note("el mensaje llegó a la cola muerta antes de que Ollama volviera: se reinyecta")
        c.publish(c.REVIEW_QUEUE, dead[0]["payload"], headers={})
        settled = f.wait_reviewed(case, attempt_id, len(exam["questions"]))
    rows = f.answer_rows(attempt_id)
    f.record_lanes(case, rows)
    f.measure_proposals(case, exam, rows)
    verdicts = {r["answer_id"]: len(case.tail.grep(c.REVIEWED_MSG, "answer_id=" + r["answer_id"]))
                for r in rows}
    case.measure("verdict_lines_per_answer", verdicts)
    case.measure("resumed_only_missing", all(n <= 1 for n in verdicts.values()))
    case.measure("reached_dead_letter", bool(dead))
    case.check("al final no queda ninguna respuesta pending", [],
               [r["answer_id"] for r in rows if r["review_status"] == "pending"])
    case.check("candado liberado al final", None, f.attempt_row(attempt_id)["ai_review_claimed_at"])


def case_r13(case):
    cfg = CASES["R13"]
    actors = ACTORS[cfg["actors"]]
    c.start_worker(case.tail)
    # No fallback to another school: there the probe would call the model. If the probe
    # cannot be seeded in the Academia, the case is an environment error.
    exam = f.create_draft(case, actors, cfg["title"], [])
    probe_off = c.school_settings(actors["school"]).get("llm.review.mode") == "off"
    suffix = uuid.uuid4().hex[:10]
    unknown_type = "edugo.test.unknown_" + suffix
    unknown = json.dumps({"event_id": str(uuid.uuid4()), "event_type": unknown_type,
                          "event_version": "1.0", "timestamp": c.now_iso(), "payload": {}})
    broken = '{"broken_' + suffix + '": '
    case.ids["suffix"] = suffix
    t0 = time.time()
    c.publish(c.REVIEW_QUEUE, unknown)
    c.publish(c.REVIEW_QUEUE, broken)
    # The probe: creating a question makes learning publish a real message meanwhile.
    probe = f.add_question(actors, exam["assessment_id"], cfg["probe_question"])
    case.ids["probe_question_id"] = probe["id"]
    probe_msg = PREP_OFF_MSG if probe_off else PREPARED_MSG
    probe_lines = case.tail.wait_for((probe_msg, "question_id=" + probe["id"]),
                                     60 if probe_off else c.PER_ANSWER_TIMEOUT)
    case.check("el worker sigue atendiendo: la sonda de preparación se procesa", True,
               bool(probe_lines),
               evidence=probe_lines[-1] if probe_lines else None)
    dead = c.poll_soft(lambda: (lambda found: found if len(found) >= 2 else None)(
        c.find_messages(c.dlq(c.REVIEW_QUEUE), suffix, 500)), 240, 3) or \
        c.find_messages(c.dlq(c.REVIEW_QUEUE), suffix, 500)
    case.measure("seconds_to_dead_letter", round(time.time() - t0, 1))
    case.check("los dos mensajes acaban en la cola muerta de revisión", 2, len(dead))
    for message in dead:
        kind = "tipo desconocido" if unknown_type in message["payload"] else "JSON roto"
        case.check(f"{kind}: x-retry-count (no va directo: tres reintentos)", 3,
                   message["headers"].get("x-retry-count"), evidence=message["headers"])
    unknown_lines, unknown_gaps = error_times(case, unknown_type)
    case.check("tipo desconocido: cuatro ejecuciones fallidas", 4, len(unknown_lines),
               evidence=unknown_lines[-1] if unknown_lines else None)
    broken_lines, broken_gaps = error_times(case, "invalid message format")
    case.check("JSON roto: cuatro ejecuciones fallidas", 4, len(broken_lines),
               evidence=broken_lines[-1] if broken_lines else None)
    case.measure("seconds_between_failures", {"unknown": unknown_gaps, "broken": broken_gaps})
    # Both poison messages share the review queue: the second one fails for the first time
    # while the first one is still in its 5 s wait. That is the consumer serving another message.
    first_unknown = c.line_time(unknown_lines[0]) if unknown_lines else None
    first_broken = c.line_time(broken_lines[0]) if broken_lines else None
    gap = None if first_unknown is None or first_broken is None else \
        round(first_broken - first_unknown, 2)
    case.check("el segundo mensaje se atiende durante la espera del primero (menos de 5 s "
               "entre sus primeras fallas)", True, gap is not None and abs(gap) < 5, evidence=gap)
    case.check("el worker sigue vivo", True, c.worker_alive())
    case.check("la cola de revisión queda vacía", True, c.wait_queue_empty(c.REVIEW_QUEUE, 60))
    case.note("los dos mensajes de prueba quedan en la cola muerta de revisión (sufijo "
              + suffix + "); la evaluación de la sonda queda en borrador")


FUNCTIONS = {"P1": case_p1, "P2": case_p2, "P3": case_p3, "R1": case_r1, "R2": case_r2,
             "R3": case_r3, "R4": case_r4, "R5": case_r5, "R6": case_r6, "R7": case_r7,
             "R7n": case_r7n, "R8": case_r8, "R9": case_r9, "R10": case_r10, "R11": case_r11,
             "R12": case_r12, "R12n": case_r12n, "R12b": case_r12b, "R13": case_r13}


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    parser.add_argument("--cases", help="comma-separated, e.g. P1,R1,R2 (run in that order)")
    parser.add_argument("--no-model", action="store_true",
                        help="run only the cases that do not need a model")
    parser.add_argument("--out-dir", help="required to run: one folder per run is created in it")
    parser.add_argument("--list", action="store_true", help="list the cases and exit")
    args = parser.parse_args(argv)
    if args.list:
        for name in ORDER:
            mark = "modelo" if CASES[name]["needs_model"] else "sin modelo"
            print(f"{name:5} {mark:10} {CASES[name]['title']}")
        return e2e_env.EXIT_OK
    if bool(args.cases) == args.no_model:
        print("give exactly one of --cases or --no-model (or --list)", file=sys.stderr)
        return e2e_env.EXIT_USAGE
    if args.no_model:
        names = [n for n in ORDER if not CASES[n]["needs_model"]]
    else:
        names = [n.strip() for n in args.cases.split(",") if n.strip()]
    unknown = [n for n in names if n not in FUNCTIONS]
    if unknown:
        print("unknown case(s): " + ", ".join(unknown), file=sys.stderr)
        return e2e_env.EXIT_USAGE
    try:
        e2e_env.check_out_dir(args.out_dir)
        e2e_env.enforce_guard()
        e2e_env.require_external_paths()
        run_dir = e2e_env.new_run_dir(args.out_dir)
    except e2e_env.UsageError as err:
        print(f"usage: {err}", file=sys.stderr)
        return e2e_env.EXIT_USAGE
    except e2e_env.GuardError as err:
        print(f"guard: {err}", file=sys.stderr)
        return e2e_env.EXIT_GUARD
    except e2e_env.MissingPathError as err:
        print(f"environment: {err}", file=sys.stderr)
        return e2e_env.EXIT_FAILED
    print(f"run folder: {run_dir}")
    summaries = []
    for name in names:
        cfg = CASES[name]
        summary = c.run_case(name, cfg["title"], FUNCTIONS[name], run_dir, cfg["needs_model"])
        print(json.dumps(summary, ensure_ascii=False))
        summaries.append(summary)
    print(e2e_env.unmet_line(summaries))
    return e2e_env.exit_code(summaries)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
