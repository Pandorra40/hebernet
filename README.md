# Hébernet

Panel d’hébergement **OSS** (AGPL-3.0) — dual Admin / Client, packages + quotas, PHP-FPM 8.5.

**One-clicks :** statique, Hugo, PHP, Bludit, CodeIgniter (stack légère).

## Stack

- Agent root (Go) — socket Unix, mode `--dry-run` pour la démo sans root
- API (Go) — SQLite, rôles `admin` | `client`
- UI — Vue 3 + Vite (SPA, FR)

## Démo locale

```bash
./scripts/demo.sh
```

Puis ouvrir **http://127.0.0.1:5173/**

| Compte | Mot de passe |
|--------|----------------|
| `admin@hebernet.local` | `admin` |
| `client@hebernet.local` | `client` |

L’agent tourne en **dry-run** : fichiers sous `data/` (homes, nginx, php-fpm simulés).

## Build

```bash
export PATH="$PWD/.tools/go/bin:$PATH"   # si Go local du repo
make build
cd ui && npm install && npm run build
```
