#!/usr/bin/env bash
# Lance l’API Hébernet avec les variables Stripe / mail.
# Usage :
#   1. Copier env.example → .env et remplir sk_test_ / whsec_
#   2. ./scripts/run-api.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

ENV_FILE="${HEBERNET_ENV_FILE:-$ROOT/.env}"
if [[ -f "$ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
  echo "env chargé : $ENV_FILE"
else
  echo "Pas de $ENV_FILE — utilisez les variables déjà exportées dans ce shell."
  echo "Modèle : cp env.example .env"
fi

: "${HEBERNET_CHECKOUT_DEMO:=0}"
: "${HEBERNET_VITRINE_URL:=http://127.0.0.1:8088}"
: "${HEBERNET_PANEL_URL:=http://127.0.0.1:5173}"
: "${HEBERNET_PRICE_CENTS:=100}"

if [[ -z "${STRIPE_SECRET_KEY:-}" ]]; then
  echo "ATTENTION : STRIPE_SECRET_KEY manquant → pas de sync Checkout / pas de vrai Stripe."
  echo "Ajoutez-le dans .env ou : export STRIPE_SECRET_KEY='sk_test_…'"
fi
if [[ -z "${STRIPE_WEBHOOK_SECRET:-}" ]]; then
  echo "ATTENTION : STRIPE_WEBHOOK_SECRET manquant (stripe listen)."
fi

mkdir -p data
# Toujours un agent frais sur le socket
if [[ -S data/agent.sock ]]; then
  if ! python3 - <<'PY' 2>/dev/null
import socket, json, sys
s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
s.settimeout(1)
try:
    s.connect("data/agent.sock")
    s.sendall((json.dumps({"id": "1", "op": "ping", "params": {}}) + "\n").encode())
    raw = s.recv(4096)
    sys.exit(0 if b'"ok":true' in raw or b'"ok": true' in raw else 1)
except Exception:
    sys.exit(1)
PY
  then
    echo "Agent socket mort — redémarrage…"
    pkill -f './bin/agent' 2>/dev/null || true
    sleep 0.3
    rm -f data/agent.sock
  fi
fi
if [[ ! -S data/agent.sock ]]; then
  echo "Démarrage agent…"
  ./bin/agent -dry-run -data data -socket data/agent.sock >data/agent.log 2>&1 &
  sleep 0.5
fi

exec ./bin/api \
  -addr "${HEBERNET_API_ADDR:-127.0.0.1:8787}" \
  -data data \
  -socket data/agent.sock \
  -db data/hebernet.db
