# Installation Hébernet (hôte Debian 12 / Ubuntu 24.04+)

## Une commande (Ubuntu Server neuf)

Sur une machine **vide** (pas de panneau, pas de vitrine) : tout est installé par le script.

```bash
# Depuis la machine de dév — copier les DEUX dossiers
scp -r hebernet vitrine user@IP_VM:~/
ssh user@IP_VM 'cd hebernet && sudo ./scripts/install.sh'
```

Résultat :
- paquets (Nginx, MariaDB, PHP-FPM, certbot, quotas, Go, Node si besoin)
- agent + API (systemd)
- **panneau** → `http://IP/`
- **vitrine** → `http://IP:8088/` (paiement Stripe)
- `/etc/hebernet/api.env` avec `PANEL_URL=http://IP` et `VITRINE_URL=http://IP:8088`

Options :

| Variable | Effet |
|---|---|
| `HEBERNET_SKIP_DEPS=1` | Ne pas `apt install` |
| `HEBERNET_SKIP_UI=1` | Ne pas builder / déployer le panneau |
| `HEBERNET_SKIP_VITRINE=1` | Ne pas déployer la vitrine |
| `HEBERNET_SKIP_NGINX=1` | Ne pas toucher Nginx |
| `HEBERNET_VITRINE_SRC=/chemin` | Forcer l’emplacement de la vitrine |

Après install : éditer `/etc/hebernet/api.env` (Stripe), puis `systemctl restart hebernet-api`.

## Prérequis (si install manuelle)

- Root ou sudo
- Nginx, MariaDB, certbot
- **PHP-FPM 8.5** — suivre [install.fpm.install](https://www.php.net/manual/en/install.fpm.install.php) / paquets de votre dépôt (sinon 8.4 / 8.3)
- Quotas kernel (`quota` package, `usrquota` sur le FS des homes)

## Binaires (manuel)

```bash
make build
install -m 755 bin/hebernet-agent /usr/local/sbin/hebernet-agent
install -m 755 bin/hebernet-api /usr/local/bin/hebernet-api
```

Unités prêtes à l’emploi : `deploy/hebernet-agent.service`, `deploy/hebernet-api.service`, `deploy/nginx-panel.conf`.

## Agent (root)

Systemd exemple — **sans** dry-run : voir `deploy/hebernet-agent.service`.

Socket Unix : permissions restreintes (groupe dédié `hebernet`).

## API (non-root)

Voir `deploy/hebernet-api.service` — `EnvironmentFile=/etc/hebernet/api.env`.

## UI

```bash
cd ui && npm ci && npm run build
# servir ui/dist via Nginx (location /) + proxy /api vers 127.0.0.1:8787
```

Ou laisser `scripts/install.sh` déployer vers `/var/www/hebernet`.

## SSL

Certbot + plugin Nginx ; l’API appelle l’agent `issue_ssl` (domaine du site).

## Sécurité

- L’agent n’accepte que des ops allowlistées (pas de shell libre).
- Authz des sites côté API (rôle + owner).
- Ne jamais exposer le socket agent sur le réseau.
