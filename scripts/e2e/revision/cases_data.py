"""The cases as data: actors, questions, student answers and pinned `llm_prep` artifacts.

Identifiers come from the seed `playground_v2/base` (read, not executed, on 2026-11-24).
"""

SAN_IGNACIO = "b1000000-0000-0000-0000-000000000001"
GLOBAL_ENGLISH = "b3000000-0000-0000-0000-000000000003"

# One (teacher, student, subject, offering) per school and unit. `prof.martinez` and
# `est.carlos` belong to both schools: the context is always sent on the exchange.
ACTORS = {
    # Science, 5to B: est.diego is enrolled (period ff…01, is_active).
    "science_5b": {
        "school": SAN_IGNACIO, "unit": "ac000000-0000-0000-0000-000000000004",
        "teacher": "prof.gonzalez@edugo.test", "student": "est.diego@edugo.test",
        "student_membership": "bb000000-0000-0000-0000-000000000004",
        "subject": "dd000000-0000-0000-0000-000000000002",
        "offering": "c5000000-0000-0000-0000-000000000004"},
    # Math, 5to A: est.carlos is enrolled (period ff…01, is_active). Used by R6 and R7.
    "math_5a": {
        "school": SAN_IGNACIO, "unit": "ac000000-0000-0000-0000-000000000003",
        "teacher": "prof.martinez@edugo.test", "student": "est.carlos@edugo.test",
        "student_membership": "bb000000-0000-0000-0000-000000000001",
        "subject": "dd000000-0000-0000-0000-000000000001",
        "offering": "c5000000-0000-0000-0000-000000000001"},
    # Academia Global English, Class Monday: no school_settings rows (AI off). Used by R8, R13.
    "english_a2": {
        "school": GLOBAL_ENGLISH, "unit": "ac000000-0000-0000-0000-000000000014",
        "teacher": "prof.martinez@edugo.test", "student": "est.carlos@edugo.test",
        "student_membership": "bb000000-0000-0000-0000-000000000002",
        "subject": "dd000000-0000-0000-0000-000000000007",
        "offering": "c5000000-0000-0000-0000-000000000002"},
}

SHORT, OPEN = "short_answer", "open_ended"


def short(key, text, correct, answer, points=2, prep=None, spec=None):
    body = {"question_type": SHORT, "question_text": text, "correct_answer": correct,
            "points": points}
    if spec:
        body["answer_spec"] = spec
    return {"key": key, "body": body, "answer": answer, "prep": prep}


def open_q(key, text, answer, expected=None, rubric=None, points=4, prep=None):
    body = {"question_type": OPEN, "question_text": text, "points": points}
    if expected is not None:
        body["correct_answer"] = expected
    if rubric is not None:
        body["explanation"] = rubric
    return {"key": key, "body": body, "answer": answer, "prep": prep}


SKIP = object()  # the student never saves an answer for this question

GRAN_COLOMBIA_TEXT = "Nombra los tres países que formaron la Gran Colombia"
GRAN_COLOMBIA_EXPECTED = "Colombia, Venezuela y Ecuador"
GRAN_COLOMBIA_PREP = {
    "version": 1, "question_type": SHORT, "content_kind": "list",
    "items": ["colombia", "venezuela", "ecuador"],
    "items_verbatim": ["Colombia", "Venezuela", "Ecuador"]}

WATER_TEXT = "Explica el ciclo del agua"
WATER_EXPECTED = "Evaporación, condensación, precipitación y escorrentía."
WATER_RUBRIC = "Menciona la evaporación, la condensación y la precipitación."
WATER_PREP = {
    "version": 1, "question_type": OPEN,
    "question_intent": "Mide si conoce las etapas del ciclo del agua.",
    "main_ideas": ["evaporación", "condensación", "precipitación"],
    "criteria": ["Menciona la evaporación", "Menciona la condensación",
                 "Menciona la precipitación"]}
WATER_GOOD = ("El sol calienta el agua de mares y ríos y la evapora. El vapor sube, se enfría y "
              "se condensa formando nubes. Cuando las gotas pesan, caen como lluvia o nieve: "
              "eso es la precipitación, y el agua vuelve a los ríos y al mar.")
WATER_PARTIAL = "El agua se evapora con el calor del sol y sube al cielo."
WATER_BAD = "El agua sale del grifo porque la manda la empresa de agua."

CASES = {
    "P1": {"title": "Preparación: crear una corta y una abierta en San Ignacio",
           "actors": "science_5b",
           "questions": [
               short("corta", "¿Cuál es la capital del Perú? (P1)", "Lima", None),
               open_q("abierta", "Explica con tus palabras por qué llueve. (P1)", None,
                      expected="El vapor de agua se condensa en nubes y cae como lluvia.",
                      rubric="Menciona la condensación y la precipitación.")]},
    "P2": {"title": "Preparación: backfill interno con límite", "limit": 3},
    "P3": {"title": "Preparación: reentrega de una ya preparada y 409 por hash viejo",
           "actors": "science_5b",
           "questions": [short("corta", "¿Cuál es el planeta más cercano al Sol? (P3)",
                               "Mercurio", None)],
           "stale_hash": "0" * 64},
    "R1": {"title": "Revisión: corta con la lista fijada, tres respuestas",
           "actors": "science_5b", "prep_mode": "offline",
           "questions": [
               short("a_falta_un_item", GRAN_COLOMBIA_TEXT + " (a).", GRAN_COLOMBIA_EXPECTED,
                     "Colombia y Venezuela", prep=GRAN_COLOMBIA_PREP),
               short("b_otra_cosa", GRAN_COLOMBIA_TEXT + " (b).", GRAN_COLOMBIA_EXPECTED,
                     "Colombia, Venezuela y Quito", prep=GRAN_COLOMBIA_PREP),
               short("c_conjunto_exacto", GRAN_COLOMBIA_TEXT + " (c).", GRAN_COLOMBIA_EXPECTED,
                     "Colombia, Venezuela y Ecuador", prep=GRAN_COLOMBIA_PREP)]},
    "R2": {"title": "Revisión: corta sin lista fijada (el control del campo)",
           "actors": "science_5b", "prep_mode": "model",
           "questions": [
               short("otro_genero", "¿De qué color es la leche? (R2)", "blanco", "blanca"),
               short("mala", "¿Qué planeta es el más cercano al Sol? (R2)", "Mercurio", "Venus")]},
    "R3": {"title": "Revisión: abierta con los criterios fijados (buena, parcial y mala)",
           "actors": "science_5b", "prep_mode": "offline",
           "questions": [
               open_q("buena", WATER_TEXT + " (a).", WATER_GOOD, WATER_EXPECTED, WATER_RUBRIC,
                      prep=WATER_PREP),
               open_q("parcial", WATER_TEXT + " (b).", WATER_PARTIAL, WATER_EXPECTED, WATER_RUBRIC,
                      prep=WATER_PREP),
               open_q("mala", WATER_TEXT + " (c).", WATER_BAD, WATER_EXPECTED, WATER_RUBRIC,
                      prep=WATER_PREP)]},
    "R4": {"title": "Revisión: abierta sin preparación (juicio global)",
           "actors": "science_5b", "prep_mode": "offline",
           "questions": [open_q("sin_prep", WATER_TEXT + " (R4).", WATER_PARTIAL, WATER_EXPECTED,
                                WATER_RUBRIC)]},
    "R5": {"title": "Revisión: respuesta en blanco, por regla y sin modelo",
           "actors": "science_5b", "prep_mode": "offline",
           "questions": [
               short("corta_vacia", "¿Cuál es la capital de Chile? (R5 a)", "Santiago", ""),
               short("corta_puntos", "¿Cuál es la capital de Bolivia? (R5 b)", "Sucre", "..."),
               open_q("abierta_vacia", WATER_TEXT + " (R5 c).", "", rubric=WATER_RUBRIC),
               open_q("abierta_puntos", WATER_TEXT + " (R5 d).", "...", expected=WATER_EXPECTED),
               open_q("abierta_saltada", WATER_TEXT + " (R5 e).", SKIP, rubric=WATER_RUBRIC)]},
    "R6": {"title": "Revisión: intento mixto (abierta y corta), T5a del plan 040",
           "actors": "math_5a", "prep_mode": "model",
           "questions": [
               open_q("abierta", "Explica qué es un número primo y da dos ejemplos. (R6)",
                      "Es un número que solo se divide entre 1 y él mismo, como el 2 y el 7.",
                      expected="Un número mayor que 1 con solo dos divisores: 1 y él mismo. "
                               "Ejemplos: 2, 3, 5, 7.",
                      rubric="Define primo y da dos ejemplos correctos."),
               short("corta", "¿Cómo se llama el resultado de una multiplicación? (R6)",
                     "producto", "el resultado de multiplicar")]},
    "R7": {"title": "Revisión: flow=direct con solo abiertas pendientes",
           "actors": "math_5a", "prep_mode": "offline",
           "questions": [
               open_q("abierta_1", "Explica qué es una fracción. (R7 a)",
                      "Es una parte de un entero, con numerador y denominador.",
                      expected="Una parte de un todo dividido en partes iguales.",
                      rubric="Menciona el todo y las partes iguales."),
               open_q("abierta_2", "Explica qué es un número par. (R7 b)",
                      "Los que terminan en 1, 3 o 5.",
                      expected="Un número divisible entre 2.",
                      rubric="Menciona la división exacta entre 2.")]},
    "R7n": {"title": "Revisión: flow=direct con solo abiertas en blanco (sin modelo)",
            "actors": "math_5a", "prep_mode": "offline",
            "questions": [
                open_q("abierta_vacia", "Explica qué es una fracción. (R7n a)", "",
                       expected="Una parte de un todo dividido en partes iguales.",
                       rubric="Menciona el todo y las partes iguales."),
                open_q("abierta_puntos", "Explica qué es un número par. (R7n b)", "...",
                       expected="Un número divisible entre 2.",
                       rubric="Menciona la división exacta entre 2.")]},
    "R8": {"title": "Revisión: escuela con la IA apagada (Academia Global English)",
           "actors": "english_a2", "prep_mode": "offline",
           "questions": [open_q("abierta", "Describe your daily routine in three sentences. (R8)",
                                "I wake up at seven. I go to school. I play football.",
                                rubric="Three sentences in present simple.")]},
    "R9": {"title": "Revisión: la docente puntúa antes que la IA",
           "actors": "science_5b", "prep_mode": "offline",
           "teacher_points": 1, "teacher_feedback": "Puntuada por la docente antes que la IA.",
           "questions": [
               open_q("la_puntua_la_docente", WATER_TEXT + " (R9 a).", WATER_PARTIAL,
                      WATER_EXPECTED, WATER_RUBRIC),
               open_q("la_revisa_la_ia", WATER_TEXT + " (R9 b).", WATER_BAD, WATER_EXPECTED,
                      WATER_RUBRIC)]},
    "R10": {"title": "Revisión: evento repetido de un intento ya revisado",
            "actors": "science_5b", "prep_mode": "offline",
            "questions": [open_q("abierta", WATER_TEXT + " (R10).", WATER_PARTIAL, WATER_EXPECTED,
                                 WATER_RUBRIC)]},
    "R11": {"title": "Revisión: el candado ya está tomado",
            "actors": "science_5b", "prep_mode": "offline",
            "questions": [open_q("abierta", WATER_TEXT + " (R11).", WATER_PARTIAL, WATER_EXPECTED,
                                 WATER_RUBRIC)]},
    "R12": {"title": "Revisión: el modelo no responde (Ollama parado antes de entregar)",
            "actors": "science_5b", "prep_mode": "offline",
            "questions": [open_q("abierta", WATER_TEXT + " (R12).", WATER_PARTIAL, WATER_EXPECTED,
                                 WATER_RUBRIC)]},
    "R12n": {"title": "Revisión: el modelo no responde, hasta la cola muerta (sin modelo)",
             "actors": "science_5b", "prep_mode": "offline",
             "questions": [open_q("abierta", WATER_TEXT + " (R12n).", WATER_PARTIAL,
                                  WATER_EXPECTED, WATER_RUBRIC)]},
    "R12b": {"title": "Revisión (opcional): Ollama parado a mitad de un intento de varias abiertas",
             "actors": "science_5b", "prep_mode": "offline",
             "questions": [open_q(f"abierta_{i}", f"{WATER_TEXT} (R12b {i}).", WATER_GOOD,
                                  WATER_EXPECTED, WATER_RUBRIC) for i in range(1, 5)]},
    "R13": {"title": "Revisión: tipo de evento desconocido y JSON roto",
            "actors": "english_a2",
            "probe_question": open_q("sonda", "Write one sentence about your school. (R13)", None,
                                     rubric="One correct sentence.")},
}

ORDER = ["P1", "P2", "P3", "R1", "R2", "R3", "R4", "R5", "R6", "R7", "R7n", "R8", "R9", "R10",
         "R11", "R12", "R12n", "R12b", "R13"]

# The cases whose green does not depend on what a model writes (ADR 0048): the only ones
# that can be a gate. Every other case is measured. `needs_model` is set on each case.
NO_MODEL = ("R5", "R7n", "R8", "R11", "R12n", "R13")
for _name, _cfg in CASES.items():
    _cfg["needs_model"] = _name not in NO_MODEL
