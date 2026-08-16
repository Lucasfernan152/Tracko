#!/usr/bin/env bash
set -euo pipefail

BROKER="${BROKER:-localhost}"
PORT="${PORT:-1883}"
DRIVER_ID="${DRIVER_ID:-driver-123}"
TRIP_ID="${TRIP_ID:-}"
API_BASE="${API_BASE:-http://localhost:8080}"
ORIGIN_BRANCH="${ORIGIN_BRANCH:-Downtown Branch}"
DEST_BRANCH="${DEST_BRANCH:-North Branch}"
INTERVAL_MS="${INTERVAL_MS:-800}"
STEPS_PER_SEGMENT="${STEPS_PER_SEGMENT:-8}"
LOOPS="${LOOPS:-1}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --broker) BROKER="$2"; shift 2 ;;
    --port) PORT="$2"; shift 2 ;;
    --driver) DRIVER_ID="$2"; shift 2 ;;
    --trip) TRIP_ID="$2"; shift 2 ;;
    --api-base) API_BASE="$2"; shift 2 ;;
    --origin) ORIGIN_BRANCH="$2"; shift 2 ;;
    --destination) DEST_BRANCH="$2"; shift 2 ;;
    --interval-ms) INTERVAL_MS="$2"; shift 2 ;;
    --steps) STEPS_PER_SEGMENT="$2"; shift 2 ;;
    --loops) LOOPS="$2"; shift 2 ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
done

if ! command -v mosquitto_pub >/dev/null 2>&1; then
  echo "mosquitto_pub was not found in PATH. Install Mosquitto and try again." >&2
  exit 1
fi

if [[ -z "$TRIP_ID" ]]; then
  TRIP_ID="$(curl -sS -X POST "${API_BASE}/api/trips" \
    -H "Content-Type: application/json" \
    -d "{\"metadata\":{\"origin_branch\":\"${ORIGIN_BRANCH}\",\"destination_branch\":\"${DEST_BRANCH}\",\"cargo\":\"General cargo\"}}" \
    | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')"
  echo "1) Business created the trip: ${TRIP_ID}"

  curl -sS -X PATCH "${API_BASE}/api/trips/${TRIP_ID}" \
    -H "Content-Type: application/json" \
    -d "{\"driver_id\":\"${DRIVER_ID}\"}" >/dev/null
  echo "2) Business assigned driver: ${DRIVER_ID}"

  curl -sS -X PATCH "${API_BASE}/api/trips/${TRIP_ID}" \
    -H "Content-Type: application/json" \
    -d '{"status":"in_progress"}' >/dev/null
  echo "3) Driver started the trip"
fi

TOPIC="trips/${TRIP_ID}/location"
SLEEP_SECS="$(awk -v ms="$INTERVAL_MS" 'BEGIN { printf "%.3f", ms / 1000 }')"

echo "Publishing Buenos Aires route"
echo "Broker: ${BROKER}:${PORT}  Topic: ${TOPIC}  Loops: ${LOOPS}"
echo

heading() {
  awk -v lat1="$1" -v lon1="$2" -v lat2="$3" -v lon2="$4" 'BEGIN {
    pi = atan2(0, -1)
    lat1 *= pi / 180
    lat2 *= pi / 180
    dlon = (lon2 - lon1) * pi / 180
    y = sin(dlon) * cos(lat2)
    x = cos(lat1) * sin(lat2) - sin(lat1) * cos(lat2) * cos(dlon)
    h = atan2(y, x) * 180 / pi
    if (h < 0) h += 360
    printf "%.1f", h
  }'
}

lerp() {
  awk -v a="$1" -v b="$2" -v t="$3" 'BEGIN { printf "%.6f", a + (b - a) * t }'
}

ROUTE=(
  "Obelisco|-34.6037|-58.3816|28"
  "9 de Julio y Lavalle|-34.6020|-58.3814|35"
  "9 de Julio y Cordoba|-34.5990|-58.3812|40"
  "Plaza San Martin|-34.5954|-58.3746|32"
  "Retiro|-34.5912|-58.3748|30"
  "Av. Libertador|-34.5885|-58.3705|45"
  "Puerto Madero Norte|-34.5985|-58.3668|38"
  "Puente de la Mujer|-34.6083|-58.3656|25"
  "Puerto Madero Sur|-34.6148|-58.3648|33"
  "Casa Rosada|-34.6081|-58.3703|22"
  "Plaza de Mayo|-34.6083|-58.3712|20"
  "Av. de Mayo|-34.6086|-58.3785|36"
  "Congreso|-34.6098|-58.3925|24"
  "Callao y Corrientes|-34.6038|-58.3922|34"
  "Corrientes y Uruguay|-34.6037|-58.3865|38"
  "Obelisco|-34.6037|-58.3816|30"
)

for ((loop=1; loop<=LOOPS; loop++)); do
  echo "=== Lap ${loop}/${LOOPS} ==="

  for ((i=0; i<${#ROUTE[@]}-1; i++)); do
    IFS='|' read -r from_name from_lat from_lng from_speed <<< "${ROUTE[$i]}"
    IFS='|' read -r to_name to_lat to_lng to_speed <<< "${ROUTE[$((i+1))]}"
    hdg="$(heading "$from_lat" "$from_lng" "$to_lat" "$to_lng")"

    echo "  ${from_name} -> ${to_name}"

    for ((step=0; step<=STEPS_PER_SEGMENT; step++)); do
      t="$(awk -v s="$step" -v n="$STEPS_PER_SEGMENT" 'BEGIN { printf "%.6f", s / n }')"
      lat="$(lerp "$from_lat" "$to_lat" "$t")"
      lng="$(lerp "$from_lng" "$to_lng" "$t")"
      speed="$(awk -v a="$from_speed" -v b="$to_speed" -v t="$t" 'BEGIN { printf "%.1f", a + (b - a) * t }')"
      timestamp="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

      payload="$(printf '{"latitude":%s,"longitude":%s,"speed":%s,"heading":%s,"timestamp":"%s"}' \
        "$lat" "$lng" "$speed" "$hdg" "$timestamp")"

      mosquitto_pub -h "$BROKER" -p "$PORT" -t "$TOPIC" -m "$payload"
      sleep "$SLEEP_SECS"
    done
  done
done

curl -sS -X PATCH "${API_BASE}/api/trips/${TRIP_ID}" \
  -H "Content-Type: application/json" \
  -d '{"status":"completed","metadata":{"result":"delivered"}}' >/dev/null

echo
echo "Route finished. Trip ID: ${TRIP_ID}"
