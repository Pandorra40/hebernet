#!/usr/bin/env bash
# Hébernet — installation Debian 12 / Ubuntu 24.04+
#
# Depuis le dépôt déjà présent sur la machine :
#   sudo ./scripts/install.sh
#
# Une ligne depuis la machine de dév (Ubuntu Server neuf = rien d’installé) :
#   scp -r hebernet vitrine user@vm:~/
#   ssh user@vm 'cd hebernet && sudo ./scripts/install.sh'
#
# Le script pose : paquets, agent, API, panneau (:80), vitrine (:8088), systemd.
# Plus tard (quand le dépôt est public) :
#   curl -fsSL https://raw.githubusercontent.com/…/install.sh | sudo bash
#
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PREFIX="${HEBERNET_PREFIX:-/usr/local}"
DATA_DIR="${HEBERNET_DATA_DIR:-/var/lib/hebernet}"
ETC_DIR="${HEBERNET_ETC_DIR:-/etc/hebernet}"
WWW_DIR="${HEBERNET_WWW_DIR:-/var/www/hebernet}"
VITRINE_WWW="${HEBERNET_VITRINE_WWW:-/var/www/vitrine}"
GO_VERSION="${HEBERNET_GO_VERSION:-1.25.0}"
SKIP_DEPS="${HEBERNET_SKIP_DEPS:-0}"
SKIP_UI="${HEBERNET_SKIP_UI:-0}"
SKIP_VITRINE="${HEBERNET_SKIP_VITRINE:-0}"
SKIP_NGINX="${HEBERNET_SKIP_NGINX:-0}"

log()  { printf '\n==> %s\n' "$*"; }
die()  { printf 'erreur: %s\n' "$*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "commande manquante: $1"; }

[[ "$(id -u)" -eq 0 ]] || die "lancez en root : sudo $0"

if [[ -r /etc/os-release ]]; then
  # shellcheck disable=SC1091
  . /etc/os-release
else
  die "impossible de détecter la distro (/etc/os-release)"
fi

case "${ID:-}:${VERSION_ID:-}" in
  debian:12*|debian:13*|ubuntu:24.*|ubuntu:25.*|ubuntu:26.*) ;;
  *)
    printf 'Attention: cible prévue = Debian 12+/Ubuntu 24.04+ (détecté: %s %s).\n' "${ID:-?}" "${VERSION_ID:-?}"
    printf 'Continuer quand même ? [y/N] '
    read -r ans
    [[ "${ans:-}" =~ ^[yY]$ ]] || exit 1
    ;;
esac

[[ -f "$ROOT/go.mod" && -d "$ROOT/cmd/agent" ]] || die "dépôt Hébernet introuvable dans $ROOT"

# --- dépendances système ----------------------------------------------------
if [[ "$SKIP_DEPS" != "1" ]]; then
  log "Paquets système"
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq
  apt-get install -y -qq \
    ca-certificates curl git build-essential rsync unzip \
    nginx mariadb-server \
    certbot python3-certbot-nginx \
    quota \
    sqlite3
  # Laravel one-click (optionnel)
  apt-get install -y -qq composer 2>/dev/null || log "composer non disponible via apt — Laravel = page stub"
  # PHP-FPM 8.5 si disponible, sinon le plus récent du dépôt
  if apt-cache show php8.5-fpm >/dev/null 2>&1; then
    apt-get install -y -qq php8.5-fpm php8.5-cli php8.5-mysql php8.5-xml php8.5-mbstring php8.5-curl php8.5-zip php8.5-sqlite3 php8.5-gd php8.5-intl
  elif apt-cache show php8.4-fpm >/dev/null 2>&1; then
    log "php8.5 indisponible — installation de php8.4-fpm (adaptez les pools si besoin)"
    apt-get install -y -qq php8.4-fpm php8.4-cli php8.4-mysql php8.4-xml php8.4-mbstring php8.4-curl php8.4-zip php8.4-sqlite3 php8.4-gd php8.4-intl
  elif apt-cache show php8.3-fpm >/dev/null 2>&1; then
    log "php8.5 indisponible — installation de php8.3-fpm (adaptez les pools si besoin)"
    apt-get install -y -qq php8.3-fpm php8.3-cli php8.3-mysql php8.3-xml php8.3-mbstring php8.3-curl php8.3-zip php8.3-sqlite3 php8.3-gd php8.3-intl
  else
    log "Aucun paquet php*-fpm trouvé — installez PHP-FPM manuellement"
  fi
fi

# --- Go ---------------------------------------------------------------------
ensure_go() {
  if command -v go >/dev/null 2>&1; then
    local ver
    ver="$(go env GOVERSION 2>/dev/null || go version | awk '{print $3}')"
    log "Go déjà présent: $ver"
    return 0
  fi
  log "Installation Go ${GO_VERSION}"
  local arch tarball
  case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) die "architecture non supportée: $(uname -m)" ;;
  esac
  tarball="go${GO_VERSION}.linux-${arch}.tar.gz"
  curl -fsSL "https://go.dev/dl/${tarball}" -o "/tmp/${tarball}"
  rm -rf /usr/local/go
  tar -C /usr/local -xzf "/tmp/${tarball}"
  rm -f "/tmp/${tarball}"
  export PATH="/usr/local/go/bin:$PATH"
}
ensure_go
export PATH="/usr/local/go/bin:${PATH:-}"
need go

# --- Node (UI) --------------------------------------------------------------
ensure_node() {
  if command -v node >/dev/null 2>&1 && command -v npm >/dev/null 2>&1; then
    log "Node déjà présent: $(node -v)"
    return 0
  fi
  log "Installation Node.js 22 (NodeSource)"
  curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
  apt-get install -y -qq nodejs
}
if [[ "$SKIP_UI" != "1" ]]; then
  ensure_node
  need npm
fi

# --- utilisateur / répertoires ----------------------------------------------
log "Utilisateur et répertoires"
if ! getent group hebernet >/dev/null; then
  groupadd --system hebernet
fi
if ! id hebernet >/dev/null 2>&1; then
  useradd --system --gid hebernet --home-dir "$DATA_DIR" --shell /usr/sbin/nologin hebernet
fi
install -d -m 0750 -o hebernet -g hebernet "$DATA_DIR"
install -d -m 0750 -o root -g hebernet "$ETC_DIR"
install -d -m 0755 -o root -g root "$WWW_DIR"

# --- build ------------------------------------------------------------------
log "Compilation agent + API"
cd "$ROOT"
make build
install -m 0755 "$ROOT/bin/hebernet-agent" "$PREFIX/sbin/hebernet-agent"
install -m 0755 "$ROOT/bin/hebernet-api"   "$PREFIX/bin/hebernet-api"

if [[ "$SKIP_UI" != "1" ]]; then
  log "Build UI (panneau)"
  (cd "$ROOT/ui" && npm ci --silent && npm run build)
  rsync -a --delete "$ROOT/ui/dist/" "$WWW_DIR/"
  chown -R root:root "$WWW_DIR"
fi

# Vitrine (dossier frère ../vitrine, ou hebernet/vitrine, ou HEBERNET_VITRINE_SRC)
VITRINE_SRC="${HEBERNET_VITRINE_SRC:-}"
if [[ -z "$VITRINE_SRC" ]]; then
  if [[ -f "$ROOT/../vitrine/index.html" ]]; then
    VITRINE_SRC="$(cd "$ROOT/../vitrine" && pwd)"
  elif [[ -f "$ROOT/vitrine/index.html" ]]; then
    VITRINE_SRC="$ROOT/vitrine"
  fi
fi

if [[ "$SKIP_VITRINE" != "1" ]]; then
  if [[ -n "${VITRINE_SRC:-}" && -f "$VITRINE_SRC/index.html" ]]; then
    log "Déploiement vitrine → $VITRINE_WWW (depuis $VITRINE_SRC)"
    install -d -m 0755 -o root -g root "$VITRINE_WWW"
    rsync -a --delete "$VITRINE_SRC/" "$VITRINE_WWW/"
    chown -R root:root "$VITRINE_WWW"
  else
    log "Vitrine introuvable — copiez le dossier vitrine à côté de hebernet (scp -r hebernet vitrine …)"
    log "  attendu : $ROOT/../vitrine/index.html"
  fi
fi

# --- config -----------------------------------------------------------------
HOST_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
HOST_IP="${HOST_IP:-127.0.0.1}"

if [[ ! -f "$ETC_DIR/api.env" ]]; then
  log "Création $ETC_DIR/api.env"
  if [[ -f "$ROOT/env.example" ]]; then
    install -m 0640 -o root -g hebernet "$ROOT/env.example" "$ETC_DIR/api.env"
  else
    cat >"$ETC_DIR/api.env" <<EOF
HEBERNET_CHECKOUT_DEMO=1
HEBERNET_PRICE_CENTS=100
HEBERNET_PANEL_URL=http://${HOST_IP}
HEBERNET_VITRINE_URL=http://${HOST_IP}:8088
# STRIPE_SECRET_KEY=
# STRIPE_WEBHOOK_SECRET=
# RESEND_API_KEY=
# HEBERNET_MAIL_FROM=
EOF
    chown root:hebernet "$ETC_DIR/api.env"
    chmod 0640 "$ETC_DIR/api.env"
  fi
  sed -i "s|HEBERNET_PANEL_URL=.*|HEBERNET_PANEL_URL=http://${HOST_IP}|" "$ETC_DIR/api.env" || true
  sed -i "s|HEBERNET_VITRINE_URL=.*|HEBERNET_VITRINE_URL=http://${HOST_IP}:8088|" "$ETC_DIR/api.env" || true
else
  log "Conservé: $ETC_DIR/api.env (déjà présent)"
fi

# --- systemd ----------------------------------------------------------------
log "Unités systemd"
install -m 0644 "$ROOT/deploy/hebernet-agent.service" /etc/systemd/system/hebernet-agent.service
install -m 0644 "$ROOT/deploy/hebernet-api.service"   /etc/systemd/system/hebernet-api.service
# Ajuster chemins si PREFIX non standard
if [[ "$PREFIX" != "/usr/local" ]]; then
  sed -i "s|/usr/local/sbin/hebernet-agent|${PREFIX}/sbin/hebernet-agent|" /etc/systemd/system/hebernet-agent.service
  sed -i "s|/usr/local/bin/hebernet-api|${PREFIX}/bin/hebernet-api|" /etc/systemd/system/hebernet-api.service
fi
systemctl daemon-reload
systemctl enable --now hebernet-agent.service
systemctl enable --now hebernet-api.service

# --- quotas disque (best effort) --------------------------------------------
if [[ "${HEBERNET_SKIP_QUOTAS:-0}" != "1" && -x "$ROOT/scripts/enable-quotas.sh" ]]; then
  log "Activation usrquota (best effort)"
  bash "$ROOT/scripts/enable-quotas.sh" || log "Quotas non activés — setquota restera non bloquant"
fi

# --- nginx ------------------------------------------------------------------
if [[ "$SKIP_NGINX" != "1" ]]; then
  log "Nginx panneau (:80) + vitrine (:8088)"
  install -m 0644 "$ROOT/deploy/nginx-panel.conf" "$ETC_DIR/nginx-panel.conf"
  ln -sfn "$ETC_DIR/nginx-panel.conf" /etc/nginx/sites-available/hebernet
  ln -sfn /etc/nginx/sites-available/hebernet /etc/nginx/sites-enabled/hebernet

  if [[ -f "$ROOT/deploy/nginx-vitrine.conf" ]]; then
    install -m 0644 "$ROOT/deploy/nginx-vitrine.conf" "$ETC_DIR/nginx-vitrine.conf"
    ln -sfn "$ETC_DIR/nginx-vitrine.conf" /etc/nginx/sites-available/hebernet-vitrine
    ln -sfn /etc/nginx/sites-available/hebernet-vitrine /etc/nginx/sites-enabled/hebernet-vitrine
  fi

  # Éviter le conflit avec le site default sur :80 en lab
  if [[ -L /etc/nginx/sites-enabled/default ]]; then
    rm -f /etc/nginx/sites-enabled/default
  fi
  nginx -t
  systemctl enable --now nginx
  systemctl reload nginx
fi

# --- fin --------------------------------------------------------------------
cat <<EOF

Hébernet installé (Ubuntu Server neuf → stack complète).

  Panneau : http://${HOST_IP}/
  Vitrine : http://${HOST_IP}:8088/
  API     : 127.0.0.1:8787 (proxifiée sous /api/ sur :80 et :8088)
  Config  : $ETC_DIR/api.env
  Données : $DATA_DIR
  Services: systemctl status hebernet-agent hebernet-api nginx

Prochaines étapes :
  1. Éditer $ETC_DIR/api.env (Stripe sk_test_ / whsec_, Resend)
  2. systemctl restart hebernet-api
  3. Panneau : admin@hebernet.local / admin  (changez ce mot de passe)
  4. Pas de site démo fantôme — créez un vrai site via Stripe ou Admin

EOF
