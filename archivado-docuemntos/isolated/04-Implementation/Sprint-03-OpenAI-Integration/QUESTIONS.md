# Preguntas Sprint 03

## Q001: ¿Qué modelo de OpenAI usar?
**Decisión:** **gpt-4-turbo-preview**
**Justificación:** Balance calidad/costo/velocidad

## Q002: ¿Temperature?
**Decisión:** **0.3**
**Justificación:** Determinístico pero con variación

## Q003: ¿Retry si falla OpenAI?
**Decisión:** **Sí, 5 intentos con backoff exponencial**
