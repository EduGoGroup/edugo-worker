#!/usr/bin/env python3
"""Concurrency case of the inference cap (HU-IA.10): three reviews and two short PDFs at once.

C1 seeds three exams with the preparation pinned by hand, saves one attempt for each and,
with the worker stopped, submits the three attempts and requests the generation of two
short PDFs. Then it starts the worker: the five messages arrive at once.

It asserts clocks and queues only, never what the model writes (ADR 0048): no expired call
and no consumer retry in the worker log during the window, and the three dead-letter queues
as they were. It writes down, without asserting, the final state of every answer and job,
the wait of every turn and the wall clock.

Standard library only. Reuses the helpers of `revision/` and `material/`; nothing there
changes. Usage, from the worker root:

    python3 scripts/e2e/concurrency/run.py --list
    EDUGO_BASE_PASSWORD=<clave> python3 scripts/e2e/concurrency/run.py --out-dir <carpeta>
"""
import argparse
import importlib.util
import json
import os
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
E2E = os.path.dirname(HERE)
sys.path.insert(0, os.path.join(E2E, "revision"))

import common as c  # noqa: E402 - needs the path above
import flow as f  # noqa: E402
from cases_data import ACTORS, CASES as REVISION_CASES  # noqa: E402
from common import e2e_env  # noqa: E402


def _load_material():
    """material/run.py under its own module name: `run` would clash with revision's."""
    spec = importlib.util.spec_from_file_location("material_run",
                                                  os.path.join(E2E, "material", "run.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)  # its main() is behind `if __name__ == "__main__"`
    return module


m = _load_material()

CASES = {
    "C1": {"title": "Tres revisiones y dos PDF cortos a la vez: ninguna llamada vence por espera",
           "needs_model": True, "reviews": 3, "pdfs": 2, "pdf": "ciclo_del_agua.pdf",
           "actors": "science_5b", "exam": "R3", "timeout_min": 40},
}

# Measured on 2026-11-25 without the cap, with LESS load (2 short PDFs and 1 review).
BASELINE_NO_CAP_SECONDS = 1023

CLIENT_TIMEOUT_NEEDLE = "Client.Timeout exceeded"
RETRY_MSG = "Mensaje fallido: se reintenta"
TURN_TAKEN_MSG = "turno de inferencia tomado"
TURN_RELEASED_MSG = "turno de inferencia soltado"
SLOW_CALL_TURN_MSG = "turno de inferencia por llamada tras una espera larga"


def dead_letter_depths():
    return {c.dlq(queue): c.queue_depth(c.dlq(queue))["total"] for queue in c.WORK_QUEUES}


def turn_waits(lines):
    """The turn lines of the worker log, as rows: who, which limiter, wait and held time."""
    rows = []
    for line in lines:
        kind = next((k for k, msg in (("mensaje", TURN_TAKEN_MSG), ("soltado", TURN_RELEASED_MSG),
                                      ("llamada_lenta", SLOW_CALL_TURN_MSG)) if msg in line), None)
        if kind is None:
            continue
        fields = c.parse_line(line)
        rows.append({"kind": kind, "attempt_id": fields.get("attempt_id"),
                     "limiter": fields.get("limiter"), "wait_ms": fields.get("wait_ms"),
                     "held_ms": fields.get("held_ms")})
    return rows


def seed_reviews(case, cfg):
    """Three exams with pinned preparation and one saved (not submitted) attempt each.

    prepare_exam (offline) leaves the worker STOPPED: nothing is consumed while seeding.
    """
    actors = ACTORS[cfg["actors"]]
    base = REVISION_CASES[cfg["exam"]]
    seeded = []
    for index in range(1, cfg["reviews"] + 1):
        exam_cfg = dict(base, title=f"Concurrencia {index} de {cfg['reviews']}", prep_mode="offline")
        exam = f.prepare_exam(case, actors, exam_cfg)
        attempt_id = f.start_attempt(case, actors, exam)
        seeded.append({"assessment_id": exam["assessment_id"], "attempt_id": attempt_id,
                       "answers": len(exam["questions"])})
    case.ids["reviews"] = seeded
    return actors, seeded


def request_pdfs(case, cfg):
    """Uploads the short PDF twice and requests both generations; the worker is stopped."""
    school = m.SAN_IGNACIO
    token = m.teacher_token(school)
    pdf = m.resolve_pdf(cfg["pdf"])
    jobs = []
    for index in range(1, cfg["pdfs"] + 1):
        res = m.Result("C1", f"PDF {index}")
        material_id = m.upload_material(res, token, school, pdf, f"C1-{index}")
        status, body = m.request_generation(token, material_id)
        job_id = (body or {}).get("id") if isinstance(body, dict) else None
        if status != 202 or not job_id:
            raise c.ApiError(f"generate-assessment del PDF {index} → {status}: {body}")
        jobs.append({"material_id": material_id, "job_id": job_id})
    case.ids["jobs"] = jobs
    return jobs


def settled(reviews, jobs):
    for review in reviews:
        attempt = f.attempt_row(review["attempt_id"])
        pending = [r for r in f.answer_rows(review["attempt_id"])
                   if r["question_type"] in f.REVIEWABLE and r["review_status"] == "pending"]
        if attempt["status"] != "completed" and (pending or attempt["ai_review_claimed_at"]):
            return False
    for job in jobs:
        row = m.job_row(job["job_id"])
        if not (row["status"] == "failed" or (row["status"] == "done" and row["phase"] == 2)):
            return False
    return True


def case_c1(case, timeout_min=None):
    cfg = CASES["C1"]
    dead_before = dead_letter_depths()
    case.measure("dead_letters_before", dead_before)
    c.start_ollama()

    actors, reviews = seed_reviews(case, cfg)
    jobs = request_pdfs(case, cfg)
    for review in reviews:
        f.submit_attempt(case, actors, review["attempt_id"])
    case.measure("queued_before_worker", c.depths())

    # The window: the worker starts with the five messages already in its queues.
    started = time.time()
    c.start_worker(case.tail)
    timeout = (timeout_min or cfg["timeout_min"]) * 60
    done = c.poll_soft(lambda: settled(reviews, jobs), timeout, 5)
    wall = round(time.time() - started, 1)
    if not done:
        case.note(f"no todo quedó resuelto dentro del tope de {timeout // 60} min")
    for queue in c.WORK_QUEUES:
        c.wait_queue_empty(queue, 60)

    lines = case.tail.drain()
    expired = [ln for ln in lines if CLIENT_TIMEOUT_NEEDLE in ln]
    retries = [ln for ln in lines if RETRY_MSG in ln]
    case.check("ninguna llamada al modelo agota su tiempo en la ventana", 0, len(expired),
               evidence=expired[:5])
    case.check("el consumidor no reintenta ningún mensaje en la ventana", 0, len(retries),
               evidence=retries[:5])
    case.check("las tres colas muertas quedan como estaban", dead_before, dead_letter_depths())

    # Written down, never asserted: this is what the model produced (ADR 0048).
    case.measure("wall_seconds", wall)
    case.measure("baseline_no_cap", {"seconds": BASELINE_NO_CAP_SECONDS,
                                     "load": "2 PDF cortos y 1 revisión, sin tope (2026-11-25)"})
    case.measure("all_settled", bool(done))
    case.measure("answers", {r["attempt_id"]: [
        {"answer_id": row["answer_id"], "review_status": row["review_status"],
         "points_awarded": row["points_awarded"]} for row in f.answer_rows(r["attempt_id"])]
        for r in reviews})
    case.measure("attempts", {r["attempt_id"]: f.attempt_row(r["attempt_id"])["status"]
                              for r in reviews})
    case.measure("jobs", {j["job_id"]: {k: m.job_row(j["job_id"])[k]
                                        for k in ("status", "phase", "last_error")} for j in jobs})
    case.measure("turn_waits", turn_waits(lines))


FUNCTIONS = {"C1": case_c1}


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    parser.add_argument("--cases", default="C1", help="comma-separated; today only C1")
    parser.add_argument("--out-dir", help="required to run: one folder per run is created in it")
    parser.add_argument("--timeout-min", type=int, help="tope de la ventana, en minutos")
    parser.add_argument("--list", action="store_true", help="list the cases and exit")
    args = parser.parse_args(argv)
    if args.list:
        for name, cfg in CASES.items():
            mark = "modelo" if cfg["needs_model"] else "sin modelo"
            print(f"{name:5} {mark:10} {cfg['title']}")
        return e2e_env.EXIT_OK
    names = [n.strip() for n in args.cases.split(",") if n.strip()]
    unknown = [n for n in names if n not in FUNCTIONS]
    if unknown or not names:
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
        summary = c.run_case(name, cfg["title"],
                             lambda case, fn=FUNCTIONS[name]: fn(case, args.timeout_min),
                             run_dir, cfg["needs_model"])
        print(json.dumps(summary, ensure_ascii=False))
        summaries.append(summary)
    print(e2e_env.unmet_line(summaries))
    return e2e_env.exit_code(summaries)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
