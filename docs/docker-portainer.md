# Hébernet — stack Docker (Portainer / ZimaOS)

Usage **solo** : panneau + Nginx + PHP-FPM + MariaDB + Node (Nuxt), sans VM Ubuntu.

## Contenu

| Service | Rôle |
|---------|------|
| `db` | MariaDB 11 |
| `hebernet` | Nginx (:80), php-fpm, Node 24, supervisord, agent + API Go, UI Vue |

Données persistantes : volumes `hebernet_data` (sites, SQLite, configs) et `hebernet_db`.

Sur Zima / Portainer, le volume est typiquement sous `/DATA/AppData/…` ; **dans le conteneur** les sites sont toujours sous `/data/homes/<linux_user>/`.

## Déploiement rapide

```bash
cd hebernet-compose
cp .env.example .env
# éditer MARIADB_ROOT_PASSWORD et HEBERNET_PANEL_URL
docker compose up -d --build
```

Panneau : `http://IP:18787/` (port peu courant ; éviter 80 / 8080 / 8181).

Compte seed : `admin@hebernet.local` / `admin` — changez le mot de passe.

## Portainer (ZimaOS T330)

1. **Stacks** → **Add stack**
2. Nom : `hebernet`
3. Coller `docker-compose.yml`
4. **Environment variables** : `MARIADB_ROOT_PASSWORD`, `HEBERNET_PANEL_URL`, `HEBERNET_PUBLIC_HOST`, `HEBERNET_HTTP_PORT`
5. **Build** : context = **répertoire parent** (`docker-mailserver/`), pas seulement `hebernet-compose/`.

Exemple d’URL panneau : `http://172.16.17.16:18787`.

## Site Nuxt + API PHP (ex. Annuaire TSA)

Après **Admin → Sites** (type Nuxt), arborescence dans le conteneur :

| Chemin | Contenu |
|--------|---------|
| `/data/homes/<user>/app/` | Build Nitro (`.output/` → `server/index.mjs`) |
| `/data/homes/<user>/public_html/api/` | PHP + `vendor/` |
| `/data/homes/<user>/.env.hebernet` | `DB_HOST=db`, `DB_NAME`, `DB_USER`, `DB_PASSWORD`, `NODE_PORT` |

Déploiement typique :

1. `pnpm build` → copier le contenu de `.output/` dans `app/`
2. Copier l’API dans `public_html/api/` (garder / générer `config.php` qui lit `.env.hebernet`)
3. `supervisorctl restart hebernet-node-<user>`

Recommandations :

- **`apiBase`** front → `/api` (même origine) ; éviter un `https://www.…/api` figé au build
- Nginx site : `server_name` **apex + www** (sinon `Host: www` → 404 API)
- `.env.hebernet` en **0640**, groupe `hebernet` ; `www-data` dans ce groupe (fait au démarrage du conteneur)
- L’image inclut **Node 24** (`/usr/bin/node`) pour les process supervisord

Credentials MariaDB : visibles dans `.env.hebernet` **via** `docker exec` (pas sur l’hôte Zima sous `/data/…`).

## SSL / site public (VPS + Netbird)

Le stack écoute en **HTTP** sur le port hôte (ex. `18787`). TLS et DNS publics = **VPS**.

| Élément | Rôle |
|---------|------|
| DNS (`A` apex + www) | IP **publique du VPS** |
| Nginx Proxy Manager (ou Nginx) | Let’s Encrypt + reverse proxy |
| Forward | `http://<IP_Netbird_T330>:18787` (ex. `100.104.x.x`) |
| Host | conserver `annuaire-tsa.fr` / `www` vers Hébernet |
| Netbird | lien privé VPS ↔ T330 ; le public ne voit que le VPS |

Avant de demander un certificat LE : le DNS public doit déjà pointer vers le VPS (sinon « Internal Error » / challenge HTTP en échec).

Exemple NPM :

- Domaines : `annuaire-tsa.fr`, `www.annuaire-tsa.fr`
- Schéma `http` → IP Netbird du T330, port `18787`
- Bloquer exploits + Websockets on ; SSL Let’s Encrypt + Forcer SSL

## vs install VM (`scripts/install.sh`)

| | VM Ubuntu | Docker Compose |
|--|-----------|----------------|
| Déploiement | ZVM + SSH + install | Portainer / une stack |
| Isolation site | user Linux + systemd | volumes + supervisord |
| MariaDB | sur l’hôte | conteneur `db` |
| Agent | root sur l’hôte | `HEBERNET_RUNTIME=docker` |
| Node sites | NodeSource sur l’hôte | Node 24 dans l’image |

## Arrêt / données

```bash
docker compose down          # garde les volumes
docker compose down -v       # ⚠️ efface BDD + sites
```
