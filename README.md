# Hébernet

<p align="center">
  <strong>Panel d’hébergement web open source</strong><br>
  Un site · un prix · une année
</p>

<p align="center">
  <img alt="Statut" src="https://img.shields.io/badge/statut-BETA-orange?style=for-the-badge">
  <img alt="Licence" src="https://img.shields.io/badge/licence-AGPL--3.0-blue?style=for-the-badge">
  <img alt="Go" src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white">
  <img alt="Vue" src="https://img.shields.io/badge/Vue-3-42b883?style=for-the-badge&logo=vuedotjs&logoColor=white">
</p>

> **Phase Beta — en cours de test**  
> Hébernet est utilisable en lab (VM Ubuntu) et a passé un premier parcours : install, paiement Stripe test, stack légère (statique / Hugo / PHP / Bludit / CodeIgniter).  
> **Ce n’est pas encore une version de production.** APIs, chemins et UX peuvent encore bouger. Merci de tester, signaler, ne pas y mettre de vrais clients payants sans revue.

---

## L’idée en une phrase

Un panneau d’hébergement **simple** (fichiers, SSL, base, quotas) + une **vitrine** pour vendre l’offre à l’année — sans usine à gaz, sans BlueOnyx. Orientation **sites légers** (densité serveur), pas d’e-commerce lourd ni de CMS monolithiques.

| Vous apportez | Hébernet fournit |
|---|---|
| Le nom de domaine | Espace disque, compte, base MariaDB (si besoin) |
| Le site (statique, Hugo, PHP, Bludit, CodeIgniter…) | Panneau client + SFTP + SSL |
| Le paiement (Stripe) | Provision automatique après Checkout |

---

## Ce que contient le dépôt

```
hebernet/
├── cmd/            # Binaires agent (root) + API
├── internal/       # Cœur métier (provision, Stripe, store…)
├── ui/             # Panneau Vue 3 (admin + client)
├── vitrine/        # Site marketing + parcours paiement
├── deploy/         # systemd + Nginx
├── scripts/        # install.sh (Ubuntu Server neuf)
└── docs/           # Install, Stripe
```

### Panneau
- Rôles **admin** / **client**
- Packages & quotas
- Sites : **statique**, **Hugo**, **PHP**, **Bludit**, **CodeIgniter**
- Fichiers (FileBrowser), Adminer, SSL, cron
- Pools PHP-FPM serrés par site (`pm=ondemand`, `max_children=5`, `memory_limit=128M`)

### Vitrine
- Pages offre / conditions / contact
- Checkout Stripe → suivi d’activation
- Design aligné sur le panneau (thème sombre / teal)

### Agent
- Socket Unix, ops allowlistées
- Mode `--dry-run` pour démo sans root
- En prod : vrais `useradd`, Nginx, PHP-FPM, MariaDB
- Hugo téléchargé à la demande (ou préinstallé dans `/var/lib/hebernet/bin`)

---

## Stack

| Couche | Techno |
|---|---|
| Agent | Go, socket Unix |
| API | Go, SQLite |
| UI | Vue 3 + Vite |
| Sites clients | Nginx · PHP-FPM 8.5 · MariaDB |
| Paiement | Stripe Checkout + webhooks |
| Mail (optionnel) | Resend |

Cible install : **Debian 12 / Ubuntu 24.04+** (testé aussi en 26.04).

---

## Démarrage rapide

### Démo locale (dry-run)

```bash
./scripts/demo.sh
# Panneau : http://127.0.0.1:5173/
# admin@hebernet.local / admin
```

### Ubuntu Server neuf (lab / VM)

Rien n’est préinstallé : le script pose **tout** (paquets, panneau, vitrine, systemd).

```bash
# Depuis votre machine
scp -r . user@IP_VM:~/hebernet
ssh user@IP_VM 'cd hebernet && sudo ./scripts/install.sh'
```

| Service | URL |
|---|---|
| Panneau | `http://IP/` |
| Vitrine | `http://IP:8088/` |
| Config | `/etc/hebernet/api.env` |

Comptes seed : `admin@hebernet.local` / `admin` (packages Starter/Pro/Business).  
Pas de faux site en install serveur — le seed démo (`-seed-demo`) est réservé à `./scripts/demo.sh`.

Détail : [docs/install.md](docs/install.md) · Stripe : [docs/stripe.md](docs/stripe.md).

---

## Parcours client (Beta)

1. Vitrine → domaine + e-mail + type de site  
2. Stripe Checkout (carte test `4242…`)  
3. Webhook → création compte + site  
4. Page d’activation → panneau  

En lab, pointez le domaine vers la VM via `/etc/hosts` :

```text
192.168.x.x  mon-site.fr www.mon-site.fr
```

---

## État des one-clicks (retour lab)

| Type | Statut lab |
|---|---|
| Statique | OK (index.html par défaut, puis SFTP) |
| Hugo | OK (squelette + build) |
| PHP | OK (index.php stub + pool FPM + BDD) |
| Bludit | OK |
| CodeIgniter | OK (docroot `public/`, composer) |

Hors catalogue (non proposés) : WordPress, PrestaShop, Laravel — trop lourds pour la densité visée. Des types legacy peuvent encore exister en base pour d’anciens sites lab.

Les dépendances système comptent : `install.sh` inclut zip, GD, intl, sqlite, composer quand disponible. Hugo peut être préinstallé ou téléchargé par l’agent au premier provisionnement.

---

## Sécurité (rappel Beta)

- Ne jamais exposer le socket agent sur le réseau  
- Clés Stripe / Resend uniquement dans `/etc/hebernet/api.env` (hors git)  
- Mode test Stripe tant que vous êtes en Beta  
- Quotas disque Linux optionnels (souvent absents sur VM lab)

---

## Licence

[AGPL-3.0](LICENSE) — libre d’utiliser, de modifier, de redistribuer ; les déploiements réseau doivent redistribuer le code source modifié.

---

## Contribution / retours

Projet jeune. Les retours de test (bugs d’install, one-clicks, UX panneau) sont les plus utiles pour sortir de Beta.

Issues et PR bienvenues une fois le dépôt public.

---

<p align="center">
  <em>Hébernet — hébergement web, sans le superflu.</em><br>
  <sub>Beta · tests en cours · AGPL-3.0</sub>
</p>
