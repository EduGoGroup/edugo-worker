#!/bin/zsh
# requeue-material.sh — re-encola el procesamiento de un material (pipeline 043)
# tras un fallo. Cubre los 3 escenarios reales (ver manual:
# docs/manuales/sistema/re-encolar-pipeline-material.md):
#
#   A) Job `processing` con mensaje en la DLQ (agotó los 3 reintentos):
#      re-inyecta el mensaje de la DLQ a la cola principal LIMPIANDO el header
#      x-retry-count (si no, el worker lo devuelve a la DLQ al primer tropiezo).
#      La reanudación (F3d) retoma en el primer chunk `pending` sin repetir `done`.
#
#   B) Job `failed` (error permanente): resetea el job a `processing` por SQL
#      (la API M2M no permite failed→processing) y luego hace (A) o re-publica.
#
#   C) Chunk envenenado (p. ej. H2: summary vacío determinista): con
#      --saltar-trozo SEQ lo marca `failed` para que la reanudación lo salte.
#      Úsalo ANTES de re-encolar si el mismo trozo tumbó el evento 3+ veces.
#
# Uso:
#   ./requeue-material.sh <job_id>                     # escenario A (y B si el job está failed)
#   ./requeue-material.sh <job_id> --saltar-trozo 12   # marca el trozo 12 failed y re-encola
#   ./requeue-material.sh <job_id> --solo-estado       # no toca nada, solo muestra
#
# Requiere: docker (BD local), curl, python3, y RABBITMQ_URL (env o .env.staging del worker).
set -euo pipefail

JOB_ID="${1:?uso: requeue-material.sh <job_id> [--saltar-trozo SEQ] [--solo-estado]}"
shift || true
SALTAR_SEQ=""; SOLO_ESTADO=0
while [ $# -gt 0 ]; do
  case "$1" in
    --saltar-trozo) SALTAR_SEQ="$2"; shift 2;;
    --solo-estado)  SOLO_ESTADO=1; shift;;
    *) echo "flag desconocido: $1"; exit 1;;
  esac
done

PSQL="docker exec edugo-migrator-postgres psql -U edugo -d edugo"
QUEUE="edugo.material.assessment.requested"
DLQ="$QUEUE.dlq"
EXCHANGE="edugo.materials"
RK="material.assessment_requested"

ENVF="$(dirname "$0")/../../.env.staging"
URL="${RABBITMQ_URL:-$(grep '^RABBITMQ_URL=' "$ENVF" | cut -d= -f2-)}"
U=$(echo "$URL" | sed -E 's|amqps?://([^:]+):.*|\1|'); P=$(echo "$URL" | sed -E 's|amqps?://[^:]+:([^@]+)@.*|\1|')
H=$(echo "$URL" | sed -E 's|amqps?://[^@]+@([^/]+)/.*|\1|'); V=$(echo "$URL" | sed -E 's|.*/([^/]+)$|\1|')
API="https://$H/api"

estado() {
  $=PSQL -tA -c "SELECT status FROM content.material_pipeline_job WHERE id='$JOB_ID';"
}

ST=$(estado)
[ -z "$ST" ] && { echo "job $JOB_ID no existe"; exit 1; }
echo ">> job $JOB_ID status=$ST"
$=PSQL -c "SELECT status, count(*) FROM content.material_pipeline_chunk WHERE job_id='$JOB_ID' GROUP BY status;"
DLQ_N=$(curl -s -u "$U:$P" "$API/queues/$V/$DLQ" | python3 -c "import sys,json; print(json.load(sys.stdin).get('messages',0))")
echo ">> mensajes en DLQ: $DLQ_N"
[ "$SOLO_ESTADO" = "1" ] && exit 0

# C) saltar trozo envenenado
if [ -n "$SALTAR_SEQ" ]; then
  echo ">> marcando trozo seq=$SALTAR_SEQ como failed (la reanudación lo salta)"
  $=PSQL -c "UPDATE content.material_pipeline_chunk SET status='failed', attempts=attempts+1, updated_at=now()
             WHERE job_id='$JOB_ID' AND seq=$SALTAR_SEQ AND status IN ('pending','processing') RETURNING seq, status;"
fi

# B) job failed → resetear a processing (la API M2M no lo permite; SQL directo)
if [ "$ST" = "failed" ]; then
  echo ">> job failed: reseteando a processing (y material a processing)"
  $=PSQL -c "UPDATE content.material_pipeline_job SET status='processing', last_error=NULL, updated_at=now() WHERE id='$JOB_ID';"
  $=PSQL -c "UPDATE content.materials SET status='processing', updated_at=now()
             WHERE id=(SELECT material_id FROM content.material_pipeline_job WHERE id='$JOB_ID');"
fi
if [ "$ST" = "done" ]; then
  # 044: done SIN assessment_id = fase 1 completa, la fase 2 (reduce) queda por correr;
  # done CON assessment_id = entregado de verdad, terminal.
  AID=$($=PSQL -tA -c "SELECT COALESCE(assessment_id::text,'') FROM content.material_pipeline_job WHERE id='$JOB_ID';")
  if [ -n "$AID" ]; then
    echo ">> el job ya entregó su evaluación (assessment_id=$AID): nada que re-encolar"; exit 0
  fi
  echo ">> job done de fase 1 sin assessment: re-encolando para la fase 2 (reduce, plan 044)"
fi

# A) re-inyectar desde la DLQ (o re-publicar el evento reconstruido si la DLQ está vacía)
if [ "$DLQ_N" -gt 0 ]; then
  echo ">> sacando 1 mensaje de la DLQ (ack definitivo) y re-publicando limpio"
  MSG=$(curl -s -u "$U:$P" -X POST "$API/queues/$V/$DLQ/get" \
        -H 'content-type: application/json' \
        -d '{"count":1,"ackmode":"ack_requeue_false","encoding":"auto"}')
  MSG_JSON="$MSG" python3 - "$API" "$V" "$EXCHANGE" "$RK" "$U" "$P" <<'PY'
import sys, os, json, subprocess
api, vhost, exchange, rk, user, pwd = sys.argv[1:7]
msgs = json.loads(os.environ["MSG_JSON"])
if not msgs:
    print("DLQ vacía al leer (carrera): reintenta"); sys.exit(1)
m = msgs[0]
props = m.get("properties") or {}
headers = props.get("headers") or {}
# CRÍTICO: limpiar el contador de reintentos y las marcas de muerte del DLX
for k in ("x-retry-count", "x-death", "x-first-death-reason", "x-first-death-queue", "x-first-death-exchange"):
    headers.pop(k, None)
props["headers"] = headers
body = {"properties": props, "routing_key": rk, "payload": m["payload"],
        "payload_encoding": m.get("payload_encoding", "string")}
r = subprocess.run(["curl","-s","-u",f"{user}:{pwd}","-X","POST",
                    f"{api}/exchanges/{vhost}/{exchange}/publish",
                    "-H","content-type: application/json","-d",json.dumps(body)],
                   capture_output=True, text=True)
out = json.loads(r.stdout or "{}")
print(">> publicado:", out)
sys.exit(0 if out.get("routed") else 2)
PY
else
  echo ">> DLQ vacía: reconstruyo el evento desde la BD (contrato v1.0 de edugo-shared,"
  echo ">>            events/material_assessment_requested.go — solo referencias, sin contenido)"
  REFS=$($=PSQL -tA -c "SELECT json_build_object('job_id', j.id, 'material_id', j.material_id, 'school_id', m.school_id)::text
                        FROM content.material_pipeline_job j JOIN content.materials m ON m.id=j.material_id
                        WHERE j.id='$JOB_ID';")
  REFS_JSON="$REFS" python3 - "$API" "$V" "$EXCHANGE" "$RK" "$U" "$P" <<'PY'
import sys, os, json, uuid, subprocess, datetime
api, vhost, exchange, rk, user, pwd = sys.argv[1:7]
refs = json.loads(os.environ["REFS_JSON"])
event = {
    "event_id": str(uuid.uuid4()),
    "event_type": "material.assessment_requested",
    "event_version": "1.0",
    "timestamp": datetime.datetime.now(datetime.timezone.utc).isoformat().replace("+00:00", "Z"),
    "payload": refs,
}
body = {"properties": {"content_type": "application/json"}, "routing_key": rk,
        "payload": json.dumps(event), "payload_encoding": "string"}
r = subprocess.run(["curl","-s","-u",f"{user}:{pwd}","-X","POST",
                    f"{api}/exchanges/{vhost}/{exchange}/publish",
                    "-H","content-type: application/json","-d",json.dumps(body)],
                   capture_output=True, text=True)
out = json.loads(r.stdout or "{}")
print(">> publicado:", out, "| event_id:", event["event_id"])
sys.exit(0 if out.get("routed") else 2)
PY
fi

echo ">> listo. Vigila el log del worker: debe decir 'job ya porcionado ... reanudación → fase 1'"
