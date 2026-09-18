#!/usr/bin/env bash
# Simule checkout.session.completed signé vers l’API locale.
# Usage : STRIPE_WEBHOOK_SECRET=whsec_test_local ./scripts/stripe-webhook-sim.sh domaine.fr email@x.fr
set -euo pipefail
DOMAIN="${1:?domaine}"
EMAIL="${2:?email}"
SECRET="${STRIPE_WEBHOOK_SECRET:-whsec_test_local}"
API="${HEBERNET_API:-http://127.0.0.1:8787}"
SID="cs_test_sim_$(date +%s)"
TS="$(date +%s)"

read -r PAYLOAD SIG <<EOF
$(SECRET="$SECRET" TS="$TS" SID="$SID" DOMAIN="$DOMAIN" EMAIL="$EMAIL" python3 - <<'PY'
import hmac, hashlib, json, os
secret = os.environ["SECRET"]
ts = os.environ["TS"]
payload = json.dumps({
  "id": f"evt_sim_{ts}",
  "type": "checkout.session.completed",
  "data": {"object": {
    "id": os.environ["SID"],
    "object": "checkout.session",
    "payment_status": "paid",
    "customer_email": os.environ["EMAIL"],
    "metadata": {"domain": os.environ["DOMAIN"], "app_type": "wordpress"},
  }},
}, separators=(",", ":"))
sig = hmac.new(secret.encode(), f"{ts}.{payload}".encode(), hashlib.sha256).hexdigest()
# single-line payload for bash read
print(payload.replace("\n", "") + "\t" + sig)
PY
)
EOF

curl -sS -X POST "$API/api/stripe/webhook" \
  -H "Content-Type: application/json" \
  -H "Stripe-Signature: t=${TS},v1=${SIG}" \
  -d "$PAYLOAD"
echo
