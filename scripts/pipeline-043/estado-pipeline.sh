#!/bin/zsh
# estado-pipeline.sh — radiografía de un job del pipeline material→evaluación (plan 043).
#
# Uso:
#   ./estado-pipeline.sh                # último job creado
#   ./estado-pipeline.sh <job_id>       # job específico
#
# Requiere: docker con el contenedor edugo-migrator-postgres (BD local dev).
# Muestra: job (status/phase/last_error), conteo de chunks por estado, chunks failed
# con sus primeros caracteres (para reconocer trozos envenenados tipo H2), candidatas,
# y mensajes en la DLQ de CloudAMQP (si hay credenciales).
set -euo pipefail

PSQL="docker exec edugo-migrator-postgres psql -U edugo -d edugo"

JOB_ID="${1:-}"
if [ -z "$JOB_ID" ]; then
  JOB_ID=$($=PSQL -tA -c "SELECT id FROM content.material_pipeline_job ORDER BY created_at DESC LIMIT 1;")
  echo ">> Sin job_id: uso el último creado: $JOB_ID"
fi

echo "== JOB =="
$=PSQL -c "SELECT j.id, j.status, j.phase, j.assessment_id, LEFT(COALESCE(j.last_error,'—'),80) AS last_error, m.status AS material_status, m.id AS material_id
FROM content.material_pipeline_job j JOIN content.materials m ON m.id=j.material_id WHERE j.id='$JOB_ID';"

echo "== CHUNKS por estado =="
$=PSQL -c "SELECT status, count(*), max(attempts) AS max_attempts FROM content.material_pipeline_chunk WHERE job_id='$JOB_ID' GROUP BY status ORDER BY status;"

echo "== CHUNKS failed (¿envenenados H2?) =="
$=PSQL -c "SELECT seq, attempts, LEFT(regexp_replace(chunk_text,'\s+',' ','g'),70) AS inicio_texto
FROM content.material_pipeline_chunk WHERE job_id='$JOB_ID' AND status='failed' ORDER BY seq;"

echo "== CANDIDATAS =="
$=PSQL -c "SELECT status, count(*) FROM content.material_pipeline_candidate c
JOIN content.material_pipeline_chunk ch ON ch.id=c.chunk_id WHERE ch.job_id='$JOB_ID' GROUP BY c.status;" 2>/dev/null || \
$=PSQL -c "SELECT count(*) AS candidatas FROM content.material_pipeline_candidate;"

# DLQ (best-effort: necesita RABBITMQ_URL con credenciales de CloudAMQP)
ENVF="$(dirname "$0")/../../.env.staging"
URL="${RABBITMQ_URL:-$(grep '^RABBITMQ_URL=' "$ENVF" 2>/dev/null | cut -d= -f2- || true)}"
if [ -n "${URL:-}" ]; then
  U=$(echo "$URL" | sed -E 's|amqps?://([^:]+):.*|\1|'); P=$(echo "$URL" | sed -E 's|amqps?://[^:]+:([^@]+)@.*|\1|')
  H=$(echo "$URL" | sed -E 's|amqps?://[^@]+@([^/]+)/.*|\1|'); V=$(echo "$URL" | sed -E 's|.*/([^/]+)$|\1|')
  echo "== DLQ (edugo.material.assessment.requested.dlq) =="
  curl -s -u "$U:$P" "https://$H/api/queues/$V/edugo.material.assessment.requested.dlq" \
    | python3 -c "import sys,json; d=json.load(sys.stdin); print('mensajes:', d.get('messages','?'))" || echo "(management API no accesible)"
fi
