#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export PATH="$ROOT/.tools/go/bin:$PATH"

mkdir -p bin data
if [[ ! -x bin/hebernet-agent || ! -x bin/hebernet-api ]]; then
  echo "==> build"
  go build -o bin/hebernet-agent ./cmd/agent
  go build -o bin/hebernet-api ./cmd/api
fi

pkill -f 'hebernet-agent' 2>/dev/null || true
pkill -f 'hebernet-api' 2>/dev/null || true
sleep 0.3
rm -f data/agent.sock

echo "==> agent (dry-run)"
./bin/hebernet-agent -socket data/agent.sock -data data -dry-run=true &
AGENT_PID=$!

echo "==> api"
./bin/hebernet-api -addr 127.0.0.1:8787 -db data/hebernet.db -socket data/agent.sock -seed -seed-demo &
API_PID=$!

cleanup() {
  kill "$AGENT_PID" "$API_PID" 2>/dev/null || true
}
trap cleanup EXIT

sleep 0.5
curl -sf http://127.0.0.1:8787/api/health >/dev/null

echo "==> ui (vite)"
cd ui
if [[ ! -d node_modules ]]; then
  npm install
fi
echo ""
echo "  Ouvrir http://127.0.0.1:5173/"
echo "  admin@hebernet.local / admin"
echo "  client@hebernet.local / client"
echo ""
npm run dev -- --host 127.0.0.1 --port 5173
